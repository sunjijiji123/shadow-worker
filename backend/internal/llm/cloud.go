package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"shadow-worker/backend/internal/config"
)

// cloudEngine 实现 OpenAI 兼容的 /chat/completions 文字润色接口。
//
// 请求体：{model, messages:[{role:system, content:prompt}, {role:user, content:text}]}
// 认证三态：api-key 头 / Bearer 头 / 无（APIKey 为空）——仿 vlm/cloud.go。
// 响应解析：{choices:[{message:{content}}]}，取 choices[0].message.content。
type cloudEngine struct {
	cfg        config.LLMProvider
	prompt     string
	httpClient *http.Client
}

func newCloudEngine(cfg config.LLMProvider, prompt string) (Engine, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("cloud LLM: base_url 不能为空")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("cloud LLM: model 不能为空")
	}
	if cfg.AuthType == "" {
		cfg.AuthType = "bearer"
	}
	return &cloudEngine{
		cfg:        cfg,
		prompt:     prompt,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func (e *cloudEngine) Name() string {
	return fmt.Sprintf("cloud-llm (%s)", e.cfg.Model)
}

func (e *cloudEngine) Polish(ctx context.Context, text string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("润色输入为空")
	}

	// system = 润色 prompt，user = 待润色原文
	body := map[string]any{
		"model": e.cfg.Model,
		"messages": []map[string]any{
			{"role": "system", "content": e.prompt},
			{"role": "user", "content": text},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("序列化润色请求失败: %w", err)
	}

	endpoint := strings.TrimRight(e.cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("创建润色请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// 三态认证（仿 vlm/cloud.go）：api-key 头 / Bearer 头 / 无
	if e.cfg.AuthType == "api-key" {
		req.Header.Set("api-key", e.cfg.APIKey)
	} else if e.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.cfg.APIKey)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求润色失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取润色响应失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("润色 API 状态 %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("解析润色响应失败: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("润色返回空 choices")
	}
	out := strings.TrimSpace(result.Choices[0].Message.Content)
	if out == "" {
		return "", fmt.Errorf("润色返回空文本")
	}
	return out, nil
}
