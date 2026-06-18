package mcp

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"shadow-worker/backend/internal/storage"
)

func newTestDB(t *testing.T) *storage.DB {
	t.Helper()
	db, err := storage.OpenAt(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestMCPServerListsTools(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	server := mcp.NewServer(&mcp.Implementation{Name: "shadow-worker", Version: "0.1.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{Name: "get_worklog"}, (&Server{db: db}).handleGetWorklog)
	mcp.AddTool(server, &mcp.Tool{Name: "get_summary"}, (&Server{db: db}).handleGetSummary)
	mcp.AddTool(server, &mcp.Tool{Name: "search_events"}, (&Server{db: db}).handleSearchEvents)
	mcp.AddTool(server, &mcp.Tool{Name: "list_apps"}, (&Server{db: db}).handleListApps)

	t1, t2 := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, t1, nil); err != nil {
		t.Fatalf("server connect 失败: %v", err)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	cs, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client connect 失败: %v", err)
	}
	defer cs.Close()

	var names []string
	for tool, err := range cs.Tools(ctx, nil) {
		if err != nil {
			t.Fatalf("遍历工具失败: %v", err)
		}
		names = append(names, tool.Name)
	}

	if len(names) != 4 {
		t.Fatalf("应有 4 个工具，实际 %d: %v", len(names), names)
	}
}

func TestMCPServerGetWorklog(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	appPath := `C:\TestApps\Cursor.exe`
	if err := db.AddAppCategory(storage.AppCategory{Path: appPath, Name: "Cursor", Category: "coding"}); err != nil {
		t.Fatalf("添加白名单失败: %v", err)
	}
	if _, err := db.InsertActivitySegment(storage.ActivitySegment{
		StartTS:  today().Add(time.Hour),
		EndTS:    today().Add(2 * time.Hour),
		AppPath:  appPath,
		AppName:  "Cursor",
		Category: "coding",
		State:    "active",
	}); err != nil {
		t.Fatalf("插入段失败: %v", err)
	}

	srv := NewServer(db)
	server := mcp.NewServer(&mcp.Implementation{Name: "shadow-worker", Version: "0.1.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "get_worklog"}, srv.handleGetWorklog)

	t1, t2 := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, t1, nil); err != nil {
		t.Fatalf("server connect 失败: %v", err)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	cs, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client connect 失败: %v", err)
	}
	defer cs.Close()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_worklog",
		Arguments: map[string]any{"date": today().Format("2006-01-02")},
	})
	if err != nil {
		t.Fatalf("CallTool 失败: %v", err)
	}
	if len(res.Content) == 0 {
		t.Fatal("结果内容为空")
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if text == "" {
		t.Fatal("返回文本为空")
	}
	t.Logf("get_worklog result: %s", text)
}

func today() time.Time {
	return time.Now().UTC().Truncate(24 * time.Hour)
}
