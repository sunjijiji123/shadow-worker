package collector

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"shadow-worker/backend/internal/config"
	"shadow-worker/backend/internal/storage"
	"shadow-worker/backend/internal/vlm"
	"shadow-worker/backend/internal/winapi"
)

// VLMCapturer 负责定时/按需截图并调用 VLM 生成摘要。
type VLMCapturer struct {
	cfg    config.VLMConfig
	engine vlm.Engine
	db     *storage.DB
	logger *slog.Logger
	stopCh chan struct{}

	// Stop 幂等：VLMHolder.Rebuild 重建时会调旧实例 Stop，
	// 重复 close stopCh 会 panic，用 sync.Once 保护。
	stopOnce sync.Once

	// on_demand 触发机制（仅 on_demand 模式启用）：
	//   - triggerCh: Collector.loop 在活跃信号上回调 OnActivity → 入队（cap=1，满则丢）
	//   - onDemandLoop goroutine 消费：300ms 延迟 → Trigger
	//   - lastCaptureUnix: 上次采集时刻（unix 纳秒），用 atomic 保护跨 goroutine读写
	//     两个 gap（switch/motion）共用此时间戳，任一触发成功后都更新它。
	triggerCh       chan triggerEvent
	lastCaptureUnix atomic.Int64

	// inputActiveS: 输入活跃阈值(秒)，由 main.go 从 MovementConfig 注入。
	// 两处打字守卫共用：① OnActivity 的 motion 回调（入队时拦）；② onDemandLoop
	// 截图前（300ms 延迟后拦，覆盖 motion/switch）。若近该秒数内有键鼠输入
	// (正在打字)即跳过截图——打字时 VLM 截图冗余，且 PrintWindow 会卡顿目标窗口。
	// 默认 0 → isTypingActive 内兜底 8s。
	// 用 atomic 保护：main.go 装配阶段写、onDemandLoop goroutine 读。
	inputActiveS atomic.Int64
}

// triggerEvent 是 on_demand 触发事件。
type triggerEvent struct {
	Reason string // "switch"（切窗口）/ "motion"（活跃点）
	App    App    // 触发时的前台应用（仅用于日志）
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
		cfg:       cfg,
		engine:    engine,
		db:        db,
		logger:    logger,
		stopCh:    make(chan struct{}),
		triggerCh: make(chan triggerEvent, 1), // cap=1：防重入，满则丢
	}, nil
}

// Start 在后台启动截图采集。
//   - scheduled: 起定时 loop（按 ScheduleIntervalMin 周期截图）
//   - on_demand:  起消费 goroutine，由 Collector 的活跃信号回调 OnActivity 驱动
//   - screen + on_demand: 非法组合，VLMHolder.Rebuild 不应构造此状态；防御性不启动
func (v *VLMCapturer) Start() {
	switch v.cfg.Mode {
	case "scheduled":
		go v.loop()
		v.logger.Info("VLM 定时截图已启动", "interval_min", v.cfg.ScheduleIntervalMin)
	case "on_demand":
		if v.cfg.CaptureRange == "screen" {
			// 整屏模式无"活跃窗口"概念，on_demand 无触发源，不启动。
			// 正常不会走到这（VLMHolder.Rebuild 会降级），此处为防御兜底。
			v.logger.Warn("VLM on_demand 模式不支持整屏截图，不启动采集")
			return
		}
		go v.onDemandLoop()
		v.logger.Info("VLM 按需截图已启动",
			"switch_gap_s", v.cfg.OnDemandSwitchGapS, "motion_gap_s", v.cfg.OnDemandMotionGapS)
	}
}

// Stop 停止采集。幂等：VLMHolder.Rebuild 重建时可能对旧实例重复调用。
// 只关 stopCh 停 goroutine，不强杀正在跑的 in-flight Trigger（它持有 db 引用，
// 跑完会落库；这是热重载时序上可接受的行为）。
func (v *VLMCapturer) Stop() {
	v.stopOnce.Do(func() {
		close(v.stopCh)
	})
}

// SetInputActiveS 注入输入活跃阈值(秒)。应在 Start() 之前调用（main.go 装配阶段）。
// 用于打字守卫（OnActivity 的 motion 回调 + onDemandLoop 截图前）。<=0 时兜底 8s。
func (v *VLMCapturer) SetInputActiveS(sec int) {
	v.inputActiveS.Store(int64(sec))
}

// isTypingActive 判断用户是否正在打字：近 inputActiveS 秒内有键鼠输入即视为活跃。
// 复用于 on_demand 的 motion/switch 两条触发路径——PrintWindow 是同步跨进程 GDI
// 调用，会阻塞目标窗口 UI 线程，对 Electron 应用(ZCode/Qoder)会卡住 IME 导致
// 中文输入丢字/中断，故截图前需统一判定。inputActiveS<=0 兜底 8s；
// LastInputInfo 取数失败保守返回 false（不阻断采集，宁可偶发卡顿也不漏采）。
func (v *VLMCapturer) isTypingActive() bool {
	sec := v.inputActiveS.Load()
	if sec <= 0 {
		sec = 8 // 默认兜底（与 movement medium 档 InputActiveS 一致）
	}
	tick, ok := winapi.LastInputTick()
	if !ok {
		return false
	}
	idleMs := winapi.GetTickCount64() - uint64(tick)
	return idleMs > 0 && idleMs < uint64(sec)*1000
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

// DescribePath 读取指定路径的 PNG 文件并送 VLM 分析，返回摘要。
// 用于"快捷工具-桌面截图"：用户框选并保存的截图由前端送到这里分析，
// 保证 VLM 分析的就是用户框选的那块图（而非后端重新截图）。
// prompt 是桌面截图识别专用提示词（与引擎构造时的全局 vlm.prompt 区分），
// 为空时由引擎回落默认。不写时间线事件、不重新截图——只做"看图说话"。
func (v *VLMCapturer) DescribePath(ctx context.Context, path, prompt string) (string, error) {
	if v.engine == nil {
		return "", fmt.Errorf("VLM 未启用")
	}
	png, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("读取截图失败: %w", err)
	}
	summary, err := v.engine.DescribeWith(ctx, png, prompt)
	if err != nil {
		return "", fmt.Errorf("VLM 识别失败: %w", err)
	}
	v.logger.Info("VLM 分析截图文件", "path", path, "summary", summary)
	return summary, nil
}

// OnActivity 由 Collector.loop 在判出活跃信号时调用（非阻塞，运行在 loop goroutine）。
//
//   - reason="switch": 切换了前台应用（进程路径变化）→ 受 OnDemandSwitchGapS 冷却
//   - reason="motion": 当前窗口出现活跃点（帧差/键鼠/标题变化）→ 受 OnDemandMotionGapS 冷却
//
// 两个 gap 共用 lastCaptureUnix（上次采集时刻）。
// gap 判定通过才 non-blocking 入队 triggerCh；channel 满（上次还没处理完）则丢，
// 这天然实现防重入——消费 goroutine 串行处理，不会并发 Trigger。
func (v *VLMCapturer) OnActivity(reason string, app App, at time.Time) {
	// motion 回调打字守卫：用户正在打字时跳过 VLM 截图。
	// 打字时画面变化会高频触发 motion，但此刻截图既冗余又会卡顿目标窗口
	// （PrintWindow 同步阻塞目标 UI 线程）。键鼠信号已证明在用电脑，无需再"看"。
	// switch 回调(切窗口)不在此拦——切窗口瞬间用户尚未打字，真正的拦截在
	// onDemandLoop 截图前（300ms 延迟后用户可能已开始输入）。
	if reason == "motion" && v.isTypingActive() {
		return // 正在打字，跳过 motion 触发
	}

	// 按触发原因选 gap。切窗口是"明确换场景"的强信号，冷却短；
	// 活跃点是"还在同一场景动"，冷却长。
	gapS := v.cfg.OnDemandSwitchGapS
	if reason == "motion" {
		gapS = v.cfg.OnDemandMotionGapS
	}
	if gapS <= 0 {
		gapS = 60 // 防御：配置异常回落
	}

	last := time.Unix(0, v.lastCaptureUnix.Load())
	sinceLast := at.Sub(last)
	gapDur := time.Duration(gapS) * time.Second
	if !last.IsZero() && sinceLast < gapDur {
		v.logger.Debug("被冷却挡住",
			"reason", reason, "app", app.Name, "gap_s", gapS,
			"since_last_s", sinceLast.Seconds())
		return // 冷却期内，跳过本次触发
	}

	// gap 通过 → non-blocking 入队，满则丢（防重入）。
	v.logger.Debug("冷却通过，入队",
		"reason", reason, "app", app.Name, "gap_s", gapS,
		"since_last_s", sinceLast.Seconds())
	select {
	case v.triggerCh <- triggerEvent{Reason: reason, App: app}:
	default:
	}
}

// onDemandLoop 是 on_demand 模式的消费 goroutine。
// 串行处理 triggerCh 的事件：300ms 延迟（让新窗口/画面绘制稳定）→ 打字守卫
// （正在打字则放弃本次截图）→ Trigger。截图前的守卫是中文输入防卡顿的关键：
// 切窗口后 300ms 正是用户在新窗口开始输入的时刻，此时 PrintWindow 会卡住 IME。
func (v *VLMCapturer) onDemandLoop() {
	for {
		select {
		case <-v.stopCh:
			return
		case ev := <-v.triggerCh:
			// 硬编码 300ms 延迟：切窗口瞬间（甚至 Alt+Tab 预览阶段）新窗口可能没画完，
			// PrintWindow(PW_RENDERFULLCONTENT) 会拿到半成品。等一帧再截拿稳定画面。
			select {
			case <-time.After(300 * time.Millisecond):
			case <-v.stopCh:
				return
			}

			// 截图前打字守卫：300ms 等待期间用户可能已经开始在新窗口打字（尤其切到
			// 编辑器后立刻输入）。此时 PrintWindow 会阻塞目标 UI 线程，卡住 IME 导致
			// 中文输入丢字/中断。检测到正在打字则放弃本次截图，等下一个活跃点重试。
			// 覆盖 motion/switch 两条路径——motion 在入队时已守过一次，这里是第二道
			// 保险（入队到此刻可能又过了几秒、用户恢复打字）；switch 入队时未守，全靠这里。
			if v.isTypingActive() {
				v.logger.Debug("打字守卫跳过 on-demand 截图", "reason", ev.Reason, "app", ev.App.Name)
				continue
			}

			// Trigger 内部会重新读 ForegroundApp 拿延迟后的真实前台。
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			_, err := v.Trigger(ctx)
			cancel()
			if err != nil {
				// 失败不更新 lastCaptureUnix → 下一个活跃点能立即重试，不死等 gap。
				// （引擎层 DoWithRetry 已对 429/5xx/网络错误重试 3 次，这里失败=重试也救不回）
				v.logger.Warn("on-demand VLM 触发失败", "reason", ev.Reason, "app", ev.App.Name, "err", err)
				continue
			}
			// 成功才标记采集时刻，两个 gap 都从此起算。
			v.lastCaptureUnix.Store(time.Now().UTC().UnixNano())
		}
	}
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
