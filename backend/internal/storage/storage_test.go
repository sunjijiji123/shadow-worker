package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	db, err := OpenAt(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestOpen(t *testing.T) {
	db := newTestDB(t)
	if db == nil {
		t.Fatal("db 为 nil")
	}
}

func TestWhitelistCRUD(t *testing.T) {
	db := newTestDB(t)

	app := AppCategory{
		Path:     `C:\Program Files\Cursor\Cursor.exe`,
		Name:     "Cursor",
		Category: "coding",
	}
	if err := db.AddAppCategory(app); err != nil {
		t.Fatalf("添加应用失败: %v", err)
	}

	got, err := db.GetAppCategory(app.Path)
	if err != nil {
		t.Fatalf("查询应用失败: %v", err)
	}
	if got == nil {
		t.Fatal("应查到应用，但返回 nil")
	}
	if got.Name != "Cursor" || got.Category != "coding" {
		t.Fatalf("应用字段不匹配: %+v", got)
	}

	if err := db.UpdateAppCategory(app.Path, "Cursor IDE", "coding", "icon.png"); err != nil {
		t.Fatalf("更新应用失败: %v", err)
	}
	got, _ = db.GetAppCategory(app.Path)
	if got.Name != "Cursor IDE" || got.IconPath != "icon.png" {
		t.Fatalf("更新后字段不匹配: %+v", got)
	}

	apps, err := db.ListAppCategories()
	if err != nil {
		t.Fatalf("列出应用失败: %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("应用数量应为 1，实际 %d", len(apps))
	}

	if err := db.RemoveAppCategory(app.Path); err != nil {
		t.Fatalf("删除应用失败: %v", err)
	}
	got, _ = db.GetAppCategory(app.Path)
	if got != nil {
		t.Fatalf("删除后应查不到应用，但返回 %+v", got)
	}
}

func TestActivitySegmentCRUD(t *testing.T) {
	db := newTestDB(t)

	now := time.Now().UTC()
	seg := ActivitySegment{
		StartTS:     now.Add(-2 * time.Minute),
		EndTS:       now,
		AppPath:     `C:\Program Files\Cursor\Cursor.exe`,
		AppName:     "Cursor",
		Category:    "coding",
		WindowTitle: "main.go - shadow-worker",
		State:       "active",
	}
	id, err := db.InsertActivitySegment(seg)
	if err != nil {
		t.Fatalf("插入活动段失败: %v", err)
	}
	if id == 0 {
		t.Fatal("活动段 ID 不应为 0")
	}

	got, err := db.GetActivitySegment(id)
	if err != nil {
		t.Fatalf("查询活动段失败: %v", err)
	}
	if got == nil || got.AppName != "Cursor" {
		t.Fatalf("活动段不匹配: %+v", got)
	}

	newEnd := now.Add(time.Minute).Truncate(time.Second)
	if err := db.UpdateActivitySegmentEndTS(id, newEnd); err != nil {
		t.Fatalf("更新结束时间失败: %v", err)
	}
	got, _ = db.GetActivitySegment(id)
	if !got.EndTS.Equal(newEnd) {
		t.Fatalf("结束时间未更新: want %v got %v", newEnd, got.EndTS)
	}

	segs, err := db.ListActivitySegmentsByDate(now)
	if err != nil {
		t.Fatalf("按日期列出失败: %v", err)
	}
	if len(segs) != 1 {
		t.Fatalf("活动段数量应为 1，实际 %d", len(segs))
	}

	minutes, count, err := db.TodayActivityMinutes()
	if err != nil {
		t.Fatalf("统计今日活跃时长失败: %v", err)
	}
	if minutes != 3 {
		t.Fatalf("今日活跃分钟数应为 3，实际 %d", minutes)
	}
	if count != 1 {
		t.Fatalf("今日活动段数应为 1，实际 %d", count)
	}
}

func TestEventCRUD(t *testing.T) {
	db := newTestDB(t)

	now := time.Now().UTC()
	ev := Event{
		TS:      now,
		Type:    EventTypeVoice,
		AppPath: `C:\Program Files\Cursor\Cursor.exe`,
		AppName: "Cursor",
		Content: "和产品确认了需求边界",
	}
	id, err := db.InsertEvent(ev)
	if err != nil {
		t.Fatalf("插入事件失败: %v", err)
	}

	got, err := db.GetEvent(id)
	if err != nil {
		t.Fatalf("查询事件失败: %v", err)
	}
	if got == nil || got.Content != ev.Content {
		t.Fatalf("事件不匹配: %+v", got)
	}

	events, err := db.ListEventsByDate(now)
	if err != nil {
		t.Fatalf("按日期列出事件失败: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("事件数量应为 1，实际 %d", len(events))
	}

	results, err := db.SearchEvents("需求", now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("搜索事件失败: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("搜索结果应为 1，实际 %d", len(results))
	}
}

func TestDataDir(t *testing.T) {
	tmp := t.TempDir()
	_ = os.Setenv("APPDATA", tmp)
	defer os.Unsetenv("APPDATA")

	dir, err := dataDir()
	if err != nil {
		t.Fatalf("dataDir 失败: %v", err)
	}
	expected := filepath.Join(tmp, "shadow-worker")
	if dir != expected {
		t.Fatalf("dataDir 路径不匹配: want %s got %s", expected, dir)
	}
	if _, err := os.Stat(expected); os.IsNotExist(err) {
		t.Fatalf("dataDir 未创建目录: %s", expected)
	}
}
