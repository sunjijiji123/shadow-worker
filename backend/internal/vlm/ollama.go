package vlm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"shadow-worker/backend/internal/config"
)

// ollamaEngine 实现 Ollama /api/generate 本地视觉接口。
type ollamaEngine struct {
	cfg        config.VLMProvider
	httpClient *http.Client
}

func newOllamaEngine(cfg config.VLMProvider) (Engine, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("ollama VLM: base_url 不能为空")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("ollama VLM: model 不能为空")
	}
	return &ollamaEngine{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}, nil
}

func (e *ollamaEngine) Name() string {
	return fmt.Sprintf("ollama-vlm (%s)", e.cfg.Model)
}

func (e *ollamaEngine) Describe(ctx context.Context, imagePNG []byte) (string, error) {
	if len(imagePNG) == 0 {
		return "", fmt.Errorf("空图片")
	}

	body := map[string]any{
		"model":  e.cfg.Model,
		"prompt": vlmPrompt,
		"images": []string{base64.StdEncoding.EncodeToString(imagePNG)},
		"stream": false,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	endpoint := strings.TrimRight(e.cfg.BaseURL, "/") + "/api/generate"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求 Ollama 失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取 Ollama 响应失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Ollama API 状态 %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("解析 Ollama 响应失败: %w", err)
	}
	out := strings.TrimSpace(result.Response)
	if out == "" {
		return "", fmt.Errorf("Ollama 返回空文本")
	}
	return out, nil
}
