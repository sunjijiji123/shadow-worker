// Package main 是 shadow-worker MCP server 的独立入口（stdio）。
//
// 为什么独立 exe：MCP server 只读 SQLite（modernc.org/sqlite，纯 Go，无 CGO），
// 不依赖 whisper/asr/collector/llm 等模块。把它从主后端 shadow-worker.exe 拆出，
// 编译成 shadow-worker-mcp.exe（纯 Go，几秒构建，无需 gcc），让外部 AI agent
// 持有的 MCP 子进程跑在独立文件上——升级主程序时覆盖 shadow-worker.exe 不再被
// MCP 子进程文件锁阻断（覆盖安装卡死的根因，见 AGENTS.md 坑 50）。
//
// 启动：
//
//	shadow-worker-mcp.exe
//
// agent 的 MCP 配置示例（由客户端「系统设置 → MCP 服务」生成）：
//
//	{ "mcpServers": { "shadow-worker": { "command": "...\\shadow-worker-mcp.exe" } } }
package main

import (
	"context"
	"log"

	mcpServer "shadow-worker/backend/internal/mcp"
	"shadow-worker/backend/internal/storage"
)

func main() {
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
