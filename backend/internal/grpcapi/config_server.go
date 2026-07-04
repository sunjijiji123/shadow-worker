package grpcapi

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"shadow-worker/backend/internal/asr"
	"shadow-worker/backend/internal/collector"
	"shadow-worker/backend/internal/config"
	"shadow-worker/backend/internal/llm"
	"shadow-worker/backend/internal/storage"
)

// ConfigServer 实现 ConfigService。
type ConfigServer struct {
	UnimplementedConfigServiceServer
	cfg       *config.Config
	holder    *asr.EngineHolder     // 保存配置后热重载 ASR 引擎
	llmHolder *llm.EngineHolder     // 保存配置后热重载润色引擎
	vlmHolder *collector.VLMHolder  // 保存配置后热重载 VLM 采集器（含定时器/消费协程）
	db        *storage.DB           // VLM 热重载重建 capturer 时需要
	logger    *slog.Logger          // VLM 热重载日志
}

// NewConfigServer 创建 ConfigServer。holder/llmHolder/vlmHolder 用于配置变更后热重载引擎。
// db/logger 供 VLMHolder.Rebuild 重建 capturer 使用。
func NewConfigServer(cfg *config.Config, holder *asr.EngineHolder, llmHolder *llm.EngineHolder, vlmHolder *collector.VLMHolder, db *storage.DB, logger *slog.Logger) *ConfigServer {
	return &ConfigServer{cfg: cfg, holder: holder, llmHolder: llmHolder, vlmHolder: vlmHolder, db: db, logger: logger}
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

	// Log/Debug 段和 *SaveScreenshots 字段是"开发者级/运行时注入"配置，不通过
	// 设置页 UI 往返（proto 里没有这些字段，buildConfig 不填）。protoToConfig 转换后
	// 它们是 config.Default() 的零值（false/空），若直接 Save 会把 yaml 里手改的
	// log.level / debug.save_vlm_screenshots 覆盖成零值，并丢失 main.go 启动时注入的
	// SaveScreenshots（导致 on_demand 截图不落盘）。保留当前内存配置（s.cfg，启动时
	// Load + main.go 注入的正确值），不参与 UI 往返。
	newCfg.Log = s.cfg.Log
	newCfg.Debug = s.cfg.Debug
	newCfg.VLM.SaveScreenshots = s.cfg.VLM.SaveScreenshots
	newCfg.Movement.SaveScreenshots = s.cfg.Movement.SaveScreenshots

	if err := newCfg.Save(); err != nil {
		return nil, fmt.Errorf("保存配置失败: %w", err)
	}
	*s.cfg = *newCfg

	// 热重载 ASR 引擎（失败仅记录，不阻断保存）。
	// 严重故障：配置已保存到 DB，但引擎没换成——用户录音会继续用旧引擎
	// （可能已不匹配新配置）。holder.Rebuild 内部也会打一条，这里补充"配置已保存"上下文。
	if s.holder != nil {
		if err := s.holder.Rebuild(s.cfg); err != nil {
			s.logger.Error("ASR 热重载失败（配置已保存）", "err", err)
		}
	}
	// 热重载润色引擎（失败仅记录，不阻断保存）。
	if s.llmHolder != nil {
		if err := s.llmHolder.Rebuild(s.cfg); err != nil {
			s.logger.Error("LLM 热重载失败（配置已保存）", "err", err)
		}
	}
	// 热重载 VLM 采集器（失败仅记录，不阻断保存）。与 ASR/LLM 不同：
	// VLMCapturer 有定时器/消费协程生命周期，需整体重建（先启新后停旧）。
	// screen+on_demand 非法组合在此降级为不启动（Rebuild 内处理）。
	if s.vlmHolder != nil {
		if err := s.vlmHolder.Rebuild(s.cfg.VLM, s.db, s.logger); err != nil {
			s.logger.Error("VLM 热重载失败（配置已保存）", "err", err)
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
		MovementAwayThresholdS:   int32(cfg.Movement.AwayThresholdS),

		LogLevel:        cfg.Log.Level,
		LogConsole:      cfg.Log.Console,
		LogRetentionDays: int32(cfg.Log.RetentionDays),
		VlmMode:                  cfg.VLM.Mode,
		VlmActiveProvider:        cfg.VLM.ActiveProvider,
		VlmProviders:             make(map[string]*ProviderConfig),
		VlmScheduleIntervalMin:   int32(cfg.VLM.ScheduleIntervalMin),
		VlmCaptureRange:          cfg.VLM.CaptureRange,
		VlmOnDemandSwitchGapS:    int32(cfg.VLM.OnDemandSwitchGapS),
		VlmOnDemandMotionGapS:    int32(cfg.VLM.OnDemandMotionGapS),
		VlmPrompt:                cfg.VLM.Prompt,
		McpEnabled:               true,
		HotkeyRecord:             cfg.Hotkeys.Record,
		HotkeyScreenshot:         cfg.Hotkeys.Screenshot,
		HotkeyPromptPrefix:       cfg.Hotkeys.PromptPrefix,
		ScreenshotWithVlm:        cfg.Screenshot.WithVLM,
		ScreenshotPrompt:         cfg.Screenshot.Prompt,
		Autostart:                false,
		CollectOnStart:           true,

		UpdateServerUrl:        cfg.Update.ServerURL,
		UpdateCheckOnStartup:   cfg.Update.CheckOnStartup,
		UpdateCheckDaily:       cfg.Update.CheckDaily,
		UpdateChannel:          cfg.Update.Channel,
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
	// on_demand gap 兼容旧配置：≤0 回落默认（switch 20 / motion 60）。
	// 不用 config.Default() 的零值，因为 proto int32 零值 = 未设置。
	cfg.VLM.OnDemandSwitchGapS = normalizeGap(data.VlmOnDemandSwitchGapS, 20)
	cfg.VLM.OnDemandMotionGapS = normalizeGap(data.VlmOnDemandMotionGapS, 60)
	// prompt 不可为空：旧配置/异常数据缺该字段或被清空时回落默认，避免引擎拒绝分析。
	cfg.VLM.Prompt = data.VlmPrompt
	if strings.TrimSpace(cfg.VLM.Prompt) == "" {
		cfg.VLM.Prompt = config.DefaultVLMPrompt
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
	cfg.Movement.AwayThresholdS = int(data.MovementAwayThresholdS)

	cfg.Log.Level = data.LogLevel
	cfg.Log.Console = data.LogConsole
	cfg.Log.RetentionDays = int(data.LogRetentionDays)

	cfg.Hotkeys.Record = data.HotkeyRecord
	cfg.Hotkeys.Screenshot = data.HotkeyScreenshot
	cfg.Hotkeys.PromptPrefix = data.HotkeyPromptPrefix
	cfg.Screenshot.WithVLM = data.ScreenshotWithVlm
	// screenshot.prompt 不可为空：旧配置/异常数据缺该字段或被清空时回落默认，
	// 避免桌面截图识别因空提示词失败（与 vlm.prompt 同范式）。
	cfg.Screenshot.Prompt = data.ScreenshotPrompt
	if strings.TrimSpace(cfg.Screenshot.Prompt) == "" {
		cfg.Screenshot.Prompt = config.DefaultScreenshotPrompt
	}

	// Update 配置：空 channel 回落 stable，兼容旧配置。
	cfg.Update.ServerURL = data.UpdateServerUrl
	cfg.Update.CheckOnStartup = data.UpdateCheckOnStartup
	cfg.Update.CheckDaily = data.UpdateCheckDaily
	cfg.Update.Channel = data.UpdateChannel
	if strings.TrimSpace(cfg.Update.Channel) == "" {
		cfg.Update.Channel = "stable"
	}

	return cfg
}

// normalizeGap 把 proto 回来的 on_demand gap 字段规范化：≤0（未设置/非法）
// 回落到 def 默认值。与 capture_range 的兼容处理同理。
func normalizeGap(v int32, def int) int {
	if v <= 0 {
		return def
	}
	return int(v)
}
