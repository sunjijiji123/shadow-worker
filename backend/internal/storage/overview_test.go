package storage

import (
	"testing"
	"time"
)

// 纯函数测试:RangeBounds 的 day/week/month 边界。

func TestRangeBounds_Day(t *testing.T) {
	day := time.Date(2026, 6, 18, 10, 30, 0, 0, time.UTC)
	start, end := RangeBounds(day, "day")
	wantStart := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) || !end.Equal(wantEnd) {
		t.Fatalf("day 边界错误: got [%s, %s), want [%s, %s)", start, end, wantStart, wantEnd)
	}
}

func TestRangeBounds_Week(t *testing.T) {
	// 2026-06-18 是周四,周一起点应为 2026-06-15
	cases := []struct {
		name string
		day  time.Time
		want string // 周一日期 YYYY-MM-DD
	}{
		{"周四", time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC), "2026-06-15"},
		{"周日(归到上周一起)", time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC), "2026-06-15"},
		{"周一", time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC), "2026-06-15"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			start, end := RangeBounds(c.day, "week")
			got := start.Format("2006-01-02")
			if got != c.want {
				t.Fatalf("week 起点错误: got %s, want %s", got, c.want)
			}
			// 周跨度 = 7 天
			if end.Sub(start) != 7*24*time.Hour {
				t.Fatalf("week 跨度错误: got %v", end.Sub(start))
			}
		})
	}
}

func TestRangeBounds_Month(t *testing.T) {
	day := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	start, end := RangeBounds(day, "month")
	wantStart := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) || !end.Equal(wantEnd) {
		t.Fatalf("month 边界错误: got [%s, %s), want [%s, %s)", start, end, wantStart, wantEnd)
	}
}

func TestRangeBounds_DefaultIsDay(t *testing.T) {
	// range 为空或未知值,默认走 day
	day := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	start1, end1 := RangeBounds(day, "")
	start2, end2 := RangeBounds(day, "unknown")
	start3, end3 := RangeBounds(day, "day")
	if !start1.Equal(start3) || !end1.Equal(end3) || !start2.Equal(start3) || !end2.Equal(end3) {
		t.Fatal("空/未知 range 应默认 day")
	}
}

func TestPreviousRangeBounds(t *testing.T) {
	day := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	// day: 昨天
	ps, _ := PreviousRangeBounds(day, "day")
	if !ps.Equal(time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("day 前一期起点错误: %s", ps)
	}
	// week: 上周一起
	ps, _ = PreviousRangeBounds(day, "week")
	if ps.Format("2006-01-02") != "2026-06-08" {
		t.Fatalf("week 前一期起点错误: %s", ps.Format("2006-01-02"))
	}
	// month: 上月 1 号
	ps, _ = PreviousRangeBounds(day, "month")
	if !ps.Equal(time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("month 前一期起点错误: %s", ps)
	}
}

// MinutesToLevel 分档测试。

func TestMinutesToLevel(t *testing.T) {
	cases := []struct {
		min  int
		want int
	}{
		{0, 0},
		{15, 1},   // <30
		{30, 2},   // <60
		{59, 2},
		{60, 3},   // <120
		{150, 4},  // <180
		{180, 5},  // >=180
		{500, 5},
	}
	for _, c := range cases {
		got := MinutesToLevel(c.min)
		if got != c.want {
			t.Errorf("MinutesToLevel(%d) = %d, want %d", c.min, got, c.want)
		}
	}
}

// InterruptCount 打断计数测试（段间空档 >= 阈值 = 一次中断）。

func TestInterruptCount(t *testing.T) {
	db := newTestDB(t)
	day := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	start, end := RangeBounds(day, "day")

	// 构造序列（不同 app，避免聚合合并）：
	//   App1 engaged 09:00-10:00 → 空档 2h → App2 engaged 12:00-13:00 → 空档 5min → App3 engaged 13:05-14:00
	// awayThresholdS=600(10min)：2h >= 600s → 1 次中断；5min < 600s → 不算
	awayThreshold := 600
	for _, s := range []struct {
		app   string
		start time.Time
		end   time.Time
	}{
		{"App1", time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC), time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)},
		{"App2", time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC), time.Date(2026, 6, 18, 13, 0, 0, 0, time.UTC)},
		{"App3", time.Date(2026, 6, 18, 13, 5, 0, 0, time.UTC), time.Date(2026, 6, 18, 14, 0, 0, 0, time.UTC)},
	} {
		if _, err := db.InsertActivitySegment(ActivitySegment{
			StartTS: s.start, EndTS: s.end,
			AppPath: "C:\\" + s.app + ".exe", AppName: s.app, Category: "coding", State: "engaged",
		}); err != nil {
			t.Fatalf("插入段失败: %v", err)
		}
	}

	got, err := db.InterruptCount(start, end, awayThreshold)
	if err != nil {
		t.Fatalf("InterruptCount 失败: %v", err)
	}
	if got != 1 {
		t.Fatalf("打断次数错误: got %d, want 1 (2h 空档 >= 600s 算中断，5min 空档 < 600s 不算)", got)
	}
}

func TestInterruptCount_NoIdle(t *testing.T) {
	db := newTestDB(t)
	day := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	start, end := RangeBounds(day, "day")

	// 同 app 连续段（无空档），聚合后为一段 → 无打断
	for _, h := range []int{9, 10, 11} {
		s := time.Date(2026, 6, 18, h, 0, 0, 0, time.UTC)
		if _, err := db.InsertActivitySegment(ActivitySegment{
			StartTS: s, EndTS: s.Add(time.Hour),
			AppPath: "C:\\Cursor.exe", AppName: "Cursor", Category: "coding", State: "active",
		}); err != nil {
			t.Fatalf("插入段失败: %v", err)
		}
	}
	got, _ := db.InterruptCount(start, end, 600)
	if got != 0 {
		t.Fatalf("连续同 app 应无打断: got %d, want 0", got)
	}
}

// RangeActiveMinutes + AppMinutesByRange + CategoryAggregate 集成测试。

func TestRangeAggregations(t *testing.T) {
	db := newTestDB(t)
	day := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	start, end := RangeBounds(day, "day")

	// Cursor coding 60min active + Chrome browser 30min active
	if _, err := db.InsertActivitySegment(ActivitySegment{
		StartTS: time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC),
		EndTS:   time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC),
		AppPath: "C:\\Cursor.exe", AppName: "Cursor", Category: "coding", State: "active",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.InsertActivitySegment(ActivitySegment{
		StartTS: time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC),
		EndTS:   time.Date(2026, 6, 18, 10, 30, 0, 0, time.UTC),
		AppPath: "C:\\Chrome.exe", AppName: "Chrome", Category: "browser", State: "active",
	}); err != nil {
		t.Fatal(err)
	}

	// 总分钟
	min, err := db.RangeActiveMinutes(start, end)
	if err != nil {
		t.Fatal(err)
	}
	if min != 90 {
		t.Fatalf("RangeActiveMinutes = %d, want 90", min)
	}

	// 活动段数
	segN, _ := db.RangeActiveSegments(start, end)
	if segN != 2 {
		t.Fatalf("RangeActiveSegments = %d, want 2", segN)
	}

	// 应用排行(按分钟降序,Cursor 60 在前)
	apps, _ := db.AppMinutesByRange(start, end)
	if len(apps) != 2 || apps[0].Name != "Cursor" || apps[0].Minutes != 60 {
		t.Fatalf("AppMinutesByRange 错误: %+v", apps)
	}

	// 类别聚合
	cats, _ := db.CategoryAggregate(start, end)
	if len(cats) != 2 {
		t.Fatalf("CategoryAggregate 应有 2 类: %+v", cats)
	}
	total := 0
	for _, c := range cats {
		total += c.Minutes
	}
	if total != 90 {
		t.Fatalf("类别总分钟 = %d, want 90", total)
	}
}

func TestDailyMinutes(t *testing.T) {
	db := newTestDB(t)
	// 范围:6-17 ~ 6-19
	start := time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)

	// 6-18 有 60min,6-19 有 30min
	if _, err := db.InsertActivitySegment(ActivitySegment{
		StartTS: time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC),
		EndTS:   time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC),
		AppPath: "C:\\Cursor.exe", AppName: "Cursor", Category: "coding", State: "active",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.InsertActivitySegment(ActivitySegment{
		StartTS: time.Date(2026, 6, 19, 9, 0, 0, 0, time.UTC),
		EndTS:   time.Date(2026, 6, 19, 9, 30, 0, 0, time.UTC),
		AppPath: "C:\\Cursor.exe", AppName: "Cursor", Category: "coding", State: "active",
	}); err != nil {
		t.Fatal(err)
	}

	daily, err := db.DailyMinutes(start, end)
	if err != nil {
		t.Fatal(err)
	}
	byDate := map[string]int{}
	for _, d := range daily {
		byDate[d.Date] = d.Minutes
	}
	if byDate["2026-06-18"] != 60 {
		t.Fatalf("6-18 分钟 = %d, want 60", byDate["2026-06-18"])
	}
	if byDate["2026-06-19"] != 30 {
		t.Fatalf("6-19 分钟 = %d, want 30", byDate["2026-06-19"])
	}
}

// TestRangeAggregationsThreeState 验证三态聚合:engaged 和 active 都计入工作时长,idle 不计。
// 注意：同 app 连续段会被 AggregateSegments 合并，state 取最后一条。
// 故 idle 段用不同 app（Chrome），避免合并后覆盖 Cursor 的 active 状态。
func TestRangeAggregationsThreeState(t *testing.T) {
	db := newTestDB(t)
	day := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	start, end := RangeBounds(day, "day")

	// Cursor:engaged 40min + active 20min（连续同 app → 合并为 60min，state=active）
	// Chrome:idle 30min + idle 50min（不同 app，不与 Cursor 合并；同 app 但有空档 → 2 段 idle）
	segs := []struct {
		s, e  time.Time
		app   string
		state string
	}{
		{time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC), time.Date(2026, 6, 18, 9, 40, 0, 0, time.UTC), "Cursor", "engaged"},
		{time.Date(2026, 6, 18, 9, 40, 0, 0, time.UTC), time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC), "Cursor", "active"},
		{time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC), time.Date(2026, 6, 18, 10, 30, 0, 0, time.UTC), "Chrome", "idle"},
		{time.Date(2026, 6, 18, 11, 0, 0, 0, time.UTC), time.Date(2026, 6, 18, 11, 50, 0, 0, time.UTC), "Chrome", "idle"},
	}
	for _, s := range segs {
		if _, err := db.InsertActivitySegment(ActivitySegment{
			StartTS: s.s, EndTS: s.e,
			AppPath: "C:\\" + s.app + ".exe", AppName: s.app, Category: "coding", State: s.state,
		}); err != nil {
			t.Fatalf("插入段失败: %v", err)
		}
	}

	// 总分钟:Cursor 合并后 60min（engaged+active），Chrome idle 不计
	min, err := db.RangeActiveMinutes(start, end)
	if err != nil {
		t.Fatal(err)
	}
	if min != 60 {
		t.Fatalf("RangeActiveMinutes(三态) = %d, want 60 (Cursor 合并后 engaged+active)", min)
	}

	// 活动段数:Cursor 合并为 1 段 active（Chrome 2 段 idle 不计）
	segN, _ := db.RangeActiveSegments(start, end)
	if segN != 1 {
		t.Fatalf("RangeActiveSegments(三态) = %d, want 1 (Cursor 合并为 1 段 active)", segN)
	}

	// 应用排行:Cursor 60min
	apps, _ := db.AppMinutesByRange(start, end)
	if len(apps) != 1 || apps[0].Name != "Cursor" || apps[0].Minutes != 60 {
		t.Fatalf("AppMinutesByRange(三态) 错误: %+v", apps)
	}

	// 类别:coding 60min
	cats, _ := db.CategoryAggregate(start, end)
	if len(cats) != 1 || cats[0].Minutes != 60 {
		t.Fatalf("CategoryAggregate(三态) 错误: %+v", cats)
	}
}

// TestInterruptCountGapThreshold 验证空档阈值边界：
// 空档恰好等于阈值算中断；小于阈值不算；idle 段被聚合吸收后不影响计数。
func TestInterruptCountGapThreshold(t *testing.T) {
	db := newTestDB(t)
	day := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	start, end := RangeBounds(day, "day")

	// 不同 app，避免聚合合并：
	//   App1 engaged 09:00-10:00 → 空档恰好 10min → App2 engaged 10:10-11:00 → 空档 9min → App3 engaged 11:09-12:00
	// awayThresholdS=600(10min)：10min >= 600s → 1 次；9min < 600s → 不算
	awayThreshold := 600
	for _, s := range []struct {
		app   string
		start time.Time
		end   time.Time
	}{
		{"App1", time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC), time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)},
		{"App2", time.Date(2026, 6, 18, 10, 10, 0, 0, time.UTC), time.Date(2026, 6, 18, 11, 0, 0, 0, time.UTC)},
		{"App3", time.Date(2026, 6, 18, 11, 9, 0, 0, time.UTC), time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)},
	} {
		if _, err := db.InsertActivitySegment(ActivitySegment{
			StartTS: s.start, EndTS: s.end,
			AppPath: "C:\\" + s.app + ".exe", AppName: s.app, Category: "coding", State: "engaged",
		}); err != nil {
			t.Fatalf("插入段失败: %v", err)
		}
	}

	got, err := db.InterruptCount(start, end, awayThreshold)
	if err != nil {
		t.Fatalf("InterruptCount 失败: %v", err)
	}
	if got != 1 {
		t.Fatalf("空档阈值边界错误: got %d, want 1 (10min >= 600s 算中断，9min < 600s 不算)", got)
	}
}

func TestCategoryColor(t *testing.T) {
	cases := map[string]string{
		"coding":  "#3B82F6",
		"office":  "#8B5CF6",
		"browser": "#F59E0B",
		"chat":    "#10B981",
		"other":   "#6B7280",
		"unknown": "#6B7280",
	}
	for cat, want := range cases {
		if got := CategoryColor(cat); got != want {
			t.Errorf("CategoryColor(%q) = %q, want %q", cat, got, want)
		}
	}
}
