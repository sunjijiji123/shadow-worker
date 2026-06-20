package grpcapi

import (
	"context"
	"log"
	"sync"
	"time"

	"shadow-worker/backend/internal/asr"
	"shadow-worker/backend/internal/audio"
	"shadow-worker/backend/internal/storage"
)

// VoiceServer implements VoiceService: in-process microphone capture with a
// live 16-band FFT spectrum stream, and ASR on stop. PCM never leaves the
// backend process — the client only receives spectrum frames + the final text.
type VoiceServer struct {
	UnimplementedVoiceServiceServer
	engine asr.Engine
	db     *storage.DB

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
// StartRecording.
func NewVoiceServer(db *storage.DB, engine asr.Engine) *VoiceServer {
	return &VoiceServer{engine: engine, db: db}
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

	text, err := s.engine.Recognize(ctx, pcm)
	if err != nil {
		log.Printf("[voice] ASR failed: %v", err)
		return &VoiceResult{Error: err.Error(), DurationMs: int32(durationMs)}, nil
	}

	// persist as a voice event (best-effort)
	if s.db != nil {
		_, _ = s.db.InsertEvent(storage.Event{
			Type:    storage.EventTypeVoice,
			Content: text,
			Meta:    "engine=" + s.engine.Name(),
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
