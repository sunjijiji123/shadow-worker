// Package llm 是 Shadow Worker 的文字润色引擎抽象。
//
// 对 ASR 识别出的文字调用云端 LLM（OpenAI 兼容 /chat/completions）做润色，
// 例如去口语填充词、加标点、整理成流畅文字。润色 prompt 来自配置 llm.prompt
// （默认见 internal/config/default_prompt.txt）。
//
// 所有引擎统一接收一段文本，返回润色后的文本。
package llm

import (
	"context"
	"fmt"

	"shadow-worker/backend/internal/config"
)

// Engine 是润色引擎接口。
type Engine interface {
	Name() string
	Polish(ctx context.Context, text string) (string, error)
}

// NewCloudEngineForTest 暴露给 grpcapi 用作临时连接测试，不依赖完整 config。
// prompt 用一个最小占位，只验证连通性。
func NewCloudEngineForTest(cfg config.LLMProvider) (Engine, error) {
	return newCloudEngine(cfg, "test")
}

// New 根据配置创建润色引擎。
//
// LLM 未启用时返回 nil（调用方据此跳过润色）。provider 找不到时返回 error。
func New(cfg *config.Config) (Engine, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config 不能为空")
	}
	if cfg.LLM.Enabled != "on" {
		return nil, nil // 润色未启用，返回 nil（非错误）
	}
	p, ok := cfg.GetLLMProvider()
	if !ok {
		return nil, fmt.Errorf("未找到 LLM provider: %s", cfg.LLM.ActiveProvider)
	}
	return newCloudEngine(p, cfg.LLM.Prompt)
}
