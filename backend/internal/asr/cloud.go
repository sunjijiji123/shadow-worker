package asr

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"shadow-worker/backend/internal/config"
)

// cloudEngine 实现 OpenAI 兼容的 /audio/transcriptions SSE 识别。
type cloudEngine struct {
	cfg        config.ASRProvider
	prompt     string
	httpClient *http.Client
}

func newCloudEngine(cfg config.ASRProvider, hotwords []string) (Engine, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("cloud ASR: base_url 不能为空")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("cloud ASR: model 不能为空")
	}
	if cfg.AuthType == "" {
		cfg.AuthType = "bearer"
	}
	return &cloudEngine{
		cfg:        cfg,
		prompt:     joinHotwords(hotwords),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func (e *cloudEngine) Name() string {
	return fmt.Sprintf("cloud-asr (%s)", e.cfg.Model)
}

func (e *cloudEngine) Recognize(ctx context.Context, pcm []byte) (string, error) {
	if len(pcm) == 0 {
		return "", nil
	}

	wav := wrapWAV(pcm)
	body, contentType, err := buildMultipart(wav, e.cfg.Model, e.cfg.Language, e.prompt)
	if err != nil {
		return "", fmt.Errorf("构建请求失败: %w", err)
	}

	endpoint := strings.TrimRight(e.cfg.BaseURL, "/") + "/audio/transcriptions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	if e.cfg.AuthType == "api-key" {
		req.Header.Set("api-key", e.cfg.APIKey)
	} else {
		req.Header.Set("Authorization", "Bearer "+e.cfg.APIKey)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求 ASR 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ASR API 状态 %d: %s", resp.StatusCode, string(b))
	}

	var text strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		text.WriteString(chunk.Text)
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("读取 SSE 失败: %w", err)
	}

	out := strings.TrimSpace(text.String())
	if out == "" {
		return "", fmt.Errorf("ASR 返回空文本")
	}
	return out, nil
}

func (e *cloudEngine) RecognizeStreaming(ctx context.Context, pcm []byte, onPartial func(string)) (string, error) {
	// 云端接口仍按整段识别,partial 通过 SSE 累积输出
	if onPartial == nil {
		return e.Recognize(ctx, pcm)
	}
	if len(pcm) == 0 {
		return "", nil
	}

	wav := wrapWAV(pcm)
	body, contentType, err := buildMultipart(wav, e.cfg.Model, e.cfg.Language, e.prompt)
	if err != nil {
		return "", fmt.Errorf("构建请求失败: %w", err)
	}

	endpoint := strings.TrimRight(e.cfg.BaseURL, "/") + "/audio/transcriptions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	if e.cfg.AuthType == "api-key" {
		req.Header.Set("api-key", e.cfg.APIKey)
	} else {
		req.Header.Set("Authorization", "Bearer "+e.cfg.APIKey)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求 ASR 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ASR API 状态 %d: %s", resp.StatusCode, string(b))
	}

	var text strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		text.WriteString(chunk.Text)
		onPartial(strings.TrimSpace(text.String()))
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("读取 SSE 失败: %w", err)
	}

	return strings.TrimSpace(text.String()), nil
}

func buildMultipart(wav []byte, model, lang, prompt string) (*bytes.Buffer, string, error) {
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)

	if err := mw.WriteField("model", model); err != nil {
		return nil, "", err
	}
	if err := mw.WriteField("stream", "true"); err != nil {
		return nil, "", err
	}
	if lang != "" && lang != "auto" {
		if err := mw.WriteField("language", lang); err != nil {
			return nil, "", err
		}
	}
	if prompt != "" {
		if err := mw.WriteField("prompt", prompt); err != nil {
			return nil, "", err
		}
	}
	fw, err := mw.CreateFormFile("file", "audio.wav")
	if err != nil {
		return nil, "", err
	}
	if _, err := fw.Write(wav); err != nil {
		return nil, "", err
	}
	if err := mw.Close(); err != nil {
		return nil, "", err
	}
	return buf, mw.FormDataContentType(), nil
}

func wrapWAV(pcm []byte) []byte {
	const byteRate = SampleRate * Channels * BitsPerSample / 8
	const blockAlign = Channels * BitsPerSample / 8

	buf := &bytes.Buffer{}
	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, uint32(36+len(pcm)))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))
	binary.Write(buf, binary.LittleEndian, uint16(1))          // PCM
	binary.Write(buf, binary.LittleEndian, uint16(Channels))
	binary.Write(buf, binary.LittleEndian, uint32(SampleRate))
	binary.Write(buf, binary.LittleEndian, uint32(byteRate))
	binary.Write(buf, binary.LittleEndian, uint16(blockAlign))
	binary.Write(buf, binary.LittleEndian, uint16(BitsPerSample))
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, uint32(len(pcm)))
	buf.Write(pcm)
	return buf.Bytes()
}

func joinHotwords(words []string) string {
	const maxWords = 30
	if len(words) == 0 {
		return ""
	}
	if len(words) > maxWords {
		words = words[:maxWords]
	}
	return strings.Join(words, ", ")
}
