package grpcapi

import (
	"context"
	"fmt"
	"log"

	"shadow-worker/backend/internal/asr"
	"shadow-worker/backend/internal/config"
	"shadow-worker/backend/internal/llm"
)

// ConfigServer 实现 ConfigService。
type ConfigServer struct {
	UnimplementedConfigServiceServer
	cfg       *config.Config
	holder    *asr.EngineHolder // 保存配置后热重载 ASR 引擎
	llmHolder *llm.EngineHolder // 保存配置后热重载润色引擎
}

// NewConfigServer 创建 ConfigServer。holder/llmHolder 用于配置变更后热重载引擎。
func NewConfigServer(cfg *config.Config, holder *asr.EngineHolder, llmHolder *llm.EngineHolder) *ConfigServer {
	return &ConfigServer{cfg: cfg, holder: holder, llmHolder: llmHolder}
}

// GetConfig 返回当前配置。
func (s *ConfigServer) GetConfig(ctx context.Context, req *GetConfigRequest) (*ConfigData, error) {
	return configToProto(s.cfg), nil
}

// SaveConfig 保存配置。落盘 + 更新内存配置 + 热重载 ASR 引擎。
//
// 引擎重建失败（如模型路径无效）不会阻断保存：配置已写入 YAML 和内存，
// 用户下次修正配置后再次保存即可重建。旧引擎继续服务直到重建成功。
func (s *ConfigServer) SaveConfig(ctx context.Context, req *ConfigData) (*Result, error) {
	newCfg := protoToConfig(req)
	if err := newCfg.Save(); err != nil {
		return nil, fmt.Errorf("保存配置失败: %w", err)
	}
	*s.cfg = *newCfg

	// 热重载 ASR 引擎（失败仅记录，不阻断保存）。
	if s.holder != nil {
		if err := s.holder.Rebuild(s.cfg); err != nil {
			log.Printf("[config] ASR 引擎热重载失败（配置已保存）: %v", err)
		}
	}
	// 热重载润色引擎（失败仅记录，不阻断保存）。
	if s.llmHolder != nil {
		if err := s.llmHolder.Rebuild(s.cfg); err != nil {
			log.Printf("[config] LLM 引擎热重载失败（配置已保存）: %v", err)
		}
	}
	return &Result{Ok: true}, nil
}

func providerToProto(p config.ASRProvider) *ProviderConfig {
	return &ProviderConfig{
		Name:           p.Name,
		BaseUrl:        p.BaseURL,
		Model:          p.Model,
		ApiKey:         p.APIKey,
		AuthType:       p.AuthType,
		ApiFormat:      p.APIFormat,
		NumCtx:         int32(p.NumCtx),
		Type:           p.Type,
		Language:       p.Language,
		Stream:         p.Stream,
		LocalModelPath: p.LocalModelPath,
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
		Type:      p.Type,
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
		Type:      p.Type,
	}
}

func configToProto(cfg *config.Config) *ConfigData {
	data := &ConfigData{
		AsrMode:                  cfg.ASR.Mode,
		AsrActiveProvider:        cfg.ASR.ActiveProvider,
		AsrProviders:             make(map[string]*ProviderConfig),
		AsrLocalModelPath:        cfg.ASR.Local.ModelPath,
		AsrLocalModelName:        cfg.ASR.Local.ModelName,
		AsrLocalLanguage:         cfg.ASR.Local.Language,
		AsrRecordMode:            cfg.ASR.RecordMode,
		PolishEnabled:            cfg.LLM.Enabled == "on",
		PolishActiveProvider:     cfg.LLM.ActiveProvider,
		PolishProviders:          make(map[string]*ProviderConfig),
		PolishPrompt:             cfg.LLM.Prompt,
		InjectMode:               cfg.LLM.InjectMode,
		MovementSampleIntervalMs: int32(cfg.Movement.SampleIntervalMs),
		MovementIdleTimeoutS:     int32(cfg.Movement.IdleTimeoutS),
		MovementPrecision:        cfg.Movement.Precision,
		MovementInputIdleS:       int32(cfg.Movement.InputIdleS),
		MovementDisplayIdleS:     int32(cfg.Movement.DisplayIdleS),
		VlmMode:                  cfg.VLM.Mode,
		VlmActiveProvider:        cfg.VLM.ActiveProvider,
		VlmProviders:             make(map[string]*ProviderConfig),
		VlmScheduleIntervalMin:   int32(cfg.VLM.ScheduleIntervalMin),
		VlmCaptureRange:          cfg.VLM.CaptureRange,
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
	cfg.ASR.RecordMode = data.AsrRecordMode
	if data.AsrProviders != nil {
		cfg.ASR.Providers = make(map[string]config.ASRProvider)
		for k, p := range data.AsrProviders {
			ptype := p.Type
			if ptype == "" {
				ptype = "cloud" // 兼容旧配置（无 type 字段时默认云端）
			}
			cfg.ASR.Providers[k] = config.ASRProvider{
				Name:           p.Name,
				BaseURL:        p.BaseUrl,
				Model:          p.Model,
				APIKey:         p.ApiKey,
				AuthType:       p.AuthType,
				APIFormat:      p.ApiFormat,
				NumCtx:         int(p.NumCtx),
				Type:           ptype,
				Language:       p.Language,
				Stream:         p.Stream,
				LocalModelPath: p.LocalModelPath,
			}
		}
	}

	cfg.VLM.Mode = data.VlmMode
	cfg.VLM.ActiveProvider = data.VlmActiveProvider
	cfg.VLM.ScheduleIntervalMin = int(data.VlmScheduleIntervalMin)
	// capture_range 兼容旧配置：空值回落到默认 active
	if cr := data.VlmCaptureRange; cr == "screen" {
		cfg.VLM.CaptureRange = "screen"
	} else {
		cfg.VLM.CaptureRange = "active"
	}
	if data.VlmProviders != nil {
		cfg.VLM.Providers = make(map[string]config.VLMProvider)
		for k, p := range data.VlmProviders {
			ptype := p.Type
			if ptype == "" {
				ptype = "cloud"
			}
			cfg.VLM.Providers[k] = config.VLMProvider{
				Name:      p.Name,
				BaseURL:   p.BaseUrl,
				Model:     p.Model,
				APIKey:    p.ApiKey,
				AuthType:  p.AuthType,
				APIFormat: p.ApiFormat,
				NumCtx:    int(p.NumCtx),
				Type:      ptype,
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
			ptype := p.Type
			if ptype == "" {
				ptype = "cloud"
			}
			cfg.LLM.Providers[k] = config.LLMProvider{
				Name:      p.Name,
				BaseURL:   p.BaseUrl,
				Model:     p.Model,
				APIKey:    p.ApiKey,
				AuthType:  p.AuthType,
				APIFormat: p.ApiFormat,
				NumCtx:    int(p.NumCtx),
				Type:      ptype,
			}
		}
	}

	cfg.Movement.SampleIntervalMs = int(data.MovementSampleIntervalMs)
	cfg.Movement.IdleTimeoutS = int(data.MovementIdleTimeoutS)
	cfg.Movement.Precision = data.MovementPrecision
	cfg.Movement.InputIdleS = int(data.MovementInputIdleS)
	cfg.Movement.DisplayIdleS = int(data.MovementDisplayIdleS)

	cfg.Hotkeys.Record = data.HotkeyRecord
	cfg.Hotkeys.Screenshot = data.HotkeyScreenshot
	cfg.Hotkeys.PromptPrefix = data.HotkeyPromptPrefix

	return cfg
}
