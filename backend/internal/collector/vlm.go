package collector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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

	// on_demand 冷却时间戳（仅 on_demand 模式使用）：
	//   - lastSwitchCaptureUnix / lastMotionCaptureUnix: 上次"采集"时刻（unix 纳秒），
	//     两路冷却独立计时、互不干扰。switch 只读写自己的戳，motion 只读写自己的戳。
	//     切换应用（switch 冷却通过）时把 lastMotionCaptureUnix 重置为切换时刻——
	//     即"每个应用的活动冷却独立，切换后重新计时"。被挡的 switch（Alt+Tab 抖动）
	//     不重置 motion，避免抖动打断当前应用的活动计时。
	//   - lastEnqueueUnix + lastEnqueueApp: 入队去重，防 motion 洪水把同一画面塞满队列。
	lastSwitchCaptureUnix atomic.Int64
	lastMotionCaptureUnix atomic.Int64
	lastEnqueueUnix       atomic.Int64
	lastEnqueueApp        atomic.Pointer[string]

	// inputActiveS: 输入活跃阈值(秒)，由 main.go 从 MovementConfig 注入。
	// 打字守卫：OnActivity 的 motion 回调在入队时判定，近该秒数内有键鼠输入
	// (正在打字)即跳过——打字时 VLM 截图冗余，且 PrintWindow 会卡顿目标窗口。
	// 默认 0 → isTypingActive 内兜底 8s。
	// 用 atomic 保护：main.go 装配阶段写、OnActivity 读（Collector.loop goroutine）。
	inputActiveS atomic.Int64
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

// Start 在后台启动截图采集。
//   - scheduled: 起定时 loop（按 ScheduleIntervalMin 周期截图）
//   - on_demand: 由 Collector 的活跃信号回调 OnActivity 驱动采集（无需独立 goroutine，
//     OnActivity 在 Collector.loop 内同步执行截图+落盘），识别由 recognitionLoop 消费
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
		} else {
			v.logger.Info("VLM 按需截图已启动",
				"switch_gap_s", v.cfg.OnDemandSwitchGapS, "motion_gap_s", v.cfg.OnDemandMotionGapS)
		}
	}
	// 识别 worker：所有模式都启（采集与识别解耦，识别由独立 worker 按间隔扫描 pending）。
	// 即使 VLM mode=off 不走这（Start 只在 mode 非 off 时由 VLMHolder 调用）。
	go v.recognitionLoop()
	intervalS := v.cfg.RecognitionIntervalS
	if intervalS <= 0 {
		intervalS = 300
	}
	v.logger.Info("VLM 识别 worker 已启动", "scan_interval_s", intervalS)
}

// buildPromptWithApp 在基础 prompt 前拼接当前应用上下文，帮助 VLM 准确识别应用、
// 减少把 A 应用误判成 B 应用的错误。纯内部增强：
//   - 不改变用户在设置页配置的 prompt 文本（不落 UI、不进 DB）
//   - appName 来自前台应用进程名（App.Name / VLMTask.AppName），三个调用点一致
//   - appName 为空（理论上不会发生）或 basePrompt 为空时原样返回，不掩盖配置问题
func (v *VLMCapturer) buildPromptWithApp(basePrompt, appName string) string {
	if strings.TrimSpace(basePrompt) == "" {
		return basePrompt // 空，让引擎报错提示用户配置，不绕过
	}
	appName = strings.TrimSpace(appName)
	if appName == "" {
		return basePrompt
	}
	return "当前用户正在使用的应用是 " + appName + "。" + basePrompt
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
// 用于打字守卫（OnActivity 的 motion 回调）。<=0 时兜底 8s。
func (v *VLMCapturer) SetInputActiveS(sec int) {
	v.inputActiveS.Store(int64(sec))
}

// isTypingActive 判断用户是否正在打字：近 inputActiveS 秒内有键鼠输入即视为活跃。
// 复用于 on_demand 的 motion/switch 两条触发路径——PrintWindow 是同步跨进程 GDI
// 调用，会阻塞目标窗口 UI 线程，对 Electron 应用(ZCode/Qoder)会卡住 IME 导致
// 中文输入丢字/中断，故截图/分析前需统一判定。inputActiveS<=0 兜底 8s；
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

// Trigger 立即执行一次截图理解（同步：截图 → 分析 → 落库）。
// 供 scheduled 定时 loop、TriggerVLM gRPC 手动触发等"不入队"的路径使用。
// on_demand 模式不走这里——它在 OnActivity 截图+落盘(pending)、recognitionLoop 消费识别。
func (v *VLMCapturer) Trigger(ctx context.Context) (string, error) {
	app, png, shotPath, err := v.capture()
	if err != nil {
		// 截图失败（PrintWindow 返回空/窗口关闭/UWP 全屏）也落一条事件，
		// 让时间轴可见（与 analyze 失败对称，分类 capture_failed）。
		v.recordVLMFailure(app, time.Now().UTC(), err)
		return "", err
	}
	// capture 返回 png==nil 表示"静默跳过"（如前台不在白名单），不算错误。
	if png == nil {
		return "", nil
	}
	return v.analyze(ctx, app, png, shotPath, time.Now().UTC())
}

// pendingScreenshotDir 返回待识别截图目录（screenshots/pending/）绝对路径。
// 由 enqueueTask 写入、recognitionLoop 读取、识别成功后删除。
func pendingScreenshotDir() (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("获取配置目录失败: %w", err)
	}
	dir := filepath.Join(cfgDir, "shadow-worker", "screenshots", "pending")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("创建待识别截图目录失败: %w", err)
	}
	return dir, nil
}

// enqueueTask 是采集与识别解耦的入库入口：截图落盘 + 写 vlm_tasks(pending)。
// OnActivity（on_demand）和 runOnce（scheduled）截图后调此方法，不再立即识别。
// 识别由 recognitionLoop worker 每5分钟扫描 pending 消费。
// 失败（落盘失败/DB写失败）只打 Warn 日志——采集本身轻量，单次失败不影响后续。
func (v *VLMCapturer) enqueueTask(app App, png []byte, ts time.Time) {
	dir, err := pendingScreenshotDir()
	if err != nil {
		v.logger.Warn("待识别截图目录不可用", "err", err)
		return
	}
	// 先 INSERT 拿 task_id，再写文件 <task_id>.png，再 UPDATE image_path。
	// 这样文件名与 DB 行一一对应，不会因文件名冲突导致混乱。
	id, err := v.db.InsertVLMTask(app.Path, app.Name, "", ts)
	if err != nil {
		v.logger.Warn("写入 VLM 任务失败", "err", err)
		return
	}
	imagePath := filepath.Join(dir, fmt.Sprintf("%d.png", id))
	if err := os.WriteFile(imagePath, png, 0o644); err != nil {
		v.logger.Warn("待识别截图落盘失败", "id", id, "err", err)
		// 文件写失败，删 DB 行避免留下无图 task。
		_ = v.db.DeleteVLMTask(id)
		return
	}
	if err := v.db.UpdateVLMTaskImage(id, imagePath); err != nil {
		v.logger.Warn("回填图片路径失败", "id", id, "err", err)
	}
	v.logger.Debug("VLM 任务已入队", "id", id, "app", app.Name, "png_bytes", len(png))
}

// capture 是截图阶段：取前台（或整屏）+ 白名单过滤 + 截图 + debug 落盘。
// 返回 (app, png, shotPath, err)。png==nil 且 err==nil 表示静默跳过（不在白名单）。
// 截图阶段的产物绑定后不再变，analyze 阶段重试时复用同一份 png（坑 #46 方向①）。
func (v *VLMCapturer) capture() (app App, png []byte, shotPath string, err error) {
	if v.cfg.CaptureRange == "screen" {
		// 整屏模式：截取虚拟屏（多显示器并集），用合成 app 信息标识。
		// 整屏不按白名单过滤——整屏本身是用户明确的全局采集意图。
		png = CaptureScreenPNG()
		if png == nil {
			return App{}, nil, "", fmt.Errorf("截图失败")
		}
		app = App{Name: "Screen", Path: ""}
	} else {
		// 活动窗口模式（默认）：前台窗口 + 白名单过滤。
		app, err = ForegroundApp()
		if err != nil {
			return App{}, nil, "", fmt.Errorf("获取前台应用失败: %w", err)
		}
		// 白名单过滤：仅采集用户添加到采集列表的应用。
		// 不在白名单 → 静默跳过本次（不算错误），与 movement 采集的过滤口径一致。
		if !v.isWhitelisted(app.Path) {
			v.logger.Debug("前台应用不在白名单，跳过 VLM 截图", "app", app.Name, "path", app.Path)
			return app, nil, "", nil
		}
		png = CaptureWindowPNG(app.HWND)
		if png == nil {
			return app, nil, "", fmt.Errorf("截图失败")
		}
	}

	// debug 模式：把截图落盘到 screenshots/ 目录，便于排查"VLM 识别了什么内容"。
	// 默认关闭（不落盘），只在 config.yaml 设 debug.save_vlm_screenshots: true 时开启。
	if v.cfg.SaveScreenshots {
		if p, saveErr := saveScreenshot(png, app.Name, time.Now().UTC()); saveErr != nil {
			v.logger.Warn("debug 保存截图失败", "err", saveErr)
		} else {
			shotPath = p
		}
	}
	return app, png, shotPath, nil
}

// analyze 是 API 阶段：用已绑定的 png 调 VLM 引擎（内部 DoWithRetry 按 retry_count 重试，
// 复用同一份 jsonBody）→ 成功后写 events 表。
// app/png/shotPath 全部来自 capture 阶段，本阶段不重新截图、不读盘（坑 #46 方向① + 坑 #48）。
// ts 是事件落库时间（on_demand 用入队时刻，scheduled/Trigger 用当前时刻）。
func (v *VLMCapturer) analyze(ctx context.Context, app App, png []byte, shotPath string, ts time.Time) (string, error) {
	summary, err := v.engine.DescribeWith(ctx, png, v.buildPromptWithApp(v.cfg.Prompt, app.Name))
	if err != nil {
		// 失败也落一条事件，让时间轴事件列表能标记（灰色空心圆感叹号 + hover 看详情）。
		// 分类错误（限流/鉴权/解析/网络）写入 Content（简短）和 Meta（JSON 含 detail）。
		v.recordVLMFailure(app, ts, err)
		return "", fmt.Errorf("VLM 识别失败: %w", err)
	}

	if _, err := v.db.InsertEvent(storage.Event{
		TS:             ts,
		Type:           storage.EventTypeVLMSummary,
		AppPath:        app.Path,
		AppName:        app.Name,
		Content:        summary,
		ScreenshotPath: shotPath, // debug 模式才有值
	}); err != nil {
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
// 两路冷却【独立计时、互不干扰】：switch 只读写 lastSwitchCaptureUnix，motion 只读写
// lastMotionCaptureUnix。这样在同一应用内频繁 motion 截图不会"吃掉"后续 switch 的冷却
// 额度（否则会出现"在 A 应用用了 38s、中途 motion 采过、切到 B 时 switch 被错误挡住"）。
//
// 每次切换应用（switch 冷却通过）后，把 lastMotionCaptureUnix 重置为切换时刻——
// 即"每个应用的活动冷却独立，切换后从零重新计时"。注意只有【冷却通过】的真切换才重置，
// 被自己 gap 挡住的 Alt+Tab 抖动（A→B→A 第二次）不重置 motion，避免抖动打断当前应用计时。
//
// 入队前完成截图并绑定 app+png，识别阶段（recognitionLoop）不重新截图（坑 #46 方向① + 坑 #48）。
// 入队去重：同一 app 在 gap 内只入一次，防 motion 洪水塞满队列。
func (v *VLMCapturer) OnActivity(reason string, app App, at time.Time) {
	// motion 回调打字守卫：用户正在打字时跳过 VLM 截图。
	// 打字时画面变化会高频触发 motion，但此刻截图既冗余又会卡顿目标窗口
	// （PrintWindow 同步阻塞目标 UI 线程）。键鼠信号已证明在用电脑，无需再"看"。
	// switch 回调(切窗口)不拦——切窗口瞬间用户尚未打字。
	if reason == "motion" && v.isTypingActive() {
		return // 正在打字，跳过 motion 触发
	}

	isSwitch := reason == "switch"
	// 按触发原因选 gap。切窗口是"明确换场景"的强信号，冷却短；
	// 活跃点是"还在同一场景动"，冷却长。
	gapS := v.cfg.OnDemandSwitchGapS
	if !isSwitch {
		gapS = v.cfg.OnDemandMotionGapS
	}
	if gapS <= 0 {
		gapS = 60 // 防御：配置异常回落
	}

	// 只读自己那一路的冷却戳——两路独立，互不干扰。
	var lastTS int64
	if isSwitch {
		lastTS = v.lastSwitchCaptureUnix.Load()
	} else {
		lastTS = v.lastMotionCaptureUnix.Load()
	}
	last := time.Unix(0, lastTS)
	sinceLast := at.Sub(last)
	gapDur := time.Duration(gapS) * time.Second
	if !last.IsZero() && sinceLast < gapDur {
		v.logger.Debug("被冷却挡住",
			"reason", reason, "app", app.Name, "gap_s", gapS,
			"since_last_s", sinceLast.Seconds())
		// 被挡的 switch（Alt+Tab 抖动）不重置 motion——抖动不算换场景。
		return
	}

	// gap 通过 → 入队前截图（绑定 app+png，消费者不再重新截）。
	// 注：app 由调用方传入（Collector 已读的前台），这里直接用它截图；
	// 不再像旧 Trigger 那样在消费时重新读前台——绑定后语义固定（坑 #48）。
	if !v.isWhitelisted(app.Path) {
		v.logger.Debug("前台应用不在白名单，跳过 VLM 截图", "app", app.Name, "path", app.Path)
		return
	}
	png := CaptureWindowPNG(app.HWND)
	if png == nil {
		v.logger.Warn("VLM 入队前截图失败", "app", app.Name)
		return
	}

	// 入队去重：同一 app 在 gap 内只入一次，防 motion 洪水塞满队列。
	// lastEnqueueUnix/lastEnqueueApp 在入队成功后更新。
	if prevAppPtr := v.lastEnqueueApp.Load(); prevAppPtr != nil {
		prevApp := *prevAppPtr
		prevTS := time.Unix(0, v.lastEnqueueUnix.Load())
		if prevApp == app.Path && !prevTS.IsZero() && at.Sub(prevTS) < gapDur {
			v.logger.Debug("入队去重：同 app gap 内已入队",
				"reason", reason, "app", app.Name, "gap_s", gapS)
			return
		}
	}

	v.logger.Debug("冷却通过，入队（含截图）",
		"reason", reason, "app", app.Name, "gap_s", gapS,
		"since_last_s", sinceLast.Seconds(), "png_bytes", len(png))

	// 采集与识别解耦：截图落盘 + 写 vlm_tasks(pending)，不再推内存 channel。
	// 识别由 recognitionLoop worker 每5分钟扫描 pending 消费。
	v.enqueueTask(app, png, at)

	// 更新各自冷却戳 + 入队去重时间戳。
	if isSwitch {
		v.lastSwitchCaptureUnix.Store(at.UnixNano())
		// 核心诉求：切换应用后 motion 冷却重新计时（重置为切换时刻）。
		// 从现在起算满 OnDemandMotionGapS 才采新应用的活动态——switch 刚在切换瞬间
		// 采过初始状态，motion 负责采"用了一会儿之后"的状态，避免紧跟着重复采同一画面。
		v.lastMotionCaptureUnix.Store(at.UnixNano())
	} else {
		v.lastMotionCaptureUnix.Store(at.UnixNano())
	}
	appPath := app.Path
	v.lastEnqueueApp.Store(&appPath)
	v.lastEnqueueUnix.Store(at.UnixNano())
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
	// scheduled 定时采集：截图 → 入库 pending（不再立即识别，由 recognitionLoop 消费）。
	app, png, _, err := v.capture()
	if err != nil {
		v.recordVLMFailure(app, time.Now().UTC(), err)
		v.logger.Warn("VLM 定时截图失败", "err", err)
		return
	}
	if png == nil {
		return // 静默跳过（前台不在白名单）
	}
	v.enqueueTask(app, png, time.Now().UTC())
}

// recognitionLoop 是识别消费 worker：每5分钟扫描 pending 任务 → 识别 → 成功清理/失败分类。
// 与采集（OnActivity/runOnce）完全解耦：采集只负责截图+落盘+写 pending，
// 本 worker 负责慢慢消费，不阻塞采集。识别失败按错误类型决定是否可重试：
//   - 429/5xx/网络：可重试，保持 pending，等下一轮扫描
//   - 鉴权/解析/截图：不可重试，标记 permanent_fail，保留数据等手动重试
func (v *VLMCapturer) recognitionLoop() {
	// 扫描间隔由 config.yaml 的 vlm.recognition_interval_s 配置，≤0 兜底 300（5分钟）。
	intervalS := v.cfg.RecognitionIntervalS
	if intervalS <= 0 {
		intervalS = 300
	}
	scanInterval := time.Duration(intervalS) * time.Second
	const batchSize = 10               // 每轮最多处理10条，避免长时间占用
	retryMinAge := scanInterval        // 失败任务至少等一个扫描周期再重试

	ticker := time.NewTicker(scanInterval)
	defer ticker.Stop()

	// 启动后立即扫一轮（不等5分钟），让积压的 pending 尽快处理。
	v.processPendingBatch(batchSize, retryMinAge)
	v.cleanupTasks()

	for {
		select {
		case <-v.stopCh:
			return
		case <-ticker.C:
			v.processPendingBatch(batchSize, retryMinAge)
			v.cleanupTasks()
		}
	}
}

// processPendingBatch 扫描一批 pending 任务并逐条识别。
func (v *VLMCapturer) processPendingBatch(limit int, retryMinAge time.Duration) {
	tasks, err := v.db.ListPendingVLMTasks(limit, retryMinAge)
	if err != nil {
		v.logger.Warn("查询待识别 VLM 任务失败", "err", err)
		return
	}
	for _, t := range tasks {
		v.processTask(t)
	}
	if len(tasks) > 0 {
		v.logger.Info("VLM 识别批次完成", "processed", len(tasks))
	}
}

// processTask 处理单条任务：读图 → 识别 → 更新结果。
func (v *VLMCapturer) processTask(t storage.VLMTask) {
	png, err := os.ReadFile(t.ImagePath)
	if err != nil {
		// 图片文件丢失（被手动删/磁盘问题）：标记 permanent_fail，不再重试。
		v.db.UpdateVLMTaskResult(t.ID, storage.VLMTaskStatusPermanentFail, t.Attempts+1,
			"capture_failed", "截图文件丢失: "+err.Error())
		v.logger.Warn("VLM 任务图片丢失", "id", t.ID, "path", t.ImagePath)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	summary, err := v.engine.DescribeWith(ctx, png, v.buildPromptWithApp(v.cfg.Prompt, t.AppName))
	cancel()

	if err == nil {
		// 成功：写 vlm_summary event + 删图片 + 删 task 行。
		v.recordVLMSuccess(t, summary)
		_ = os.Remove(t.ImagePath)
		_ = v.db.DeleteVLMTask(t.ID)
		v.logger.Info("VLM 摘要已生成(异步)", "id", t.ID, "app", t.AppName, "summary", summary)
		return
	}

	// 失败：按错误类型分类，决定是否可重试。
	kind, _ := classifyVLMError(err)
	retryable := isRetryableKind(kind)
	status := storage.VLMTaskStatusPermanentFail
	if retryable {
		status = storage.VLMTaskStatusPending // 保持 pending，等下一轮扫描重试
	}
	v.db.UpdateVLMTaskResult(t.ID, status, t.Attempts+1, kind, err.Error())
	v.logger.Warn("VLM 识别失败",
		"id", t.ID, "app", t.AppName, "kind", kind,
		"retryable", retryable, "attempts", t.Attempts+1, "err", err)
}

// RetryTaskSync 同步识别一条任务（用于用户手动重试）。
// 与 processTask（异步 worker 用）的区别：
//   - 本方法返回结果（success/fail/not_found），调用方据此给用户即时反馈
//   - 不改 task 状态为 pending（不依赖 recognitionLoop 异步扫描）
//   - 直接调 engine.Describe 同步等待结果
//
// 返回值：summary（成功时）、err（失败时，含分类信息）。
// 图片文件不存在时返回特殊错误（调用方统计 not_found_count）。
var errImageNotFound = fmt.Errorf("截图文件不存在，无法重试")

func (v *VLMCapturer) RetryTaskSync(t storage.VLMTask) (string, error) {
	png, err := os.ReadFile(t.ImagePath)
	if err != nil {
		// 图片丢失：标记 permanent_fail（如果还没有的话），返回特殊错误。
		v.db.UpdateVLMTaskResult(t.ID, storage.VLMTaskStatusPermanentFail, t.Attempts+1,
			"capture_failed", "截图文件丢失: "+err.Error())
		return "", errImageNotFound
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	summary, err := v.engine.DescribeWith(ctx, png, v.buildPromptWithApp(v.cfg.Prompt, t.AppName))
	cancel()

	if err == nil {
		v.recordVLMSuccess(t, summary)
		_ = os.Remove(t.ImagePath)
		_ = v.db.DeleteVLMTask(t.ID)
		v.logger.Info("VLM 手动重试成功", "id", t.ID, "app", t.AppName, "summary", summary)
		return summary, nil
	}

	// 失败：保持 permanent_fail，记录错误。
	// 返回干净中文 message（不传原始 err.Error()——可能含 GBK 解码后的响应体，
	// 经 gRPC protobuf 传输到前端会变成乱码）。原始 err 仍写入 DB 供日志排查。
	kind, message := classifyVLMError(err)
	v.db.UpdateVLMTaskResult(t.ID, storage.VLMTaskStatusPermanentFail, t.Attempts+1, kind, err.Error())
	v.logger.Warn("VLM 手动重试失败", "id", t.ID, "kind", kind, "err", err)
	return "", errors.New(message)
}

// RetryTasksInRange 同步重试时间窗内最新一条 permanent_fail 任务。
// 点一次重试只处理最新的一条——多条失败时用户可多次点重试。
// 返回成功数(0或1)、失败数(0或1)、图片不存在数(0或1)、失败原因列表。
func (v *VLMCapturer) RetryTasksInRange(start, end time.Time, appPath string) (success, fail, notFound int, errList []string) {
	t, err := v.db.LatestPermanentFailInRange(start, end, appPath)
	if err != nil {
		errList = append(errList, "查询失败任务出错: "+err.Error())
		return 0, 0, 0, errList
	}
	if t == nil {
		// 该范围内无 permanent_fail 任务（可能已被重试成功/清理）。
		return 0, 0, 0, nil
	}
	_, err = v.RetryTaskSync(*t)
	if err == nil {
		success = 1
	} else if err == errImageNotFound {
		notFound = 1
	} else {
		fail = 1
		errList = append(errList, err.Error())
	}
	return success, fail, notFound, errList
}

// recordVLMSuccess 把异步识别成功的摘要写入 events 表（与 analyze 成功路径一致）。
func (v *VLMCapturer) recordVLMSuccess(t storage.VLMTask, summary string) {
	if _, err := v.db.InsertEvent(storage.Event{
		TS:      t.CreatedTS, // 用采集时刻（而非识别时刻），保证时间轴段关联正确
		Type:    storage.EventTypeVLMSummary,
		AppPath: t.AppPath,
		AppName: t.AppName,
		Content: summary,
	}); err != nil {
		v.logger.Warn("写入 VLM 摘要事件失败", "id", t.ID, "err", err)
	}
}

// isRetryableKind 判断错误类型是否可自动重试。
// 429/5xx/网络瞬时故障可重试（服务可能恢复）；鉴权/解析/截图失败不可重试（重试也无用）。
func isRetryableKind(kind string) bool {
	switch kind {
	case "rate_limit", "request_failed":
		return true
	default:
		return false // auth_error / parse_error / capture_failed / unknown
	}
}

// cleanupTasks 阈值清理：pending + permanent_fail 合计超过 maxTasks 条时，
// 删最旧的 permanent_fail 项（含 task 行 + 图片文件），释放空间。
// permanent_fail 保留是为了让用户手动重试，但积累太多会占磁盘，需兜底清理。
func (v *VLMCapturer) cleanupTasks() {
	const maxTasks = 100
	count, err := v.db.CountAllVLMTasks()
	if err != nil {
		v.logger.Warn("统计 VLM 任务失败，跳过清理", "err", err)
		return
	}
	if count <= maxTasks {
		return
	}
	// 删最旧的 permanent_fail（pending 不删——那是还没识别完的）。
	toDelete := count - maxTasks
	fails, err := v.db.ListOldPermanentFails(toDelete)
	if err != nil {
		v.logger.Warn("查询永久失败任务失败，跳过清理", "err", err)
		return
	}
	for _, f := range fails {
		_ = os.Remove(f.ImagePath)
		_ = v.db.DeleteVLMTask(f.ID)
	}
	if len(fails) > 0 {
		v.logger.Info("VLM 任务清理", "deleted", len(fails), "reason", "超过阈值", "max", maxTasks)
	}
}

// classifyVLMError 把底层 VLM 错误归类为前端可识别的 kind。
// kind 决定时间轴事件列表里 hover 气泡的标题（限流/鉴权失败/解析失败/请求失败/截图失败）。
// 解析依据是 cloud.go/ollama.go 产生的 error 字符串（"VLM API 状态 %d: ..."）。
func classifyVLMError(err error) (kind, message string) {
	if err == nil {
		return "request_failed", "未知错误"
	}
	msg := err.Error()
	lower := strings.ToLower(msg)

	// 限流：429。最常见的"频繁失败"根因，重试也救不回时落事件让用户可见。
	if strings.Contains(msg, "状态 429") || strings.Contains(lower, "rate limit") || strings.Contains(lower, "too many requests") {
		return "rate_limit", "VLM 触发限流，稍后重试"
	}
	// 鉴权失败：401/403。通常是 api_key 配错或过期，不会自愈，需用户检查配置。
	if strings.Contains(msg, "状态 401") || strings.Contains(msg, "状态 403") || strings.Contains(lower, "unauthorized") || strings.Contains(lower, "forbidden") {
		return "auth_error", "VLM 鉴权失败，请检查 API Key"
	}
	// 解析失败：上游返回了响应但结构不符（空 choices/空文本/JSON 解析失败）。
	// 常见诱因：上游返回非 OpenAI 标准结构、max_tokens 截断、代理返回 HTML。
	if strings.Contains(msg, "空 choices") || strings.Contains(msg, "空文本") || strings.Contains(msg, "解析 VLM 响应失败") || strings.Contains(msg, "解析 Ollama 响应失败") {
		return "parse_error", "VLM 响应解析失败"
	}
	// 截图失败：PrintWindow 返回空（窗口关闭/UWP 全屏）。
	if strings.Contains(msg, "截图失败") {
		return "capture_failed", "截图失败，未采集到画面"
	}
	// 其它 HTTP 错误 / 网络错误 / 超时。
	return "request_failed", "VLM 请求失败"
}

// recordVLMFailure 写一条 vlm_summary_fail 事件，记录识别失败。
// Content 存简短描述（事件列表显示），Meta 存 JSON {"kind","detail"}（hover 详情）。
// detail 含解码后的真实错误（见 vlm/bodydecode.go），让用户能判断具体原因。
func (v *VLMCapturer) recordVLMFailure(app App, ts time.Time, err error) {
	if v.db == nil {
		return
	}
	kind, message := classifyVLMError(err)
	metaJSON, _ := json.Marshal(map[string]string{
		"kind":   kind,
		"detail": err.Error(),
	})
	if _, werr := v.db.InsertEvent(storage.Event{
		TS:      ts,
		Type:    storage.EventTypeVLMSummaryFail,
		AppPath: app.Path,
		AppName: app.Name,
		Content: message,
		Meta:    string(metaJSON),
	}); werr != nil {
		v.logger.Warn("写入 VLM 失败事件失败", "err", werr)
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
