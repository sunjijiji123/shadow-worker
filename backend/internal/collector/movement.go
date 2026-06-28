package collector

import (
	"fmt"
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
//   - AwayThresholdS: 离开阈值。idle 持续超该秒数 → 判定"离开",
//     结束当前段(不再写覆盖整个离开期间的 idle 段),回来后开新段。
//     短 idle(< 阈值)仍视为"思考",段不断开。
type PrecisionConfig struct {
	Thresh       uint8
	ChangeRatio  float64
	SampleMs     int
	InputIdleS   int
	DisplayIdleS int
	AwayThresholdS int
	// SaveScreenshots 由 NewCollector 从 MovementConfig 复制；debug 模式时为 true。
	SaveScreenshots bool
}

// Presets 是预定义的精度档位。
// AwayThresholdS 各档统一 600s(10 分钟)——离开判定与精度无关:
// 精度只影响"正在动/思考"的灵敏度,而"离开"是绝对时长概念(10 分钟无交互即离开)。
var Presets = map[string]PrecisionConfig{
	"low":    {Thresh: 50, ChangeRatio: 0.005, SampleMs: 500, InputIdleS: 20, DisplayIdleS: 120, AwayThresholdS: 600},
	"medium": {Thresh: 30, ChangeRatio: 0.002, SampleMs: 300, InputIdleS: 15, DisplayIdleS: 90, AwayThresholdS: 600},
	"high":   {Thresh: 15, ChangeRatio: 0.001, SampleMs: 200, InputIdleS: 10, DisplayIdleS: 60, AwayThresholdS: 600},
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

	// away 标记"已离开"(idle 超 AwayThresholdS)。仅 loop goroutine 读写,
	// CurrentApp 不读,故无需锁。置 true 时已 curSegID=0 关段；离开期间每 tick
	// 都命中离开分支跳过写段;只有再次强信号才清零,此时 updateSegment 因
	// curSegID==0 自然开新段。
	away bool

	// vlmHolder 注入 VLM 采集器持有者，供 loop 在活跃信号上回调 on_demand 触发。
	// 装配阶段（main.go，Start 之后）通过 SetVLMHolder 写，loop goroutine 读，
	// 用 c.mu 保护指针读写（holder 内部的 atomic.Pointer 保证 Get 并发安全）。
	vlmHolder *VLMHolder
}

// SetVLMHolder 注入 VLM holder，供 loop 在活跃信号上回调 on_demand 触发。
// 应在 Start() 之前或装配阶段调用（main.go 在 coll.Start() 后、gRPC 注册前注入）。
func (c *Collector) SetVLMHolder(h *VLMHolder) {
	c.mu.Lock()
	c.vlmHolder = h
	c.mu.Unlock()
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
	if mc.AwayThresholdS > 0 {
		cfg.AwayThresholdS = mc.AwayThresholdS
	}
	cfg.SaveScreenshots = mc.SaveScreenshots
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

// AwayThresholdS 返回离开判定阈值（秒）。
// 供概览页 InterruptCount 使用：段间空档 >= 该阈值 = 一次中断（离开再回来）。
// 采集层的离开检测用此值断段（见 loop 的 inputIdleMs >= awayThresholdMs 判据），
// 查询层用同值判定"中断"，确保两边语义一致。
func (c *Collector) AwayThresholdS() int {
	return c.cfg.AwayThresholdS
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
	// 诊断用：记录上一 tick 的 strong/state，仅在翻转时打日志（避免每 tick 刷屏）。
	var prevStrong bool
	var prevState string
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
		// 记录每路信号的具体值（用于日志诊断 7.5h active 段问题：
		// 确认是哪路信号在"续命"，便于定位帧差误报/键鼠噪声/标题抖动）。
		strong := false
		var frameRatio float64   // s1 帧差比例（<0 表示未计算/截图失败）
		var inputIdleMs uint64   // s2 距上次系统输入的毫秒
		var titleChanged bool    // s3 标题是否变化
		const sigNone = ""       // 无信号
		strongReason := sigNone  // 触发强信号的路："frame"/"input"/"title"（调试用）

		// s1: 帧差(屏幕在变)
		curr := CaptureWindow(app.HWND)
		if curr == nil {
			c.logger.Debug("截图失败", "app", app.Name)
			// 截图失败不阻断其它信号判定，但帧差缺失。
			frameRatio = -1
		} else if prevFrame != nil {
			ratio, err := FrameDiff(prevFrame, curr, CaptureTargetWidth, CaptureTargetHeight)
			if err != nil {
				c.logger.Warn("帧差计算失败", "err", err)
				frameRatio = -1
			} else {
				frameRatio = ratio
				if ratio > c.cfg.ChangeRatio {
					strong = true
					strongReason = "frame"
					// debug 模式：画面变化时保存完整截图，便于排查 movement 判断依据。
					// 帧差超阈值才保存，避免每 tick 落盘（300ms 一次太密）。
					if c.cfg.SaveScreenshots {
						if debugPng := CaptureWindowPNG(app.HWND); debugPng != nil {
							if _, saveErr := saveScreenshot(debugPng, "mv-"+app.Name, time.Now().UTC()); saveErr != nil {
								c.logger.Debug("debug 保存 movement 截图失败", "err", saveErr)
							}
						}
					}
				}
			}
			prevFrame = curr
		} else {
			// 第一帧:无前帧可比,默认认为有变化(沿用旧行为)。
			strong = true
			strongReason = "frame(first)"
			frameRatio = -1 // 首帧未实际计算
			prevFrame = curr
		}

		// s2: GetLastInputInfo(系统级键鼠输入)。前台=白名单时,
		// 系统输入几乎必然进了该前台应用,故可作为"该应用在用"的强信号。
		// 每 tick 无条件计算 inputIdleMs——它除驱动本路强信号判定外,还供
		// 离开检测（loop 末尾）直接使用:离开判据看真实键鼠空闲时间,不再依赖
		// 被帧差污染的 lastEngaged（见坑 #45/#49 修复）。
		if tick, ok := winapi.LastInputTick(); ok {
			inputIdleMs = winapi.GetTickCount64() - uint64(tick)
			if !strong && inputIdleMs < uint64(inputIdle/time.Millisecond) {
				strong = true
				strongReason = "input"
			}
		}

		// s3: 窗口标题变化(切标签/导航/文件)。
		if !strong && app.WindowTitle != prevTitle {
			strong = true
			strongReason = "title"
		}
		titleChanged = app.WindowTitle != prevTitle
		prevTitle = app.WindowTitle

		if strong {
			lastEngaged = now
			// 活跃点 → on_demand 回调（非阻塞）。strong 是三路信号
			// （帧差/键鼠/标题变化）的合集，完全复用，不新写活跃判断。
			c.notifyVLMActivity("motion", app, now)
		}

		state := inferState(strong, time.Since(lastEngaged), displayIdle)

		// 状态变化诊断日志：只在 strong 翻转 或 state 变化 时打，
		// 避免每 tick（300ms）刷屏。覆盖 7.5h 段排查所需的全部信息：
		// 哪路信号触发、帧差比例、输入空闲时长、标题是否变、推断的 state。
		// idle→active/engaged 等"人在回来"的翻转最关键（能看到是什么唤醒的）。
		if c.logger.Enabled(nil, slog.LevelDebug) {
			sinceEngagedSec := int(time.Since(lastEngaged) / time.Second)
			if strong != prevStrong || state != prevState {
				c.logger.Debug("信号/状态翻转",
					"app", app.Name,
					"strong", strong, "reason", strongReason, "prev_strong", prevStrong,
					"frame_ratio", fmt.Sprintf("%.4f", frameRatio),
					"frame_thresh", c.cfg.ChangeRatio,
					"input_idle_ms", inputIdleMs, "input_thresh_ms", inputIdle/time.Millisecond,
					"title_changed", titleChanged,
					"state", state, "prev_state", prevState,
					"since_engaged_s", sinceEngagedSec,
					"seg_id", c.curSegID, "away", c.away)
			}
			prevStrong = strong
			prevState = state
		}

		// 离开检测：真实键鼠空闲超 AwayThresholdS → 判定"离开"。
		// 离开 ≠ 思考(短 idle)：离开时结束当前段，段在最后一次真实交互处
		// 干净结束（end=最后一次键鼠输入时刻，state 回写 engaged），避免离开期间写成
		// 覆盖数小时的 idle/active 段把时间轴撑爆。离开期间（away=true）每 tick 都
		// 跳过写段，DB 里留下真正的空档（时间轴轨道上显示为空白断档）。
		// 只有再次强信号才解除 away（下一行 updateSegment 会因 curSegID==0 开新段）。
		//
		// 判据直接看 inputIdleMs（真实键鼠空闲），不再看 state==idle。
		// 历史缺陷（坑 #45/#49）：旧判据 state==idle && Since(lastEngaged)>=阈值，
		// 但 lastEngaged 被三路 OR 的 strong 刷新，Electron 应用(ZCode/VSCode)空闲时
		// GPU 合成层帧差误报让 strong 常真 → lastEngaged 永不老化 → 永远到不了 idle
		// → 断段失效，产生跨数小时甚至跨天的巨怪段。改用 inputIdleMs 彻底绕开帧差污染：
		// 键鼠是物理输入，无法被屏幕动画伪造。
		awayThresholdMs := uint64(c.cfg.AwayThresholdS) * 1000
		if inputIdleMs >= awayThresholdMs {
			if c.curSegID != 0 && !c.away {
				// 段在最后一次真实键鼠输入处干净结束。inputIdleMs 是"距上次输入的毫秒数"，
				// 反推最后一次输入时刻 = now - inputIdleMs。该时刻必然是 engaged（人在动）。
				lastRealInput := time.Now().UTC().Add(-time.Duration(inputIdleMs) * time.Millisecond)
				if err := c.db.UpdateActivitySegmentEndTSAndState(c.curSegID, lastRealInput, StateEngaged); err != nil {
					c.logger.Warn("离开断段失败", "err", err)
				}
				// 关键事件：记录段被"离开"打断。含 input_idle_ms 便于回归验证
				// 离开判据确实由真实键鼠空闲触发（而非帧差/state）。
				c.logger.Info("离开断段",
					"seg_id", c.curSegID, "end", lastRealInput.Format("15:04:05"),
					"input_idle_ms", inputIdleMs, "away_thresh_ms", awayThresholdMs)
				c.curSegID = 0
				c.curState = ""
				c.curApp = App{}
			}
			c.away = true
			continue
		}
		if strong {
			// 强信号解除 away；接着 updateSegment 会因 curSegID==0 开新段。
			c.away = false
		}

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
//
// 段的合并粒度（用户语义）："在同一应用上持续工作"应合并为一段，
// 只有切换应用才结束旧段开新段。engaged/active/idle 之间的翻转
// 都在段内进行——idle 只是"在这件事上暂时静默/思考"，不算离开。
// 这样 worklog 每个应用是一段连续记录，不会被每秒的 state 翻转碎片化。
// 段的 state 字段滚动更新为最新值（记录最后一次活跃强度），end_ts 每tick延长。
func (c *Collector) updateSegment(app App, state string) {
	now := time.Now().UTC()

	// 唯一的开新段判据：首次、或切换了应用。
	needNew := c.curSegID == 0 || c.curApp.Path != app.Path

	if needNew {
		// hadPrev 标记切换前已有段（非首次开段）——切窗口 on_demand 回调用。
		hadPrev := c.curSegID != 0
		if hadPrev {
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

		// 切窗口 → on_demand 回调（非阻塞）。判据与开新段一致（进程路径变化）。
		// 注意：curSegID==0 的"首次开段"不算切换，不发回调。
		// 若上面的 InsertActivitySegment 失败已提前 return，这里不会执行。
		if hadPrev {
			c.notifyVLMActivity("switch", app, time.Now().UTC())
		}
		return
	}

	// 同一应用段内：延长结束时间，并滚动更新 state 为最新活跃强度。
	c.curState = state
	if err := c.db.UpdateActivitySegmentEndTSAndState(c.curSegID, now, state); err != nil {
		c.logger.Warn("更新活动段失败", "err", err)
	}
}

// notifyVLMActivity 是 on_demand VLM 触发的钩子入口（非阻塞）。
//
// 由 loop 在两处调用：
//   - updateSegment 开新段且非首次时（切窗口）→ reason="switch"
//   - loop strong 信号成立时（活跃点）→ reason="motion"
//
// holder/Get 均可为 nil（VLM 关闭 / 未配置 / screen 模式降级），此时跳过。
// OnActivity 内部做 gap 判定和防重入，这里只负责取实例并转发。
func (c *Collector) notifyVLMActivity(reason string, app App, at time.Time) {
	c.mu.RLock()
	holder := c.vlmHolder
	c.mu.RUnlock()
	if holder == nil {
		return
	}
	if cap := holder.Get(); cap != nil {
		cap.OnActivity(reason, app, at)
	}
}
