# MCP Server 实现规范

> Shadow Worker 通过 stdio MCP 暴露 4 个工作日志查询工具给 AI agent。
> Agent(ZCode/Claude/Cursor)启动 `shadow-worker.exe --mcp` 子进程,stdin/stdout 通信。

---

## 1. SDK 选型:官方 modelcontextprotocol/go-sdk

**理由:**
- 2026 已成熟,mark3labs/mcp-go(社区最流行,400+ 包用)基本被官方吸收
- 官方维护,长期跟 MCP 协议规范同步
- API 设计干净:`mcp.NewServer` + `server.AddTool`

```go
// go.mod 加
require github.com/modelcontextprotocol/go-sdk v0.x.x
```

---

## 2. 启动模式(一个二进制两种角色)

```go
// cmd/shadow-worker/main.go

func main() {
    if len(os.Args) > 1 && os.Args[1] == "--mcp" {
        // MCP server 模式:stdio 通信,不启动 gRPC/采集
        runMCPServer()
        return
    }
    // 默认:后台服务模式(gRPC + 采集)
    runBackgroundService()
}
```

**关键:MCP 模式和后台模式共享 SQLite 文件,各自独立读 DB。** MCP 模式是只读客户端,不写数据。

---

## 3. 4 个工具实现

### 工具清单

| 工具 | 用途 | 入参 | 出参 |
|------|------|------|------|
| get_worklog | 生成日报的主力 | date, category?, app? | 工作日志 JSON |
| get_summary | 聚合统计 | date, group_by? | 时长分组 |
| search_events | 关键词搜 | query, date?, type? | 事件列表 |
| list_apps | 列白名单 | date? | 应用列表 |

### 框架代码

```go
// internal/mcp/server.go
package mcp

import (
    "context"
    "github.com/modelcontextprotocol/go-sdk/mcp"
)

func RunServer(store *storage.Store) error {
    s := mcp.NewServer("shadow-worker", "0.1.0", nil)

    // 工具 1: get_worklog
    s.AddTool(mcp.NewTool("get_worklog",
        "获取指定日期的工作日志(时段+事件+摘要),用于生成日报",
        GetWorklogSchema{},
        func(ctx context.Context, req *mcp.CallToolRequest, args GetWorklogArgs) (*mcp.CallToolResult, error) {
            data, err := store.GetWorklog(ctx, args.Date, args.Category, args.App)
            if err != nil {
                return nil, err
            }
            jsonBytes, _ := json.Marshal(data)
            return &mcp.CallToolResult{
                Content: []mcp.Content{&mcp.TextContent{Text: string(jsonBytes)}},
            }, nil
        }),
    )

    // 工具 2-4: 同样模式注册

    // 启动 stdio transport
    return s.RunStdio(context.Background())
}
```

### 入参/出参 schema(对应 docs/grpc-mcp-api.md)

```go
type GetWorklogArgs struct {
    Date     string `json:"date"     description:"日期 YYYY-MM-DD"`
    Category string `json:"category,omitempty" description:"只返该类别"`
    App      string `json:"app,omitempty"      description:"只返该应用"`
}

type WorklogResult struct {
    Date               string         `json:"date"`
    TotalActiveMinutes int            `json:"total_active_minutes"`
    Segments           []SegmentOut   `json:"segments"`
    Events             []EventOut     `json:"events"`
}

type SegmentOut struct {
    Start    string `json:"start"`     // "09:00"
    End      string `json:"end"`       // "11:00"
    App      string `json:"app"`
    Category string `json:"category"`
    Minutes  int    `json:"minutes"`
    Summary  string `json:"summary,omitempty"` // VLM 摘要,没有则省略
    Hint     string `json:"hint,omitempty"`    // coding 段:"use code-worklog skill"
}
```

---

## 4. 数据装配逻辑(关键!)

`store.GetWorklog` 是核心,从 SQLite 拼装:

```go
// internal/storage/worklog.go

func (s *Store) GetWorklog(ctx context.Context, date, category, app string) (*WorklogResult, error) {
    // 1. 查当天 activity_segments
    segs, _ := s.querySegments(date, category, app)

    // 2. 查当天 events(voice/prompt_inject/vlm_summary)
    events, _ := s.queryEvents(date)

    // 3. 装配 segment,关键:summary_source 区分
    out := &WorklogResult{Date: date}
    for _, seg := range segs {
        so := SegmentOut{
            Start:    formatTime(seg.StartTs),
            End:      formatTime(seg.EndTs),
            App:      seg.AppName,
            Category: seg.Category,
            Minutes:  int(seg.EndTs-seg.StartTs) / 60,
        }
        // coding 类:summary 来自 code-worklog,我们不提供,加 hint
        if seg.Category == "coding" {
            so.Hint = "use code-worklog skill"
        } else if seg.SummarySource == "vlm" {
            so.Summary = seg.ContentSummary
        }
        out.Segments = append(out.Segments, so)
        out.TotalActiveMinutes += so.Minutes
    }

    // 4. events 转 EventOut
    for _, e := range events {
        out.Events = append(out.Events, EventOut{
            Time: formatTime(e.Ts),
            Type: e.Type,
            Text: e.Content,
        })
    }
    return out, nil
}
```

---

## 5. agent 侧配置示例

用户在 ZCode/Claude/Cursor 的 MCP 配置加:

```json
{
  "mcpServers": {
    "shadow-worker": {
      "command": "C:\\Users\\Administrator\\code\\1-ai\\shadow-worker\\backend\\bin\\shadow-worker.exe",
      "args": ["--mcp"]
    }
  }
}
```

测试:`shadow-worker.exe --mcp` 启动后,stdin 发 `{"jsonrpc":"2.0","method":"tools/list","id":1}` 应返回 4 个工具定义。

---

## 6. 注意事项

1. **stdio 模式不能写 stderr 干扰协议**:日志写文件,不写 stderr
2. **MCP 模式不启动采集**:只读 SQLite,避免和后台服务进程竞争
3. **SQLite 并发**:多进程读单进程写,SQLite 默认支持,但要用 `?_journal_mode=WAL` 连接串
4. **截图不返回**:用户选了"只返摘要文字",日后要再加 `get_screenshot` 工具
