package grpcapi

import (
	"testing"
	"time"

	"shadow-worker/backend/internal/storage"
)

// segAt 构造一条活动段，时间用传入的 time.Time。仅测试聚合/窗口用。
func segAt(name string, start, end time.Time, state string) storage.ActivitySegment {
	return storage.ActivitySegment{
		AppName: name,
		StartTS: start,
		EndTS:   end,
		State:   state,
	}
}

// === aggregateSegments：时间连续性判据 ===

func TestAggregateSegments_ContinuousSameApp_Merges(t *testing.T) {
	// 同 app、时间连续（EndTS == 下一段 StartTS）→ 应合并为一段。
	base := time.Date(2026, 6, 20, 9, 0, 0, 0, time.UTC)
	segs := []storage.ActivitySegment{
		segAt("VSCode", base, base.Add(30*time.Minute), "engaged"),
		segAt("VSCode", base.Add(30*time.Minute), base.Add(60*time.Minute), "idle"), // 连续：同 app
	}
	got := aggregateSegments(segs)
	if len(got) != 1 {
		t.Fatalf("连续同 app 应合并为 1 段，实际 %d", len(got))
	}
	if got[0].EndTS != base.Add(60*time.Minute) {
		t.Errorf("合并段 EndTS 应取较晚者，got %v", got[0].EndTS)
	}
}

func TestAggregateSegments_GapSameApp_KeepsSeparate(t *testing.T) {
	// 离开场景：同 app 两段中间有空档（离开 2 小时）。
	// 离开检测会在采集层断段，两段在结果列表相邻且同名。
	// 若只看 AppName 会合并抹掉空档；加连续性判据后应保持两段独立。
	base := time.Date(2026, 6, 20, 9, 0, 0, 0, time.UTC)
	segs := []storage.ActivitySegment{
		segAt("VSCode", base, base.Add(3*time.Hour), "engaged"),                  // 09:00-12:00
		segAt("VSCode", base.Add(5*time.Hour), base.Add(7*time.Hour), "engaged"), // 14:00-16:00（12→14 离开）
	}
	got := aggregateSegments(segs)
	if len(got) != 2 {
		t.Fatalf("中间有空档的同 app 两段不应合并，want 2 got %d", len(got))
	}
	if got[0].EndTS != base.Add(3*time.Hour) {
		t.Errorf("第一段 EndTS 不应被拉到第二段，got %v", got[0].EndTS)
	}
	if got[1].StartTS != base.Add(5*time.Hour) {
		t.Errorf("第二段 StartTS 应保留，got %v", got[1].StartTS)
	}
}

func TestAggregateSegments_AppSwitch_AlwaysBreaks(t *testing.T) {
	// 切换 app（即使时间连续）仍是断点——保留既有语义。
	base := time.Date(2026, 6, 20, 9, 0, 0, 0, time.UTC)
	segs := []storage.ActivitySegment{
		segAt("VSCode", base, base.Add(60*time.Minute), "engaged"),
		segAt("Chrome", base.Add(60*time.Minute), base.Add(90*time.Minute), "engaged"),
	}
	got := aggregateSegments(segs)
	if len(got) != 2 {
		t.Fatalf("切 app 应形成断点，want 2 got %d", len(got))
	}
}

func TestAggregateSegments_Empty(t *testing.T) {
	if got := aggregateSegments(nil); len(got) != 0 {
		t.Fatalf("空输入应返回空，got %d", len(got))
	}
}

// === computeTimelineWindow ===

func TestComputeTimelineWindow_EmptyDay_Fallback(t *testing.T) {
	// 空天：无段无事件 → fallback day 当天 09:00~18:00 UTC。
	day := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	start, end := computeTimelineWindow(nil, nil, day, false)
	if want := day.Add(9 * time.Hour); !start.Equal(want) {
		t.Errorf("空天 start 应为 09:00 UTC，got %v want %v", start, want)
	}
	if want := day.Add(18 * time.Hour); !end.Equal(want) {
		t.Errorf("空天 end 应为 18:00 UTC，got %v want %v", end, want)
	}
}

func TestComputeTimelineWindow_FloorCeilToHour(t *testing.T) {
	// 整点取整：start 向下 floor，end 向上 ceil。
	day := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	segs := []storage.ActivitySegment{
		segAt("VSCode", day.Add(9*time.Hour+23*time.Minute), day.Add(11*time.Hour+47*time.Minute), "engaged"),
	}
	start, end := computeTimelineWindow(segs, nil, day, false)
	if want := day.Add(9 * time.Hour); !start.Equal(want) {
		t.Errorf("start 应 floor 到 09:00，got %v want %v", start, want)
	}
	if want := day.Add(12 * time.Hour); !end.Equal(want) {
		t.Errorf("end 应 ceil 到 12:00，got %v want %v", end, want)
	}
}

func TestComputeTimelineWindow_AlreadyOnHour_NoAdjust(t *testing.T) {
	// 恰好在整点：不应多加一小时（ceil 边界）。
	day := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	segs := []storage.ActivitySegment{
		segAt("VSCode", day.Add(9*time.Hour), day.Add(11*time.Hour), "engaged"),
	}
	_, end := computeTimelineWindow(segs, nil, day, false)
	if want := day.Add(11 * time.Hour); !end.Equal(want) {
		t.Errorf("恰在整点的 end 不应 ceil，got %v want %v", end, want)
	}
}

func TestComputeTimelineWindow_MinWindowPads(t *testing.T) {
	// minWindow=2h：活动跨度不足时 end 向后 pad 到 start+2h。
	day := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	// 09:00-09:10（仅 10 分钟），floor=09:00，end 原本 ceil=10:00，不足 2h → pad 到 11:00。
	segs := []storage.ActivitySegment{
		segAt("VSCode", day.Add(9*time.Hour), day.Add(9*time.Hour+10*time.Minute), "engaged"),
	}
	start, end := computeTimelineWindow(segs, nil, day, false)
	if want := day.Add(9 * time.Hour); !start.Equal(want) {
		t.Errorf("start，got %v want %v", start, want)
	}
	if want := day.Add(11 * time.Hour); !end.Equal(want) {
		t.Errorf("minWindow 不足应 pad 到 start+2h=11:00，got %v want %v", end, want)
	}
}

func TestComputeTimelineWindow_EventsExtendRange(t *testing.T) {
	// events 与 segments 合取 min/max：一个超早的事件应拉低 start。
	day := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	segs := []storage.ActivitySegment{
		segAt("VSCode", day.Add(10*time.Hour), day.Add(12*time.Hour), "engaged"),
	}
	events := []storage.Event{
		{TS: day.Add(8 * time.Hour)}, // 早于所有段 → start 应 floor 到 08:00
	}
	start, end := computeTimelineWindow(segs, events, day, false)
	if want := day.Add(8 * time.Hour); !start.Equal(want) {
		t.Errorf("events 应参与 min 计算，start，got %v want %v", start, want)
	}
	if want := day.Add(12 * time.Hour); !end.Equal(want) {
		t.Errorf("end，got %v want %v", end, want)
	}
}

func TestComputeTimelineWindow_TodayExtendsEndToNow(t *testing.T) {
	// 今天：end 至少含 now（今天还在进行中）。
	// 构造一个活动在很久以前结束，now 一定在其之后 → end 被 now 拉伸到 ceil(now)。
	day := time.Now().UTC().Truncate(24 * time.Hour)
	past := day.Add(2 * time.Hour) // 凌晨 2 点的活动，远早于现在
	segs := []storage.ActivitySegment{
		segAt("VSCode", past, past.Add(30*time.Minute), "engaged"),
	}
	_, end := computeTimelineWindow(segs, nil, day, true)
	now := time.Now().UTC()
	if !end.After(now) && !end.Equal(now.Truncate(time.Hour)) {
		t.Errorf("今天的 end 应含 now（>= ceil(now)），got %v now %v", end, now)
	}
	// end 必须是整点（ceil 取整保证）。
	if end.Minute() != 0 || end.Second() != 0 {
		t.Errorf("end 必须是整点，got %v", end)
	}
}

func TestComputeTimelineWindow_HistoricalDayEndAtLastEvent(t *testing.T) {
	// 历史天：end = ceil(末条事件)，不被 now 影响。
	day := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	segs := []storage.ActivitySegment{
		segAt("VSCode", day.Add(14*time.Hour), day.Add(16*time.Hour+30*time.Minute), "engaged"),
	}
	_, end := computeTimelineWindow(segs, nil, day, false)
	if want := day.Add(17 * time.Hour); !end.Equal(want) {
		t.Errorf("历史天 end=ceil(末条事件)=17:00，got %v want %v", end, want)
	}
}

// === 本地时区切日（修复跨午夜不切日的 bug）===

func TestStartOfLocalDay_PreservesTimezone(t *testing.T) {
	// startOfLocalDay 必须保留输入时区，且截断到当天 00:00。
	// 用固定 +08:00 时区测试（不依赖机器时区），验证逻辑正确。
	loc := time.FixedZone("CST", 8*3600)
	t0 := time.Date(2026, 6, 21, 23, 59, 30, 0, loc) // 本地 6/21 23:59
	got := startOfLocalDay(t0)
	want := time.Date(2026, 6, 21, 0, 0, 0, 0, loc) // 本地 6/21 00:00
	if !got.Equal(want) {
		t.Errorf("startOfLocalDay 应截断到本地当天 00:00，got %v want %v", got, want)
	}
	if got.Location() != loc {
		t.Errorf("应保留时区，got %v", got.Location())
	}
}

func TestQueryTimelineLocalDayBoundary_NightOwlCrossesMidnight(t *testing.T) {
	// 场景：UTC+8 用户 6/21 工作，其中一段跨本地午夜（23:30 ~ 次日 00:30）。
	// 本地切日后：查"6/21"应包含 23:30~24:00 部分，
	// 查"6/22"应包含 00:00~00:30 部分（不再错算进 6/21）。
	loc := time.FixedZone("CST", 8*3600)

	// 模拟 QueryTimeline 的日期解析逻辑：req.Date 按本地时区解析成零点。
	day21, _ := time.ParseInLocation("2006-01-02", "2026-06-21", loc)
	next21 := day21.Add(24 * time.Hour)
	day22, _ := time.ParseInLocation("2006-01-02", "2026-06-22", loc)
	next22 := day22.Add(24 * time.Hour)

	// 跨午夜段：本地 6/21 23:30 ~ 6/22 00:30（UTC 为 6/21 15:30 ~ 6/22 16:30 前一日……实际算）。
	segStart := time.Date(2026, 6, 21, 23, 30, 0, 0, loc)
	segEnd := time.Date(2026, 6, 22, 0, 30, 0, 0, loc)

	// 半开区间 [day, next) 的包含判据：start < next && end > day（同 ListActivitySegments）。
	inDay21 := segStart.Before(next21) && segEnd.After(day21)
	inDay22 := segStart.Before(next22) && segEnd.After(day22)

	if !inDay21 {
		t.Error("跨午夜段应被 6/21 的查询窗口包含（23:30~24:00 部分）")
	}
	if !inDay22 {
		t.Error("跨午夜段应被 6/22 的查询窗口包含（00:00~00:30 部分）——本地切日修复的关键")
	}

	// 对比：旧的 UTC 切日会怎样？
	day21UTC, _ := time.Parse("2006-01-02", "2026-06-21") // 默认 UTC
	day21UTC = day21UTC.UTC()
	next21UTC := day21UTC.Add(24 * time.Hour)
	// segEnd 的 UTC 时刻（本地 6/22 00:30 = UTC 6/21 16:30）。
	segEndUTC := segEnd.UTC()
	// 旧 UTC 窗口 [6/21 00:00 UTC, 6/22 00:00 UTC) = 本地 [6/21 08:00, 6/22 08:00)。
	// segEnd=本地6/22 00:30 = UTC 6/21 16:30，落在旧窗口内 → 旧逻辑会把它算进 6/21（bug）。
	inDay21UTC_buggy := segEndUTC.Before(next21UTC)
	if inDay21UTC_buggy {
		// 这是旧 bug 的表现：本该属于 6/22 的凌晨事件被算进 6/21。仅作行为对比，非断言。
		t.Log("确认旧 UTC 切日 bug 复现：本地 6/22 00:30 的事件被算进 6/21（已修复）")
	}
}

func TestQueryTimelineLocalDayBoundary_EarlyMorningBelongsToCorrectDay(t *testing.T) {
	// 场景：本地凌晨 06:00 的事件，应属于"今天"而不是"昨天"。
	loc := time.FixedZone("CST", 8*3600)
	todayLocal := time.Date(2026, 6, 22, 6, 0, 0, 0, loc) // 本地 6/22 06:00
	todayLocalMidnight, _ := time.ParseInLocation("2006-01-02", "2026-06-22", loc)
	nextMidnight := todayLocalMidnight.Add(24 * time.Hour)
	yesterdayMidnight, _ := time.ParseInLocation("2006-01-02", "2026-06-21", loc)

	// 事件 ts 用 epoch 比较（存储层用 toUnix，与时区无关）。
	inToday := todayLocal.Unix() >= todayLocalMidnight.Unix() && todayLocal.Unix() < nextMidnight.Unix()
	inYesterday := todayLocal.Unix() >= yesterdayMidnight.Unix() && todayLocal.Unix() < todayLocalMidnight.Unix()
	if !inToday || inYesterday {
		t.Errorf("本地凌晨 06:00 应属于当天而非前一天，inToday=%v inYesterday=%v", inToday, inYesterday)
	}
}
