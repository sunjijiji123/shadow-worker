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

	// 工具 1: get_worklog
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_worklog",
		Description: "获取指定日期的工作日志（时段+事件），用于生成日报/周报",
	}, s.handleGetWorklog)

	// 工具 2: get_summary
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_summary",
		Description: "按类别或应用聚合统计某一天的活跃时长",
	}, s.handleGetSummary)

	// 工具 3: search_events
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_events",
		Description: "按关键词搜索语音/VLM/截图事件",
	}, s.handleSearchEvents)

	// 工具 4: list_apps
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_apps",
		Description: "列出已加入白名单的应用及今日活跃时长",
	}, s.handleListApps)

	return server.Run(ctx, &mcp.StdioTransport{})
}

// GetWorklogArgs 是 get_worklog 的入参。
type GetWorklogArgs struct {
	Date     string `json:"date" jsonschema:"日期，格式 YYYY-MM-DD，必填"`
	Category string `json:"category,omitempty" jsonschema:"可选：只返回该类别"`
	App      string `json:"app,omitempty" jsonschema:"可选：只返回该应用"`
}

func (s *Server) handleGetWorklog(ctx context.Context, req *mcp.CallToolRequest, args GetWorklogArgs) (*mcp.CallToolResult, *storage.WorklogResult, error) {
	if args.Date == "" {
		return nil, nil, fmt.Errorf("date 不能为空")
	}
	res, err := s.db.GetWorklog(ctx, args.Date, args.Category, args.App)
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

