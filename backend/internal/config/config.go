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
}

// VLMConfig 是 VLM 配置块。
type VLMConfig struct {
	Mode                string                 `yaml:"mode"` // scheduled | on_demand | off
	ActiveProvider      string                 `yaml:"active_provider"`
	Providers           map[string]VLMProvider `yaml:"providers"`
	ScheduleIntervalMin int                    `yaml:"schedule_interval_min"`
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
}

// Config 是整体配置。
type Config struct {
	ASR      ASRConfig      `yaml:"asr"`
	VLM      VLMConfig      `yaml:"vlm"`
	LLM      LLMConfig      `yaml:"llm"`
	Movement MovementConfig `yaml:"movement"`
	Hotkeys  HotkeyConfig   `yaml:"hotkeys"`
	Hotwords []string       `yaml:"hotwords"`
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
		},
		Hotkeys: HotkeyConfig{
			Record:       "F9",
			Screenshot:   "",
			PromptPrefix: "Ctrl",
		},
		Hotwords: []string{},
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
	return cfg, nil
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
