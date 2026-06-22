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
	"log/slog"

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
	return newCloudEngine(cfg, "test", nil)
}

// New 根据配置创建润色引擎。
//
// 引擎是否创建只取决于"是否配置了有效的 LLM provider"，而**不**取决于
// cfg.LLM.Enabled（那是"自动润色"开关，只控制识别后是否自动触发润色，
// 不应影响手动润色的可用性）。provider 找不到时返回 error（main.go 据此
// 跳过引擎创建，holder 为 nil，手动润色返回"LLM 未启用"）。
//
// 这样语义：配了 provider → 引擎存在 → 手动 Polish 永远可用；
// 关闭"自动润色" → 仅不自动触发，手动点 Polish 仍生效。
//
// logger 为 nil 时回退到 slog.Default()。当前 cloud 引擎内部未直接使用 logger
// （润色走 httputil.DoWithRetry，那里有自己的 logger），保留参数为将来扩展
// 并与 asr.New 构造签名保持一致。
func New(cfg *config.Config, logger *slog.Logger) (Engine, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config 不能为空")
	}
	if logger == nil {
		logger = slog.Default()
	}
	p, ok := cfg.GetLLMProvider()
	if !ok {
		return nil, fmt.Errorf("未找到 LLM provider: %s", cfg.LLM.ActiveProvider)
	}
	return newCloudEngine(p, cfg.LLM.Prompt, logger)
}
