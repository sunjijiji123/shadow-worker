// Package main 是 shadow-worker Go 后台服务的入口。
//
// 两种运行模式:
//   1. 默认(无参): 启动 gRPC + 采集引擎 + SQLite 的后台服务
//   2. --mcp:      启动 stdio MCP server,只读 SQLite,供 AI agent 调用
//
// 启动后台:
//   go run cmd/shadow-worker/main.go
//
// 启动 MCP:
//   go run cmd/shadow-worker/main.go --mcp
package main

import (
	"context"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"google.golang.org/grpc"

	"shadow-worker/backend/internal/asr"
	"shadow-worker/backend/internal/collector"
	"shadow-worker/backend/internal/config"
	pb "shadow-worker/backend/internal/grpcapi"
	"shadow-worker/backend/internal/llm"
	"shadow-worker/backend/internal/logging"
	mcpServer "shadow-worker/backend/internal/mcp"
	"shadow-worker/backend/internal/storage"
	"shadow-worker/backend/internal/winapi"
)

const (
	// gRPC 监听地址。本机内部通信,用 localhost 而非 0.0.0.0。
	grpcAddr = "127.0.0.1:50051"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--mcp" {
		runMCPServer()
		return
	}
	runBackgroundService()
}

// runMCPServer 启动 stdio MCP server。
func runMCPServer() {
	db, err := storage.Open()
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer db.Close()

	log.Println("Shadow Worker MCP server 已启动(stdio)")
	if err := mcpServer.NewServer(db).RunStdio(context.Background()); err != nil {
		log.Fatalf("MCP server 失败: %v", err)
	}
}

// readVersion 从 exe 同目录的 VERSION 文件读取版本号。
// 文件不存在或读取失败时返回 "unknown"。
func readVersion() string {
	exePath, err := os.Executable()
	if err != nil {
		return "unknown"
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(exePath), "VERSION"))
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(data))
}

// runBackgroundService 启动后台服务(gRPC + 采集引擎 + ASR)。
func runBackgroundService() {
	// 0. 单例检查（在所有初始化之前）。
	// 用 Local\ 前缀：Global\ 需要 SeCreateGlobalPrivilege，非管理员用户会
	// ACCESS_DENIED。桌面应用每个会话独立，不同 RDP 用户各跑一份。
	// 进程崩溃时内核自动回收 mutex，不会死锁。
	mu, exists, err := winapi.CreateMutex("Local\\Shadow-Worker-Backend")
	if err != nil {
		log.Fatalf("创建互斥体失败: %v", err)
	}
	if exists {
		log.Fatal("另一个 Shadow Worker 后端实例已在运行，即将退出。")
	}
	defer winapi.ReleaseMutex(mu)

	// 1. 读取版本号（从 exe 同目录的 VERSION 文件）
	version := readVersion()

	// 2. 加载配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 1b. 初始化日志（按天文件 + 可选控制台 + 级别）。
	// 必须在 config.Load 之后、业务启动之前。返回 closer 用于退出时刷盘。
	logCloser, err := logging.Init(cfg.Log.Level, cfg.Log.Console, cfg.Log.RetentionDays)
	if err != nil {
		log.Fatalf("初始化日志失败: %v", err)
	}
	defer logCloser()

	slog.Info("Shadow Worker 后端启动", "version", version)

	// 将 debug 截图开关注入到各采集模块（独立控制，避免互相干扰）。
	// 这样模块内部只需读自己的 cfg.SaveScreenshots，无需感知全局 Debug 结构。
	cfg.VLM.SaveScreenshots = cfg.Debug.SaveVLMScreenshots
	cfg.Movement.SaveScreenshots = cfg.Debug.SaveMotionScreenshots
	if cfg.Debug.SaveVLMScreenshots {
		slog.Info("VLM 截图落盘已开启（送去 VLM 分析的图将保存到 screenshots/ 目录）")
	}
	if cfg.Debug.SaveMotionScreenshots {
		slog.Info("帧差截图落盘已开启（movement 活动窗口帧将保存到 screenshots/ 目录，Electron 应用会高频）")
	}

	// 2. 打开 SQLite
	db, err := storage.Open()
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer db.Close()

	// 3. 启动采集引擎（传入配置好的 logger，采集日志进文件）。
	coll := collector.NewCollector(db, cfg.Movement, slog.Default())
	coll.Start()
	defer coll.Stop()

	// 4. 创建 ASR 引擎（用 holder 包装，支持配置变更后热重载）。
	// ASR 是可选模块：用户可能只用采集功能、不配 ASR。创建失败（如未配 provider）
	// 不应阻断后端启动，仅禁用语音识别，采集/白名单/时间线等照常工作。
	// 用户后续在设置页配置好 ASR 后，ConfigServer.Rebuild 会热重建引擎。
	asrEngine, err := asr.New(cfg, slog.Default())
	if err != nil {
		// 可选模块失败：语音识别不可用，但其它功能正常。需关注（Error），但不致命。
		slog.Error("创建 ASR 引擎失败（语音识别不可用，其它功能正常）", "err", err)
	} else {
		slog.Info("ASR 引擎已就绪", "engine", asrEngine.Name())
	}
	holder := asr.NewEngineHolder(asrEngine, slog.Default())

	// 4b. 创建润色引擎。只要配置了有效的 LLM provider 就创建（手动润色可用），
	// 不受"自动润色"开关（cfg.LLM.Enabled）影响——后者只控制识别后是否自动触发。
	// 没配 provider 时 llmEngine 为 nil，手动润色返回"LLM 未启用"。
	llmEngine, err := llm.New(cfg, slog.Default())
	if err != nil {
		slog.Error("创建 LLM 引擎失败（润色不可用，请在设置页配置 LLM provider）", "err", err)
	}
	llmHolder := llm.NewEngineHolder(llmEngine, slog.Default())
	if llmEngine != nil {
		slog.Info("LLM 引擎已就绪", "engine", llmEngine.Name())
	} else {
		slog.Info("LLM 引擎未启用（未配置 provider，润色不可用）")
	}

	// 5. 启动 VLM 截图理解(可选)
	// 与 ASR/LLM 一样用 holder 包装：配置变更后 SaveConfig → vlmHolder.Rebuild 热重载。
	// off 模式 / screen+on_demand 非法组合 → capturer 为 nil，holder 也允许持有 nil。
	var vlmCapturer *collector.VLMCapturer
	if cfg.VLM.Mode != "off" && cfg.VLM.Mode != "" {
		vlmCapturer, err = collector.NewVLMCapturer(cfg.VLM, db, slog.Default())
		if err != nil {
			slog.Error("创建 VLM 引擎失败", "err", err)
		} else {
			vlmCapturer.Start()
			slog.Info("VLM 引擎已就绪", "engine", vlmCapturer.EngineName(), "mode", cfg.VLM.Mode)
		}
	}
	vlmHolder := collector.NewVLMHolder(vlmCapturer)
	// 注入 VLM holder 到采集引擎：loop 在活跃信号上回调 on_demand 触发。
	// holder 可为 nil（VLM 未配置），notifyVLMActivity 内部会判空跳过。
	coll.SetVLMHolder(vlmHolder)
	defer func() {
		// 停 holder 内当前 capturer（热重载可能已替换实例）。
		if c := vlmHolder.Get(); c != nil {
			c.Stop()
		}
	}()

	// 5. 监听 TCP
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("监听 %s 失败: %v", grpcAddr, err)
	}

	// 6. 创建并注册 gRPC server
	grpcServer := grpc.NewServer()
	pb.RegisterOverviewServiceServer(grpcServer, pb.NewOverviewServer(db, coll))
	pb.RegisterWhitelistServiceServer(grpcServer, pb.NewWhitelistServer(db, slog.Default()))
	pb.RegisterAsrServiceServer(grpcServer, pb.NewAsrServer(db, holder, slog.Default()))
	pb.RegisterVoiceServiceServer(grpcServer, pb.NewVoiceServer(db, holder, llmHolder, slog.Default()))
	pb.RegisterCollectionServiceServer(grpcServer, pb.NewCollectionServer(db, coll, vlmHolder))
	pb.RegisterConfigServiceServer(grpcServer, pb.NewConfigServer(cfg, holder, llmHolder, vlmHolder, db, slog.Default()))

	slog.Info("后台服务已启动", "grpc_addr", grpcAddr)
	slog.Info("Qt 客户端请连接", "addr", grpcAddr)

	// 7. 优雅退出：signal.NotifyContext + goroutine Serve。
	// taskkill（不带 /F）或 Ctrl+C 触发 SIGTERM → GracefulStop → defer 正常执行。
	// taskkill /F 是强杀，不触发信号——作为客户端的兜底手段。
	ctx, stop := signal.NotifyContext(
		context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverDone := make(chan struct{})
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("gRPC 服务失败: %v", err)
		}
		close(serverDone)
	}()

	select {
	case <-ctx.Done():
		slog.Info("收到退出信号，开始优雅关闭...")
		grpcServer.GracefulStop()
		<-serverDone
		slog.Info("已优雅关闭")
	case <-serverDone:
	}
}
