package grpcapi

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"shadow-worker/backend/internal/collector"
	"shadow-worker/backend/internal/storage"
)

// WhitelistServer 实现 WhitelistService 的 gRPC 服务。
type WhitelistServer struct {
	UnimplementedWhitelistServiceServer
	db *storage.DB
}

// NewWhitelistServer 创建 WhitelistServer 实例。
func NewWhitelistServer(db *storage.DB) *WhitelistServer {
	return &WhitelistServer{db: db}
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
	log.Printf("[whitelist] ListWindows 返回 %d 个可见窗口", len(apps))

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

func fileName(path string) string {
	return filepath.Base(path)
}
