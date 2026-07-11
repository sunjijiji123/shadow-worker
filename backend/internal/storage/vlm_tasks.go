package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// VLM 任务状态。
const (
	VLMTaskStatusPending       = "pending"         // 待识别（含可重试的失败，如429）
	VLMTaskStatusDone          = "done"            // 识别成功（清理时删行+删图）
	VLMTaskStatusPermanentFail = "permanent_fail"  // 不可重试的失败（鉴权/解析/截图），保留等手动重试
)

// VLMTask 对应 vlm_tasks 表：采集的截图等待 VLM 识别的任务队列。
// 采集与识别解耦：OnActivity/scheduled 只截图+落盘+写 pending，
// recognitionLoop worker 每5分钟扫描 pending → 识别 → 成功清理/失败分类。
type VLMTask struct {
	ID          int64
	CreatedTS   time.Time // 采集时刻
	AppPath     string
	AppName     string
	ImagePath   string // screenshots/pending/<id>.png 绝对路径
	Status      string // pending | done | permanent_fail
	Attempts    int    // 已尝试次数
	ErrorKind   string // 最后一次失败的分类（rate_limit/auth_error/...）
	ErrorDetail string // 最后一次失败的详情
	UpdatedTS   time.Time // 最后尝试时刻（用于扫描间隔判定）
}

// InsertVLMTask 插入一条 pending 任务，返回自增 ID。
// image_path 可为空（先拿 ID 再写文件再 UPDATE），但正常流程落盘后才入队。
func (db *DB) InsertVLMTask(appPath, appName, imagePath string, ts time.Time) (int64, error) {
	const q = `INSERT INTO vlm_tasks(created_ts, app_path, app_name, image_path, status, attempts, updated_ts)
		VALUES(?, ?, ?, ?, ?, 0, ?)`
	res, err := db.Exec(q,
		toUnix(ts), appPath, appName, imagePath,
		VLMTaskStatusPending, toUnix(ts))
	if err != nil {
		return 0, fmt.Errorf("插入 VLM 任务失败: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("获取 VLM 任务 ID 失败: %w", err)
	}
	return id, nil
}

// UpdateVLMTaskImage 在拿到自增 ID 后回填 image_path（先 INSERT 拿 ID，再写文件，再 UPDATE）。
func (db *DB) UpdateVLMTaskImage(id int64, imagePath string) error {
	_, err := db.Exec("UPDATE vlm_tasks SET image_path = ? WHERE id = ?", imagePath, id)
	if err != nil {
		return fmt.Errorf("更新 VLM 任务图片路径失败: %w", err)
	}
	return nil
}

// ListPendingVLMTasks 查询待识别任务。
// minAge：updated_ts 距今至少 minAge 的才返回（避免刚失败的任务被立即重试）。
// attempts=0 的全新任务无视 minAge（updated_ts=created_ts，首次识别不等待）。
// 按 created_ts 升序（先入先出），限制 limit 条。
func (db *DB) ListPendingVLMTasks(limit int, minAge time.Duration) ([]VLMTask, error) {
	cutoff := toUnix(time.Now().Add(-minAge))
	const q = `SELECT id, created_ts, app_path, app_name, image_path, status, attempts,
		error_kind, error_detail, updated_ts
		FROM vlm_tasks
		WHERE status = ? AND (attempts = 0 OR updated_ts <= ?)
		ORDER BY created_ts ASC LIMIT ?`
	rows, err := db.Query(q, VLMTaskStatusPending, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("查询待识别 VLM 任务失败: %w", err)
	}
	defer rows.Close()

	var tasks []VLMTask
	for rows.Next() {
		t, err := scanVLMTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, *t)
	}
	return tasks, rows.Err()
}

// UpdateVLMTaskResult 更新任务状态：成功→done（后续清理删行）、
// 可重试失败→保持 pending + attempts+1 + 记录错误、
// 不可重试失败→permanent_fail + 记录错误。
func (db *DB) UpdateVLMTaskResult(id int64, status string, attempts int, errKind, errDetail string) error {
	_, err := db.Exec(
		`UPDATE vlm_tasks SET status = ?, attempts = ?, error_kind = ?, error_detail = ?, updated_ts = ? WHERE id = ?`,
		status, attempts, errKind, errDetail, toUnix(time.Now()), id)
	if err != nil {
		return fmt.Errorf("更新 VLM 任务结果失败: %w", err)
	}
	return nil
}

// DeleteVLMTask 删除任务行（识别成功后配合删图片文件）。
func (db *DB) DeleteVLMTask(id int64) error {
	_, err := db.Exec("DELETE FROM vlm_tasks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("删除 VLM 任务失败: %w", err)
	}
	return nil
}

// CountVLMTasksByStatus 统计某状态的 task 数量（阈值清理用）。
func (db *DB) CountVLMTasksByStatus(status string) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM vlm_tasks WHERE status = ?", status).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("统计 VLM 任务失败: %w", err)
	}
	return count, nil
}

// CountAllVLMTasks 统计所有 task 数量（pending + permanent_fail 合计，阈值清理用）。
func (db *DB) CountAllVLMTasks() (int, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM vlm_tasks WHERE status IN (?, ?)`,
		VLMTaskStatusPending, VLMTaskStatusPermanentFail,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("统计 VLM 任务总数失败: %w", err)
	}
	return count, nil
}

// ListOldPermanentFails 查询最旧的 permanent_fail 任务（按 created_ts 升序）。
// 阈值清理用：超过数量/磁盘限制时，删最旧的永久失败项释放空间。
func (db *DB) ListOldPermanentFails(limit int) ([]VLMTask, error) {
	const q = `SELECT id, created_ts, app_path, app_name, image_path, status, attempts,
		error_kind, error_detail, updated_ts
		FROM vlm_tasks WHERE status = ?
		ORDER BY created_ts ASC LIMIT ?`
	rows, err := db.Query(q, VLMTaskStatusPermanentFail, limit)
	if err != nil {
		return nil, fmt.Errorf("查询永久失败 VLM 任务失败: %w", err)
	}
	defer rows.Close()

	var tasks []VLMTask
	for rows.Next() {
		t, err := scanVLMTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, *t)
	}
	return tasks, rows.Err()
}

// ResetPermanentFails 把指定时间窗+app 的 permanent_fail 任务重置为 pending。
// 用于"手动重试"：用户改了 API key 等配置后，把之前鉴权失败等不可重试的任务
// 重新标记为 pending，让 recognitionLoop 下一轮重新识别（判据与段关联一致：
// 半开区间 + app_path）。返回重置的行数。
func (db *DB) ResetPermanentFails(start, end time.Time, appPath string) (int64, error) {
	var res sql.Result
	var err error
	if appPath == "" {
		res, err = db.Exec(
			`UPDATE vlm_tasks SET status = ?, attempts = 0, updated_ts = ?
			 WHERE status = ? AND created_ts >= ? AND created_ts < ?`,
			VLMTaskStatusPending, toUnix(time.Now()), VLMTaskStatusPermanentFail,
			toUnix(start), toUnix(end),
		)
	} else {
		res, err = db.Exec(
			`UPDATE vlm_tasks SET status = ?, attempts = 0, updated_ts = ?
			 WHERE status = ? AND created_ts >= ? AND created_ts < ?
			 AND (app_path = ? OR app_path = '')`,
			VLMTaskStatusPending, toUnix(time.Now()), VLMTaskStatusPermanentFail,
			toUnix(start), toUnix(end), appPath,
		)
	}
	if err != nil {
		return 0, fmt.Errorf("重置 VLM 任务失败: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// ListPermanentFailsByRange 查询时间窗内的 permanent_fail 任务（用于手动同步重试）。
// 判据与段关联一致：半开区间 + app_path。按 created_ts 升序。
func (db *DB) ListPermanentFailsByRange(start, end time.Time, appPath string) ([]VLMTask, error) {
	const baseQuery = `SELECT id, created_ts, app_path, app_name, image_path, status, attempts,
		error_kind, error_detail, updated_ts
		FROM vlm_tasks WHERE status = ? AND created_ts >= ? AND created_ts < ?`
	const appFilter = ` AND (app_path = ? OR app_path = '')`

	var rows *sql.Rows
	var err error
	if appPath == "" {
		rows, err = db.Query(baseQuery+" ORDER BY created_ts ASC",
			VLMTaskStatusPermanentFail, toUnix(start), toUnix(end))
	} else {
		rows, err = db.Query(baseQuery+appFilter+" ORDER BY created_ts ASC",
			VLMTaskStatusPermanentFail, toUnix(start), toUnix(end), appPath)
	}
	if err != nil {
		return nil, fmt.Errorf("查询永久失败任务失败: %w", err)
	}
	defer rows.Close()

	var tasks []VLMTask
	for rows.Next() {
		t, err := scanVLMTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, *t)
	}
	return tasks, rows.Err()
}

// LatestPermanentFailInRange 查询时间窗内最新一条 permanent_fail 任务。
// 用于"单条重试"：点一次重试只处理最新的一条，多条时用户可多次点。
// 返回 nil, nil 表示该范围内无 permanent_fail 任务。
func (db *DB) LatestPermanentFailInRange(start, end time.Time, appPath string) (*VLMTask, error) {
	const baseQuery = `SELECT id, created_ts, app_path, app_name, image_path, status, attempts,
		error_kind, error_detail, updated_ts
		FROM vlm_tasks WHERE status = ? AND created_ts >= ? AND created_ts < ?`
	const appFilter = ` AND (app_path = ? OR app_path = '')`

	if appPath == "" {
		row := db.QueryRow(baseQuery+" ORDER BY created_ts DESC LIMIT 1",
			VLMTaskStatusPermanentFail, toUnix(start), toUnix(end))
		return scanVLMTask(row)
	}
	row := db.QueryRow(baseQuery+appFilter+" ORDER BY created_ts DESC LIMIT 1",
		VLMTaskStatusPermanentFail, toUnix(start), toUnix(end), appPath)
	return scanVLMTask(row)
}

// scanVLMTask 从 sql.Row/sql.Rows 扫描 VLMTask。
// error_kind/error_detail 可能为 NULL（首次 pending 的任务无错误信息），用 NullString 兜底。
func scanVLMTask(sc interface {
	Scan(dest ...any) error
}) (*VLMTask, error) {
	var t VLMTask
	var createdTS, updatedTS int64
	var errKind, errDetail sql.NullString
	if err := sc.Scan(
		&t.ID, &createdTS, &t.AppPath, &t.AppName, &t.ImagePath,
		&t.Status, &t.Attempts, &errKind, &errDetail, &updatedTS,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("扫描 VLM 任务失败: %w", err)
	}
	t.CreatedTS = fromUnix(createdTS)
	t.UpdatedTS = fromUnix(updatedTS)
	t.ErrorKind = errKind.String
	t.ErrorDetail = errDetail.String
	return &t, nil
}
