package collector

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"shadow-worker/backend/internal/config"
	"shadow-worker/backend/internal/storage"
	"shadow-worker/backend/internal/vlm"
)

// VLMCapturer 负责定时/按需截图并调用 VLM 生成摘要。
type VLMCapturer struct {
	cfg    config.VLMConfig
	engine vlm.Engine
	db     *storage.DB
	logger *slog.Logger
	stopCh chan struct{}
}

// EngineName 返回当前使用的 VLM 引擎名称。
func (v *VLMCapturer) EngineName() string {
	if v.engine == nil {
		return ""
	}
	return v.engine.Name()
}

// NewVLMCapturer 创建 VLM 截图理解器。
func NewVLMCapturer(cfg config.VLMConfig, db *storage.DB, logger *slog.Logger) (*VLMCapturer, error) {
	if cfg.Mode == "off" || cfg.Mode == "" {
		return nil, fmt.Errorf("VLM 已关闭")
	}
	if cfg.ScheduleIntervalMin <= 0 {
		cfg.ScheduleIntervalMin = 5
	}
	engine, err := vlm.New(cfg)
	if err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &VLMCapturer{
		cfg:    cfg,
		engine: engine,
		db:     db,
		logger: logger,
		stopCh: make(chan struct{}),
	}, nil
}

// Start 在后台启动定时截图(仅 scheduled 模式)。
func (v *VLMCapturer) Start() {
	if v.cfg.Mode != "scheduled" {
		return
	}
	go v.loop()
	v.logger.Info("VLM 定时截图已启动", "interval_min", v.cfg.ScheduleIntervalMin)
}

// Stop 停止定时截图。
func (v *VLMCapturer) Stop() {
	close(v.stopCh)
}

// Trigger 立即执行一次截图理解(按需模式也走这里)。
func (v *VLMCapturer) Trigger(ctx context.Context) (string, error) {
	var (
		png []byte
		app App
		err error
	)
	if v.cfg.CaptureRange == "screen" {
		// 整屏模式：截取虚拟屏（多显示器并集），用合成 app 信息标识。
		// 整屏不按白名单过滤——整屏本身是用户明确的全局采集意图，
		// 按前台 app 白名单过滤会导致永远采不到（整屏不属于单一应用）。
		png = CaptureScreenPNG()
		if png == nil {
			return "", fmt.Errorf("截图失败")
		}
		app = App{Name: "Screen", Path: ""}
	} else {
		// 活动窗口模式（默认）：前台窗口 + 白名单过滤。
		app, err = ForegroundApp()
		if err != nil {
			return "", fmt.Errorf("获取前台应用失败: %w", err)
		}
		// 白名单过滤：仅采集用户添加到采集列表的应用。
		// 不在白名单 → 静默跳过本次（不算错误），与 movement 采集的过滤口径一致。
		if !v.isWhitelisted(app.Path) {
			v.logger.Debug("前台应用不在白名单，跳过 VLM 截图", "app", app.Name, "path", app.Path)
			return "", nil
		}
		png = CaptureWindowPNG(app.HWND)
		if png == nil {
			return "", fmt.Errorf("截图失败")
		}
	}

	// debug 模式：把截图落盘到 screenshots/ 目录，便于排查"VLM 识别了什么内容"。
	// 默认关闭（不落盘），只在 config.yaml 设 debug.save_screenshots: true 时开启。
	var shotPath string
	if v.cfg.SaveScreenshots {
		if p, saveErr := saveScreenshot(png, app.Name, time.Now().UTC()); saveErr != nil {
			v.logger.Warn("debug 保存截图失败", "err", saveErr)
		} else {
			shotPath = p
		}
	}

	summary, err := v.engine.Describe(ctx, png)
	if err != nil {
		return "", fmt.Errorf("VLM 识别失败: %w", err)
	}

	_, err = v.db.InsertEvent(storage.Event{
		TS:             time.Now().UTC(),
		Type:           storage.EventTypeVLMSummary,
		AppPath:        app.Path,
		AppName:        app.Name,
		Content:        summary,
		ScreenshotPath: shotPath, // debug 模式才有值
	})
	if err != nil {
		v.logger.Warn("写入 VLM 事件失败", "err", err)
	}

	v.logger.Info("VLM 摘要已生成", "app", app.Name, "range", v.cfg.CaptureRange, "summary", summary)
	return summary, nil
}

// isWhitelisted 检查应用是否在采集白名单中（与 movement.go 的过滤口径一致）。
// 白名单存在 SQLite 的 app_categories 表，由前端"采集应用"设置页维护。
func (v *VLMCapturer) isWhitelisted(path string) bool {
	if v.db == nil || path == "" {
		return false
	}
	app, err := v.db.GetAppCategory(path)
	if err != nil {
		v.logger.Warn("查询白名单失败", "err", err)
		return false
	}
	return app != nil
}

func (v *VLMCapturer) loop() {
	ticker := time.NewTicker(time.Duration(v.cfg.ScheduleIntervalMin) * time.Minute)
	defer ticker.Stop()

	// 启动后立即执行一次
	v.runOnce()

	for {
		select {
		case <-v.stopCh:
			return
		case <-ticker.C:
			v.runOnce()
		}
	}
}

func (v *VLMCapturer) runOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if _, err := v.Trigger(ctx); err != nil {
		v.logger.Warn("VLM 定时任务失败", "err", err)
	}
}

// saveScreenshot 把 PNG 数据保存到 screenshots/<日期>/ 目录，返回绝对路径。
// 仅在 debug.save_screenshots=true 时调用。
func saveScreenshot(data []byte, appName string, t time.Time) (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dateDir := t.Format("2006-01-02")
	fileName := t.Format("150405") + "-" + appName + ".png"
	dir := filepath.Join(cfgDir, "shadow-worker", "screenshots", dateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fileName)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
