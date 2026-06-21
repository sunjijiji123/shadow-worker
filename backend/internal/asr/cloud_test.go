package asr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"shadow-worker/backend/internal/config"
)

// makeSilentPCM 生成 n 毫秒的静音 PCM（16kHz/mono/16bit），仅用于让请求体非空。
func makeSilentPCM(ms int) []byte {
	samples := SampleRate * ms / 1000
	return make([]byte, samples*2)
}

// newTestCloudEngine 构造一个指向 httptest server 的云引擎。
func newTestCloudEngine(t *testing.T, baseURL string, stream bool) Engine {
	t.Helper()
	e, err := newCloudEngine(config.ASRProvider{
		BaseURL:  baseURL,
		Model:    "test-model",
		APIKey:   "sk-test",
		AuthType: "bearer",
		Language: "zh",
		Stream:   stream,
	}, nil)
	if err != nil {
		t.Fatalf("newCloudEngine: %v", err)
	}
	return e
}

func TestCloudEngineJSONResponse(t *testing.T) {
	// 标准 OpenAI 风格：单个 JSON {"text":"..."}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"task":"transcribe","language":"english","duration":1.0,"text":"hello world"}`))
	}))
	defer srv.Close()

	e := newTestCloudEngine(t, srv.URL, false)
	text, err := e.Recognize(context.Background(), makeSilentPCM(500))
	if err != nil {
		t.Fatalf("Recognize 失败: %v", err)
	}
	if text != "hello world" {
		t.Fatalf("期望 'hello world', 实际 %q", text)
	}
}

func TestCloudEngineSSEResponse(t *testing.T) {
	// 流式 SSE 风格：多个 data: 分块
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"text\":\"你好\"}\n\n"))
		w.Write([]byte("data: {\"text\":\"世界\"}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	e := newTestCloudEngine(t, srv.URL, true)
	text, err := e.Recognize(context.Background(), makeSilentPCM(500))
	if err != nil {
		t.Fatalf("Recognize 失败: %v", err)
	}
	if text != "你好世界" {
		t.Fatalf("期望 '你好世界', 实际 %q", text)
	}
}

// TestCloudEngineStreamMismatch 验证兜底逻辑：配置成非流式，但服务端返回 SSE。
func TestCloudEngineStreamMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"text\":\"mismatch ok\"}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	// 配置成非流式（false），但服务端返回 SSE —— 应回退到 SSE 解析。
	e := newTestCloudEngine(t, srv.URL, false)
	text, err := e.Recognize(context.Background(), makeSilentPCM(500))
	if err != nil {
		t.Fatalf("Recognize 失败: %v", err)
	}
	if text != "mismatch ok" {
		t.Fatalf("期望 'mismatch ok', 实际 %q", text)
	}
}

func TestCloudEngineErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid api key"}`))
	}))
	defer srv.Close()

	e := newTestCloudEngine(t, srv.URL, false)
	_, err := e.Recognize(context.Background(), makeSilentPCM(500))
	if err == nil {
		t.Fatal("期望 401 报错")
	}
}

func TestCloudEngineStreamingPartial(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"text\":\"first\"}\n\n"))
		w.Write([]byte("data: {\"text\":\" second\"}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	e := newTestCloudEngine(t, srv.URL, true)
	var partials []string
	se := e.(StreamingEngine)
	text, err := se.RecognizeStreaming(context.Background(), makeSilentPCM(500), func(s string) {
		partials = append(partials, s)
	})
	if err != nil {
		t.Fatalf("RecognizeStreaming 失败: %v", err)
	}
	if text != "first second" {
		t.Fatalf("期望 'first second', 实际 %q", text)
	}
	if len(partials) != 2 {
		t.Fatalf("期望 2 个 partial 回调, 实际 %d (%v)", len(partials), partials)
	}
}
