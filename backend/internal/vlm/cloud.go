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
	"shadow-worker/backend/internal/httputil"
)

const vlmPrompt = "请用一句话概括这张屏幕截图里用户正在做什么，不超过 50 字。"

// cloudEngine 实现 OpenAI 兼容的 /chat/completions 视觉接口。
type cloudEngine struct {
	cfg        config.VLMProvider
	httpClient *http.Client
}

func newCloudEngine(cfg config.VLMProvider) (Engine, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("cloud VLM: base_url 不能为空")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("cloud VLM: model 不能为空")
	}
	if cfg.AuthType == "" {
		cfg.AuthType = "bearer"
	}
	return &cloudEngine{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func (e *cloudEngine) Name() string {
	return fmt.Sprintf("cloud-vlm (%s)", e.cfg.Model)
}

func (e *cloudEngine) Describe(ctx context.Context, imagePNG []byte) (string, error) {
	if len(imagePNG) == 0 {
		return "", fmt.Errorf("空图片")
	}

	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(imagePNG)
	body := map[string]any{
		"model": e.cfg.Model,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": vlmPrompt},
					{"type": "image_url", "image_url": map[string]string{"url": dataURL}},
				},
			},
		},
		// 128 太小：中文摘要常被截断成半句（"用户正在查看"）。提到 1024
		// （中文约 500~700 字）保证一段完整描述不被 token 上限硬切。
		"max_tokens": 1024,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	endpoint := strings.TrimRight(e.cfg.BaseURL, "/") + "/chat/completions"

	// DoWithRetry 对 429/5xx/网络错误自动重试（指数退避）。
	// 闭包捕获 jsonBody（值不变，重建只读），每次重试重建 request body reader。
	resp, err := httputil.DoWithRetry(ctx, e.httpClient, func() (*http.Request, error) {
		r, rerr := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(jsonBody))
		if rerr != nil {
			return nil, rerr
		}
		r.Header.Set("Content-Type", "application/json")
		if e.cfg.AuthType == "api-key" {
			r.Header.Set("api-key", e.cfg.APIKey)
		} else if e.cfg.APIKey != "" {
			r.Header.Set("Authorization", "Bearer "+e.cfg.APIKey)
		}
		return r, nil
	}, nil)
	if err != nil {
		return "", fmt.Errorf("请求 VLM 失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取 VLM 响应失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("VLM API 状态 %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("解析 VLM 响应失败: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("VLM 返回空 choices")
	}
	out := strings.TrimSpace(result.Choices[0].Message.Content)
	if out == "" {
		return "", fmt.Errorf("VLM 返回空文本")
	}
	return out, nil
}
