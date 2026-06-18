// Package grpcapi 实现 Qt 客户端 → Go 后台的 gRPC 服务。
package grpcapi

import (
	"context"
	"time"

	"shadow-worker/backend/internal/collector"
	"shadow-worker/backend/internal/storage"
)

// OverviewServer 实现 OverviewService。
type OverviewServer struct {
	UnimplementedOverviewServiceServer
	db   *storage.DB
	coll *collector.Collector
}

// NewOverviewServer 创建 OverviewService 实现。
func NewOverviewServer(db *storage.DB, coll *collector.Collector) *OverviewServer {
	return &OverviewServer{db: db, coll: coll}
}

// GetOverview 返回概览快照。
func (s *OverviewServer) GetOverview(ctx context.Context, req *GetOverviewRequest) (*OverviewData, error) {
	minutes, segments, err := s.db.TodayActivityMinutes()
	if err != nil {
		return nil, err
	}

	status := "stopped"
	if s.coll != nil {
		if s.coll.IsRunning() {
			status = "running"
		} else {
			status = "paused"
		}
	}

	return &OverviewData{
		TodayMinutes:     int32(minutes),
		ActiveSegments:   int32(segments),
		CollectionStatus: status,
		AsrStatus:        "ready",
		VlmStatus:        "ready",
		McpStatus:        "running",
		Apps:             nil,
	}, nil
}

// WatchOverview 持续推送状态变化。
func (s *OverviewServer) WatchOverview(req *WatchOverviewRequest, stream OverviewService_WatchOverviewServer) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-ticker.C:
		}

		minutes, _, err := s.db.TodayActivityMinutes()
		if err != nil {
			continue
		}

		status := "stopped"
		if s.coll != nil {
			if s.coll.IsRunning() {
				status = "running"
			} else {
				status = "paused"
			}
		}

		activeApp := ""
		app, err := collector.ForegroundApp()
		if err == nil {
			cat, _ := s.db.GetAppCategory(app.Path)
			if cat != nil {
				activeApp = app.Name
			}
		}

		if err := stream.Send(&OverviewUpdate{
			TodayMinutes:     int32(minutes),
			CollectionStatus: status,
			ActiveApp:        activeApp,
		}); err != nil {
			return err
		}
	}
}

// today 返回当前 UTC 日期 00:00:00。
func today() time.Time {
	return time.Now().UTC().Truncate(24 * time.Hour)
}

// dayRange 返回某天的 [start, end)。
func dayRange(day time.Time) (time.Time, time.Time) {
	day = day.UTC().Truncate(24 * time.Hour)
	return day, day.Add(24 * time.Hour)
}
