// Package vlm 是 Shadow Worker 的视觉理解引擎抽象。
//
// 当前支持:
//   - cloud: OpenAI 兼容的 /chat/completions 视觉接口
//   - ollama: Ollama /api/generate 本地接口
//
// 所有引擎统一接收 PNG 字节,返回图片描述文本。
package vlm

import (
	"context"
	"fmt"
	"strings"

	"shadow-worker/backend/internal/config"
)

// Engine 是 VLM 引擎接口。
type Engine interface {
	Name() string
	// Describe 对一张截图进行文字描述。
	Describe(ctx context.Context, imagePNG []byte) (string, error)
}

// New 根据配置创建 VLM 引擎。
func New(cfg config.VLMConfig) (Engine, error) {
	switch strings.ToLower(cfg.Mode) {
	case "off", "":
		return nil, fmt.Errorf("VLM 已关闭")
	case "cloud", "scheduled", "on_demand":
		p, ok := cfg.Providers[cfg.ActiveProvider]
		if !ok {
			return nil, fmt.Errorf("未找到 VLM provider: %s", cfg.ActiveProvider)
		}
		return newCloudEngine(p)
	case "ollama":
		p, ok := cfg.Providers[cfg.ActiveProvider]
		if !ok {
			return nil, fmt.Errorf("未找到 VLM provider: %s", cfg.ActiveProvider)
		}
		return newOllamaEngine(p)
	default:
		return nil, fmt.Errorf("不支持的 VLM 模式: %s", cfg.Mode)
	}
}
