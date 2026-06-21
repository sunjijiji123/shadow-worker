package collector

import (
	"log/slog"
	"sync"
	"time"

	"shadow-worker/backend/internal/storage"
)

// PrecisionConfig 定义采集精度档位。
type PrecisionConfig struct {
	Thresh       uint8
	ChangeRatio  float64
	SampleMs     int
	IdleTimeoutS int
}

// Presets 是预定义的精度档位。
var Presets = map[string]PrecisionConfig{
	"low":    {Thresh: 50, ChangeRatio: 0.005, SampleMs: 500, IdleTimeoutS: 15},
	"medium": {Thresh: 30, ChangeRatio: 0.002, SampleMs: 300, IdleTimeoutS: 10},
	"high":   {Thresh: 15, ChangeRatio: 0.001, SampleMs: 200, IdleTimeoutS: 5},
}

// Collector 是行为采集引擎。
type Collector struct {
	db       *storage.DB
	cfg      PrecisionConfig
	logger   *slog.Logger
	running  bool
	mu       sync.RWMutex
	stopCh   chan struct{}
	pauseCh  chan struct{}
	resumeCh chan struct{}

	// 当前段状态。curApp/curActive/curCategory 主要由 loop goroutine 写，
	// 但 CurrentApp() 从 gRPC handler goroutine 读，故用 c.mu 保护读写。
	curSegID    int64
	curApp      App
	curActive   bool
	curCategory string
}

// NewCollector 创建采集引擎。
func NewCollector(db *storage.DB, precision string, logger *slog.Logger) *Collector {
	if logger == nil {
		logger = slog.Default()
	}
	cfg, ok := Presets[precision]
	if !ok {
		cfg = Presets["medium"]
	}
	return &Collector{
		db:       db,
		cfg:      cfg,
		logger:   logger,
		stopCh:   make(chan struct{}),
		pauseCh:  make(chan struct{}),
		resumeCh: make(chan struct{}),
	}
}

// Start 在后台启动采集循环。
func (c *Collector) Start() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		return
	}
	c.running = true
	go c.loop()
	c.logger.Info("collector 已启动")
}

// Stop 停止采集循环。
func (c *Collector) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running {
		return
	}
	close(c.stopCh)
	c.running = false
	c.logger.Info("collector 已停止")
}

// Pause 暂停采集。
func (c *Collector) Pause() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running {
		return
	}
	select {
	case c.pauseCh <- struct{}{}:
	default:
	}
}

// Resume 恢复采集。
func (c *Collector) Resume() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running {
		return
	}
	select {
	case c.resumeCh <- struct{}{}:
	default:
	}
}

// IsRunning 返回采集是否在运行。
func (c *Collector) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

// CurrentApp 返回 collector 当前正在记录的白名单应用（名 + 类别 + 是否活跃）。
// 注意：curApp 由 loop goroutine 写、本方法读，用 c.mu 保护。
//
// 语义：这是"真正正在被采集的应用"，而非瞬时前台窗口。当用户切到非白名单
// 应用（如本客户端自身、系统设置）时，curApp 仍保留上一个白名单应用，
// 直到 idle 超时或切到另一个白名单应用。供概览页"当前应用"显示，避免
// 用户看一眼概览就显示空白。
func (c *Collector) CurrentApp() (name string, category string, active bool, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.running || c.curApp.Path == "" {
		return "", "", false, false
	}
	// curCategory 在 updateSegment 里从白名单查得后缓存。
	return c.curApp.Name, c.curCategory, c.curActive, true
}

// loop 是采集主循环。
func (c *Collector) loop() {
	ticker := time.NewTicker(time.Duration(c.cfg.SampleMs) * time.Millisecond)
	defer ticker.Stop()

	var prevFrame []byte
	var lastChange time.Time
	idleTimeout := time.Duration(c.cfg.IdleTimeoutS) * time.Second

	for {
		select {
		case <-c.stopCh:
			return
		case <-c.pauseCh:
			c.logger.Info("collector 已暂停")
			<-c.resumeCh
			c.logger.Info("collector 已恢复")
			continue
		case <-ticker.C:
		}

		app, err := ForegroundApp()
		if err != nil {
			c.logger.Debug("获取前台应用失败", "err", err)
			continue
		}

		if !c.isWhitelisted(app.Path) {
			continue
		}

		curr := CaptureWindow(app.HWND)
		if curr == nil {
			c.logger.Debug("截图失败", "app", app.Name)
			continue
		}

		active := true
		if prevFrame != nil {
			ratio, err := FrameDiff(prevFrame, curr, CaptureTargetWidth, CaptureTargetHeight)
			if err != nil {
				c.logger.Warn("帧差计算失败", "err", err)
			} else if ratio > c.cfg.ChangeRatio {
				lastChange = time.Now().UTC()
			}
			active = time.Since(lastChange) < idleTimeout
		} else {
			// 第一帧默认认为有变化
			lastChange = time.Now().UTC()
		}
		prevFrame = curr

		c.updateSegment(app, active)
	}
}

// isWhitelisted 检查应用是否在白名单中。
func (c *Collector) isWhitelisted(path string) bool {
	app, err := c.db.GetAppCategory(path)
	if err != nil {
		c.logger.Warn("查询白名单失败", "err", err)
		return false
	}
	return app != nil
}

// updateSegment 根据活跃状态维护 activity_segments 表。
func (c *Collector) updateSegment(app App, active bool) {
	now := time.Now().UTC()

	// 应用或 active/idle 状态变化时，结束当前段并开启新段
	if c.curSegID == 0 || c.curApp.Path != app.Path || c.curActive != active {
		if c.curSegID != 0 {
			if err := c.db.UpdateActivitySegmentEndTS(c.curSegID, now); err != nil {
				c.logger.Warn("结束活动段失败", "err", err)
			}
		}

		cat, err := c.db.GetAppCategory(app.Path)
		if err != nil {
			c.logger.Warn("获取应用分类失败", "err", err)
			return
		}
		category := "other"
		if cat != nil {
			category = cat.Category
		}

		id, err := c.db.InsertActivitySegment(storage.ActivitySegment{
			StartTS:     now,
			EndTS:       now,
			AppPath:     app.Path,
			AppName:     app.Name,
			Category:    category,
			WindowTitle: app.WindowTitle,
			State:       stateString(active),
		})
		if err != nil {
			c.logger.Warn("插入活动段失败", "err", err)
			return
		}
		c.curSegID = id
		c.curApp = app
		c.curCategory = category
		c.curActive = active
		return
	}

	// 同一段内只更新结束时间
	if err := c.db.UpdateActivitySegmentEndTS(c.curSegID, now); err != nil {
		c.logger.Warn("更新活动段结束时间失败", "err", err)
	}
}

func stateString(active bool) string {
	if active {
		return "active"
	}
	return "idle"
}
