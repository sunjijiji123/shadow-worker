package asr

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"

	"shadow-worker/backend/internal/config"
)

// localEngine runs whisper.cpp inference locally via the official Go binding
// (CGO). The .bin model is loaded once per engine and reused for every
// Recognize call. PCM is 16kHz mono int16 LE, converted to float32 [-1,1].
type localEngine struct {
	cfg      config.LocalASRConfig
	hotwords []string
	model    whisper.Model
	logger   *slog.Logger
}

func newLocalEngine(cfg config.LocalASRConfig, hotwords []string, logger *slog.Logger) (Engine, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.ModelPath == "" {
		return nil, fmt.Errorf("local ASR: model_path is empty")
	}
	if _, err := os.Stat(cfg.ModelPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("local ASR: model not found: %s", cfg.ModelPath)
	}

	m, err := whisper.New(cfg.ModelPath)
	if err != nil {
		return nil, fmt.Errorf("local ASR: failed to load model %s: %w",
			cfg.ModelPath, err)
	}
	logger.Info("本地 whisper 模型已加载", "model", cfg.ModelPath)

	return &localEngine{
		cfg:      cfg,
		hotwords: hotwords,
		model:    m,
		logger:   logger,
	}, nil
}

func (e *localEngine) Name() string {
	return fmt.Sprintf("local-whisper (%s)", e.cfg.ModelName)
}

func (e *localEngine) Recognize(ctx context.Context, pcm []byte) (string, error) {
	if len(pcm) < 320 { // < 10ms
		return "", nil
	}

	// int16 LE PCM → float32 [-1, 1]
	samples := make([]float32, len(pcm)/2)
	for i := range samples {
		s := int16(binary.LittleEndian.Uint16(pcm[i*2 : i*2+2]))
		samples[i] = float32(s) / 32768.0
	}

	// create a fresh context per call (thread-safe; model is shared)
	wctx, err := e.model.NewContext()
	if err != nil {
		return "", fmt.Errorf("whisper NewContext: %w", err)
	}

	// configure
	threads := runtime.NumCPU()
	if threads > 8 {
		threads = 8
	}
	wctx.SetThreads(uint(threads))

	// language
	lang := e.cfg.Language
	if lang == "" {
		lang = "auto"
	}
	if err := wctx.SetLanguage(lang); err != nil {
		// 可恢复：回退到自动语言检测，识别继续。
		e.logger.Warn("SetLanguage 失败，回退自动检测", "lang", lang, "err", err)
	}

	// hotwords as initial prompt
	if len(e.hotwords) > 0 {
		prompt := strings.Join(e.hotwords, ",")
		if len(prompt) > 200 {
			prompt = prompt[:200]
		}
		wctx.SetInitialPrompt(prompt)
	}

	// run inference
	if err := wctx.Process(samples, nil, nil, nil); err != nil {
		return "", fmt.Errorf("whisper Process: %w", err)
	}

	// collect segments
	var b strings.Builder
	for {
		seg, err := wctx.NextSegment()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("whisper NextSegment: %w", err)
		}
		b.WriteString(seg.Text)
	}

	text := strings.TrimSpace(b.String())
	return text, nil
}

func (e *localEngine) RecognizeStreaming(ctx context.Context, pcm []byte, onPartial func(string)) (string, error) {
	// For local engine, streaming just calls Recognize with a segment callback
	// that forwards partial text.
	if len(pcm) < 320 {
		return "", nil
	}
	samples := make([]float32, len(pcm)/2)
	for i := range samples {
		s := int16(binary.LittleEndian.Uint16(pcm[i*2 : i*2+2]))
		samples[i] = float32(s) / 32768.0
	}

	wctx, err := e.model.NewContext()
	if err != nil {
		return "", err
	}
	threads := runtime.NumCPU()
	if threads > 8 {
		threads = 8
	}
	wctx.SetThreads(uint(threads))
	if lang := e.cfg.Language; lang != "" {
		wctx.SetLanguage(lang)
	}

	var segCb whisper.SegmentCallback
	if onPartial != nil {
		segCb = func(seg whisper.Segment) {
			onPartial(seg.Text)
		}
	}

	if err := wctx.Process(samples, nil, segCb, nil); err != nil {
		return "", err
	}

	var b strings.Builder
	for {
		seg, err := wctx.NextSegment()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		b.WriteString(seg.Text)
	}
	return strings.TrimSpace(b.String()), nil
}
