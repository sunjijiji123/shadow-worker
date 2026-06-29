// Package mcp 实现 Shadow Worker 的 stdio MCP server。
//
// 暴露 4 个工具：get_worklog / get_summary / search_events / list_apps。
// agent 通过启动 shadow-worker.exe --mcp 子进程调用。
package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"shadow-worker/backend/internal/storage"
)

// Server 包装 MCP server 和数据库。
type Server struct {
	db *storage.DB
}

// NewServer 创建 MCP server 实例。
func NewServer(db *storage.DB) *Server {
	return &Server{db: db}
}

// RunStdio 启动 stdio 模式的 MCP server。
func (s *Server) RunStdio(ctx context.Context) error {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "shadow-worker",
		Version: "0.1.0",
	}, nil)

	// 工具 1: get_worklog（详细工作日志，分页防截断）
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_worklog",
		Description: "获取指定日期的详细工作日志（活动时段 + 事件），用于生成日报/周报。segments 默认只返回前50条（用 limit/offset 翻页，返回含 total_segments/has_more）。events 默认只返回 voice（语音转写），vlm_summary 按需用 event_types 参数开启（高频碎、单日可达数百条）。建议先用 get_worklog_summary 看全局概况，再按需带 category/app 过滤下钻。",
	}, s.handleGetWorklog)

	// 工具 2: get_worklog_summary（压缩摘要，三时段聚合，~1KB）
	// 与 get_summary 区别：后者纯统计（按 category/app 聚合，不分时段）；
	// 本工具面向工作日志的"上午/下午/晚上"作息概览，先看全局再下钻。
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_worklog_summary",
		Description: "获取指定日期工作日志的压缩摘要（按上午/下午/晚上三时段聚合：每段的类别分钟数 + top3 应用）。token 开销小（~1KB），适合先了解一天全局概况。需要逐条细节时改用 get_worklog（支持 category/app 过滤 + limit/offset 分页）。",
	}, s.handleGetWorklogSummary)

	// 工具 3: get_summary（纯统计聚合）
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_summary",
		Description: "按类别或应用聚合统计某一天的活跃时长",
	}, s.handleGetSummary)

	// 工具 4: search_events
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_events",
		Description: "按关键词搜索语音/VLM/截图事件",
	}, s.handleSearchEvents)

	// 工具 5: list_apps
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_apps",
		Description: "列出已加入白名单的应用及今日活跃时长",
	}, s.handleListApps)

	return server.Run(ctx, &mcp.StdioTransport{})
}

// GetWorklogArgs 是 get_worklog 的入参。
type GetWorklogArgs struct {
	Date       string   `json:"date" jsonschema:"日期，格式 YYYY-MM-DD，必填"`
	Category   string   `json:"category,omitempty" jsonschema:"可选：只返回该类别"`
	App        string   `json:"app,omitempty" jsonschema:"可选：只返回该应用"`
	Limit      int      `json:"limit,omitempty" jsonschema:"可选：返回段数上限，默认50，最大200。日志量大时分页拉取"`
	Offset     int      `json:"offset,omitempty" jsonschema:"可选：跳过的段数，用于分页翻页，默认0"`
	EventTypes []string `json:"event_types,omitempty" jsonschema:"可选：返回的事件类型，如 [\"voice\",\"vlm_summary\"]。默认只返回 voice（语音转写）。vlm_summary（VLM屏幕摘要）高频且碎、单日可达数百条，按需显式开启；可用值：voice/prompt_inject/screenshot/vlm_summary"`
}

func (s *Server) handleGetWorklog(ctx context.Context, req *mcp.CallToolRequest, args GetWorklogArgs) (*mcp.CallToolResult, *storage.WorklogResult, error) {
	if args.Date == "" {
		return nil, nil, fmt.Errorf("date 不能为空")
	}
	// 业务默认：不传 event_types 时只返回 voice（语音转写对日报有价值且量小）。
	// vlm_summary 高频碎（实测单日 422 条 ~16KB），默认不返回，按需显式开启。
	// 见坑 #55。传空切片 [] 表示"不要任何事件"（与 nil 的"要全部"不同语义，
	// 但此处把 nil 也当作默认 voice，避免 agent 忘传时拉爆上下文）。
	eventTypes := args.EventTypes
	if eventTypes == nil {
		eventTypes = []string{string(storage.EventTypeVoice)}
	}
	res, err := s.db.GetWorklog(ctx, args.Date, args.Category, args.App, args.Limit, args.Offset, eventTypes)
	if err != nil {
		return nil, nil, err
	}
	return nil, res, nil
}

// GetWorklogSummaryArgs 是 get_worklog_summary 的入参。
type GetWorklogSummaryArgs struct {
	Date string `json:"date" jsonschema:"日期，格式 YYYY-MM-DD，必填"`
}

func (s *Server) handleGetWorklogSummary(ctx context.Context, req *mcp.CallToolRequest, args GetWorklogSummaryArgs) (*mcp.CallToolResult, *storage.WorklogSummaryResult, error) {
	if args.Date == "" {
		return nil, nil, fmt.Errorf("date 不能为空")
	}
	res, err := s.db.GetWorklogSummary(ctx, args.Date)
	if err != nil {
		return nil, nil, err
	}
	return nil, res, nil
}

// GetSummaryArgs 是 get_summary 的入参。
type GetSummaryArgs struct {
	Date    string `json:"date" jsonschema:"日期，格式 YYYY-MM-DD，必填"`
	GroupBy string `json:"group_by,omitempty" jsonschema:"分组维度：category(默认) 或 app"`
}

func (s *Server) handleGetSummary(ctx context.Context, req *mcp.CallToolRequest, args GetSummaryArgs) (*mcp.CallToolResult, *storage.SummaryResult, error) {
	if args.Date == "" {
		return nil, nil, fmt.Errorf("date 不能为空")
	}
	res, err := s.db.GetSummary(ctx, args.Date, args.GroupBy)
	if err != nil {
		return nil, nil, err
	}
	return nil, res, nil
}

// SearchEventsArgs 是 search_events 的入参。
type SearchEventsArgs struct {
	Query string `json:"query" jsonschema:"搜索关键词，必填"`
	Date  string `json:"date,omitempty" jsonschema:"可选：限定日期 YYYY-MM-DD"`
	Type  string `json:"type,omitempty" jsonschema:"可选：事件类型 voice/prompt_inject/screenshot/vlm_summary"`
}

func (s *Server) handleSearchEvents(ctx context.Context, req *mcp.CallToolRequest, args SearchEventsArgs) (*mcp.CallToolResult, *storage.SearchEventsResult, error) {
	if args.Query == "" && args.Type == "" {
		return nil, nil, fmt.Errorf("query 和 type 至少填一个")
	}
	res, err := s.db.SearchEventsByQuery(ctx, args.Query, args.Date, args.Type)
	if err != nil {
		return nil, nil, err
	}
	return nil, res, nil
}

// ListAppsArgs 是 list_apps 的入参。
type ListAppsArgs struct{}

func (s *Server) handleListApps(ctx context.Context, req *mcp.CallToolRequest, args ListAppsArgs) (*mcp.CallToolResult, *storage.ListAppsResult, error) {
	res, err := s.db.ListAppsWithTodayMinutes(ctx)
	if err != nil {
		return nil, nil, err
	}
	return nil, res, nil
}

