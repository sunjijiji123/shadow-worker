// Package config 加载 YAML 配置。
//
// 配置文件路径: %APPDATA%/shadow-worker/config.yaml
package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

//go:embed default_prompt.txt
var defaultPrompt string

// DefaultVLMPrompt 是 VLM 视觉理解提示词的默认值。
// 用户可在设置页修改（存入 config.yaml 的 vlm.prompt），但不可为空——
// 空时引擎拒绝分析（Describe 返回错误），保存时 UI 也会拦截。
const DefaultVLMPrompt = "请用一句话概括这张屏幕截图里用户正在做什么，不超过 50 字。"

// DefaultScreenshotPrompt 是"快捷工具-桌面截图"识别专用提示词的默认值。
// 与 vlm_prompt 区分：VLM 那条用于后台定时/on_demand 截图（强调概括"在做什么"），
// 桌面截图是用户主动框选一块区域做识别，语义偏"描述这块截图里有什么"。
// 存入 config.yaml 的 screenshot.prompt，不可为空（空时回落本默认值）。
const DefaultScreenshotPrompt = "请详细描述这张截图中的内容：用户正在使用什么应用、界面展示了什么信息、正在进行什么操作。用中文简洁回答。"

// ASRMode 是 ASR 模式。
type ASRMode string

const (
	ASRModeCloud ASRMode = "cloud"
	ASRModeLocal ASRMode = "local"
)

// ASRProvider 是单个 ASR 供应商配置。
type ASRProvider struct {
	Name      string `yaml:"name"`
	BaseURL   string `yaml:"base_url"`
	Model     string `yaml:"model"`
	APIKey    string `yaml:"api_key"`
	AuthType  string `yaml:"auth_type"`  // bearer | api-key
	APIFormat string `yaml:"api_format"` // openai | anthropic
	Language  string `yaml:"language"`
	NumCtx    int    `yaml:"num_ctx"`
	Type      string `yaml:"type"` // cloud | local
	// Stream 控制是否按 SSE 流式请求/解析。小米等支持流式分块的 ASR 设 true；
	// 标准 OpenAI whisper-1 不支持流式（返回普通 JSON），设 false。
	// 默认 false（零值）。
	Stream bool `yaml:"stream"`
	// LocalModelPath 仅对 type=local 的 provider 有效：whisper .bin 模型文件路径。
	// 每个 local provider 各自持有，支持多个本地模型独立切换。
	// （历史遗留：早期只有全局唯一的 cfg.ASR.Local，现已改为 per-provider。）
	LocalModelPath string `yaml:"local_model_path"`
	// RetryCount 是云请求 HTTP 重试次数（覆盖 429/5xx/网络错误，含指数退避）。
	// 0 = 不重试（只发 1 次）；3 = 默认（共 4 次请求）。
	// 由 httputil.DoWithRetry 读取。type=local 的 provider 不使用此字段。
	RetryCount int `yaml:"retry_count"`
}

// LocalASRConfig 是本地 whisper 配置。
type LocalASRConfig struct {
	ModelPath string `yaml:"model_path"`
	ModelName string `yaml:"model_name"`
	Language  string `yaml:"language"`
}

// ASRConfig 是 ASR 配置块。
type ASRConfig struct {
	Mode           string                 `yaml:"mode"`
	ActiveProvider string                 `yaml:"active_provider"`
	Providers      map[string]ASRProvider `yaml:"providers"`
	Local          LocalASRConfig         `yaml:"local"`
	RecordMode     string                 `yaml:"record_mode"` // hold | press (recording trigger)
}

// VLMProvider 是单个 VLM 供应商配置。
type VLMProvider struct {
	Name      string `yaml:"name"`
	BaseURL   string `yaml:"base_url"`
	Model     string `yaml:"model"`
	APIKey    string `yaml:"api_key"`
	AuthType  string `yaml:"auth_type"`  // bearer | api-key
	APIFormat string `yaml:"api_format"` // openai | ollama
	NumCtx    int    `yaml:"num_ctx"`
	Type      string `yaml:"type"` // cloud | local
	// RetryCount 是云请求 HTTP 重试次数（覆盖 429/5xx/网络错误，含指数退避）。
	// 0 = 不重试；3 = 默认。由 httputil.DoWithRetry 读取。
	// type=local 的 provider 不使用此字段。
	RetryCount int `yaml:"retry_count"`
}

// VLMConfig 是 VLM 配置块。
type VLMConfig struct {
	Mode                string                 `yaml:"mode"` // scheduled | on_demand | off
	ActiveProvider      string                 `yaml:"active_provider"`
	Providers           map[string]VLMProvider `yaml:"providers"`
	ScheduleIntervalMin int                    `yaml:"schedule_interval_min"`
	// CaptureRange 控制定时/按需截图的范围：
	//   active = 当前前台窗口（默认，与白名单配合聚焦正在做的事）
	//   screen = 整个虚拟屏幕（含所有显示器，适合多屏工作流）
	CaptureRange string `yaml:"capture_range"`
	// OnDemandSwitchGapS 仅 on_demand 模式：切换到新活跃窗口后，距上次 switch
	// 采集超过该秒数才触发一次截图。用于过滤 Alt+Tab 抖动（人在两窗口间来回切）。
	// 冷却与 motion 独立计时（各看各的"上次采集时刻"）。默认 20s。
	OnDemandSwitchGapS int `yaml:"on_demand_switch_gap_s"`
	// OnDemandMotionGapS 仅 on_demand 模式：当前窗口出现活跃信号（画面运动/
	// 键鼠输入/标题变化）后，距上次 motion 采集超过该秒数才触发一次截图。同一场景内
	// 不频繁采集，避免资源损耗。冷却与 switch 独立计时；切换应用（switch 冷却通过）
	// 后会重置为切换时刻，即从现在起算满该秒数才采新应用的活动态截图。默认 60s。
	OnDemandMotionGapS int `yaml:"on_demand_motion_gap_s"`
	// Prompt 是送给 VLM 的提示词（所有 provider 共用一份，与 LLMConfig.Prompt 对称）。
	// 不可为空：空时 Describe 返回错误、保存时 UI 拦截。Default 为 DefaultVLMPrompt。
	Prompt string `yaml:"prompt"`
	// RecognitionIntervalS 是识别 worker 扫描 pending 任务队列的间隔（秒）。
	// 采集与识别解耦后，识别由独立 worker 按此间隔扫描 screenshots/pending/ 里的
	// 待识别截图，逐条调 VLM。≤0 时用默认 300（5分钟）。
	// 调小可加快积压任务消化，但会增加 API 调用频率（可能加剧 429）。
	// 仅 yaml 配置，不进设置页/proto/前端（高级参数）。
	RecognitionIntervalS int `yaml:"recognition_interval_s"`
	// SaveScreenshots 由 main.go 从 cfg.Debug.SaveVLMScreenshots 注入（非 yaml 字段）。
	// true 时送去 VLM 分析的截图落盘到 screenshots/ 目录供调试（文件名无 -mv- 前缀）。
	SaveScreenshots bool `yaml:"-"`
}

// LLMProvider 是单个 LLM/Polish 供应商配置。
type LLMProvider struct {
	Name      string `yaml:"name"`
	BaseURL   string `yaml:"base_url"`
	Model     string `yaml:"model"`
	APIKey    string `yaml:"api_key"`
	AuthType  string `yaml:"auth_type"`  // bearer | api-key
	APIFormat string `yaml:"api_format"` // openai | anthropic
	NumCtx    int    `yaml:"num_ctx"`
	Type      string `yaml:"type"` // cloud | local
	// RetryCount 是云请求 HTTP 重试次数（覆盖 429/5xx/网络错误，含指数退避）。
	// 0 = 不重试；3 = 默认。由 httputil.DoWithRetry 读取。
	// type=local 的 provider 不使用此字段。
	RetryCount int `yaml:"retry_count"`
}

// LLMConfig 是 LLM(Polish) 配置块。
type LLMConfig struct {
	Enabled        string                 `yaml:"enabled"` // on | off
	ActiveProvider string                 `yaml:"active_provider"`
	Providers      map[string]LLMProvider `yaml:"providers"`
	Prompt         string                 `yaml:"prompt"`
	InjectMode     string                 `yaml:"inject_mode"` // preview | auto
}

// HotkeyConfig 是热键配置块。
type HotkeyConfig struct {
	Record       string `yaml:"record"`
	Screenshot   string `yaml:"screenshot"`
	PromptPrefix string `yaml:"prompt_prefix"` // Ctrl | Alt | Win
}

// ScreenshotConfig 是"快捷工具-桌面截图"配置块。
type ScreenshotConfig struct {
	// WithVLM: 区域截图完成后是否自动触发一次 VLM 截图理解，把摘要写进时间线。
	// 默认 false（纯截图落盘 + 写剪贴板，不调 VLM）。
	WithVLM bool `yaml:"with_vlm"`
	// Prompt 是桌面截图识别专用提示词（与 vlm.prompt 区分）。
	// 适用于"完成（自动识别）"和"识别"按钮两条路径。空时回落 DefaultScreenshotPrompt。
	Prompt string `yaml:"prompt"`
}

// UpdateConfig 是软件更新配置块。
type UpdateConfig struct {
	// ServerURL 是更新服务器地址，空表示不检查更新。
	ServerURL string `yaml:"server_url"`
	// CheckOnStartup 是否在客户端启动时检查一次更新。
	CheckOnStartup bool `yaml:"check_on_startup"`
	// CheckDaily 是否每天后台检查一次更新。
	CheckDaily bool `yaml:"check_daily"`
	// Channel 更新通道：stable | beta。
	Channel string `yaml:"channel"`
}

// MovementConfig 是采集配置块。
type MovementConfig struct {
	SampleIntervalMs int    `yaml:"sample_interval_ms"`
	IdleTimeoutS     int    `yaml:"idle_timeout_s"` // 保留供设置页回显;collector 实际使用 InputIdleS/DisplayIdleS 两层超时
	Precision        string `yaml:"precision"`
	// InputIdleS:输入超时。GetLastInputInfo 显示近该秒数内有输入 → 判为强信号。
	// DisplayIdleS:展示超时。距上次 engaged 超过该秒数 → 从 active 退化为 idle。
	// 两者为 0 时 NewCollector 用 Precision 对应 Preset 的默认值。
	InputIdleS   int `yaml:"input_idle_s"`
	DisplayIdleS int `yaml:"display_idle_s"`
	// AwayThresholdS:离开阈值。idle 持续超该秒数 → 判定"离开"，
	// 结束当前段（不再写覆盖整个离开期间的 idle 段），回来后开新段。
	// 短 idle（< 阈值）仍视为"思考"，段不断（见 movement.go 的段合并语义）。
	// 为 0 时 NewCollector 用 Preset 默认值（600=10min）。
	AwayThresholdS int `yaml:"away_threshold_s"`
	// PauseOnLock:锁屏时是否暂停采集并把当前段收尾（判定离开）。
	// true 时锁屏等价于“离开”，解锁后恢复并开新段。
	PauseOnLock bool `yaml:"pause_on_lock"`
	// InputActiveS:输入活跃阈值。近该秒数内有键鼠输入 → 判定"正在打字"，
	// 跳过帧差截图(s1)，只信键鼠信号(s2)。
	// 原因：PrintWindow 是同步跨进程 GDI 调用，会阻塞目标窗口 UI 线程做合成
	// 重绘，对 Electron 应用(VS Code/ZCode)每 300ms 截一次会让打字卡顿/丢字。
	// 而打字时 s2 已证明"在用电脑"，帧差信息完全冗余；帧差唯一不可替代的场景
	// 是"无输入但画面在变"(看视频)，那种场景用户本就不打字。故输入活跃时跳过
	// 截图功能无损，且消除卡顿。
	// 必须小于 InputIdleS（否则 s1 永远被 s2 的宽限期罩住，帧差永不工作）。
	// 为 0 时 NewCollector 用 Preset 默认值（medium=8s）。
	InputActiveS int `yaml:"input_active_s"`
	// SaveScreenshots 由 main.go 从 cfg.Debug.SaveMotionScreenshots 注入（非 yaml 字段）。
	// true 时 movement 活动窗口帧落盘到 screenshots/ 目录供调试（文件名带 -mv- 前缀）。
	SaveScreenshots bool `yaml:"-"`
}

// Config 是整体配置。
type Config struct {
	ASR        ASRConfig        `yaml:"asr"`
	VLM        VLMConfig        `yaml:"vlm"`
	LLM        LLMConfig        `yaml:"llm"`
	Movement   MovementConfig   `yaml:"movement"`
	Hotkeys    HotkeyConfig     `yaml:"hotkeys"`
	Screenshot ScreenshotConfig `yaml:"screenshot"`
	Update     UpdateConfig     `yaml:"update"`
	Hotwords   []string         `yaml:"hotwords"`
	Log        LogConfig        `yaml:"log"`
	Debug      DebugConfig      `yaml:"debug"`
}

// LogConfig 控制后端日志输出。排查采集/识别问题时设 level=debug。
//
// 日志按天滚动：每天一个 shadow-worker-YYYY-MM-DD.log，写到 logs/ 目录。
// 旧文件按 RetentionDays 自动清理（默认保留 7 天）。
// level: debug（全量，含每 tick 强信号/state 翻转）| info（关键事件）| warn（仅警告+错误）。
type LogConfig struct {
	// Level 日志级别：debug | info | warn。默认 info。
	Level string `yaml:"level"`
	// Console 是否同时输出到控制台（stderr）。默认 true。
	// 后端是控制台程序时有用；GUI 后台运行时关掉省资源。
	Console bool `yaml:"console"`
	// RetentionDays 日志文件保留天数，超过自动清理。默认 7。
	RetentionDays int `yaml:"retention_days"`
}

// DebugConfig 是调试开关。默认全部关闭，不影响正常使用。
// 两类截图独立控制，按需开启，避免互相干扰：
//   - SaveVLMScreenshots: 只保存"真正送去 VLM 分析"的截图（文件名 <时分秒>-<app>.png）。
//     排查"VLM 识别内容对不对"时开这个——能直接比对截图内容和 VLM 摘要是否一致。
//   - SaveMotionScreenshots: 只保存"帧差判定用的活动窗口帧"（文件名 <时分秒>-mv-<app>.png）。
//     排查 movement 采集/帧差判定问题时开这个。注意：Electron 类应用 GPU 动画频繁，
//     会高频落盘（每秒数张），仅短时排查用。
type DebugConfig struct {
	// SaveVLMScreenshots 只保存送去 VLM 分析的截图（推荐排查用，量小）。
	SaveVLMScreenshots bool `yaml:"save_vlm_screenshots"`
	// SaveMotionScreenshots 只保存帧差判定的活动窗口帧（高频，仅短时排查 movement 用）。
	SaveMotionScreenshots bool `yaml:"save_motion_screenshots"`
}

// Default 返回默认配置。
func Default() *Config {
	return &Config{
		ASR: ASRConfig{
			Mode:           "cloud",
			ActiveProvider: "",
			// 初始无任何 provider：用户必须点 Add Model 添加。
			// AddModel 选预设厂商（MIMO/GLM 等）时自动填充 model/baseURL/authType。
			Providers: map[string]ASRProvider{},
			Local: LocalASRConfig{
				ModelPath: "models/ggml-tiny.bin",
				ModelName: "tiny",
				Language:  "zh",
			},
			RecordMode: "hold",
		},
		VLM: VLMConfig{
			Mode:                "off",
			ActiveProvider:      "",
			ScheduleIntervalMin: 5,
			CaptureRange:        "active",
			// on_demand 默认冷却：切窗口 20s（过滤 Alt+Tab 抖动），活跃点 60s。
			OnDemandSwitchGapS: 20,
			OnDemandMotionGapS: 60,
			// 识别 worker 默认5分钟扫一次 pending 队列。
			RecognitionIntervalS: 300,
			// 默认提示词，用户可改但不可清空（空时引擎拒绝分析）。
			Prompt: DefaultVLMPrompt,
			// 初始无 provider，用户通过 Add Model 添加。
			Providers: map[string]VLMProvider{},
		},
		LLM: LLMConfig{
			Enabled:        "off",
			ActiveProvider: "",
			// 初始无 provider，用户通过 Add Model 添加。
			Providers:  map[string]LLMProvider{},
			Prompt:     defaultPrompt,
			InjectMode: "preview",
		},
		Movement: MovementConfig{
			SampleIntervalMs: 300,
			IdleTimeoutS:     10,
			Precision:        "medium",
			InputIdleS:       15,
			DisplayIdleS:     90,
			// idle 超 10 分钟判为离开（吃饭/开会），结束当前段。
			// 短 idle（看文档/思考）仍不打断段。
			AwayThresholdS: 600,
			PauseOnLock:    true,
			// 8 秒内有键鼠输入 = 正在打字，跳过帧差截图避免 PrintWindow 卡顿。
			// 小于 InputIdleS(15s)，确保"打字停了再留点宽限才恢复帧差检测"。
			InputActiveS: 8,
		},
		Hotkeys: HotkeyConfig{
			Record:       "F9",
			Screenshot:   "",
			PromptPrefix: "Ctrl",
		},
		Screenshot: ScreenshotConfig{
			WithVLM: false, // 默认纯截图，不调 VLM
			// 桌面截图识别专用提示词默认值（与 vlm.prompt 区分）。
			Prompt: DefaultScreenshotPrompt,
		},
		Update: UpdateConfig{
			ServerURL:      "",
			CheckOnStartup: true,
			CheckDaily:     true,
			Channel:        "stable",
		},
		Hotwords: []string{},
		Log: LogConfig{
			Level:         "info",
			Console:       true,
			RetentionDays: 7,
		},
	}
}

// Load 从配置文件加载配置,不存在则创建默认配置。
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := Default()
		if err := cfg.Save(); err != nil {
			return nil, fmt.Errorf("创建默认配置失败: %w", err)
		}
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	cfg.NormalizeRetryCount()
	return cfg, nil
}

// DefaultRetryCount 是 provider 云请求 HTTP 重试次数的默认值。
// 与 httputil.DefaultMaxRetries 保持一致；旧配置升级（无 retry_count 字段 → 零值）
// 或用户填 0 时由 normalizeRetryCount 规范化到此值。
const DefaultRetryCount = 3

// NormalizeRetryCount 把所有 provider 的 RetryCount ≤ 0 规范化为 DefaultRetryCount。
// 在 Load 和 protoToConfig 末尾调用，确保：
//   - 旧配置升级（无 retry_count 字段 → 零值）自动获得默认值
//   - 用户填 0 或负数时回落默认（不写 0 进 DoWithRetry 触发"不重试"语义，避免误操作）
//   - 全链路一致：config.yaml 有值、UI 显示默认、后端收到默认
//
// 用户想"完全不重试"可在 UI 填 1（只发 1 次请求，0 次重试）。
func (c *Config) NormalizeRetryCount() {
	for k, p := range c.ASR.Providers {
		if p.RetryCount <= 0 {
			p.RetryCount = DefaultRetryCount
			c.ASR.Providers[k] = p
		}
	}
	for k, p := range c.VLM.Providers {
		if p.RetryCount <= 0 {
			p.RetryCount = DefaultRetryCount
			c.VLM.Providers[k] = p
		}
	}
	for k, p := range c.LLM.Providers {
		if p.RetryCount <= 0 {
			p.RetryCount = DefaultRetryCount
			c.LLM.Providers[k] = p
		}
	}
}

// Save 保存配置到文件。
func (c *Config) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("写入配置失败: %w", err)
	}
	return nil
}

// configPath 返回配置文件路径。
func configPath() (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("获取配置目录失败: %w", err)
	}
	return filepath.Join(cfgDir, "shadow-worker", "config.yaml"), nil
}

// ConfigPath 是 configPath 的导出版本，供"系统设置-配置文件路径"展示用。
func ConfigPath() (string, error) {
	return configPath()
}

// GetASRProvider 返回当前激活的 ASR provider。
func (c *Config) GetASRProvider() (ASRProvider, bool) {
	p, ok := c.ASR.Providers[c.ASR.ActiveProvider]
	return p, ok
}

// GetVLMProvider 返回当前激活的 VLM provider。
func (c *Config) GetVLMProvider() (VLMProvider, bool) {
	p, ok := c.VLM.Providers[c.VLM.ActiveProvider]
	return p, ok
}

// GetLLMProvider 返回当前激活的 LLM provider。
func (c *Config) GetLLMProvider() (LLMProvider, bool) {
	p, ok := c.LLM.Providers[c.LLM.ActiveProvider]
	return p, ok
}
