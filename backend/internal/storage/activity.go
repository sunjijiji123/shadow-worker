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

// GetActivitySegment 按 ID 查询活动段。
func (db *DB) GetActivitySegment(id int64) (*ActivitySegment, error) {
	row := db.QueryRow(
		"SELECT id, start_ts, end_ts, app_path, app_name, category, window_title, state FROM activity_segments WHERE id = ?",
		id,
	)
	return scanActivitySegment(row)
}

// ListActivitySegments 查询时间范围内的活动段，按开始时间升序。
func (db *DB) ListActivitySegments(start, end time.Time) ([]ActivitySegment, error) {
	rows, err := db.Query(
		`SELECT id, start_ts, end_ts, app_path, app_name, category, window_title, state
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
func (db *DB) ListActivitySegmentsByDate(day time.Time) ([]ActivitySegment, error) {
	day = day.UTC().Truncate(24 * time.Hour)
	next := day.Add(24 * time.Hour)
	return db.ListActivitySegments(day, next)
}

// TodayActivityMinutes 统计今日在白名单应用上的活跃总分钟数。
func (db *DB) TodayActivityMinutes() (int, int, error) {
	now := time.Now().UTC()
	start := now.Truncate(24 * time.Hour)
	end := start.Add(24 * time.Hour)

	var totalSec int64
	err := db.QueryRow(
		`SELECT COALESCE(SUM(end_ts - start_ts), 0)
		 FROM activity_segments
		 WHERE state = 'active' AND start_ts >= ? AND end_ts <= ?`,
		toUnix(start), toUnix(end),
	).Scan(&totalSec)
	if err != nil {
		return 0, 0, fmt.Errorf("统计今日活跃时长失败: %w", err)
	}

	var segments int64
	err = db.QueryRow(
		`SELECT COUNT(*) FROM activity_segments
		 WHERE state = 'active' AND start_ts >= ? AND end_ts <= ?`,
		toUnix(start), toUnix(end),
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
	if err := sc.Scan(&seg.ID, &startSec, &endSec, &seg.AppPath, &seg.AppName, &seg.Category, &seg.WindowTitle, &seg.State); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("扫描活动段失败: %w", err)
	}
	seg.StartTS = fromUnix(startSec)
	seg.EndTS = fromUnix(endSec)
	return &seg, nil
}
