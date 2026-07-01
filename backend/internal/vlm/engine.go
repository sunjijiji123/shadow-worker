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
	_ "embed"
	"fmt"
	"strings"

	"shadow-worker/backend/internal/config"
)

// ProbePNG 是 Test Connection 用的固定测试图（test_probe.png，编译期内嵌）。
// 比 1×1 黑图更有意义：GLM 会对它返回真实描述，用户能在 toast 里看到 VLM
// 到底"看懂了什么"，从而同时验证连通性和理解效果。体积仅 ~500B，上传可忽略，
// 总链路耗时仍由 GLM 推理（2~8s）主导，与黑图探测基本持平。
//
//go:embed test_probe.png
var ProbePNG []byte

// Engine 是 VLM 引擎接口。
type Engine interface {
	Name() string
	// Describe 对一张截图进行文字描述。
	Describe(ctx context.Context, imagePNG []byte) (string, error)
}

// NewCloudEngineForTest 暴露给 grpcapi 用作临时连接测试，不依赖完整 config。
// prompt 取自当前服务端配置（voice_server 传入 s.cfg.VLM.Prompt），空时引擎兜底回落默认。
func NewCloudEngineForTest(cfg config.VLMProvider, prompt string) (Engine, error) {
	return newCloudEngine(cfg, prompt)
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
		return newCloudEngine(p, cfg.Prompt)
	case "ollama":
		p, ok := cfg.Providers[cfg.ActiveProvider]
		if !ok {
			return nil, fmt.Errorf("未找到 VLM provider: %s", cfg.ActiveProvider)
		}
		return newOllamaEngine(p, cfg.Prompt)
	default:
		return nil, fmt.Errorf("不支持的 VLM 模式: %s", cfg.Mode)
	}
}
