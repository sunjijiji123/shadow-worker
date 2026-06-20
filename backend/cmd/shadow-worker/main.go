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
	"net"
	"os"

	"google.golang.org/grpc"

	"shadow-worker/backend/internal/asr"
	"shadow-worker/backend/internal/collector"
	"shadow-worker/backend/internal/config"
	pb "shadow-worker/backend/internal/grpcapi"
	"shadow-worker/backend/internal/llm"
	mcpServer "shadow-worker/backend/internal/mcp"
	"shadow-worker/backend/internal/storage"
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

// runBackgroundService 启动后台服务(gRPC + 采集引擎 + ASR)。
func runBackgroundService() {
	// 1. 加载配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 2. 打开 SQLite
	db, err := storage.Open()
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer db.Close()

	// 3. 启动采集引擎
	coll := collector.NewCollector(db, cfg.Movement.Precision, nil)
	coll.Start()
	defer coll.Stop()

	// 4. 创建 ASR 引擎（用 holder 包装，支持配置变更后热重载）
	asrEngine, err := asr.New(cfg)
	if err != nil {
		log.Fatalf("创建 ASR 引擎失败: %v", err)
	}
	holder := asr.NewEngineHolder(asrEngine)
	log.Printf("ASR 引擎: %s", asrEngine.Name())

	// 4b. 创建润色引擎（LLM 未启用时为 nil，holder 仍可后续热重载启用）
	llmEngine, err := llm.New(cfg)
	if err != nil {
		log.Printf("创建 LLM 引擎失败（润色不可用）: %v", err)
	}
	llmHolder := llm.NewEngineHolder(llmEngine)
	if llmEngine != nil {
		log.Printf("LLM 引擎: %s", llmEngine.Name())
	} else {
		log.Printf("LLM 引擎: 未启用（润色关闭）")
	}

	// 5. 启动 VLM 截图理解(可选)
	var vlmCapturer *collector.VLMCapturer
	if cfg.VLM.Mode != "off" && cfg.VLM.Mode != "" {
		vlmCapturer, err = collector.NewVLMCapturer(cfg.VLM, db, nil)
		if err != nil {
			log.Printf("创建 VLM 引擎失败: %v", err)
		} else {
			vlmCapturer.Start()
			defer vlmCapturer.Stop()
			log.Printf("VLM 引擎: %s, mode=%s", vlmCapturer.EngineName(), cfg.VLM.Mode)
		}
	}

	// 5. 监听 TCP
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("监听 %s 失败: %v", grpcAddr, err)
	}

	// 6. 创建并注册 gRPC server
	grpcServer := grpc.NewServer()
	pb.RegisterOverviewServiceServer(grpcServer, pb.NewOverviewServer(db, coll))
	pb.RegisterWhitelistServiceServer(grpcServer, pb.NewWhitelistServer(db))
	pb.RegisterAsrServiceServer(grpcServer, pb.NewAsrServer(db, holder, nil))
	pb.RegisterVoiceServiceServer(grpcServer, pb.NewVoiceServer(db, holder, llmHolder))
	pb.RegisterCollectionServiceServer(grpcServer, pb.NewCollectionServer(db, coll, vlmCapturer))
	pb.RegisterConfigServiceServer(grpcServer, pb.NewConfigServer(cfg, holder, llmHolder))

	log.Printf("Shadow Worker 后台服务已启动,gRPC 监听 %s", grpcAddr)
	log.Printf("Qt 客户端请连接 %s", grpcAddr)

	// 7. 阻塞服务
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("gRPC 服务失败: %v", err)
	}
}
