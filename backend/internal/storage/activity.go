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

// LatestVLMSummary 查询时间窗口 [start, end) 内最近一条 vlm_summary 事件的内容。
// 走 idx_events_type_ts 索引。无结果返回空串。content 列可空，用 NullString 兜底。
//
// 半开区间 + app 校验（坑：时间轴雷同摘要）：
//   - 旧实现用闭区间 ts>=? AND ts<=?，相邻段边界上同一条 event 会被前后两段同时命中，
//     导致"两个不同应用段显示完全相同的 VLM 摘要"。改半开区间 ts>=? AND ts<?，边界
//     event 只属于前一段。
//   - appPath 非空时追加 AND (app_path = ? OR app_path = '')：精确匹配该应用的事件；
//     兼容 app_path 为空的旧/异常数据（不误杀）。appPath 为空则退化成不校验 app（防御）。
func (db *DB) LatestVLMSummary(start, end time.Time, appPath string) (string, error) {
	var content sql.NullString
	var err error
	if appPath == "" {
		err = db.QueryRow(
			`SELECT content FROM events
			 WHERE type = ? AND ts >= ? AND ts < ?
			 ORDER BY ts DESC LIMIT 1`,
			string(EventTypeVLMSummary), toUnix(start), toUnix(end),
		).Scan(&content)
	} else {
		err = db.QueryRow(
			`SELECT content FROM events
			 WHERE type = ? AND ts >= ? AND ts < ? AND (app_path = ? OR app_path = '')
			 ORDER BY ts DESC LIMIT 1`,
			string(EventTypeVLMSummary), toUnix(start), toUnix(end), appPath,
		).Scan(&content)
	}
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

// StartOfLocalDay 返回 t 所在本地日的 00:00（保留 t 的时区，通常是 time.Local）。
// 用于按天切日：与 QueryTimeline 的本地时区切日语义一致，避免 UTC 切日导致
// 跨本地午夜的事件错位归属（UTC+8 下凌晨 0-8 点事件被算进前一天）。
func StartOfLocalDay(t time.Time) time.Time {
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
	day = StartOfLocalDay(day)
	next := day.Add(24 * time.Hour)
	return db.ListActivitySegments(day, next)
}

// AggregateSegments 把连续相同 app 的段合并。
// 输入需按 start_ts 升序（ListActivitySegments 已保证）。
// 合并规则：相邻且 AppName 相同 *且时间连续*（无空档）→ 合为一段。
// start 取最早、end 取较晚者，state/windowTitle/summary 取该组最后一条。
//
// 时间连续性判据（!s.StartTS.After(cur.EndTS)）：离开检测引入后，
// 同 app 两段（如 VSCode 9-12 / VSCode 14-16，中间离开 2 小时）会在结果
// 列表里相邻且同名。若只看 AppName 会合并抹掉空档，离开就白检测了。
// 加连续性判据后：12→14 有空档（s.StartTS=14 > cur.EndTS=12）→ 不合并 →
// 空档在时间轴轨道上显示为空白断档。任何 app 切换或离开空档都形成断点。
//
// 注意：cur.EndTS 在采集层是每 tick 滚动更新的"段实际结束"，故此判据能精确
// 区分"连续工作"与"离开后回来"。历史数据（离开检测上线前的）段间空档多为
// DB tick 间隔（数百毫秒，EndTS≈下一段 StartTS），仍会被正确合并。
func AggregateSegments(segs []ActivitySegment) []ActivitySegment {
	if len(segs) == 0 {
		return segs
	}
	out := make([]ActivitySegment, 0, len(segs))
	cur := segs[0]
	for i := 1; i < len(segs); i++ {
		s := segs[i]
		if s.AppName == cur.AppName && !s.StartTS.After(cur.EndTS) {
			// 同 app 且时间连续：合并。end 取较晚者（防御性 max，正常情况 s 在 cur 之后）。
			if s.EndTS.After(cur.EndTS) {
				cur.EndTS = s.EndTS
			}
			// 状态/标题/摘要取最新一条（s 在 cur 之后，覆盖）。
			cur.State = s.State
			cur.WindowTitle = s.WindowTitle
			if s.Summary != "" {
				cur.Summary = s.Summary
			}
			continue
		}
		out = append(out, cur)
		cur = s
	}
	out = append(out, cur)
	return out
}

// ClipSegmentsToRange 把段按 [start, end) 边界虚拟裁剪，仅改返回值不落库。
//
// 背景：ListActivitySegments 用区间重叠判据（start_ts<dayEnd AND end_ts>dayStart），
// 一条横跨数天的段（如 id=518：6-22 23:37→6-24 22:19 的 46h 巨怪段）会完整命中
// 每一天。若直接交给 computeTimelineWindow，其端点会把可视窗口撑到 ~48h（见坑 #49），
// 当天真实时刻在窗口里只占一小段、刻度被压成 2h 步进，视觉上像"空白无记录"。
//
// 本函数对每条段取其与 [start, end) 的交集：
//   - 完全在范围内：原样保留。
//   - 跨越范围边界：start/end clamp 到 [start, end)，只保留范围内部分。
//   - 完全在范围外：丢弃（区间重叠查询理论上不会返回这类，防御性丢弃）。
//
// clamp 只动 StartTS/EndTS，AppName/State/WindowTitle/Summary 等沿用原段。
// 跨天段在每天的查询里各自只看到属于自己的那一天部分，显示与统计都正确。
func ClipSegmentsToRange(segs []ActivitySegment, start, end time.Time) []ActivitySegment {
	if len(segs) == 0 {
		return segs
	}
	out := make([]ActivitySegment, 0, len(segs))
	for _, s := range segs {
		// 与范围无交集（完全在范围之前或之后）——区间重叠查询不应返回，防御性跳过。
		if !s.EndTS.After(start) || !s.StartTS.Before(end) {
			continue
		}
		if s.StartTS.Before(start) {
			s.StartTS = start
		}
		if s.EndTS.After(end) {
			s.EndTS = end
		}
		// clamp 后若起止重合（边界恰好相切），不产生零宽段。
		if !s.StartTS.Before(s.EndTS) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// ActiveSegmentsByRange 返回 [start, end) 范围内的活动段（已聚合 + 已裁剪）。
// 这是概览页和时间轴页共用的唯一段列表生成入口，确保两边数据源一致。
//
// 三步流水线：
//  1. ListActivitySegments（区间重叠查询，命中任何与范围有交集的段）
//  2. AggregateSegments（合并同 app 连续段，消除采集层每 tick 的碎片段）
//  3. ClipSegmentsToRange（按 [start, end) 边界裁剪，跨范围段只保留范围内部分）
//
// 调用方拿到段列表后，"工作时长"只是对 state==engaged/active 的段求 endTs-startTs 之和，
// "段数"只是计数——简单加法，不会分叉。详见 RangeActiveMinutes / QueryTimeline。
func (db *DB) ActiveSegmentsByRange(start, end time.Time) ([]ActivitySegment, error) {
	segs, err := db.ListActivitySegments(start, end)
	if err != nil {
		return nil, err
	}
	segs = AggregateSegments(segs)
	segs = ClipSegmentsToRange(segs, start, end)
	return segs, nil
}

// TodayActivityMinutes 统计今日在白名单应用上的工作总分钟数(engaged+active)。
// "今日"按本地时区零点切（与时间轴 QueryTimeline 一致）。
//
// 跨天段处理：用区间重叠判据（start_ts<dayEnd AND end_ts>dayStart）命中当天，
// 并对时长 SUM 做"按天 clamp"——只累加当天内部分 MIN(end_ts,dayEnd)-MAX(start_ts,dayStart)，
// 避免一条横跨数天的巨怪段（如 46h 段，见坑 #49）被反复计入每一天、或因旧判据
// start_ts>=? AND end_ts<=? 漏掉跨天段。与 QueryTimeline 的 clipSegmentsToDay 语义一致。
func (db *DB) TodayActivityMinutes() (int, int, error) {
	start := StartOfLocalDay(time.Now())
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
