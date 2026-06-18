// Package asr 是 Shadow Worker 的语音识别引擎抽象。
//
// 当前支持:
//   - cloud: OpenAI 兼容的 /audio/transcriptions SSE 接口
//   - local: whisper.cpp 本地模型(接口已留,模型由用户后续导入)
//
// 所有引擎统一接收完整 PCM(16kHz/mono/int16),返回识别文本。
package asr

import (
	"context"
	"fmt"
	"strings"

	"shadow-worker/backend/internal/config"
)

// SampleRate / BitsPerSample / Channels 定义 ASR 输入音频格式。
const (
	SampleRate    = 16000
	BitsPerSample = 16
	Channels      = 1
)

// Result 是一次识别结果。
type Result struct {
	Partial string // 中间结果(可能为空)
	Final   string // 最终结果
	Done    bool   // 是否结束
}

// Engine 是 ASR 引擎接口。
type Engine interface {
	Name() string
	Recognize(ctx context.Context, pcm []byte) (string, error)
}

// StreamingEngine 是可选的流式引擎接口。
// 支持 partial 回调的引擎可以实现此接口。
type StreamingEngine interface {
	Engine
	RecognizeStreaming(ctx context.Context, pcm []byte, onPartial func(string)) (string, error)
}

// New 根据配置创建 ASR 引擎。
func New(cfg *config.Config) (Engine, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config 不能为空")
	}

	switch strings.ToLower(cfg.ASR.Mode) {
	case "local":
		return newLocalEngine(cfg.ASR.Local, cfg.Hotwords)
	case "cloud", "":
		p, ok := cfg.GetASRProvider()
		if !ok {
			return nil, fmt.Errorf("未找到 ASR provider: %s", cfg.ASR.ActiveProvider)
		}
		return newCloudEngine(p, cfg.Hotwords)
	default:
		return nil, fmt.Errorf("不支持的 ASR 模式: %s", cfg.ASR.Mode)
	}
}
