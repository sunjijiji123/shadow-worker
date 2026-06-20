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
	IdleTimeoutS     int    `yaml:"idle_timeout_s"`
	Precision        string `yaml:"precision"`
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
			ActiveProvider: "custom",
			Providers: map[string]ASRProvider{
				"custom": {
					Name:      "自定义 OpenAI 兼容",
					BaseURL:   "https://api.openai.com/v1",
					Model:     "whisper-1",
					AuthType:  "bearer",
					APIFormat: "openai",
					Language:  "zh",
				},
			},
			Local: LocalASRConfig{
				ModelPath: "models/ggml-tiny.bin",
				ModelName: "tiny",
				Language:  "zh",
			},
			RecordMode: "hold",
		},
		VLM: VLMConfig{
			Mode:                "off",
			ActiveProvider:      "openai",
			ScheduleIntervalMin: 5,
			Providers: map[string]VLMProvider{
				"openai": {
					Name:      "OpenAI 兼容",
					BaseURL:   "https://api.openai.com/v1",
					Model:     "gpt-4o",
					AuthType:  "bearer",
					APIFormat: "openai",
				},
				"ollama": {
					Name:      "Ollama 本地",
					BaseURL:   "http://127.0.0.1:11434",
					Model:     "llava",
					AuthType:  "",
					APIFormat: "ollama",
				},
			},
		},
		LLM: LLMConfig{
			Enabled:        "off",
			ActiveProvider: "openai",
			Providers: map[string]LLMProvider{
				"openai": {
					Name:      "OpenAI 兼容",
					BaseURL:   "https://api.openai.com/v1",
					Model:     "gpt-4o",
					AuthType:  "bearer",
					APIFormat: "openai",
				},
				"ollama": {
					Name:      "Ollama 本地",
					BaseURL:   "http://127.0.0.1:11434",
					Model:     "qwen2.5",
					AuthType:  "",
					APIFormat: "openai",
				},
			},
			Prompt:     defaultPrompt,
			InjectMode: "preview",
		},
		Movement: MovementConfig{
			SampleIntervalMs: 300,
			IdleTimeoutS:     10,
			Precision:        "medium",
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
