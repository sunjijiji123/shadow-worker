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
	seg1ID := coll.curSegID
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

	// engaged → active：同应用，应合并为同一段（不开新段）。
	coll.updateSegment(app, StateActive)
	if coll.curSegID != seg1ID {
		t.Fatalf("engaged→active 应合并同一段，curSegID 不应变，旧=%d 新=%d", seg1ID, coll.curSegID)
	}

	// active → idle（静默思考）：同应用，idle 不打断聚合，仍合并为同一段。
	coll.updateSegment(app, StateIdle)
	if coll.curSegID != seg1ID {
		t.Fatalf("active→idle 同应用应合并（idle 不打断），curSegID 不应变，旧=%d 新=%d", seg1ID, coll.curSegID)
	}
	// 段 state 应滚动更新为最新值（idle）。
	seg2, err := db.GetActivitySegment(coll.curSegID)
	if err != nil {
		t.Fatalf("查询段失败: %v", err)
	}
	if seg2 == nil || seg2.State != StateIdle {
		t.Fatalf("合并段 state 应已滚动为 idle，实际 %+v", seg2)
	}

	// 切换到另一个应用：才开新段。
	otherApp := App{
		Path:        `C:\TestApps\Other.exe`,
		Name:        "Other",
		WindowTitle: "other",
	}
	if err := db.AddAppCategory(storage.AppCategory{
		Path: otherApp.Path, Name: "Other", Category: "browser",
	}); err != nil {
		t.Fatalf("添加第二个白名单失败: %v", err)
	}
	coll.updateSegment(otherApp, StateEngaged)
	if coll.curSegID == seg1ID {
		t.Fatal("切换应用应开新段，curSegID 应改变")
	}

	// 检查总段数：同 app 的 engaged/active/idle 合并为 1 段 + 切换后的新 app 1 段 = 2
	segs, err := db.ListActivitySegmentsByDate(time.Now().UTC())
	if err != nil {
		t.Fatalf("列出段失败: %v", err)
	}
	if len(segs) != 2 {
		t.Fatalf("应有 2 个段（同app合并 + 切换app新段），实际 %d", len(segs))
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
