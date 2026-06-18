package asr

import (
	"context"
	"fmt"
	"os"

	"shadow-worker/backend/internal/config"
)

// localEngine 是本地 whisper.cpp 引擎的占位实现。
//
// 设计目标:
//   - 接口已经留好,用户后续导入 ggml 模型后替换实现即可
//   - 当前不引入 CGO/whisper.cpp,避免构建依赖模型
//
// TODO: 第 4 周绑定 whisper.cpp (ai-voice-tool/floatwindow/whisper 可复用)
type localEngine struct {
	cfg      config.LocalASRConfig
	hotwords []string
}

func newLocalEngine(cfg config.LocalASRConfig, hotwords []string) (Engine, error) {
	if cfg.ModelPath == "" {
		return nil, fmt.Errorf("local ASR: model_path 不能为空")
	}
	return &localEngine{
		cfg:      cfg,
		hotwords: hotwords,
	}, nil
}

func (e *localEngine) Name() string {
	return fmt.Sprintf("local-whisper (%s)", e.cfg.ModelPath)
}

func (e *localEngine) Recognize(ctx context.Context, pcm []byte) (string, error) {
	if len(pcm) == 0 {
		return "", nil
	}

	// 占位：检查模型文件存在，实际识别返回提示文本
	if _, err := os.Stat(e.cfg.ModelPath); os.IsNotExist(err) {
		return "", fmt.Errorf("本地模型未找到: %s (请导入模型后重试)", e.cfg.ModelPath)
	}

	// TODO: 接入 whisper.cpp 进行真实推理
	return fmt.Sprintf("[local-whisper stub 识别到 %d 采样]", len(pcm)/2), nil
}

func (e *localEngine) RecognizeStreaming(ctx context.Context, pcm []byte, onPartial func(string)) (string, error) {
	return e.Recognize(ctx, pcm)
}
