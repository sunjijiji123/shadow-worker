# Shadow Worker

> 一个跑在你电脑里的「行为采集器」。默默记录你的工作——语音、屏幕、应用切换,
> 生成带时间轴的结构化工作日志,通过本地 MCP 暴露给 AI agent,
> 让 agent 帮你生成日报、周报、汇报。

## 这是什么

Shadow Worker 是你的**本地工作行为数据源**。它:

- **采集**:你按热键说的话(语音转文字)、你在哪些应用上花了多久、屏幕内容理解(VLM)
- **结构化**:把零散行为组织成带时间轴的工作日志,存进本地 SQLite
- **暴露**:通过 stdio MCP 让任意 AI agent(ZCode/Claude/Cursor)查询你的工作日志

它**不做**日报生成——那交给 agent 和它的 LLM。它只做 agent 做不了的事:本地行为采集。

## 架构(一图)

```
            ┌──────────────────────────────────┐
            │        Shadow Worker             │
            │  ┌────────────┐  ┌────────────┐ │
   工作 ───→│  │ Go 服务    │  │ Qt 客户端  │ │
            │  │ (后台长驻) │←─│ (UI 可选)  │ │
            │  │ MCP+gRPC   │  │ 浮窗/设置  │ │
            │  │ 采集+SQLite│  │ 录音/注入  │ │
            │  └─────┬──────┘  └────────────┘ │
            └────────┼─────────────────────────┘
                     │ stdio MCP
                     ▼
            ┌──────────────────┐
            │  AI Agent        │ ──→ 日报/周报
            │  (ZCode/Claude)  │
            │  + code-worklog  │
            └──────────────────┘
```

**技术栈**:Go 服务 + Qt 6.8 LTS 客户端 + gRPC(本机)+ MCP(stdio)+ SQLite

## 快速开始(待实现)

> 项目正在搭建中,见 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) 了解完整设计。

```bash
# 1. 启动 Go 后台服务
cd backend && go run cmd/shadow-worker/main.go

# 2. 启动 Qt 客户端
cd client && cmake --build build

# 3. 在 agent(ZCode/Claude)配置 MCP
#    mcpServers.shadow-worker.command = "shadow-worker.exe"
#    mcpServers.shadow-worker.args = ["--mcp"]
```

## 项目结构

```
shadow-worker/
├── docs/        # 架构文档 + 接口契约
├── proto/       # gRPC 接口定义(单一真相源)
├── backend/     # Go 服务(采集 + MCP + gRPC + SQLite)
├── client/      # Qt 客户端(浮窗 + 设置 + 录音/注入)
└── scripts/     # 构建辅助
```

## 文档

- [架构设计](docs/ARCHITECTURE.md) — 完整设计:定位、技术栈、目录、数据流、开工顺序
- [接口契约](docs/grpc-mcp-api.md) — gRPC + MCP 工具详细定义

## 设计理念

1. **只做 agent 做不了的事**:本地行为采集。日报生成、code 提炼交给 agent
2. **本地优先**:全本地栈(whisper + 可选 Ollama),断网也能用,隐私不出本机
3. **白名单隐私**:只采集用户主动纳入的应用,未选的完全不留痕
4. **前后端分离**:Go 后台服务(开机自启)+ Qt 客户端(可选开关),进程解耦
