package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// EventType 枚举事件类型。
type EventType string

const (
	EventTypeVoice         EventType = "voice"
	EventTypePromptInject  EventType = "prompt_inject"
	EventTypeScreenshot    EventType = "screenshot"
	EventTypeVLMSummary    EventType = "vlm_summary"
)

// Event 对应 events 表。
type Event struct {
	ID             int64
	TS             time.Time
	Type           EventType
	AppPath        string
	AppName        string
	Content        string
	ScreenshotPath string
	Meta           string
}

// InsertEvent 插入一条事件，返回自增 ID。
func (db *DB) InsertEvent(ev Event) (int64, error) {
	if ev.Type == "" {
		return 0, fmt.Errorf("事件类型不能为空")
	}

	const q = `INSERT INTO events(ts, type, app_path, app_name, content, screenshot_path, meta)
		VALUES(?, ?, ?, ?, ?, ?, ?)`

	res, err := db.Exec(q,
		toUnix(ev.TS), string(ev.Type), ev.AppPath, ev.AppName,
		ev.Content, ev.ScreenshotPath, ev.Meta,
	)
	if err != nil {
		return 0, fmt.Errorf("插入事件失败: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("获取事件 ID 失败: %w", err)
	}
	return id, nil
}

// GetEvent 按 ID 查询事件。
func (db *DB) GetEvent(id int64) (*Event, error) {
	row := db.QueryRow(
		"SELECT id, ts, type, app_path, app_name, content, screenshot_path, meta FROM events WHERE id = ?",
		id,
	)
	return scanEvent(row)
}

// ListEvents 查询时间范围内的事件，按时间升序。
func (db *DB) ListEvents(start, end time.Time) ([]Event, error) {
	rows, err := db.Query(
		`SELECT id, ts, type, app_path, app_name, content, screenshot_path, meta
		 FROM events
		 WHERE ts >= ? AND ts < ?
		 ORDER BY ts`,
		toUnix(start), toUnix(end),
	)
	if err != nil {
		return nil, fmt.Errorf("列出事件失败: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		ev, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, *ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历事件失败: %w", err)
	}
	return events, nil
}

// ListEventsByDate 查询某一天的事件。切日按本地时区零点（与 QueryTimeline 一致）。
func (db *DB) ListEventsByDate(day time.Time) ([]Event, error) {
	day = StartOfLocalDay(day)
	next := day.Add(24 * time.Hour)
	return db.ListEvents(day, next)
}

// SearchEvents 按关键词搜索事件内容。
func (db *DB) SearchEvents(query string, start, end time.Time) ([]Event, error) {
	pattern := "%" + query + "%"
	rows, err := db.Query(
		`SELECT id, ts, type, app_path, app_name, content, screenshot_path, meta
		 FROM events
		 WHERE ts >= ? AND ts < ? AND content LIKE ?
		 ORDER BY ts`,
		toUnix(start), toUnix(end), pattern,
	)
	if err != nil {
		return nil, fmt.Errorf("搜索事件失败: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		ev, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, *ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历事件失败: %w", err)
	}
	return events, nil
}

// ListEventsByType 按类型查询时间范围内的事件。
func (db *DB) ListEventsByType(eventType EventType, start, end time.Time) ([]Event, error) {
	rows, err := db.Query(
		`SELECT id, ts, type, app_path, app_name, content, screenshot_path, meta
		 FROM events
		 WHERE type = ? AND ts >= ? AND ts < ?
		 ORDER BY ts`,
		string(eventType), toUnix(start), toUnix(end),
	)
	if err != nil {
		return nil, fmt.Errorf("按类型列出事件失败: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		ev, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, *ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历事件失败: %w", err)
	}
	return events, nil
}

// scanEvent 从 sql.Row/sql.Rows 扫描 Event。
func scanEvent(sc interface {
	Scan(dest ...any) error
}) (*Event, error) {
	var ev Event
	var ts int64
	var typeStr string
	if err := sc.Scan(&ev.ID, &ts, &typeStr, &ev.AppPath, &ev.AppName, &ev.Content, &ev.ScreenshotPath, &ev.Meta); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("扫描事件失败: %w", err)
	}
	ev.TS = fromUnix(ts)
	ev.Type = EventType(typeStr)
	return &ev, nil
}
