package storage

import (
	"fmt"
	"time"
)

// 本文件存放概览页(Overview)所需的聚合查询。
// 配合 proto/overview.proto 的 OverviewService 使用。
//
// 关键概念:
//   - range = day / week / month,支撑概览页 今天/本周/本月 切换
//   - interrupt_count(打断次数)= 当天 active↔idle 的切换次数(决策 A)
//     即:state 从 idle 变 active 的次数(每次"离开后恢复工作"算一次打断)

// RangeBounds 把 (date, range) 解析成 [start, end) 的查询边界。
// date 为零值时取今天;range 取 day/week/month,默认 day。
// 周以周一为起点(ISO 周),月以 1 号为起点。
func RangeBounds(day time.Time, rng string) (time.Time, time.Time) {
	if day.IsZero() {
		day = time.Now().UTC()
	}
	day = day.UTC().Truncate(24 * time.Hour)

	switch rng {
	case "week":
		// 周一为一周起点:weekday 周日=0..周六=6,转成"距周一的偏移"
		wd := int(day.Weekday())
		if wd == 0 {
			wd = 7 // 周日归到上一周末尾
		}
		start := day.AddDate(0, 0, -(wd - 1))
		return start, start.AddDate(0, 0, 7)
	case "month":
		start := day.AddDate(0, 0, -(int(day.Day())-1))
		return start, start.AddDate(0, 1, 0)
	default: // day
		return day, day.Add(24 * time.Hour)
	}
}

// PreviousRangeBounds 返回上一周期(昨天/上周/上月)的边界,用于计算 delta。
// 注意:不能用 dur 减法(各月天数不同会错位),按 range 语义各自回退。
func PreviousRangeBounds(day time.Time, rng string) (time.Time, time.Time) {
	start, _ := RangeBounds(day, rng)
	switch rng {
	case "week":
		return start.AddDate(0, 0, -7), start
	case "month":
		return start.AddDate(0, -1, 0), start
	default: // day
		return start.AddDate(0, 0, -1), start
	}
}

// RangeActiveMinutes 统计时间范围内白名单应用的活跃总分钟数(state=active)。
func (db *DB) RangeActiveMinutes(start, end time.Time) (int, error) {
	var totalSec int64
	err := db.QueryRow(
		`SELECT COALESCE(SUM(end_ts - start_ts), 0)
		 FROM activity_segments
		 WHERE state = 'active' AND start_ts >= ? AND end_ts <= ?`,
		toUnix(start), toUnix(end),
	).Scan(&totalSec)
	if err != nil {
		return 0, fmt.Errorf("统计活跃时长失败: %w", err)
	}
	return int(totalSec / 60), nil
}

// RangeActiveSegments 统计时间范围内的活动段数(state=active)。
func (db *DB) RangeActiveSegments(start, end time.Time) (int, error) {
	var n int64
	err := db.QueryRow(
		`SELECT COUNT(*) FROM activity_segments
		 WHERE state = 'active' AND start_ts >= ? AND end_ts <= ?`,
		toUnix(start), toUnix(end),
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("统计活动段数失败: %w", err)
	}
	return int(n), nil
}

// InterruptCount 统计时间范围内的打断次数。
// 定义(决策 A):state 从 idle 变 active 的切换次数。
// 实现:取范围内按 start_ts 升序的相邻段,数 prev.state='idle' && curr.state='active' 的次数。
// 注意:跨范围边界时,边界前一段不参与(只看范围内),简单实现。
func (db *DB) InterruptCount(start, end time.Time) (int, error) {
	rows, err := db.Query(
		`SELECT state FROM activity_segments
		 WHERE start_ts >= ? AND start_ts < ?
		 ORDER BY start_ts`,
		toUnix(start), toUnix(end),
	)
	if err != nil {
		return 0, fmt.Errorf("查询打断次数失败: %w", err)
	}
	defer rows.Close()

	count := 0
	prevState := ""
	for rows.Next() {
		var state string
		if err := rows.Scan(&state); err != nil {
			return 0, fmt.Errorf("扫描打断状态失败: %w", err)
		}
		if prevState == "idle" && state == "active" {
			count++
		}
		prevState = state
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("遍历打断状态失败: %w", err)
	}
	return count, nil
}

// AppMinutesByRange 按应用聚合时间范围内的活跃分钟(应用排行 + 涉及应用数)。
// 返回 (appName, category, minutes) 列表,按分钟降序。
type AppMinutes struct {
	Name     string
	Category string
	Minutes  int
}

func (db *DB) AppMinutesByRange(start, end time.Time) ([]AppMinutes, error) {
	rows, err := db.Query(
		`SELECT app_name, category, COALESCE(SUM(end_ts - start_ts), 0) AS sec
		 FROM activity_segments
		 WHERE state = 'active' AND start_ts >= ? AND end_ts <= ?
		 GROUP BY app_name, category
		 ORDER BY sec DESC`,
		toUnix(start), toUnix(end),
	)
	if err != nil {
		return nil, fmt.Errorf("按应用聚合时长失败: %w", err)
	}
	defer rows.Close()

	var out []AppMinutes
	for rows.Next() {
		var am AppMinutes
		var sec int64
		if err := rows.Scan(&am.Name, &am.Category, &sec); err != nil {
			return nil, fmt.Errorf("扫描应用时长失败: %w", err)
		}
		am.Minutes = int(sec / 60)
		out = append(out, am)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历应用时长失败: %w", err)
	}
	return out, nil
}

// CategoryAggregate 按类别聚合时间范围内的活跃分钟(类别占比排行)。
// 返回 (category, minutes) 列表,按分钟降序。
type CategoryMinutes struct {
	Category string
	Minutes  int
}

func (db *DB) CategoryAggregate(start, end time.Time) ([]CategoryMinutes, error) {
	rows, err := db.Query(
		`SELECT category, COALESCE(SUM(end_ts - start_ts), 0) AS sec
		 FROM activity_segments
		 WHERE state = 'active' AND start_ts >= ? AND end_ts <= ?
		 GROUP BY category
		 ORDER BY sec DESC`,
		toUnix(start), toUnix(end),
	)
	if err != nil {
		return nil, fmt.Errorf("按类别聚合时长失败: %w", err)
	}
	defer rows.Close()

	var out []CategoryMinutes
	for rows.Next() {
		var cm CategoryMinutes
		var sec int64
		if err := rows.Scan(&cm.Category, &sec); err != nil {
			return nil, fmt.Errorf("扫描类别时长失败: %w", err)
		}
		cm.Minutes = int(sec / 60)
		out = append(out, cm)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历类别时长失败: %w", err)
	}
	return out, nil
}

// DailyMinutes 返回 [start, end) 范围内每天的活跃分钟(热力图用)。
// date 为 "YYYY-MM-DD"(UTC),minutes 为当日 active 总分钟。
type DailyMinutesRow struct {
	Date    string
	Minutes int
}

func (db *DB) DailyMinutes(start, end time.Time) ([]DailyMinutesRow, error) {
	rows, err := db.Query(
		`SELECT date(start_ts, 'unixepoch') AS day, COALESCE(SUM(end_ts - start_ts), 0) AS sec
		 FROM activity_segments
		 WHERE state = 'active' AND start_ts >= ? AND end_ts <= ?
		 GROUP BY day
		 ORDER BY day`,
		toUnix(start), toUnix(end),
	)
	if err != nil {
		return nil, fmt.Errorf("按日聚合时长失败: %w", err)
	}
	defer rows.Close()

	var out []DailyMinutesRow
	for rows.Next() {
		var r DailyMinutesRow
		var sec int64
		if err := rows.Scan(&r.Date, &sec); err != nil {
			return nil, fmt.Errorf("扫描每日时长失败: %w", err)
		}
		r.Minutes = int(sec / 60)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历每日时长失败: %w", err)
	}
	return out, nil
}

// MinutesToLevel 把分钟映射到热力图 0~5 档。
// 0=无数据,阈值 0/30/60/120/180/240+ 对应 0~5。
func MinutesToLevel(minutes int) int {
	switch {
	case minutes <= 0:
		return 0
	case minutes < 30:
		return 1
	case minutes < 60:
		return 2
	case minutes < 120:
		return 3
	case minutes < 180:
		return 4
	default:
		return 5
	}
}

// CategoryColor 返回类别固定色(v2 全局统一)。
func CategoryColor(category string) string {
	switch category {
	case "coding":
		return "#3B82F6"
	case "office":
		return "#8B5CF6"
	case "browser":
		return "#F59E0B"
	case "chat":
		return "#10B981"
	default:
		return "#6B7280"
	}
}
