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
	prompt     string
	httpClient *http.Client
}

func newOllamaEngine(cfg config.VLMProvider, prompt string) (Engine, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("ollama VLM: base_url 不能为空")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("ollama VLM: model 不能为空")
	}
	return &ollamaEngine{
		cfg:        cfg,
		prompt:     prompt,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}, nil
}

func (e *ollamaEngine) Name() string {
	return fmt.Sprintf("ollama-vlm (%s)", e.cfg.Model)
}

func (e *ollamaEngine) Describe(ctx context.Context, imagePNG []byte) (string, error) {
	return e.describe(ctx, imagePNG, e.prompt)
}

// DescribeWith 用 promptOverride 覆盖引擎默认 prompt；为空则回落 e.prompt。
func (e *ollamaEngine) DescribeWith(ctx context.Context, imagePNG []byte, promptOverride string) (string, error) {
	p := promptOverride
	if strings.TrimSpace(p) == "" {
		p = e.prompt
	}
	return e.describe(ctx, imagePNG, p)
}

// describe 是 Describe / DescribeWith 共用的请求构建+发送逻辑。
func (e *ollamaEngine) describe(ctx context.Context, imagePNG []byte, effectivePrompt string) (string, error) {
	if len(imagePNG) == 0 {
		return "", fmt.Errorf("空图片")
	}
	if strings.TrimSpace(effectivePrompt) == "" {
		return "", fmt.Errorf("VLM 提示词为空，请在设置中配置")
	}

	body := map[string]any{
		"model":  e.cfg.Model,
		"prompt": effectivePrompt,
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
