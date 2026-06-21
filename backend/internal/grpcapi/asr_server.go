package grpcapi

import (
	"fmt"
	"io"
	"log/slog"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"shadow-worker/backend/internal/asr"
	"shadow-worker/backend/internal/collector"
	"shadow-worker/backend/internal/storage"
)

// AsrServer 实现 AsrService 的 gRPC 服务。
type AsrServer struct {
	UnimplementedAsrServiceServer
	db     *storage.DB
	holder *asr.EngineHolder
	logger *slog.Logger
}

// NewAsrServer 创建 AsrServer 实例。holder 允许配置变更后热重载引擎。
func NewAsrServer(db *storage.DB, holder *asr.EngineHolder, logger *slog.Logger) *AsrServer {
	if logger == nil {
		logger = slog.Default()
	}
	return &AsrServer{db: db, holder: holder, logger: logger}
}

// StreamRecognize 是双向流：Qt 推 AudioChunk，Go 返回 AsrResult。
func (s *AsrServer) StreamRecognize(stream AsrService_StreamRecognizeServer) error {
	ctx := stream.Context()
	var pcm []byte

	// 持续接收音频块
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.logger.Warn("接收音频失败", "err", err)
			return status.Errorf(codes.Internal, "接收音频失败: %v", err)
		}
		pcm = append(pcm, chunk.Pcm...)
	}

	s.logger.Info("收到 ASR 音频", "samples", len(pcm)/2)

	// 调用引擎识别
	start := time.Now().UTC()
	engine := s.holder.Get()
	if engine == nil {
		if err := stream.Send(&AsrResult{
			Done:  true,
			Error: "ASR 未配置或创建失败，请在设置页配置 ASR 服务",
		}); err != nil {
			return status.Errorf(codes.Internal, "发送错误结果失败: %v", err)
		}
		return nil
	}
	text, err := engine.Recognize(ctx, pcm)
	elapsed := time.Since(start)
	if err != nil {
		s.logger.Warn("ASR 识别失败", "err", err)
		if err := stream.Send(&AsrResult{
			Done:  true,
			Error: err.Error(),
		}); err != nil {
			return status.Errorf(codes.Internal, "发送错误结果失败: %v", err)
		}
		return nil
	}

	s.logger.Info("ASR 识别完成", "text", text, "elapsed_ms", elapsed.Milliseconds())

	// 获取当前前台应用,写入 events
	app, appErr := collector.ForegroundApp()
	var appPath, appName string
	if appErr == nil {
		appPath = app.Path
		appName = app.Name
	}

	if _, err := s.db.InsertEvent(storage.Event{
		TS:      time.Now().UTC(),
		Type:    storage.EventTypeVoice,
		AppPath: appPath,
		AppName: appName,
		Content: text,
		Meta:    fmt.Sprintf("engine=%s;duration_ms=%d", engine.Name(), elapsed.Milliseconds()),
	}); err != nil {
		s.logger.Warn("写入 voice event 失败", "err", err)
	}

	// 返回最终结果
	if err := stream.Send(&AsrResult{
		FinalText: text,
		Done:      true,
	}); err != nil {
		return status.Errorf(codes.Internal, "发送识别结果失败: %v", err)
	}
	return nil
}
