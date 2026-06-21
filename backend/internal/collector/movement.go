package collector

import (
	"log/slog"
	"sync"
	"time"

	"shadow-worker/backend/internal/config"
	"shadow-worker/backend/internal/storage"
	"shadow-worker/backend/internal/winapi"
)

// PrecisionConfig 定义采集精度档位。
//
// 超时分层(取代旧的单层 IdleTimeoutS):
//   - InputIdleS: 输入超时。GetLastInputInfo 显示近该秒数内有键鼠输入
//     → 判为强信号(engaged)。默认短,反映"人正在动"。
//   - DisplayIdleS: 展示超时。距上次 engaged 超过该秒数 → 从 active 退化为 idle。
//     默认长,给静读/思考留宽限期(旧值 10s 是漏记主因)。
type PrecisionConfig struct {
	Thresh       uint8
	ChangeRatio  float64
	SampleMs     int
	InputIdleS   int
	DisplayIdleS int
}

// Presets 是预定义的精度档位。
var Presets = map[string]PrecisionConfig{
	"low":    {Thresh: 50, ChangeRatio: 0.005, SampleMs: 500, InputIdleS: 20, DisplayIdleS: 120},
	"medium": {Thresh: 30, ChangeRatio: 0.002, SampleMs: 300, InputIdleS: 15, DisplayIdleS: 90},
	"high":   {Thresh: 15, ChangeRatio: 0.001, SampleMs: 200, InputIdleS: 10, DisplayIdleS: 60},
}

// 三态取值常量。写入 activity_segments.state。
const (
	StateEngaged = "engaged" // 强活跃:本 tick 有真实交互信号
	StateActive  = "active"  // 余热:近期 engaged 的宽限期
	StateIdle    = "idle"    // 静默:超过展示超时仍无交互
)

// Collector 是行为采集引擎。
type Collector struct {
	db     *storage.DB
	cfg    PrecisionConfig
	logger *slog.Logger

	running  bool
	mu       sync.RWMutex
	stopCh   chan struct{}
	pauseCh  chan struct{}
	resumeCh chan struct{}

	// 当前段状态。curApp/curState/curCategory 主要由 loop goroutine 写,
	// 但 CurrentApp() 从 gRPC handler goroutine 读,故用 c.mu 保护读写。
	curSegID    int64
	curApp      App
	curState    string // engaged / active / idle
	curCategory string
}

// NewCollector 创建采集引擎。
//
// 接收完整 MovementConfig(而非仅 precision 字符串):按 Precision 选 Preset 作基线,
// 再用 config 的 SampleIntervalMs/InputIdleS/DisplayIdleS(非 0 时)覆盖 Preset。
// 这样 YAML 不填字段也能用 Preset 默认值,填了即可自定义。
func NewCollector(db *storage.DB, mc config.MovementConfig, logger *slog.Logger) *Collector {
	if logger == nil {
		logger = slog.Default()
	}
	cfg, ok := Presets[mc.Precision]
	if !ok {
		cfg = Presets["medium"]
	}
	// config 字段非 0 则覆盖 Preset 基线。
	if mc.SampleIntervalMs > 0 {
		cfg.SampleMs = mc.SampleIntervalMs
	}
	if mc.InputIdleS > 0 {
		cfg.InputIdleS = mc.InputIdleS
	}
	if mc.DisplayIdleS > 0 {
		cfg.DisplayIdleS = mc.DisplayIdleS
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
	c.logger.Info("collector 已启动",
		"sampleMs", c.cfg.SampleMs, "inputIdleS", c.cfg.InputIdleS, "displayIdleS", c.cfg.DisplayIdleS)
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

// CurrentApp 返回 collector 当前正在记录的白名单应用(名 + 类别 + 是否活跃)。
// 注意：curApp 由 loop goroutine 写、本方法读，用 c.mu 保护。
//
// 语义：这是"真正正在被采集的应用"，而非瞬时前台窗口。当用户切到非白名单
// 应用（如本客户端自身、系统设置）时，curApp 仍保留上一个白名单应用，
// 直到 idle 超时或切到另一个白名单应用。供概览页"当前应用"显示，避免
// 用户看一眼概览就显示空白。
//
// active 的定义：engaged 和 active 都算 active（即非 idle），对前端语义不变。
func (c *Collector) CurrentApp() (name string, category string, active bool, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.running || c.curApp.Path == "" {
		return "", "", false, false
	}
	// curCategory 在 updateSegment 里从白名单查得后缓存。
	return c.curApp.Name, c.curCategory, c.curState != StateIdle, true
}

// inferState 是三态判定的纯函数，便于单测。
//
// 参数：
//   - strongSignal: 本 tick 的强信号(帧差/输入/标题变化任一为真)
//   - sinceEngaged: 距上次 engaged 的时间
//   - displayIdle:  展示超时阈值
//
// 返回新的 state。strongSignal 为真 → engaged；否则按 sinceEngaged 与 displayIdle
// 比较：在宽限期内 → active，超出 → idle。
func inferState(strongSignal bool, sinceEngaged time.Duration, displayIdle time.Duration) string {
	if strongSignal {
		return StateEngaged
	}
	if sinceEngaged < displayIdle {
		return StateActive
	}
	return StateIdle
}

// loop 是采集主循环。
func (c *Collector) loop() {
	ticker := time.NewTicker(time.Duration(c.cfg.SampleMs) * time.Millisecond)
	defer ticker.Stop()

	var prevFrame []byte
	var prevTitle string
	var lastEngaged time.Time
	inputIdle := time.Duration(c.cfg.InputIdleS) * time.Second
	displayIdle := time.Duration(c.cfg.DisplayIdleS) * time.Second

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

		now := time.Now().UTC()

		// ---- 三路强信号 ----
		strong := false

		// s1: 帧差(屏幕在变)
		curr := CaptureWindow(app.HWND)
		if curr == nil {
			c.logger.Debug("截图失败", "app", app.Name)
			// 截图失败不阻断其它信号判定，但帧差缺失。
		} else if prevFrame != nil {
			ratio, err := FrameDiff(prevFrame, curr, CaptureTargetWidth, CaptureTargetHeight)
			if err != nil {
				c.logger.Warn("帧差计算失败", "err", err)
			} else if ratio > c.cfg.ChangeRatio {
				strong = true
			}
			prevFrame = curr
		} else {
			// 第一帧:无前帧可比,默认认为有变化(沿用旧行为)。
			strong = true
			prevFrame = curr
		}

		// s2: GetLastInputInfo(系统级键鼠输入)。前台=白名单时,
		// 系统输入几乎必然进了该前台应用,故可作为"该应用在用"的强信号。
		if !strong {
			if tick, ok := winapi.LastInputTick(); ok {
				idleMs := winapi.GetTickCount64() - uint64(tick)
				if idleMs < uint64(inputIdle/time.Millisecond) {
					strong = true
				}
			}
		}

		// s3: 窗口标题变化(切标签/导航/文件)。
		if !strong && app.WindowTitle != prevTitle {
			strong = true
		}
		prevTitle = app.WindowTitle

		if strong {
			lastEngaged = now
		}

		state := inferState(strong, time.Since(lastEngaged), displayIdle)
		c.updateSegment(app, state)
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
func (c *Collector) updateSegment(app App, state string) {
	now := time.Now().UTC()

	// 应用或状态变化时，结束当前段并开启新段
	if c.curSegID == 0 || c.curApp.Path != app.Path || c.curState != state {
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
			State:       state,
		})
		if err != nil {
			c.logger.Warn("插入活动段失败", "err", err)
			return
		}
		c.curSegID = id
		c.curApp = app
		c.curCategory = category
		c.curState = state
		return
	}

	// 同一段内只更新结束时间
	if err := c.db.UpdateActivitySegmentEndTS(c.curSegID, now); err != nil {
		c.logger.Warn("更新活动段结束时间失败", "err", err)
	}
}
