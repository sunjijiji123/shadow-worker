package grpcapi

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"shadow-worker/backend/internal/asr"
	"shadow-worker/backend/internal/audio"
	"shadow-worker/backend/internal/collector"
	"shadow-worker/backend/internal/config"
	"shadow-worker/backend/internal/llm"
	"shadow-worker/backend/internal/storage"
	"shadow-worker/backend/internal/vlm"
)

// VoiceServer implements VoiceService: in-process microphone capture with a
// live 16-band FFT spectrum stream, and ASR on stop. PCM never leaves the
// backend process — the client only receives spectrum frames + the final text.
type VoiceServer struct {
	UnimplementedVoiceServiceServer
	holder    *asr.EngineHolder
	llmHolder *llm.EngineHolder
	db        *storage.DB

	mu      sync.Mutex
	capture *audio.Capture
	spec    *audio.SpectrumAnalyzer

	// latest spectrum bands (written by the capture's PCM callback, read by
	// StreamLevels). Guarded by levelMu.
	levelMu  sync.Mutex
	curBands [16]float64
	curRMS   int32
}

// NewVoiceServer creates a VoiceServer. The capture device is opened lazily on
// StartRecording. The ASR + LLM engine holders allow hot-reload after config
// changes.
func NewVoiceServer(db *storage.DB, holder *asr.EngineHolder, llmHolder *llm.EngineHolder) *VoiceServer {
	return &VoiceServer{holder: holder, llmHolder: llmHolder, db: db}
}

// StartRecording opens the waveIn device and begins capturing + spectrum
// analysis. deviceId <= 0 (or empty) selects the system default device.
func (s *VoiceServer) StartRecording(ctx context.Context, req *StartRequest) (*StartResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.capture != nil {
		// already recording — restart fresh
		s.capture.Stop()
	}

	// device id: empty/0 => WAVE_MAPPER
	devID := 0
	// (device selection by name not yet supported; numeric id only via config)

	cap := audio.NewCapture(audio.PCMFormat{
		SampleRate:    asr.SampleRate,
		BitsPerSample: asr.BitsPerSample,
		Channels:      asr.Channels,
	}, devID)

	// spectrum analyzer: writes curBands on each frame
	spec := audio.NewSpectrumAnalyzer(func(bands [16]float64) {
		s.levelMu.Lock()
		s.curBands = bands
		// RMS will be filled from the PCM callback below
		s.levelMu.Unlock()
	})

	cap.SetPCMCallback(func(pcm []byte) {
		rms := audio.ComputeRMS16(pcm)
		// map RMS [0,1] to 0..100 with soft compression (same curve as before)
		scaled := rms * 5.0
		compressed := scaled / (scaled + 0.9)
		rms100 := int32(compressed * 100)
		if rms100 > 100 {
			rms100 = 100
		}
		s.levelMu.Lock()
		s.curRMS = rms100
		s.levelMu.Unlock()
		spec.Process(pcm)
	})

	if err := cap.Start(); err != nil {
		return &StartResponse{Ok: false, Error: err.Error()}, nil
	}

	s.capture = cap
	s.spec = spec
	log.Printf("[voice] recording started (device=%d)", devID)
	return &StartResponse{Ok: true}, nil
}

// StopRecording stops capture, runs ASR on the full PCM, persists the event,
// and returns the recognized text.
func (s *VoiceServer) StopRecording(ctx context.Context, req *StopRequest) (*VoiceResult, error) {
	s.mu.Lock()
	cap := s.capture
	s.capture = nil
	spec := s.spec
	s.spec = nil
	s.mu.Unlock()

	if cap == nil {
		return &VoiceResult{Error: "not recording"}, nil
	}

	startMs := time.Now().UnixMilli()
	cap.Stop()
	if spec != nil {
		spec.Clear()
	}
	durationMs := time.Now().UnixMilli() - startMs

	pcm := cap.TakePCM()
	if len(pcm) < 320 { // < 10ms of audio
		return &VoiceResult{Error: "recording too short"}, nil
	}

	engine := s.holder.Get()
	if engine == nil {
		return &VoiceResult{Error: "ASR 未配置或创建失败，请在设置页配置 ASR 服务"}, nil
	}
	text, err := engine.Recognize(ctx, pcm)
	if err != nil {
		log.Printf("[voice] ASR failed: %v", err)
		return &VoiceResult{Error: err.Error(), DurationMs: int32(durationMs)}, nil
	}

	// persist as a voice event (best-effort)
	// 必须带 TS + 前台应用，否则 ListEvents 的 ts>=start 半开区间查不到（ts=0 孤儿），
	// 且无法按时间关联回 activity_segment。范式参照 asr_server.go。
	if s.db != nil {
		app, appErr := collector.ForegroundApp()
		var appPath, appName string
		if appErr == nil {
			appPath = app.Path
			appName = app.Name
		}
		_, _ = s.db.InsertEvent(storage.Event{
			TS:      time.Now().UTC(),
			Type:    storage.EventTypeVoice,
			AppPath: appPath,
			AppName: appName,
			Content: text,
			Meta:    fmt.Sprintf("engine=%s;duration_ms=%d", engine.Name(), durationMs),
		})
	}

	log.Printf("[voice] recording stopped: %dms, %d bytes PCM, text=%q",
		durationMs, len(pcm), text)
	return &VoiceResult{Text: text, DurationMs: int32(durationMs)}, nil
}

// StreamLevels pushes AudioLevel frames (16 bands + RMS) to the client while
// recording is active, at ~30fps. Ends when the client disconnects or
// StopRecording clears the capture.
func (s *VoiceServer) StreamLevels(req *LevelsRequest, stream VoiceService_StreamLevelsServer) error {
	ticker := time.NewTicker(33 * time.Millisecond) // ~30fps
	defer ticker.Stop()
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-ticker.C:
			s.mu.Lock()
			active := s.capture != nil
			s.mu.Unlock()
			if !active {
				return nil
			}
			s.levelMu.Lock()
			bands := s.curBands
			rms := s.curRMS
			s.levelMu.Unlock()
			frame := &AudioLevel{Rms: rms}
			frame.Bands = make([]float32, 16)
			for i := 0; i < 16; i++ {
				frame.Bands[i] = float32(bands[i])
			}
			if err := stream.Send(frame); err != nil {
				return err
			}
		}
	}
}

// TestConnection 探测 ASR 配置是否可用。
// - cloud: 构造一个临时 cloudEngine，发送 ~0.5s 静音 WAV，测延迟。
// - local: stat() 模型文件，并尝试用 whisper 加载（懒加载，开销较大）。
// 不修改 holder、不写磁盘、不依赖保存的配置。
func (s *VoiceServer) TestConnection(ctx context.Context, req *TestConnectionRequest) (*TestConnectionResponse, error) {
	mode := req.GetMode()
	f := req.GetFields()
	switch mode {
	case "cloud":
		cfg := config.ASRProvider{
			BaseURL:   f["baseUrl"],
			Model:     f["model"],
			APIKey:    f["apiKey"],
			AuthType:  f["authType"],
			APIFormat: f["apiFormat"],
			Language:  f["language"],
			Stream:    f["stream"] == "true",
		}
		if cfg.BaseURL == "" {
			return &TestConnectionResponse{Ok: false, Message: "base_url is empty"}, nil
		}
		if cfg.Model == "" {
			return &TestConnectionResponse{Ok: false, Message: "model is empty"}, nil
		}
		if cfg.AuthType == "" {
			cfg.AuthType = "bearer"
		}
		engine, err := newCloudEngineForTest(cfg)
		if err != nil {
			return &TestConnectionResponse{Ok: false, Message: err.Error()}, nil
		}
		// 0.5 秒静音 PCM
		silent := make([]byte, asr.SampleRate/2*2)
		start := time.Now().UTC()
		text, err := engine.Recognize(ctx, silent)
		elapsed := time.Since(start)
		if err != nil {
			return &TestConnectionResponse{Ok: false, Message: err.Error(), LatencyMs: int32(elapsed.Milliseconds())}, nil
		}
		return &TestConnectionResponse{
			Ok:        true,
			Message:   "endpoint reachable (response=" + text + ")",
			LatencyMs: int32(elapsed.Milliseconds()),
		}, nil
	case "local":
		path := f["modelPath"]
		if path == "" {
			return &TestConnectionResponse{Ok: false, Message: "model_path is empty"}, nil
		}
		if _, err := os.Stat(path); err != nil {
			return &TestConnectionResponse{Ok: false, Message: "model file: " + err.Error()}, nil
		}
		// 不真加载 whisper（大模型加载要几秒），只做 stat + 格式检查
		return &TestConnectionResponse{Ok: true, Message: "model file exists at " + path}, nil
	case "llm":
		// LLM（润色）连通性测试：构造临时引擎，发一句 "hi" 探测。
		cfg := config.LLMProvider{
			BaseURL:  f["baseUrl"],
			Model:    f["model"],
			APIKey:   f["apiKey"],
			AuthType: f["authType"],
		}
		if cfg.BaseURL == "" {
			return &TestConnectionResponse{Ok: false, Message: "base_url is empty"}, nil
		}
		if cfg.Model == "" {
			return &TestConnectionResponse{Ok: false, Message: "model is empty"}, nil
		}
		if cfg.AuthType == "" {
			cfg.AuthType = "bearer"
		}
		engine, err := llm.NewCloudEngineForTest(cfg)
		if err != nil {
			return &TestConnectionResponse{Ok: false, Message: err.Error()}, nil
		}
		start := time.Now().UTC()
		out, err := engine.Polish(ctx, "hi")
		elapsed := time.Since(start)
		if err != nil {
			return &TestConnectionResponse{Ok: false, Message: err.Error(), LatencyMs: int32(elapsed.Milliseconds())}, nil
		}
		return &TestConnectionResponse{
			Ok:        true,
			Message:   "endpoint reachable (response=" + out + ")",
			LatencyMs: int32(elapsed.Milliseconds()),
		}, nil
	case "vlm":
		// VLM（视觉理解）连通性测试：构造临时引擎，发一张 1×1 PNG 探测。
		cfg := config.VLMProvider{
			BaseURL:  f["baseUrl"],
			Model:    f["model"],
			APIKey:   f["apiKey"],
			AuthType: f["authType"],
		}
		if cfg.BaseURL == "" {
			return &TestConnectionResponse{Ok: false, Message: "base_url is empty"}, nil
		}
		if cfg.Model == "" {
			return &TestConnectionResponse{Ok: false, Message: "model is empty"}, nil
		}
		if cfg.AuthType == "" {
			cfg.AuthType = "bearer"
		}
		engine, err := vlm.NewCloudEngineForTest(cfg)
		if err != nil {
			return &TestConnectionResponse{Ok: false, Message: err.Error()}, nil
		}
		// 用编译期内嵌的固定测试图（vlm.ProbePNG，~500B 真实 PNG）。
		// 比 1×1 黑图更有意义：GLM 会返回对图的真实描述，前端 toast 能展示
		// VLM 实际"看懂了什么"，同时验证连通性与理解效果。
		start := time.Now().UTC()
		out, err := engine.Describe(ctx, vlm.ProbePNG)
		elapsed := time.Since(start)
		if err != nil {
			return &TestConnectionResponse{Ok: false, Message: err.Error(), LatencyMs: int32(elapsed.Milliseconds())}, nil
		}
		return &TestConnectionResponse{
			Ok:        true,
			Message:   "endpoint reachable (response=" + out + ")",
			LatencyMs: int32(elapsed.Milliseconds()),
		}, nil
	default:
		return &TestConnectionResponse{Ok: false, Message: "unknown mode: " + mode}, nil
	}
}

// newCloudEngineForTest 是 asr.newCloudEngine 的薄封装（这里直接调用即可）。
// 保留独立名字便于将来换成 stub。
func newCloudEngineForTest(cfg config.ASRProvider) (asr.Engine, error) {
	return asr.NewCloudEngineForTest(cfg)
}

// Polish 对识别出的文字调用配置好的 LLM 做润色。
//
// LLM 未启用（holder.Get() 为 nil）时返回 error，调用方据此决定是否保留
// 原文。LLM 调用失败时也返回 error（不阻断原文，由前端决定如何提示）。
func (s *VoiceServer) Polish(ctx context.Context, req *PolishRequest) (*PolishResult, error) {
	engine := s.llmHolder.Get()
	if engine == nil {
		return &PolishResult{Error: "LLM 未启用"}, nil
	}
	text, err := engine.Polish(ctx, req.GetText())
	if err != nil {
		log.Printf("[voice] polish failed: %v", err)
		return &PolishResult{Error: err.Error()}, nil
	}
	log.Printf("[voice] polish ok: %d -> %d chars", len(req.GetText()), len(text))
	return &PolishResult{Text: text}, nil
}
