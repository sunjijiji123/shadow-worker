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
	db     *storage.DB
	coll   *collector.Collector
	vlm    *collector.VLMCapturer
}

// NewCollectionServer 创建 CollectionServer。
func NewCollectionServer(db *storage.DB, coll *collector.Collector, vlm *collector.VLMCapturer) *CollectionServer {
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
func (s *CollectionServer) QueryTimeline(ctx context.Context, req *TimelineRequest) (*TimelineSnapshot, error) {
	day, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return nil, fmt.Errorf("日期格式错误: %w", err)
	}
	day = day.UTC()
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
	return snapshot, nil
}

// aggregateSegments 把连续相同 app 的段合并。
// 输入需按 start_ts 升序（ListActivitySegments 已保证）。
// 合并规则：相邻且 AppName 相同 → 合为一段，start 取最早、end 取最晚、
// state/windowTitle/summary 取该组最后一条（最新状态）。
// 任何 app 切换（哪怕只切走一瞬）都形成断点，两边不合并。
func aggregateSegments(segs []storage.ActivitySegment) []storage.ActivitySegment {
	if len(segs) == 0 {
		return segs
	}
	out := make([]storage.ActivitySegment, 0, len(segs))
	cur := segs[0]
	for i := 1; i < len(segs); i++ {
		s := segs[i]
		if s.AppName == cur.AppName {
			// 同 app 连续：合并。end 取较晚者（防御性 max，正常情况 s 在 cur 之后）。
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

// TriggerVLM 手动触发一次 VLM 截图理解。
func (s *CollectionServer) TriggerVLM(ctx context.Context, req *TriggerVLMRequest) (*VLMSummary, error) {
	if s.vlm == nil {
		return nil, fmt.Errorf("VLM 未启用")
	}

	summary, err := s.vlm.Trigger(ctx)
	if err != nil {
		return nil, fmt.Errorf("VLM 触发失败: %w", err)
	}

	return &VLMSummary{Summary: summary}, nil
}
