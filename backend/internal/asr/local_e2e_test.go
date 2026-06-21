//go:build windows && asr_e2e

package asr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"shadow-worker/backend/internal/config"
)

// TestLocalEngineE2E 是 whisper.cpp 本地引擎的端到端测试：用 whisper.cpp
// 自带的 jfk.wav（肯尼迪就职演说片段，16kHz/mono/16bit）喂给 localEngine，
// 验证 CGO 链路 + 模型加载 + 推理 + 文本提取整条链路工作正常。
//
// 需要真实的 .bin 模型文件，默认指向仓库根 modules/ggml-small.bin。
// 模型不存在时跳过，不阻断默认 go test。
//
// 运行（需 CGO 工具链，同 build-whisper-cgo.bat 的环境）：
//
//	go test -tags asr_e2e -run TestLocalEngineE2E -v ./internal/asr/ -timeout 180s
func TestLocalEngineE2E(t *testing.T) {
	repoRoot := findRepoRoot(t)

	modelPath := filepath.Join(repoRoot, "modules", "ggml-small.bin")
	if _, err := os.Stat(modelPath); err != nil {
		t.Skipf("模型不存在，跳过端到端测试: %s\n提示: 下载 ggml-small.bin 到 modules/ 目录", modelPath)
	}

	// jfk.wav 来自 whisper.cpp 仓库（third_party 下），16kHz/mono/16bit PCM。
	wavPath := filepath.Join(repoRoot,
		"backend", "third_party", "whisper.cpp", "bindings", "go", "samples", "jfk.wav")
	wavBytes, err := os.ReadFile(wavPath)
	if err != nil {
		t.Skipf("jfk.wav 不存在，跳过: %v", err)
	}

	pcm, err := stripWAVHeader(wavBytes)
	if err != nil {
		t.Fatalf("解析 jfk.wav 失败: %v", err)
	}
	t.Logf("jfk.wav: %d 字节 PCM (%.1f 秒)", len(pcm), float64(len(pcm)/2)/SampleRate)

	// 构建引擎（这一步会通过 CGO 加载模型，耗时几秒）。
	cfg := config.Default()
	cfg.ASR.Mode = "local"
	cfg.ASR.Local.ModelPath = modelPath
	cfg.ASR.Local.Language = "en"

	t.Log("加载 whisper 模型（CGO），请稍候...")
	e, err := New(cfg)
	if err != nil {
		t.Fatalf("创建 local engine 失败: %v", err)
	}
	t.Logf("引擎: %s", e.Name())

	text, err := e.Recognize(context.Background(), pcm)
	if err != nil {
		t.Fatalf("Recognize 失败: %v", err)
	}

	text = strings.TrimSpace(text)
	t.Logf("识别结果: %q", text)

	if text == "" {
		t.Fatal("识别结果为空，whisper 未转出任何文字")
	}

	// jfk.wav 是 "And so, my fellow Americans..." —— 断言包含关键词。
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "fellow") && !strings.Contains(lower, "american") {
		t.Fatalf("识别结果未包含预期关键词 (fellow/american): %q", text)
	}
}

// findRepoRoot 从测试运行目录向上查找包含 modules/ 的目录作为仓库根。
func findRepoRoot(t *testing.T) string {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "modules")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("找不到仓库根（包含 modules/ 的目录）")
	return ""
}

// stripWAVHeader 去掉标准 RIFF/WAVE 头，返回 data chunk 的原始 PCM。
// 做了最小化的 chunk 遍历以兼容不同的 fmt chunk 大小。
func stripWAVHeader(wav []byte) ([]byte, error) {
	if len(wav) < 12 {
		return nil, fmt.Errorf("wav too short: %d bytes", len(wav))
	}
	if string(wav[0:4]) != "RIFF" || string(wav[8:12]) != "WAVE" {
		return nil, fmt.Errorf("不是有效 RIFF/WAVE 文件")
	}
	off := 12
	for off+8 <= len(wav) {
		id := string(wav[off : off+4])
		size := int(wav[off+4]) | int(wav[off+5])<<8 | int(wav[off+6])<<16 | int(wav[off+7])<<24
		off += 8
		if id == "data" {
			end := off + size
			if end > len(wav) {
				end = len(wav)
			}
			return wav[off:end], nil
		}
		off += size
	}
	return nil, fmt.Errorf("wav 中未找到 data chunk")
}
