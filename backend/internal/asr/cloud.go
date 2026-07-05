package asr

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"shadow-worker/backend/internal/config"
	"shadow-worker/backend/internal/httputil"
)

// cloudEngine 实现 OpenAI 兼容的 /audio/transcriptions 识别。
//
// 支持两种响应模式（由 provider 配置的 Stream 决定）：
//   - 流式（Stream=true）：请求带 stream=true，响应是 SSE（data: {...}\n\n）。
//     适用于小米 MIMO 等支持流式分块的 ASR 接口。
//   - 非流式（Stream=false，默认）：请求带 response_format=json，响应是单个
//     JSON 对象 {"text": "..."}。适用于标准 OpenAI whisper-1。
//
// 解析时通过 peek body 首个非空行自动判定 SSE 还是 JSON，因此即使配置不匹配
// 也能尽量正确解析（健壮性兜底）。
type cloudEngine struct {
	cfg        config.ASRProvider
	prompt     string
	httpClient *http.Client
}

func newCloudEngine(cfg config.ASRProvider, hotwords []string, logger *slog.Logger) (Engine, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("cloud ASR: base_url 不能为空")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("cloud ASR: model 不能为空")
	}
	if cfg.AuthType == "" {
		cfg.AuthType = "bearer"
	}
	// logger 当前未在 cloudEngine 内部使用（识别走 httputil.DoWithRetry，
	// 那里有自己的 logger）。保留参数为将来 cloud 引擎内部打日志预留，
	// 并保持与 newLocalEngine 构造签名一致。
	_ = logger
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
	resp, err := e.postTranscription(ctx, pcm)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ASR API 状态 %d: %s", resp.StatusCode, string(b))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}
	return parseTranscriptionBody(body, e.cfg.Stream, nil)
}

// RecognizeStreaming 流式识别。onPartial 在识别过程中推送增量文本给前端。
//
// 流式请求不做重试（与 Recognize 不同）：onPartial 可能已推送部分文本到前端 UI，
// 若失败后重试从头再来，前端会看到文本重复/错乱。瞬时失败由上层（录音流程）
// 决定是否整体重录，不在引擎层重试。
func (e *cloudEngine) RecognizeStreaming(ctx context.Context, pcm []byte, onPartial func(string)) (string, error) {
	if onPartial == nil {
		return e.Recognize(ctx, pcm)
	}
	if len(pcm) == 0 {
		return "", nil
	}
	resp, err := e.postTranscription(ctx, pcm)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ASR API 状态 %d: %s", resp.StatusCode, string(b))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}
	return parseTranscriptionBody(body, e.cfg.Stream, onPartial)
}

// postTranscription 构造并发送一次 transcription 请求（带重试）。
// 根据 BaseURL 自动判断协议：
//   - 含 "/chat/completions" → 小米/智谱等 chat 格式：JSON body + base64 WAV + api-key 认证
//   - 否则 → 标准 OpenAI transcription 格式：multipart + /audio/transcriptions 后缀 + Bearer 认证
//
// 用 httputil.DoWithRetry 包裹：429/5xx/网络错误自动重试。流式 ASR
// （RecognizeStreaming）不走这里，其瞬时失败不重试（见 RecognizeStreaming 注释）。
func (e *cloudEngine) postTranscription(ctx context.Context, pcm []byte) (*http.Response, error) {
	wav := wrapWAV(pcm)
	reqBuilder := func() (*http.Request, error) {
		if isChatCompletionsURL(e.cfg.BaseURL) {
			return e.buildChatCompletionsRequest(ctx, wav)
		}
		return e.buildTranscriptionMultipartRequest(ctx, wav)
	}
	return httputil.DoWithRetry(ctx, e.httpClient, reqBuilder, nil, e.cfg.RetryCount)
}

// isChatCompletionsURL 判断 URL 是否是 chat completions endpoint。
func isChatCompletionsURL(baseURL string) bool {
	u := strings.ToLower(baseURL)
	return strings.Contains(u, "/chat/completions")
}

// transcriptionEndpoint 推导 transcription 请求的完整 URL。
// 兼容两种填法：
//   - 完整路径（已含 /audio/transcriptions）：直接 TrimRight 尾斜杠后使用
//   - 根 URL（如 .../v1）：自动拼接 /audio/transcriptions
func transcriptionEndpoint(baseURL string) string {
	trimmed := strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(strings.ToLower(trimmed), "/audio/transcriptions") {
		return trimmed
	}
	return trimmed + "/audio/transcriptions"
}

// buildChatCompletionsRequest 构建 chat completions 格式请求（小米 MIMO 等），不发送。
// 由 postTranscription 在重试闭包里调用，每次重试重建 request body reader。
// body 是 JSON，audio 以 base64 data URI 嵌在 message content 中。
// 认证头用 api-key（不是 Bearer）。
func (e *cloudEngine) buildChatCompletionsRequest(ctx context.Context, wav []byte) (*http.Request, error) {
	body, err := buildChatCompletionsBody(wav, e.cfg.Model, e.cfg.Language, e.prompt)
	if err != nil {
		return nil, fmt.Errorf("构建 chat 请求失败: %w", err)
	}
	endpoint := strings.TrimRight(e.cfg.BaseURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if e.cfg.AuthType == "bearer" {
		req.Header.Set("Authorization", "Bearer "+e.cfg.APIKey)
	} else {
		// 小米用 api-key 头
		req.Header.Set("api-key", e.cfg.APIKey)
	}
	return req, nil
}

// buildTranscriptionMultipartRequest 构建标准 OpenAI transcription 格式请求，不发送。
// 由 postTranscription 在重试闭包里调用，每次重试重建 request。
// body 是 multipart/form-data。endpoint 由 BaseURL 推导：
//   - 若 BaseURL 已含 /audio/transcriptions（用户填了完整路径），直接用
//   - 否则视为根 URL，自动拼接 /audio/transcriptions
func (e *cloudEngine) buildTranscriptionMultipartRequest(ctx context.Context, wav []byte) (*http.Request, error) {
	body, contentType, err := buildMultipart(wav, e.cfg.Model, e.cfg.Language, e.prompt, e.cfg.Stream)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %w", err)
	}
	endpoint := transcriptionEndpoint(e.cfg.BaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	if e.cfg.AuthType == "api-key" {
		req.Header.Set("api-key", e.cfg.APIKey)
	} else {
		req.Header.Set("Authorization", "Bearer "+e.cfg.APIKey)
	}
	return req, nil
}

// buildChatCompletionsBody 构造小米/智谱 chat completions 格式的 JSON body。
// audio 以 data URI 形式嵌入 message content。
func buildChatCompletionsBody(wav []byte, model, lang, prompt string) ([]byte, error) {
	b64 := base64.StdEncoding.EncodeToString(wav)
	dataURI := "data:audio/wav;base64," + b64

	userMsg := map[string]any{
		"role": "user",
		"content": []map[string]any{
			{
				"type": "input_audio",
				"input_audio": map[string]any{
					"data": dataURI,
				},
			},
		},
	}

	var messages []map[string]any
	if prompt != "" {
		messages = []map[string]any{
			{
				"role":    "system",
				"content": prompt,
			},
			userMsg,
		}
	} else {
		messages = []map[string]any{userMsg}
	}

	payload := map[string]any{
		"model":    model,
		"stream":   true,
		"messages": messages,
	}
	return json.Marshal(payload)
}

// parseTranscriptionBody 解析 transcription 响应体。
//
// 策略：先尝试按配置的 expectStream 解析；如果没解析出任何文本，则用相反
// 模式再试一次（兜底，应对服务端实际返回格式与配置不符的情况）。
//
// 若 onPartial 非 nil，在流式解析时对每个累积片段调用它。
func parseTranscriptionBody(body []byte, expectStream bool, onPartial func(string)) (string, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return "", fmt.Errorf("ASR 返回空响应")
	}

	// 优先按配置模式解析。
	text, ok := tryParse(body, expectStream, onPartial)
	if ok {
		out := strings.TrimSpace(text)
		if out != "" {
			return out, nil
		}
	}

	// 兜底：用相反模式再试。
	alt, ok := tryParse(body, !expectStream, onPartial)
	if ok {
		if out := strings.TrimSpace(alt); out != "" {
			return out, nil
		}
	}

	// 最终兜底：直接 JSON 解析整个 body（有些接口返回 {"text":"..."} 但没有
	// content-type 提示，前面两种按行扫描可能漏掉）。
	var direct struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(body, &direct); err == nil {
		// JSON 能解析且含 text 字段：即使 text 为空也视为解析成功。
		// 场景：TestConnection 发静音音频，GLM-ASR 等接口返回 200 + {"text":""}，
		// 这是"连接正常但无语音内容"，不该报错。
		if onPartial != nil && direct.Text != "" {
			onPartial(strings.TrimSpace(direct.Text))
		}
		return strings.TrimSpace(direct.Text), nil
	}

	return "", fmt.Errorf("ASR 响应无法解析为文本: %s", truncate(string(body), 200))
}

// tryParse 按指定模式解析，返回 (text, ok)。ok=false 表示该模式不适用
// （例如流式模式下 body 里没有 data: 行）。
func tryParse(body []byte, asStream bool, onPartial func(string)) (string, bool) {
	if asStream {
		return parseSSE(body, onPartial)
	}
	return parseJSON(body, onPartial)
}

// parseSSE 按 SSE（data: 行）解析。返回 (text, ok)；没有 data: 行时 ok=false。
// 支持两种 SSE 格式：
//   - transcription 格式: {"text":"..."}
//   - chat completions 格式: {"choices":[{"delta":{"content":"..."}}]}
func parseSSE(body []byte, onPartial func(string)) (string, bool) {
	var text strings.Builder
	found := false
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		found = true
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		// 尝试 transcription 格式 {"text":"..."}
		var trChunk struct {
			Text string `json:"text"`
		}
		if json.Unmarshal([]byte(data), &trChunk) == nil && trChunk.Text != "" {
			text.WriteString(trChunk.Text)
			if onPartial != nil {
				onPartial(strings.TrimSpace(text.String()))
			}
			continue
		}
		// 尝试 chat completions 格式 {"choices":[{"delta":{"content":"..."}}]}
		var ccChunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(data), &ccChunk) == nil && len(ccChunk.Choices) > 0 {
			c := ccChunk.Choices[0].Delta.Content
			if c != "" {
				text.WriteString(c)
				if onPartial != nil {
					onPartial(strings.TrimSpace(text.String()))
				}
			}
			continue
		}
	}
	return text.String(), found
}

// parseJSON 按单个 JSON 对象解析。返回 (text, ok)。
func parseJSON(body []byte, onPartial func(string)) (string, bool) {
	var res struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", false
	}
	if onPartial != nil && strings.TrimSpace(res.Text) != "" {
		onPartial(strings.TrimSpace(res.Text))
	}
	return res.Text, true
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func buildMultipart(wav []byte, model, lang, prompt string, stream bool) (*bytes.Buffer, string, error) {
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)

	if err := mw.WriteField("model", model); err != nil {
		return nil, "", err
	}
	if stream {
		if err := mw.WriteField("stream", "true"); err != nil {
			return nil, "", err
		}
	} else {
		// 标准 OpenAI 不支持 stream；明确请求 JSON 格式以便可靠解析。
		if err := mw.WriteField("response_format", "json"); err != nil {
			return nil, "", err
		}
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
	binary.Write(buf, binary.LittleEndian, uint16(1)) // PCM
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
