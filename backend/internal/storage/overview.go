package storage

import (
	"fmt"
	"sort"
	"time"
)

// 本文件存放概览页(Overview)所需的聚合查询。
// 配合 proto/overview.proto 的 OverviewService 使用。
//
// 关键概念:
//   - range = day / week / month,支撑概览页 今天/本周/本月 切换
//   - "工作时长" = state 为 engaged(强活跃) 或 active(余热) 的段;idle 不计入。
//   - interrupt_count(打断次数)= state 从 idle 恢复到非 idle(engaged/active)的次数。
//     engaged↔active 之间的切换不算打断(同属"在工作")。

// RangeBounds 把 (date, range) 解析成 [start, end) 的查询边界。
// date 为零值时取今天;range 取 day/week/month,默认 day。
// 周以周一为起点(ISO 周),月以 1 号为起点。
//
// 切日按本地时区零点（与 QueryTimeline / TodayActivityMinutes 一致），
// 避免 UTC 切日导致 UTC+8 下凌晨 0-8 点的工作被算进前一天（坑 #44）。
func RangeBounds(day time.Time, rng string) (time.Time, time.Time) {
	if day.IsZero() {
		day = time.Now()
	}
	day = StartOfLocalDay(day)

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
		start := day.AddDate(0, 0, -(int(day.Day()) - 1))
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

// RangeActiveMinutes 统计时间范围内的工作总分钟数(engaged+active)。
// 与时间轴 QueryTimeline 共用 ActiveSegmentsByRange 段列表生成逻辑，
// 确保两边数字一致：区间重叠查询 + 聚合合并 + 按范围裁剪，再对 engaged/active 段求和。
func (db *DB) RangeActiveMinutes(start, end time.Time) (int, error) {
	segs, err := db.ActiveSegmentsByRange(start, end)
	if err != nil {
		return 0, err
	}
	var totalSec int64
	for _, s := range segs {
		if s.State == "engaged" || s.State == "active" {
			totalSec += int64(s.EndTS.Sub(s.StartTS) / time.Second)
		}
	}
	return int(totalSec / 60), nil
}

// RangeActiveSegments 统计时间范围内的工作段数(engaged+active)。
// 与 RangeActiveMinutes 共用 ActiveSegmentsByRange，段数即聚合裁剪后的 engaged/active 段计数。
func (db *DB) RangeActiveSegments(start, end time.Time) (int, error) {
	segs, err := db.ActiveSegmentsByRange(start, end)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, s := range segs {
		if s.State == "engaged" || s.State == "active" {
			count++
		}
	}
	return count, nil
}

// InterruptCount 统计时间范围内的打断次数。
// 定义：段间空档 >= awayThresholdS = 一次中断（离开再回来工作）。
//
// 背景：旧实现数 prev.state=='idle' && curr.state∈{engaged,active} 的次数，
// 但采集层的离开检测已改用真实键鼠空闲（inputIdleMs >= awayThresholdS）断段，
// 段以 state=engaged 收尾留 DB 空档而非 idle 段（坑 #45）。帧差污染也让 state
// 永远到不了 idle。故旧实现几乎恒返回 0——查询层与采集层语义脱节。
//
// 新实现与采集层对齐：用 ActiveSegmentsByRange 取聚合+裁剪后的段列表，
// 遍历相邻 engaged/active 段，若空档（curr.start - prev.end）>= awayThresholdS
// 则计一次中断。awayThresholdS 由调用方从 Collector.AwayThresholdS() 传入，
// 与采集层断段阈值同源。
func (db *DB) InterruptCount(start, end time.Time, awayThresholdS int) (int, error) {
	segs, err := db.ActiveSegmentsByRange(start, end)
	if err != nil {
		return 0, err
	}
	threshold := time.Duration(awayThresholdS) * time.Second
	count := 0
	var prevEnd time.Time
	for _, s := range segs {
		if s.State != "engaged" && s.State != "active" {
			continue
		}
		if !prevEnd.IsZero() {
			gap := s.StartTS.Sub(prevEnd)
			if gap >= threshold {
				count++
			}
		}
		prevEnd = s.EndTS
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
	segs, err := db.ActiveSegmentsByRange(start, end)
	if err != nil {
		return nil, err
	}
	// 按 appName+category 分组求和（与 RangeActiveMinutes 共用段列表，确保数字一致）。
	type key struct{ name, category string }
	secByKey := make(map[key]int64)
	for _, s := range segs {
		if s.State == "engaged" || s.State == "active" {
			k := key{s.AppName, s.Category}
			secByKey[k] += int64(s.EndTS.Sub(s.StartTS) / time.Second)
		}
	}
	out := make([]AppMinutes, 0, len(secByKey))
	for k, sec := range secByKey {
		out = append(out, AppMinutes{Name: k.name, Category: k.category, Minutes: int(sec / 60)})
	}
	// 按分钟降序。
	sort.Slice(out, func(i, j int) bool { return out[i].Minutes > out[j].Minutes })
	return out, nil
}

// WhitelistAppsWithMinutes 以白名单(app_categories)为基准，LEFT JOIN 活动段聚合时长。
// 返回白名单全部应用(name/category/path)，附时间范围内的活跃分钟(没用过的为 0)。
// 与 AppMinutesByRange 的区别：本方法保证返回白名单全部应用（即使今日 0 活动），
// 用于首页"采集应用"卡片——需与设置页白名单列表数量一致。
// 按分钟降序（有活动的在前，0 分钟的在后）。
func (db *DB) WhitelistAppsWithMinutes(start, end time.Time) ([]AppMinutes, error) {
	// 1. 查白名单全部应用（保证返回数量与设置页白名单一致）。
	wlRows, err := db.Query(
		`SELECT name, category, path FROM app_categories ORDER BY added_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("查询白名单应用失败: %w", err)
	}
	type wlApp struct {
		name, category, path string
		addedOrder           int
	}
	var wlApps []wlApp
	for i := 0; wlRows.Next(); i++ {
		var a wlApp
		if err := wlRows.Scan(&a.name, &a.category, &a.path); err != nil {
			wlRows.Close()
			return nil, fmt.Errorf("扫描白名单应用失败: %w", err)
		}
		a.addedOrder = i
		wlApps = append(wlApps, a)
	}
	wlRows.Close()

	// 2. 取公共段列表（与 RangeActiveMinutes 共用，确保数字一致）。
	segs, err := db.ActiveSegmentsByRange(start, end)
	if err != nil {
		return nil, err
	}
	// 按 appPath 分组求和（只算 engaged/active）。
	secByPath := make(map[string]int64)
	for _, s := range segs {
		if s.State == "engaged" || s.State == "active" {
			secByPath[s.AppPath] += int64(s.EndTS.Sub(s.StartTS) / time.Second)
		}
	}

	// 3. 合并：白名单应用 + 对应时长（0 分钟的也保留）。
	out := make([]AppMinutes, 0, len(wlApps))
	for _, a := range wlApps {
		out = append(out, AppMinutes{
			Name:     a.name,
			Category: a.category,
			Minutes:  int(secByPath[a.path] / 60),
		})
	}
	// 按分钟降序（有活动的在前，0 分钟的在后），同分钟按添加顺序。
	sort.SliceStable(out, func(i, j int) bool { return out[i].Minutes > out[j].Minutes })
	return out, nil
}

// CategoryAggregate 按类别聚合时间范围内的活跃分钟(类别占比排行)。
// 返回 (category, minutes) 列表,按分钟降序。
type CategoryMinutes struct {
	Category string
	Minutes  int
}

func (db *DB) CategoryAggregate(start, end time.Time) ([]CategoryMinutes, error) {
	segs, err := db.ActiveSegmentsByRange(start, end)
	if err != nil {
		return nil, err
	}
	// 按 category 分组求和（与 RangeActiveMinutes 共用段列表，确保数字一致）。
	secByCat := make(map[string]int64)
	for _, s := range segs {
		if s.State == "engaged" || s.State == "active" {
			secByCat[s.Category] += int64(s.EndTS.Sub(s.StartTS) / time.Second)
		}
	}
	out := make([]CategoryMinutes, 0, len(secByCat))
	for cat, sec := range secByCat {
		out = append(out, CategoryMinutes{Category: cat, Minutes: int(sec / 60)})
	}
	// 按分钟降序。
	sort.Slice(out, func(i, j int) bool { return out[i].Minutes > out[j].Minutes })
	return out, nil
}

// DailyMinutes 返回 [start, end) 范围内每天的活跃分钟(热力图用)。
// date 为 "YYYY-MM-DD"(本地时区),minutes 为当日 active 总分钟。
// 与 RangeActiveMinutes 共用段列表生成逻辑（AggregateSegments），
// 再按本地午夜逐天裁剪求和，确保跨天段不重复计也不漏计。
type DailyMinutesRow struct {
	Date    string
	Minutes int
}

func (db *DB) DailyMinutes(start, end time.Time) ([]DailyMinutesRow, error) {
	segs, err := db.ListActivitySegments(start, end)
	if err != nil {
		return nil, err
	}
	segs = AggregateSegments(segs)

	// 按本地天累加：每条段可能跨多天，逐天 clamp 求和。
	daySeconds := make(map[string]int64)
	for _, s := range segs {
		if s.State != "engaged" && s.State != "active" {
			continue
		}
		dayStart := StartOfLocalDay(s.StartTS)
		for dayStart.Before(s.EndTS) && dayStart.Before(end) {
			dayEnd := dayStart.Add(24 * time.Hour)
			// clamp 到 [dayStart, dayEnd) 与 [start, end) 的交集。
			cs := s.StartTS
			if cs.Before(dayStart) {
				cs = dayStart
			}
			ce := s.EndTS
			if ce.After(dayEnd) {
				ce = dayEnd
			}
			if ce.After(cs) {
				dayKey := dayStart.Format("2006-01-02")
				daySeconds[dayKey] += int64(ce.Sub(cs) / time.Second)
			}
			dayStart = dayEnd
		}
	}

	// 转为按日期升序的切片。
	out := make([]DailyMinutesRow, 0, len(daySeconds))
	for d := StartOfLocalDay(start); d.Before(end); d = d.Add(24 * time.Hour) {
		key := d.Format("2006-01-02")
		out = append(out, DailyMinutesRow{Date: key, Minutes: int(daySeconds[key] / 60)})
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
