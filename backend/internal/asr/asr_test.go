package asr

import (
	"context"
	"testing"

	"shadow-worker/backend/internal/config"
)

func TestNewCloudEngine(t *testing.T) {
	cfg := config.Default()
	cfg.ASR.Mode = "cloud"
	e, err := New(cfg)
	if err != nil {
		t.Fatalf("创建 cloud engine 失败: %v", err)
	}
	if e.Name() == "" {
		t.Fatal("engine name 不能为空")
	}
}

func TestNewLocalEngineMissingModel(t *testing.T) {
	cfg := config.Default()
	cfg.ASR.Mode = "local"
	cfg.ASR.Local.ModelPath = "nonexistent-model.bin"
	_, err := New(cfg)
	if err != nil {
		// 创建时不应失败,识别时才失败
		t.Fatalf("创建 local engine 不应失败: %v", err)
	}
}

func TestCloudEngineEmptyPCM(t *testing.T) {
	cfg := config.Default()
	cfg.ASR.Mode = "cloud"
	e, _ := New(cfg)
	out, err := e.Recognize(context.Background(), nil)
	if err != nil {
		t.Fatalf("空 PCM 不应报错: %v", err)
	}
	if out != "" {
		t.Fatalf("空 PCM 应返回空字符串,实际: %s", out)
	}
}

func TestWrapWAV(t *testing.T) {
	pcm := make([]byte, SampleRate*2) // 1 秒
	wav := wrapWAV(pcm)
	if len(wav) <= len(pcm) {
		t.Fatal("WAV 包应比 PCM 长")
	}
	if string(wav[0:4]) != "RIFF" {
		t.Fatal("WAV 头应为 RIFF")
	}
}
