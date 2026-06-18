package grpcapi

import (
	"context"
	"fmt"

	"shadow-worker/backend/internal/config"
)

// ConfigServer 实现 ConfigService。
type ConfigServer struct {
	UnimplementedConfigServiceServer
	cfg *config.Config
}

// NewConfigServer 创建 ConfigServer。
func NewConfigServer(cfg *config.Config) *ConfigServer {
	return &ConfigServer{cfg: cfg}
}

// GetConfig 返回当前配置。
func (s *ConfigServer) GetConfig(ctx context.Context, req *GetConfigRequest) (*ConfigData, error) {
	return configToProto(s.cfg), nil
}

// SaveConfig 保存配置。
func (s *ConfigServer) SaveConfig(ctx context.Context, req *ConfigData) (*Result, error) {
	newCfg := protoToConfig(req)
	if err := newCfg.Save(); err != nil {
		return nil, fmt.Errorf("保存配置失败: %w", err)
	}
	*s.cfg = *newCfg
	return &Result{Ok: true}, nil
}

func providerToProto(p config.ASRProvider) *ProviderConfig {
	return &ProviderConfig{
		Name:      p.Name,
		BaseUrl:   p.BaseURL,
		Model:     p.Model,
		ApiKey:    p.APIKey,
		AuthType:  p.AuthType,
		ApiFormat: p.APIFormat,
		NumCtx:    int32(p.NumCtx),
	}
}

func vlmProviderToProto(p config.VLMProvider) *ProviderConfig {
	return &ProviderConfig{
		Name:      p.Name,
		BaseUrl:   p.BaseURL,
		Model:     p.Model,
		ApiKey:    p.APIKey,
		AuthType:  p.AuthType,
		ApiFormat: p.APIFormat,
		NumCtx:    int32(p.NumCtx),
	}
}

func llmProviderToProto(p config.LLMProvider) *ProviderConfig {
	return &ProviderConfig{
		Name:      p.Name,
		BaseUrl:   p.BaseURL,
		Model:     p.Model,
		ApiKey:    p.APIKey,
		AuthType:  p.AuthType,
		ApiFormat: p.APIFormat,
		NumCtx:    int32(p.NumCtx),
	}
}

func configToProto(cfg *config.Config) *ConfigData {
	data := &ConfigData{
		AsrMode:               cfg.ASR.Mode,
		AsrActiveProvider:     cfg.ASR.ActiveProvider,
		AsrProviders:          make(map[string]*ProviderConfig),
		AsrLocalModelPath:     cfg.ASR.Local.ModelPath,
		AsrLocalModelName:     cfg.ASR.Local.ModelName,
		AsrLocalLanguage:      cfg.ASR.Local.Language,
		PolishEnabled:         cfg.LLM.Enabled == "on",
		PolishActiveProvider:  cfg.LLM.ActiveProvider,
		PolishProviders:       make(map[string]*ProviderConfig),
		PolishPrompt:          cfg.LLM.Prompt,
		InjectMode:            cfg.LLM.InjectMode,
		MovementSampleIntervalMs: int32(cfg.Movement.SampleIntervalMs),
		MovementIdleTimeoutS:     int32(cfg.Movement.IdleTimeoutS),
		MovementPrecision:        cfg.Movement.Precision,
		VlmMode:                  cfg.VLM.Mode,
		VlmActiveProvider:        cfg.VLM.ActiveProvider,
		VlmProviders:             make(map[string]*ProviderConfig),
		VlmScheduleIntervalMin:   int32(cfg.VLM.ScheduleIntervalMin),
		McpEnabled:               true,
		HotkeyRecord:             cfg.Hotkeys.Record,
		HotkeyScreenshot:         cfg.Hotkeys.Screenshot,
		HotkeyPromptPrefix:       cfg.Hotkeys.PromptPrefix,
		Autostart:                false,
		CollectOnStart:           true,
	}

	for k, p := range cfg.ASR.Providers {
		data.AsrProviders[k] = providerToProto(p)
	}
	for k, p := range cfg.VLM.Providers {
		data.VlmProviders[k] = vlmProviderToProto(p)
	}
	for k, p := range cfg.LLM.Providers {
		data.PolishProviders[k] = llmProviderToProto(p)
	}
	return data
}

func protoToConfig(data *ConfigData) *config.Config {
	cfg := config.Default()
	if data == nil {
		return cfg
	}

	cfg.ASR.Mode = data.AsrMode
	cfg.ASR.ActiveProvider = data.AsrActiveProvider
	cfg.ASR.Local.ModelPath = data.AsrLocalModelPath
	cfg.ASR.Local.ModelName = data.AsrLocalModelName
	cfg.ASR.Local.Language = data.AsrLocalLanguage
	if data.AsrProviders != nil {
		cfg.ASR.Providers = make(map[string]config.ASRProvider)
		for k, p := range data.AsrProviders {
			cfg.ASR.Providers[k] = config.ASRProvider{
				Name:      p.Name,
				BaseURL:   p.BaseUrl,
				Model:     p.Model,
				APIKey:    p.ApiKey,
				AuthType:  p.AuthType,
				APIFormat: p.ApiFormat,
				NumCtx:    int(p.NumCtx),
			}
		}
	}

	cfg.VLM.Mode = data.VlmMode
	cfg.VLM.ActiveProvider = data.VlmActiveProvider
	cfg.VLM.ScheduleIntervalMin = int(data.VlmScheduleIntervalMin)
	if data.VlmProviders != nil {
		cfg.VLM.Providers = make(map[string]config.VLMProvider)
		for k, p := range data.VlmProviders {
			cfg.VLM.Providers[k] = config.VLMProvider{
				Name:      p.Name,
				BaseURL:   p.BaseUrl,
				Model:     p.Model,
				APIKey:    p.ApiKey,
				AuthType:  p.AuthType,
				APIFormat: p.ApiFormat,
				NumCtx:    int(p.NumCtx),
			}
		}
	}

	cfg.LLM.Enabled = "off"
	if data.PolishEnabled {
		cfg.LLM.Enabled = "on"
	}
	cfg.LLM.ActiveProvider = data.PolishActiveProvider
	cfg.LLM.Prompt = data.PolishPrompt
	cfg.LLM.InjectMode = data.InjectMode
	if data.PolishProviders != nil {
		cfg.LLM.Providers = make(map[string]config.LLMProvider)
		for k, p := range data.PolishProviders {
			cfg.LLM.Providers[k] = config.LLMProvider{
				Name:      p.Name,
				BaseURL:   p.BaseUrl,
				Model:     p.Model,
				APIKey:    p.ApiKey,
				AuthType:  p.AuthType,
				APIFormat: p.ApiFormat,
				NumCtx:    int(p.NumCtx),
			}
		}
	}

	cfg.Movement.SampleIntervalMs = int(data.MovementSampleIntervalMs)
	cfg.Movement.IdleTimeoutS = int(data.MovementIdleTimeoutS)
	cfg.Movement.Precision = data.MovementPrecision

	cfg.Hotkeys.Record = data.HotkeyRecord
	cfg.Hotkeys.Screenshot = data.HotkeyScreenshot
	cfg.Hotkeys.PromptPrefix = data.HotkeyPromptPrefix

	return cfg
}
