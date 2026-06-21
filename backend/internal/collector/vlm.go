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
		png = CaptureScreenPNG()
		if png == nil {
			return "", fmt.Errorf("截图失败")
		}
		app = App{Name: "Screen", Path: ""}
	} else {
		// 活动窗口模式（默认）：前台窗口 + 白名单。
		app, err = ForegroundApp()
		if err != nil {
			return "", fmt.Errorf("获取前台应用失败: %w", err)
		}
		png = CaptureWindowPNG(app.HWND)
		if png == nil {
			return "", fmt.Errorf("截图失败")
		}
	}

	path, err := saveScreenshot(png, app.Name, time.Now().UTC())
	if err != nil {
		v.logger.Warn("保存截图失败", "err", err)
		path = ""
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
		ScreenshotPath: path,
	})
	if err != nil {
		v.logger.Warn("写入 VLM 事件失败", "err", err)
	}

	v.logger.Info("VLM 摘要已生成", "app", app.Name, "range", v.cfg.CaptureRange, "summary", summary)
	return summary, nil
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

// saveScreenshot 把 PNG 数据保存到标准截图目录,返回相对/绝对路径。
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
