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
	for _, seg := range segs {
		snapshot.Segments = append(snapshot.Segments, &TimelineSegment{
			StartTs:     seg.StartTS.Unix(),
			EndTs:       seg.EndTS.Unix(),
			AppName:     seg.AppName,
			Category:    seg.Category,
			WindowTitle: seg.WindowTitle,
			State:       seg.State,
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
