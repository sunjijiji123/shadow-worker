package mcp

import (
	"context"
	"encoding/json"
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
	mcp.AddTool(server, &mcp.Tool{Name: "get_worklog_summary"}, (&Server{db: db}).handleGetWorklogSummary)
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

	if len(names) != 5 {
		t.Fatalf("应有 5 个工具，实际 %d: %v", len(names), names)
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

	// 验证分页元数据字段（坑 #55）：单段场景下 total_segments=1, has_more=false。
	var wl struct {
		TotalSegments    int  `json:"total_segments"`
		ReturnedSegments int  `json:"returned_segments"`
		HasMore          bool `json:"has_more"`
	}
	if err := json.Unmarshal([]byte(text), &wl); err != nil {
		t.Fatalf("解析 worklog JSON 失败: %v", err)
	}
	if wl.TotalSegments != 1 || wl.ReturnedSegments != 1 {
		t.Errorf("total_segments/returned_segments 应为 1/1，实际 %d/%d", wl.TotalSegments, wl.ReturnedSegments)
	}
	if wl.HasMore {
		t.Errorf("单段场景 has_more 应为 false")
	}
}

// insertSeg 是插段 helper：在 base 时刻插一条 active 段。
func insertSeg(t *testing.T, db *storage.DB, appPath, appName, category, state string, start, end time.Time) {
	t.Helper()
	if _, err := db.InsertActivitySegment(storage.ActivitySegment{
		StartTS:  start,
		EndTS:    end,
		AppPath:  appPath,
		AppName:  appName,
		Category: category,
		State:    state,
	}); err != nil {
		t.Fatalf("插入段失败: %v", err)
	}
}

func TestMCPServerGetWorklogPagination(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	// 插 60 条段（coding），验证默认 limit=50 分页 + offset 翻页。
	appPath := `C:\TestApps\Cursor.exe`
	if err := db.AddAppCategory(storage.AppCategory{Path: appPath, Name: "Cursor", Category: "coding"}); err != nil {
		t.Fatalf("添加白名单失败: %v", err)
	}
	base := today().Add(time.Hour)
	for i := 0; i < 60; i++ {
		// 每段间隔 2 分钟，避免时间重叠；StartTS 单调递增保证 ListActivitySegments 顺序稳定。
		insertSeg(t, db, appPath, "Cursor", "coding", "active",
			base.Add(time.Duration(i*2)*time.Minute),
			base.Add(time.Duration(i*2+1)*time.Minute))
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

	parse := func(args map[string]any) (int, int, bool) {
		t.Helper()
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_worklog", Arguments: args})
		if err != nil {
			t.Fatalf("CallTool 失败: %v", err)
		}
		var wl struct {
			TotalSegments    int  `json:"total_segments"`
			ReturnedSegments int  `json:"returned_segments"`
			HasMore          bool `json:"has_more"`
		}
		if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &wl); err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		return wl.TotalSegments, wl.ReturnedSegments, wl.HasMore
	}

	date := today().Format("2006-01-02")

	// 第 1 页：limit=50（默认），应返回 50 条，还有更多。
	total, ret, more := parse(map[string]any{"date": date})
	if total != 60 || ret != 50 || !more {
		t.Errorf("第1页: total=%d(要60) returned=%d(要50) has_more=%v(要true)", total, ret, more)
	}

	// 第 2 页：offset=50，应返回 10 条，无更多。
	_, ret, more = parse(map[string]any{"date": date, "limit": 50, "offset": 50})
	if ret != 10 || more {
		t.Errorf("第2页: returned=%d(要10) has_more=%v(要false)", ret, more)
	}

	// offset 越界（offset=100）：应返回 0 条不 panic，has_more=false。
	_, ret, more = parse(map[string]any{"date": date, "offset": 100})
	if ret != 0 || more {
		t.Errorf("越界页: returned=%d(要0) has_more=%v(要false)", ret, more)
	}

	// limit=0 → 走默认 50。
	_, ret, _ = parse(map[string]any{"date": date, "limit": 0})
	if ret != 50 {
		t.Errorf("limit=0 应走默认50, 实际 returned=%d", ret)
	}
}

func TestMCPServerGetWorklogSummary(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	// 模拟一天跨三时段的活动：
	//   morning(09:00): Cursor coding 60min, Chrome browser 30min
	//   afternoon(14:00): Cursor coding 90min
	//   evening(20:00): WeChat chat 45min
	// evening 之后不插，验证"无活动时段"不出现在结果（此例三时段都有，另测空桶）。
	appCursor := `C:\TestApps\Cursor.exe`
	appChrome := `C:\TestApps\chrome.exe`
	appWeChat := `C:\TestApps\WeChat.exe`
	for _, a := range []storage.AppCategory{
		{Path: appCursor, Name: "Cursor", Category: "coding"},
		{Path: appChrome, Name: "Chrome", Category: "browser"},
		{Path: appWeChat, Name: "WeChat", Category: "chat"},
	} {
		if err := db.AddAppCategory(a); err != nil {
			t.Fatalf("添加白名单失败: %v", err)
		}
	}

	m := today() // UTC 0 点
	// 用本地小时构造：早晨 9 点、下午 2 点、晚上 8 点。
	// today() 返回 UTC 0 点，要落到本地 9 点需 +本地时区偏移。测试机若非 UTC+8，
	// 用 time.Now() 的本地偏移计算，保证 StartTS.Local().Hour() 落在目标时段。
	loc := time.Now().Location()
	dayLocal := time.Date(m.Year(), m.Month(), m.Day(), 0, 0, 0, 0, loc)

	insertSeg(t, db, appCursor, "Cursor", "coding", "active",
		dayLocal.Add(9*time.Hour), dayLocal.Add(10*time.Hour))            // morning 60min
	insertSeg(t, db, appChrome, "Chrome", "browser", "active",
		dayLocal.Add(9*time.Hour+30*time.Minute), dayLocal.Add(10*time.Hour)) // morning 30min
	insertSeg(t, db, appCursor, "Cursor", "coding", "active",
		dayLocal.Add(14*time.Hour), dayLocal.Add(15*time.Hour+30*time.Minute)) // afternoon 90min
	insertSeg(t, db, appWeChat, "WeChat", "chat", "active",
		dayLocal.Add(20*time.Hour), dayLocal.Add(20*time.Hour+45*time.Minute)) // evening 45min

	srv := NewServer(db)
	server := mcp.NewServer(&mcp.Implementation{Name: "shadow-worker", Version: "0.1.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "get_worklog_summary"}, srv.handleGetWorklogSummary)

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
		Name:      "get_worklog_summary",
		Arguments: map[string]any{"date": today().Format("2006-01-02")},
	})
	if err != nil {
		t.Fatalf("CallTool 失败: %v", err)
	}
	var sum struct {
		TotalActiveMinutes int `json:"total_active_minutes"`
		Buckets            []struct {
			Period     string `json:"period"`
			Minutes    int    `json:"minutes"`
			Categories []struct {
				Category string `json:"category"`
				Minutes  int    `json:"minutes"`
			} `json:"categories"`
			TopApps []struct {
				App     string `json:"app"`
				Minutes int    `json:"minutes"`
			} `json:"top_apps"`
		} `json:"buckets"`
	}
	if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &sum); err != nil {
		t.Fatalf("解析 summary JSON 失败: %v", err)
	}
	t.Logf("summary: %+v", sum)

	if sum.TotalActiveMinutes != 225 { // 60+30+90+45
		t.Errorf("total_active_minutes 应为 225，实际 %d", sum.TotalActiveMinutes)
	}
	if len(sum.Buckets) != 3 {
		t.Fatalf("应有 3 个时段桶（三时段都有活动），实际 %d", len(sum.Buckets))
	}

	// 找到 morning 桶验证：90 分钟（coding 60 + browser 30），top1 app=Cursor。
	var morning *struct {
		Period     string `json:"period"`
		Minutes    int    `json:"minutes"`
		Categories []struct {
			Category string `json:"category"`
			Minutes  int    `json:"minutes"`
		} `json:"categories"`
		TopApps []struct {
			App     string `json:"app"`
			Minutes int    `json:"minutes"`
		} `json:"top_apps"`
	}
	for i := range sum.Buckets {
		if sum.Buckets[i].Period == "morning" {
			morning = &sum.Buckets[i]
		}
	}
	if morning == nil {
		t.Fatal("缺少 morning 桶")
	}
	if morning.Minutes != 90 {
		t.Errorf("morning 分钟数应为 90，实际 %d", morning.Minutes)
	}
	if len(morning.TopApps) == 0 || morning.TopApps[0].App != "Cursor" {
		t.Errorf("morning top1 app 应为 Cursor(60min)，实际 %+v", morning.TopApps)
	}
	// morning 两类别，coding(60) 应排在 browser(30) 前（降序）。
	if len(morning.Categories) < 2 || morning.Categories[0].Category != "coding" {
		t.Errorf("morning 类别降序应为 coding 在前，实际 %+v", morning.Categories)
	}
}

func today() time.Time {
	return time.Now().UTC().Truncate(24 * time.Hour)
}
