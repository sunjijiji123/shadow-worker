package grpcapi

import (
	"context"
	"io"
	"path/filepath"
	"testing"
	"time"

	"shadow-worker/backend/internal/storage"

	"google.golang.org/grpc/metadata"
)

type mockASREngine struct {
	name string
}

func (m *mockASREngine) Name() string { return m.name }
func (m *mockASREngine) Recognize(ctx context.Context, pcm []byte) (string, error) {
	return "mock 识别结果", nil
}

func TestAsrServerStreamRecognize(t *testing.T) {
	db, err := storage.OpenAt(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer db.Close()

	engine := &mockASREngine{name: "mock"}
	srv := NewAsrServer(db, engine, nil)

	// 构造双向流模拟
	stream := &mockAsrStream{}
	stream.initInput()

	go func() {
		_ = stream.SendInput(&AudioChunk{Pcm: []byte{0, 1, 2, 3}})
		_ = stream.SendInput(&AudioChunk{Pcm: []byte{4, 5, 6, 7}})
		stream.CloseInput()
	}()

	if err := srv.StreamRecognize(stream); err != nil {
		t.Fatalf("StreamRecognize 失败: %v", err)
	}

	if len(stream.results) != 1 {
		t.Fatalf("应返回 1 个结果，实际 %d", len(stream.results))
	}
	if stream.results[0].FinalText != "mock 识别结果" {
		t.Fatalf("结果文本不匹配: %s", stream.results[0].FinalText)
	}

	// 验证事件写入
	events, err := db.ListEventsByDate(time.Now().UTC())
	if err != nil {
		t.Fatalf("查询事件失败: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("应写入 1 个事件，实际 %d", len(events))
	}
	if events[0].Type != storage.EventTypeVoice {
		t.Fatalf("事件类型应为 voice，实际 %s", events[0].Type)
	}
	if events[0].Content != "mock 识别结果" {
		t.Fatalf("事件内容不匹配: %s", events[0].Content)
	}
}

// mockAsrStream 是 AsrService_StreamRecognizeServer 的简单实现。
type mockAsrStream struct {
	input   chan *AudioChunk
	results []*AsrResult
}

func (m *mockAsrStream) initInput() {
	if m.input == nil {
		m.input = make(chan *AudioChunk, 4)
	}
}

func (m *mockAsrStream) SendInput(c *AudioChunk) error {
	m.initInput()
	m.input <- c
	return nil
}

func (m *mockAsrStream) CloseInput() {
	m.initInput()
	close(m.input)
}

func (m *mockAsrStream) Send(r *AsrResult) error {
	m.results = append(m.results, r)
	return nil
}

func (m *mockAsrStream) Recv() (*AudioChunk, error) {
	c, ok := <-m.input
	if !ok {
		return nil, io.EOF
	}
	return c, nil
}

func (m *mockAsrStream) Context() context.Context { return context.Background() }
func (m *mockAsrStream) SendMsg(msg any) error    { return nil }
func (m *mockAsrStream) RecvMsg(msg any) error    { return nil }
func (m *mockAsrStream) SetHeader(metadata.MD) error { return nil }
func (m *mockAsrStream) SendHeader(metadata.MD) error { return nil }
func (m *mockAsrStream) SetTrailer(metadata.MD) {}
