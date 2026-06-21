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

// GetOverview 返回概览快照(支持 day/week/month 范围 + 打断次数 + 历史对比)。
func (s *OverviewServer) GetOverview(ctx context.Context, req *GetOverviewRequest) (*OverviewData, error) {
	rng := req.Range
	if rng == "" {
		rng = "day"
	}

	var day time.Time
	if req.Date != "" {
		if parsed, err := time.Parse("2006-01-02", req.Date); err == nil {
			day = parsed
		}
	}

	// 当前范围边界
	start, end := storage.RangeBounds(day, rng)
	// 上一周期边界(算 delta)
	prevStart, prevEnd := storage.PreviousRangeBounds(day, rng)

	minutes, err := s.db.RangeActiveMinutes(start, end)
	if err != nil {
		return nil, err
	}
	segments, err := s.db.RangeActiveSegments(start, end)
	if err != nil {
		return nil, err
	}
	interrupts, err := s.db.InterruptCount(start, end)
	if err != nil {
		return nil, err
	}

	// 采集应用列表 + 涉及应用数：以白名单(app_categories)为基准，
	// 保证首页"采集应用"卡片与设置页白名单列表数量一致（没用过的应用也显示，时长 0）。
	apps, err := s.db.WhitelistAppsWithMinutes(start, end)
	if err != nil {
		return nil, err
	}
	appSummaries := make([]*AppSummary, 0, len(apps))
	for _, am := range apps {
		appSummaries = append(appSummaries, &AppSummary{
			Name:         am.Name,
			Category:     am.Category,
			TodayMinutes: int32(am.Minutes),
		})
	}

	// 上一周期数据(delta)
	prevMinutes, _ := s.db.RangeActiveMinutes(prevStart, prevEnd)
	prevInterrupts, _ := s.db.InterruptCount(prevStart, prevEnd)

	// 当前前台应用
	activeApp, activeCategory := s.currentActiveApp()

	status := s.collectionStatus()

	return &OverviewData{
		TodayMinutes:     int32(minutes),
		ActiveSegments:   int32(segments),
		Apps:             appSummaries,
		CollectionStatus: status,
		AsrStatus:        "ready",
		VlmStatus:        "ready",
		McpStatus:        "running",
		InterruptCount:   int32(interrupts),
		MinutesDelta:     int32(minutes - prevMinutes),
		InterruptDelta:   int32(interrupts - prevInterrupts),
		AppCount:         int32(len(apps)),
		ActiveApp:        activeApp,
		ActiveCategory:   activeCategory,
	}, nil
}

// GetHeatmap 返回活跃热力图(回溯 months_back 个月的每日活跃分钟 + 0~5 档)。
func (s *OverviewServer) GetHeatmap(ctx context.Context, req *HeatmapRequest) (*HeatmapData, error) {
	monthsBack := int(req.MonthsBack)
	if monthsBack <= 0 {
		monthsBack = 3
	}

	// 从 N 个月前的 1 号开始,到今天结束
	now := time.Now().UTC()
	end := now.AddDate(0, 0, 1).Truncate(24 * time.Hour) // 明天 0 点(含今天)
	start := now.AddDate(0, -monthsBack, 0)
	start = start.AddDate(0, 0, -(int(start.Day()) - 1)).Truncate(24 * time.Hour) // 回到 1 号

	daily, err := s.db.DailyMinutes(start, end)
	if err != nil {
		return nil, err
	}

	// 构建 date→minutes 索引,补齐范围内每一天(无数据天 level=0)
	idx := make(map[string]int, len(daily))
	for _, d := range daily {
		idx[d.Date] = d.Minutes
	}

	days := make([]*DayActivity, 0, int(end.Sub(start)/(24*time.Hour))+1)
	for d := start; d.Before(end); d = d.Add(24 * time.Hour) {
		m := idx[d.Format("2006-01-02")]
		days = append(days, &DayActivity{
			Date:    d.Format("2006-01-02"),
			Minutes: int32(m),
			Level:   int32(storage.MinutesToLevel(m)),
		})
	}

	return &HeatmapData{Days: days}, nil
}

// GetCategoryRank 返回类别占比排行(横条 + 占比% + 时长 + 类别色)。
func (s *OverviewServer) GetCategoryRank(ctx context.Context, req *RankRequest) (*CategoryRankData, error) {
	rng := req.Range
	if rng == "" {
		rng = "day"
	}

	var day time.Time
	if req.Date != "" {
		if parsed, err := time.Parse("2006-01-02", req.Date); err == nil {
			day = parsed
		}
	}

	start, end := storage.RangeBounds(day, rng)

	cats, err := s.db.CategoryAggregate(start, end)
	if err != nil {
		return nil, err
	}

	total := 0
	for _, c := range cats {
		total += c.Minutes
	}

	stats := make([]*CategoryStat, 0, len(cats))
	for _, c := range cats {
		var pct int
		if total > 0 {
			pct = (c.Minutes * 100) / total
		}
		stats = append(stats, &CategoryStat{
			Category: c.Category,
			Minutes:  int32(c.Minutes),
			Percent:  int32(pct),
			Color:    storage.CategoryColor(c.Category),
		})
	}

	return &CategoryRankData{
		Range:        rng,
		TotalMinutes: int32(total),
		Categories:   stats,
	}, nil
}

// collectionStatus 返回当前采集状态字符串。
func (s *OverviewServer) collectionStatus() string {
	if s.coll == nil {
		return "stopped"
	}
	if s.coll.IsRunning() {
		return "running"
	}
	return "paused"
}

// currentActiveApp 返回概览页"当前应用"（名 + 类别）。
//
// 语义：优先返回 collector 正在记录的白名单应用（coll.CurrentApp），而非瞬时
// 前台窗口。当用户切到非白名单应用（如本客户端自身、系统设置）时，collector
// 的 curApp 仍保留上一个白名单应用，避免"看一眼概览就显示空白"。
// collector 没有当前应用（未启动/首次未采集）时，回退到瞬时 ForegroundApp。
func (s *OverviewServer) currentActiveApp() (string, string) {
	if s.coll != nil {
		if name, cat, _, ok := s.coll.CurrentApp(); ok {
			return name, cat
		}
	}
	app, err := collector.ForegroundApp()
	if err != nil {
		return "", ""
	}
	cat, _ := s.db.GetAppCategory(app.Path)
	if cat == nil {
		return "", ""
	}
	return app.Name, cat.Category
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

		start, end := storage.RangeBounds(time.Time{}, "day")
		minutes, err := s.db.RangeActiveMinutes(start, end)
		if err != nil {
			continue
		}

		status := s.collectionStatus()
		activeApp, _ := s.currentActiveApp()

		if err := stream.Send(&OverviewUpdate{
			TodayMinutes:     int32(minutes),
			CollectionStatus: status,
			ActiveApp:        activeApp,
		}); err != nil {
			return err
		}
	}
}
