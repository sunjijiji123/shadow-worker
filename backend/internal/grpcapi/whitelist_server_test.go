package grpcapi

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"shadow-worker/backend/internal/storage"
)

func newWhitelistTestDB(t *testing.T) *storage.DB {
	t.Helper()
	db, err := storage.OpenAt(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestWhitelistServerAddListRemove(t *testing.T) {
	db := newWhitelistTestDB(t)
	srv := NewWhitelistServer(db)
	ctx := context.Background()

	added, err := srv.Add(ctx, &AddAppRequest{
		Path:     `C:\Program Files\Cursor\Cursor.exe`,
		Name:     "Cursor",
		Category: "coding",
	})
	if err != nil {
		t.Fatalf("Add 失败: %v", err)
	}
	if added.Name != "Cursor" {
		t.Fatalf("返回应用名不匹配: %s", added.Name)
	}

	list, err := srv.List(ctx, &ListAppsRequest{})
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(list.Apps) != 1 {
		t.Fatalf("应用数量应为 1，实际 %d", len(list.Apps))
	}

	res, err := srv.Remove(ctx, &RemoveAppRequest{Path: added.Path})
	if err != nil {
		t.Fatalf("Remove 失败: %v", err)
	}
	if !res.Ok {
		t.Fatalf("Remove 应成功，实际返回 %v", res)
	}

	list, _ = srv.List(ctx, &ListAppsRequest{})
	if len(list.Apps) != 0 {
		t.Fatalf("删除后应用数量应为 0，实际 %d", len(list.Apps))
	}
}

func TestWhitelistServerListTodayMinutes(t *testing.T) {
	db := newWhitelistTestDB(t)
	srv := NewWhitelistServer(db)
	ctx := context.Background()

	path := `C:\Program Files\Cursor\Cursor.exe`
	if _, err := srv.Add(ctx, &AddAppRequest{Path: path, Name: "Cursor", Category: "coding"}); err != nil {
		t.Fatalf("Add 失败: %v", err)
	}

	now := time.Now().UTC()
	start := now.Truncate(24 * time.Hour).Add(time.Hour)
	end := start.Add(2 * time.Hour)
	if _, err := db.InsertActivitySegment(storage.ActivitySegment{
		StartTS:  start,
		EndTS:    end,
		AppPath:  path,
		AppName:  "Cursor",
		Category: "coding",
		State:    "active",
	}); err != nil {
		t.Fatalf("插入段失败: %v", err)
	}

	list, err := srv.List(ctx, &ListAppsRequest{})
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(list.Apps) != 1 {
		t.Fatalf("应用数量应为 1，实际 %d", len(list.Apps))
	}
	if list.Apps[0].TodayMinutes != 120 {
		t.Fatalf("今日时长应为 120 分钟，实际 %d", list.Apps[0].TodayMinutes)
	}
}
