package grpcapi

import (
	"context"
	"fmt"
	"time"

	"shadow-worker/backend/internal/collector"
	"shadow-worker/backend/internal/storage"
)

// CollectionServer 实现 CollectionService。
type CollectionServer struct {
	UnimplementedCollectionServiceServer
	db   *storage.DB
	coll *collector.Collector
	// vlm 是 VLMHolder（不再是固定指针），TriggerVLM 动态 Get 当前 capturer，
	// 保证热重载后用上新实例。Get 返回 nil 表示 VLM 未启用。
	vlm *collector.VLMHolder
}

// NewCollectionServer 创建 CollectionServer。
func NewCollectionServer(db *storage.DB, coll *collector.Collector, vlm *collector.VLMHolder) *CollectionServer {
	return &CollectionServer{db: db, coll: coll, vlm: vlm}
}

// Pause 暂停采集。
func (s *CollectionServer) Pause(ctx context.Context, req *PauseRequest) (*Result, error) {
	if s.coll != nil {
		s.coll.Pause()
	}
	return &Result{Ok: true}, nil
}

// Resume 恢复采集。
func (s *CollectionServer) Resume(ctx context.Context, req *ResumeRequest) (*Result, error) {
	if s.coll != nil {
		s.coll.Resume()
	}
	return &Result{Ok: true}, nil
}

// GetStatus 查询采集状态。
func (s *CollectionServer) GetStatus(ctx context.Context, req *GetStatusRequest) (*CollectionStatus, error) {
	minutes, segments, err := s.db.TodayActivityMinutes()
	if err != nil {
		return nil, fmt.Errorf("查询今日统计失败: %w", err)
	}

	status := &CollectionStatus{
		Running:        s.coll != nil && s.coll.IsRunning(),
		TodayMinutes:   int32(minutes),
		ActiveSegments: int32(segments),
	}

	app, err := collector.ForegroundApp()
	if err == nil {
		cat, _ := s.db.GetAppCategory(app.Path)
		if cat != nil {
			status.ActiveApp = app.Name
			status.ActiveCategory = cat.Category
		}
	}
	return status, nil
}

// QueryTimeline 查询指定日期的时间线。
//
// 切日按"客户端本地时区"：req.Date（"2026-06-21"）解析为本地 00:00，
// 窗口 = [本地 06-21 00:00, 本地 06-22 00:00)。这样 UI 上的"日期"与实际
// 本地作息一致——跨本地午夜（熬夜到次日凌晨）的事件正确归属到第二天。
//
// 历史 bug：曾用 time.Parse（默认 UTC），窗口实为 [本地 08:00, 次日 08:00)，
// 导致 UTC+8 下凌晨 0-8 点的事件被错算进前一天，且晚上熬夜的工作跨午夜不切日。
// 修复前提：后端与客户端单机同部署（时区一致），用 time.Local 即客户端时区。
func (s *CollectionServer) QueryTimeline(ctx context.Context, req *TimelineRequest) (*TimelineSnapshot, error) {
	day, err := time.ParseInLocation("2006-01-02", req.Date, time.Local)
	if err != nil {
		return nil, fmt.Errorf("日期格式错误: %w", err)
	}
	next := day.Add(24 * time.Hour)

	segs, err := s.db.ListActivitySegments(day, next)
	if err != nil {
		return nil, fmt.Errorf("查询活动段失败: %w", err)
	}

	events, err := s.db.ListEvents(day, next)
	if err != nil {
		return nil, fmt.Errorf("查询事件失败: %w", err)
	}

	snapshot := &TimelineSnapshot{Date: req.Date}

	// 聚合：把连续相同 app 的细粒度段合并成一条。
	// 历史数据（采集层修复前）有大量同 app 的 engaged/active/idle 碎片段，
	// 合并后每个应用是一段连续记录，符合 worklog 的用户语义。
	// 聚合判据：相邻且 app_name 相同（任何 app 切换都打断，idle 不打断）。
	aggregated := aggregateSegments(segs)
	// 按本地午夜裁剪：横跨数天的段（如离开检测失效产生的 46h 巨怪段，见坑 #45/#49）
	// 只保留当天内部分，避免其端点把 computeTimelineWindow 撑爆、扭曲全天比例。
	aggregated = clipSegmentsToDay(aggregated, day, next)

	for _, seg := range aggregated {
		// 惰性回填：若聚合段尚无摘要，取该时间窗内最后一条 vlm_summary 事件。
		// 聚合段无单一 DB ID，回填只更新返回值，不落库（每次查询重算，开销可接受）。
		if seg.Summary == "" {
			if sum, err := s.db.LatestVLMSummary(seg.StartTS, seg.EndTS); err == nil && sum != "" {
				seg.Summary = sum
			}
		}
		snapshot.Segments = append(snapshot.Segments, &TimelineSegment{
			StartTs:     seg.StartTS.Unix(),
			EndTs:       seg.EndTS.Unix(),
			AppName:     seg.AppName,
			Category:    seg.Category,
			WindowTitle: seg.WindowTitle,
			State:       seg.State,
			Summary:     seg.Summary,
		})
	}
	for _, ev := range events {
		snapshot.Events = append(snapshot.Events, &TimelineEvent{
			Ts:      ev.TS.Unix(),
			Type:    string(ev.Type),
			Text:    ev.Content,
			AppName: ev.AppName,
		})
	}

	// 计算时间轴可视窗口并填入 snapshot，供前端画动态整点刻度。
	// isToday：今天 end 含 now（今天还在进行中）；历史天 end=末条事件。
	// 注：day 已是本地时区零点（见 QueryTimeline），now 也按本地时区取整比较。
	nowLocal := time.Now()
	isToday := day.Equal(startOfLocalDay(nowLocal))
	wStart, wEnd := computeTimelineWindow(aggregated, events, day, isToday)
	snapshot.WindowStartTs = wStart.Unix()
	snapshot.WindowEndTs = wEnd.Unix()

	return snapshot, nil
}

// aggregateSegments 把连续相同 app 的段合并。
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
func aggregateSegments(segs []storage.ActivitySegment) []storage.ActivitySegment {
	if len(segs) == 0 {
		return segs
	}
	out := make([]storage.ActivitySegment, 0, len(segs))
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

// clipSegmentsToDay 把段按本地午夜虚拟切分到目标日内，仅改返回值不落库。
//
// 背景：ListActivitySegments 用区间重叠判据（start_ts<dayEnd AND end_ts>dayStart），
// 一条横跨数天的段（如 id=518：6-22 23:37→6-24 22:19 的 46h 巨怪段）会完整命中
// 每一天。若直接交给 computeTimelineWindow，其端点会把可视窗口撑到 ~48h（见坑 #49），
// 当天真实时刻在窗口里只占一小段、刻度被压成 2h 步进，视觉上像"空白无记录"。
//
// 本函数对每条段取其与 [dayStart, dayEnd) 的交集：
//   - 完全在当天内：原样保留。
//   - 跨越当天边界：start/end clamp 到 [dayStart, dayEnd)，只保留当天内部分。
//   - 完全在当天外：丢弃（区间重叠查询理论上不会返回这类，防御性丢弃）。
//
// clamp 只动 StartTS/EndTS，AppName/State/WindowTitle/Summary 等沿用原段。
// 跨天段在 22/23/24 各天的查询里各自只看到属于自己的那一天部分，显示与统计都正确。
// 配套 storage.TodayActivityMinutes 的 SQL 已用相同 clamp 语义做按天统计（不重复计跨天段）。
func clipSegmentsToDay(segs []storage.ActivitySegment, dayStart, dayEnd time.Time) []storage.ActivitySegment {
	if len(segs) == 0 {
		return segs
	}
	out := make([]storage.ActivitySegment, 0, len(segs))
	for _, s := range segs {
		// 与当天无交集（完全在当天之前或之后）——区间重叠查询不应返回，防御性跳过。
		if !s.EndTS.After(dayStart) || !s.StartTS.Before(dayEnd) {
			continue
		}
		if s.StartTS.Before(dayStart) {
			s.StartTS = dayStart
		}
		if s.EndTS.After(dayEnd) {
			s.EndTS = dayEnd
		}
		// clamp 后若起止重合（边界恰好相切），不产生零宽段。
		if !s.StartTS.Before(s.EndTS) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// computeTimelineWindow 计算时间轴的可视窗口 [start, end]（整点）。
//
// 规则：
//   - start = floor(首条事件 整点)；end = ceil(末条事件 整点)。
//   - 首末事件取 segments(含 idle) 与 events 的并集 min(start)/max(end)。
//   - 今天：end = max(end, ceil(now))——今天还没结束，窗口要含当前时刻。
//   - minWindow=2h：窗口不足时向后 pad（避免短活动把刻度挤成密集串）。
//   - 空天：返回 day 当天的 09:00~18:00 作为 fallback 占位（前端照常渲染）。
//
// day 由调用方按本地时区零点传入（见 QueryTimeline）。整点取整用 Truncate(time.Hour)，
// 对 UTC+8 等整时区偏移，本地刻度也落在整点。
func computeTimelineWindow(segs []storage.ActivitySegment, events []storage.Event, day time.Time, isToday bool) (time.Time, time.Time) {
	var minT, maxT time.Time
	for i := range segs {
		if minT.IsZero() || segs[i].StartTS.Before(minT) {
			minT = segs[i].StartTS
		}
		if maxT.IsZero() || segs[i].EndTS.After(maxT) {
			maxT = segs[i].EndTS
		}
	}
	for i := range events {
		if minT.IsZero() || events[i].TS.Before(minT) {
			minT = events[i].TS
		}
		if maxT.IsZero() || events[i].TS.After(maxT) {
			maxT = events[i].TS
		}
	}

	// 空天 fallback：day 当天 09:00~18:00（day 已是本地零点，故 9h/18h 是本地时刻）。
	if minT.IsZero() {
		return day.Add(9 * time.Hour), day.Add(18 * time.Hour)
	}

	if isToday {
		// 今天还没结束，窗口 end 至少含当前时刻。
		if now := time.Now(); now.After(maxT) {
			maxT = now
		}
	}

	// 整点取整：start 向下 floor，end 向上 ceil。
	start := minT.Truncate(time.Hour)
	end := maxT
	if f := end.Truncate(time.Hour); !f.Equal(end) {
		end = f.Add(time.Hour)
	}

	// minWindow：窗口不足 2h 时向后 pad（防止首末事件很近时刻度密集成串）。
	if end.Sub(start) < 2*time.Hour {
		end = start.Add(2 * time.Hour)
	}
	return start, end
}

// startOfLocalDay 返回 t 所在本地日的 00:00（本地时区）。
// 用于 isToday 判断：day（QueryTimeline 解析的本地零点）与今天的本地零点比较。
func startOfLocalDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// TriggerVLM 手动触发一次 VLM 截图理解。
func (s *CollectionServer) TriggerVLM(ctx context.Context, req *TriggerVLMRequest) (*VLMSummary, error) {
	if s.vlm == nil {
		return nil, fmt.Errorf("VLM 未启用")
	}
	cap := s.vlm.Get()
	if cap == nil {
		// holder 存在但当前 capturer 为 nil：VLM 关闭 / screen+on_demand 降级 / 热重载中。
		return nil, fmt.Errorf("VLM 未启用")
	}

	summary, err := cap.Trigger(ctx)
	if err != nil {
		return nil, fmt.Errorf("VLM 触发失败: %w", err)
	}

	return &VLMSummary{Summary: summary}, nil
}
