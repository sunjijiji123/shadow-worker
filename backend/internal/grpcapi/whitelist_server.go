package grpcapi

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"shadow-worker/backend/internal/collector"
	"shadow-worker/backend/internal/storage"
	"shadow-worker/backend/internal/winapi"
)

// WhitelistServer 实现 WhitelistService 的 gRPC 服务。
type WhitelistServer struct {
	UnimplementedWhitelistServiceServer
	db     *storage.DB
	logger *slog.Logger
}

// NewWhitelistServer 创建 WhitelistServer 实例。logger 为 nil 时回退到 slog.Default()。
func NewWhitelistServer(db *storage.DB, logger *slog.Logger) *WhitelistServer {
	if logger == nil {
		logger = slog.Default()
	}
	return &WhitelistServer{db: db, logger: logger}
}

// List 列出所有白名单应用及今日时长。
func (s *WhitelistServer) List(ctx context.Context, req *ListAppsRequest) (*AppList, error) {
	apps, err := s.db.ListAppCategories()
	if err != nil {
		return nil, fmt.Errorf("列出白名单失败: %w", err)
	}

	start, end := storage.RangeBounds(time.Time{}, "day")
	segs, err := s.db.ListActivitySegments(start, end)
	if err != nil {
		return nil, fmt.Errorf("查询活动段失败: %w", err)
	}

	minutesByApp := make(map[string]int32)
	for _, seg := range segs {
		if seg.State != "active" {
			continue
		}
		m := int32(seg.EndTS.Sub(seg.StartTS).Minutes())
		if m < 1 {
			m = 1
		}
		minutesByApp[seg.AppPath] += m
	}

	out := &AppList{}
	for _, app := range apps {
		out.Apps = append(out.Apps, &App{
			Path:         app.Path,
			Name:         app.Name,
			Category:     app.Category,
			IconPath:     app.IconPath,
			TodayMinutes: minutesByApp[app.Path],
			AddedAt:      app.AddedAt.Unix(),
		})
	}
	return out, nil
}

// Add 添加应用进白名单。
func (s *WhitelistServer) Add(ctx context.Context, req *AddAppRequest) (*App, error) {
	if req.Path == "" || req.Name == "" {
		return nil, fmt.Errorf("path 和 name 不能为空")
	}
	category := req.Category
	if category == "" {
		category = "other"
	}

	app := storage.AppCategory{
		Path:     req.Path,
		Name:     req.Name,
		Category: category,
	}
	if err := s.db.AddAppCategory(app); err != nil {
		return nil, err
	}

	return &App{
		Path:     app.Path,
		Name:     app.Name,
		Category: app.Category,
		AddedAt:  app.AddedAt.Unix(),
	}, nil
}

// Remove 从白名单移除应用。
func (s *WhitelistServer) Remove(ctx context.Context, req *RemoveAppRequest) (*Result, error) {
	if req.Path == "" {
		return &Result{Ok: false, Error: "path 不能为空"}, nil
	}
	if err := s.db.RemoveAppCategory(req.Path); err != nil {
		return &Result{Ok: false, Error: err.Error()}, nil
	}
	return &Result{Ok: true}, nil
}

// UpdateCategory 修改应用类别。
func (s *WhitelistServer) UpdateCategory(ctx context.Context, req *UpdateCategoryRequest) (*App, error) {
	if req.Path == "" || req.Category == "" {
		return nil, fmt.Errorf("path 和 category 不能为空")
	}
	app, err := s.db.GetAppCategory(req.Path)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, fmt.Errorf("应用不存在: %s", req.Path)
	}

	if err := s.db.UpdateAppCategory(req.Path, app.Name, req.Category, app.IconPath); err != nil {
		return nil, err
	}
	return &App{
		Path:     app.Path,
		Name:     app.Name,
		Category: req.Category,
		IconPath: app.IconPath,
		AddedAt:  app.AddedAt.Unix(),
	}, nil
}

// ListWindows 列出当前所有可见顶层窗口，供客户端"添加采集应用"时选择。
// 复用 collector.VisibleWindows（EnumWindows + IsWindowVisible 过滤）。
func (s *WhitelistServer) ListWindows(ctx context.Context, req *ListWindowsRequest) (*WindowList, error) {
	apps := collector.VisibleWindows()
	// 高频事件（每次客户端打开添加应用弹窗都调用）：降为 Debug。
	s.logger.Debug("列举窗口", "count", len(apps))

	out := &WindowList{Windows: make([]*WindowInfo, 0, len(apps))}
	for _, app := range apps {
		out.Windows = append(out.Windows, &WindowInfo{
			Hwnd:  int64(app.HWND),
			Path:  app.Path,
			Name:  app.Name,
			Title: app.WindowTitle,
		})
	}
	return out, nil
}

// GetWindowThumbnail 截取指定窗口的 320×180 PNG 缩略图，供"添加采集应用"
// 网格预览懒加载。失败（窗口已关/hwnd 失效）返回空 png，不报 error——
// 前端 image provider 据此降级为首字母占位图。
func (s *WhitelistServer) GetWindowThumbnail(ctx context.Context, req *ThumbnailRequest) (*ThumbnailData, error) {
	png := collector.CaptureWindowThumbnail(winapi.HWND(req.Hwnd))
	s.logger.Debug("取窗口缩略图", "hwnd", req.Hwnd, "bytes", len(png))
	return &ThumbnailData{Png: png}, nil
}

func fileName(path string) string {
	return filepath.Base(path)
}
