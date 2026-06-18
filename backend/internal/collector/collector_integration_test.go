package collector

import (
	"path/filepath"
	"testing"
	"time"

	"shadow-worker/backend/internal/storage"
)

func TestCollectorUpdateSegment(t *testing.T) {
	db := openTestDB(t)

	// 把测试应用加入白名单
	appPath := `C:\TestApps\Cursor.exe`
	if err := db.AddAppCategory(storage.AppCategory{
		Path:     appPath,
		Name:     "Cursor",
		Category: "coding",
	}); err != nil {
		t.Fatalf("添加白名单失败: %v", err)
	}

	coll := NewCollector(db, "medium", nil)

	app := App{
		Path:        appPath,
		Name:        "Cursor",
		WindowTitle: "main.go",
	}

	// 模拟 active 段
	coll.updateSegment(app, true)
	if coll.curSegID == 0 {
		t.Fatal("应创建 activity_segment")
	}
	seg, err := db.GetActivitySegment(coll.curSegID)
	if err != nil {
		t.Fatalf("查询段失败: %v", err)
	}
	if seg == nil || seg.State != "active" {
		t.Fatalf("段状态应为 active，实际 %+v", seg)
	}

	// 模拟 idle 转换
	time.Sleep(10 * time.Millisecond)
	coll.updateSegment(app, false)
	seg2, err := db.GetActivitySegment(coll.curSegID)
	if err != nil {
		t.Fatalf("查询段失败: %v", err)
	}
	if seg2 == nil || seg2.State != "idle" {
		t.Fatalf("新段状态应为 idle，实际 %+v", seg2)
	}

	// 检查总段数
	segs, err := db.ListActivitySegmentsByDate(time.Now().UTC())
	if err != nil {
		t.Fatalf("列出段失败: %v", err)
	}
	if len(segs) != 2 {
		t.Fatalf("应有 2 个段，实际 %d", len(segs))
	}
}

func openTestDB(t *testing.T) *storage.DB {
	t.Helper()
	db, err := storage.OpenAt(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
