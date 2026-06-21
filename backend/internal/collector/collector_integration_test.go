package collector

import (
	"path/filepath"
	"testing"
	"time"

	"shadow-worker/backend/internal/config"
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

	coll := NewCollector(db, config.MovementConfig{Precision: "medium"}, nil)

	app := App{
		Path:        appPath,
		Name:        "Cursor",
		WindowTitle: "main.go",
	}

	// 模拟 engaged 段(强活跃)
	coll.updateSegment(app, StateEngaged)
	if coll.curSegID == 0 {
		t.Fatal("应创建 activity_segment")
	}
	seg, err := db.GetActivitySegment(coll.curSegID)
	if err != nil {
		t.Fatalf("查询段失败: %v", err)
	}
	if seg == nil || seg.State != StateEngaged {
		t.Fatalf("段状态应为 engaged，实际 %+v", seg)
	}
	if coll.curState != StateEngaged {
		t.Fatalf("curState 应为 engaged，实际 %s", coll.curState)
	}

	// engaged → active(余热,同应用不换段)
	coll.updateSegment(app, StateActive)
	if seg.State != StateEngaged {
		t.Fatalf("旧段状态不应被改动，仍为 engaged，实际 %s", seg.State)
	}
	seg2, err := db.GetActivitySegment(coll.curSegID)
	if err != nil {
		t.Fatalf("查询段失败: %v", err)
	}
	if seg2 == nil || seg2.State != StateActive {
		t.Fatalf("新段状态应为 active，实际 %+v", seg2)
	}

	// active → idle 转换
	time.Sleep(10 * time.Millisecond)
	coll.updateSegment(app, StateIdle)
	seg3, err := db.GetActivitySegment(coll.curSegID)
	if err != nil {
		t.Fatalf("查询段失败: %v", err)
	}
	if seg3 == nil || seg3.State != StateIdle {
		t.Fatalf("新段状态应为 idle，实际 %+v", seg3)
	}

	// 检查总段数:engaged / active / idle 各一段 = 3
	segs, err := db.ListActivitySegmentsByDate(time.Now().UTC())
	if err != nil {
		t.Fatalf("列出段失败: %v", err)
	}
	if len(segs) != 3 {
		t.Fatalf("应有 3 个段(engaged/active/idle)，实际 %d", len(segs))
	}

	// 验证三态常量互不相同(防止拼写/重构导致状态机退化)。
	if StateEngaged == StateActive || StateEngaged == StateIdle || StateActive == StateIdle {
		t.Fatalf("三态常量不应相等: engaged=%q active=%q idle=%q",
			StateEngaged, StateActive, StateIdle)
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
