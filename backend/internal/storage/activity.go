package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// ActivitySegment 对应 activity_segments 表。
type ActivitySegment struct {
	ID          int64
	StartTS     time.Time
	EndTS       time.Time
	AppPath     string
	AppName     string
	Category    string
	WindowTitle string
	State       string // active / idle
	Summary     string // 由 vlm_summary 事件惰性回填
}

// InsertActivitySegment 插入一条活动段，返回自增 ID。
func (db *DB) InsertActivitySegment(seg ActivitySegment) (int64, error) {
	if seg.AppPath == "" || seg.AppName == "" || seg.Category == "" || seg.State == "" {
		return 0, fmt.Errorf("app_path/app_name/category/state 不能为空")
	}
	if seg.EndTS.Before(seg.StartTS) {
		return 0, fmt.Errorf("end_ts 不能早于 start_ts")
	}

	const q = `INSERT INTO activity_segments(start_ts, end_ts, app_path, app_name, category, window_title, state)
		VALUES(?, ?, ?, ?, ?, ?, ?)`

	res, err := db.Exec(q,
		toUnix(seg.StartTS), toUnix(seg.EndTS), seg.AppPath, seg.AppName,
		seg.Category, seg.WindowTitle, seg.State,
	)
	if err != nil {
		return 0, fmt.Errorf("插入活动段失败: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("获取活动段 ID 失败: %w", err)
	}
	return id, nil
}

// UpdateActivitySegmentEndTS 更新活动段的结束时间（用于延长当前段）。
func (db *DB) UpdateActivitySegmentEndTS(id int64, endTS time.Time) error {
	_, err := db.Exec("UPDATE activity_segments SET end_ts = ? WHERE id = ?", toUnix(endTS), id)
	if err != nil {
		return fmt.Errorf("更新活动段结束时间失败: %w", err)
	}
	return nil
}

// UpdateActivitySegmentEndTSAndState 同时更新结束时间和活跃状态。
// 用于同应用段内的滚动更新：每 tick 延长 end_ts 并把 state 更新为最新活跃强度
// （engaged/active/idle 在段内翻转时记录最后值）。比分别调用两次 UPDATE 省 IO。
func (db *DB) UpdateActivitySegmentEndTSAndState(id int64, endTS time.Time, state string) error {
	_, err := db.Exec("UPDATE activity_segments SET end_ts = ?, state = ? WHERE id = ?", toUnix(endTS), state, id)
	if err != nil {
		return fmt.Errorf("更新活动段结束时间和状态失败: %w", err)
	}
	return nil
}

// UpdateActivitySegmentSummary 更新活动段的 AI 摘要（由 VLM 摘要事件惰性回填）。
func (db *DB) UpdateActivitySegmentSummary(id int64, summary string) error {
	_, err := db.Exec("UPDATE activity_segments SET summary = ? WHERE id = ?", summary, id)
	if err != nil {
		return fmt.Errorf("更新活动段摘要失败: %w", err)
	}
	return nil
}

// LatestVLMSummary 查询时间窗口 [start, end] 内最近一条 vlm_summary 事件的内容。
// 走 idx_events_type_ts 索引。无结果返回空串。content 列可空，用 NullString 兜底。
func (db *DB) LatestVLMSummary(start, end time.Time) (string, error) {
	var content sql.NullString
	err := db.QueryRow(
		`SELECT content FROM events
		 WHERE type = ? AND ts >= ? AND ts <= ?
		 ORDER BY ts DESC LIMIT 1`,
		string(EventTypeVLMSummary), toUnix(start), toUnix(end),
	).Scan(&content)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("查询 VLM 摘要失败: %w", err)
	}
	return content.String, nil
}

// GetActivitySegment 按 ID 查询活动段。
func (db *DB) GetActivitySegment(id int64) (*ActivitySegment, error) {
	row := db.QueryRow(
		"SELECT id, start_ts, end_ts, app_path, app_name, category, window_title, state, summary FROM activity_segments WHERE id = ?",
		id,
	)
	return scanActivitySegment(row)
}

// startOfLocalDay 返回 t 所在本地日的 00:00（保留 t 的时区，通常是 time.Local）。
// 用于按天切日：与 QueryTimeline 的本地时区切日语义一致，避免 UTC 切日导致
// 跨本地午夜的事件错位归属（UTC+8 下凌晨 0-8 点事件被算进前一天）。
func startOfLocalDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// ListActivitySegments 查询时间范围内的活动段，按开始时间升序。
func (db *DB) ListActivitySegments(start, end time.Time) ([]ActivitySegment, error) {
	rows, err := db.Query(
		`SELECT id, start_ts, end_ts, app_path, app_name, category, window_title, state, summary
		 FROM activity_segments
		 WHERE start_ts < ? AND end_ts > ?
		 ORDER BY start_ts`,
		toUnix(end), toUnix(start),
	)
	if err != nil {
		return nil, fmt.Errorf("列出活动段失败: %w", err)
	}
	defer rows.Close()

	var segs []ActivitySegment
	for rows.Next() {
		seg, err := scanActivitySegment(rows)
		if err != nil {
			return nil, err
		}
		segs = append(segs, *seg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历活动段失败: %w", err)
	}
	return segs, nil
}

// ListActivitySegmentsByDate 查询某一天的全部活动段。
// ListActivitySegmentsByDate 按天列出活动段。
// 切日按 day 的本地时区零点（与 QueryTimeline 一致），确保 UI"日期"与本地作息对齐。
func (db *DB) ListActivitySegmentsByDate(day time.Time) ([]ActivitySegment, error) {
	day = startOfLocalDay(day)
	next := day.Add(24 * time.Hour)
	return db.ListActivitySegments(day, next)
}

// TodayActivityMinutes 统计今日在白名单应用上的工作总分钟数(engaged+active)。
// "今日"按本地时区零点切（与时间轴 QueryTimeline 一致）。
//
// 跨天段处理：用区间重叠判据（start_ts<dayEnd AND end_ts>dayStart）命中当天，
// 并对时长 SUM 做"按天 clamp"——只累加当天内部分 MIN(end_ts,dayEnd)-MAX(start_ts,dayStart)，
// 避免一条横跨数天的巨怪段（如 46h 段，见坑 #49）被反复计入每一天、或因旧判据
// start_ts>=? AND end_ts<=? 漏掉跨天段。与 QueryTimeline 的 clipSegmentsToDay 语义一致。
func (db *DB) TodayActivityMinutes() (int, int, error) {
	start := startOfLocalDay(time.Now())
	end := start.Add(24 * time.Hour)
	dayStartUnix := toUnix(start)
	dayEndUnix := toUnix(end)

	var totalSec int64
	err := db.QueryRow(
		`SELECT COALESCE(SUM(MIN(end_ts, ?) - MAX(start_ts, ?)), 0)
		 FROM activity_segments
		 WHERE state IN ('engaged','active') AND start_ts < ? AND end_ts > ?`,
		dayEndUnix, dayStartUnix, dayEndUnix, dayStartUnix,
	).Scan(&totalSec)
	if err != nil {
		return 0, 0, fmt.Errorf("统计今日工作时长失败: %w", err)
	}

	var segments int64
	err = db.QueryRow(
		`SELECT COUNT(*) FROM activity_segments
		 WHERE state IN ('engaged','active') AND start_ts < ? AND end_ts > ?`,
		dayEndUnix, dayStartUnix,
	).Scan(&segments)
	if err != nil {
		return 0, 0, fmt.Errorf("统计今日活动段数失败: %w", err)
	}

	return int(totalSec / 60), int(segments), nil
}

// scanActivitySegment 从 sql.Row/sql.Rows 扫描 ActivitySegment。
func scanActivitySegment(sc interface {
	Scan(dest ...any) error
}) (*ActivitySegment, error) {
	var seg ActivitySegment
	var startSec, endSec int64
	// summary 列由 migrate 新增，旧行默认 NULL；
	// modernc/sqlite 不允许把 NULL 扫进 Go string，故用 NullString 兜底。
	var summary sql.NullString
	if err := sc.Scan(&seg.ID, &startSec, &endSec, &seg.AppPath, &seg.AppName, &seg.Category, &seg.WindowTitle, &seg.State, &summary); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("扫描活动段失败: %w", err)
	}
	seg.StartTS = fromUnix(startSec)
	seg.EndTS = fromUnix(endSec)
	seg.Summary = summary.String
	return &seg, nil
}
