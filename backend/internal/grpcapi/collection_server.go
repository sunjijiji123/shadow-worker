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

	segs, err := s.db.ActiveSegmentsByRange(day, next)
	if err != nil {
		return nil, fmt.Errorf("查询活动段失败: %w", err)
	}

	events, err := s.db.ListEvents(day, next)
	if err != nil {
		return nil, fmt.Errorf("查询事件失败: %w", err)
	}

	snapshot := &TimelineSnapshot{Date: req.Date}

	// segs 已由 ActiveSegmentsByRange 完成三步处理：
	//   1. ListActivitySegments（区间重叠查询）
	//   2. AggregateSegments（合并同 app 连续段，消除采集层碎片段）
	//   3. ClipSegmentsToRange（按 [day, next) 裁剪跨天段，只保留当天部分）
	// 这与概览页 RangeActiveMinutes 共用同一段列表生成逻辑，确保两边数字一致。
	aggregated := segs

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
	isToday := day.Equal(storage.StartOfLocalDay(nowLocal))
	wStart, wEnd := computeTimelineWindow(aggregated, events, day, isToday)
	snapshot.WindowStartTs = wStart.Unix()
	snapshot.WindowEndTs = wEnd.Unix()

	return snapshot, nil
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

// AnalyzeImage 分析用户框选并保存的截图文件（"快捷工具-桌面截图"）。
// 与 TriggerVLM 的区别：不重新截图，直接分析前端传来的 PNG 路径，
// 保证"用户看到什么"和"VLM 分析什么"完全一致。
func (s *CollectionServer) AnalyzeImage(ctx context.Context, req *AnalyzeImageRequest) (*VLMSummary, error) {
	if s.vlm == nil {
		return nil, fmt.Errorf("VLM 未启用")
	}
	cap := s.vlm.Get()
	if cap == nil {
		return nil, fmt.Errorf("VLM 未启用")
	}
	if req.Path == "" {
		return nil, fmt.Errorf("截图路径为空")
	}
	// req.Prompt 是桌面截图识别专用提示词（空=引擎回落默认）。
	summary, err := cap.DescribePath(ctx, req.Path, req.Prompt)
	if err != nil {
		return nil, fmt.Errorf("VLM 分析失败: %w", err)
	}
	return &VLMSummary{Summary: summary, ScreenshotPath: req.Path}, nil
}
