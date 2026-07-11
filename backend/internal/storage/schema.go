// Package storage 是 Shadow Worker 唯一接触 SQLite 的地方。
//
// 负责:建表/迁移、app_categories / activity_segments / events 的 CRUD。
// 数据文件位置: %APPDATA%/shadow-worker/data.db
package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// DB 封装 sql.DB，提供业务 CRUD 方法。
type DB struct {
	*sql.DB
}

// dataDir 返回数据目录（不存在则创建）。
func dataDir() (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("获取配置目录失败: %w", err)
	}
	dir := filepath.Join(cfgDir, "shadow-worker")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("创建数据目录失败: %w", err)
	}
	return dir, nil
}

// DataDir 是 dataDir 的导出版本，供"系统设置-数据目录"展示用。
func DataDir() (string, error) {
	return dataDir()
}

// Open 打开 SQLite 数据库，必要时自动建表。
func Open() (*DB, error) {
	dir, err := dataDir()
	if err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dir, "data.db")

	sqlDB, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	db := &DB{sqlDB}
	if err := db.migrate(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}
	return db, nil
}

// OpenAt 用于测试，在指定路径打开数据库。
func OpenAt(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	db := &DB{sqlDB}
	if err := db.migrate(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}
	return db, nil
}

// migrate 执行建表和索引。
func (db *DB) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS app_categories (
			path        TEXT PRIMARY KEY,
			name        TEXT NOT NULL,
			category    TEXT NOT NULL,
			icon_path   TEXT,
			added_at    INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS activity_segments (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			start_ts        INTEGER NOT NULL,
			end_ts          INTEGER NOT NULL,
			app_path        TEXT NOT NULL,
			app_name        TEXT NOT NULL,
			category        TEXT NOT NULL,
			window_title    TEXT,
			state           TEXT NOT NULL,
			summary         TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_segments_start ON activity_segments(start_ts);`,
		`CREATE INDEX IF NOT EXISTS idx_segments_cat_date ON activity_segments(category, start_ts);`,
		`CREATE TABLE IF NOT EXISTS events (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			ts              INTEGER NOT NULL,
			type            TEXT NOT NULL,
			app_path        TEXT,
			app_name        TEXT,
			content         TEXT,
			screenshot_path TEXT,
			meta            TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_events_ts ON events(ts);`,
		`CREATE INDEX IF NOT EXISTS idx_events_type_ts ON events(type, ts);`,
		// vlm_tasks: VLM 识别任务队列。采集与识别解耦——截图落盘+写 pending，
		// recognitionLoop worker 每5分钟扫描 pending → 识别 → 成功清理/失败分类。
		// status: pending(待识别/可重试) | done(成功,清理删行) | permanent_fail(不可重试,保留)
		`CREATE TABLE IF NOT EXISTS vlm_tasks (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			created_ts   INTEGER NOT NULL,
			app_path     TEXT,
			app_name     TEXT,
			image_path   TEXT NOT NULL,
			status       TEXT NOT NULL DEFAULT 'pending',
			attempts     INTEGER NOT NULL DEFAULT 0,
			error_kind   TEXT,
			error_detail TEXT,
			updated_ts   INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE INDEX IF NOT EXISTS idx_vlm_tasks_status ON vlm_tasks(status, updated_ts);`,
		// 清理历史脏数据：旧版 voice_server 写入的 event 未带 TS（零值），
		// 被存为 ts=0，永远无法被时间窗查询命中（ListEvents 用 ts >= start），
		// 且无任何列可推断真实时间。直接删除这些孤儿行。
		`DELETE FROM events WHERE ts = 0;`,
		// 清理历史脏数据：离开检测（idle 超 away_threshold 判为"离开"并断段）上线前，
		// 采集层把"前台是白名单 app 但人已离开"的整个期间写成一条覆盖数小时的 idle 段，
		// 把时间轴撑爆。这些段 state='idle' 且时长异常（>30min，真实"思考"不会这么久）。
		// 直接删除时长 > 30 分钟的 idle 段；真实短 idle（看文档/思考）保留。
		// 30min 阈值与离开检测的 10min 判定不冲突：检测上线后不会再产生 >10min 的 idle 段，
		// 故此清理只命中历史脏数据，对新数据是 no-op。每次启动幂等执行。
		`DELETE FROM activity_segments WHERE state = 'idle' AND (end_ts - start_ts) > 1800;`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("执行建表语句失败: %w", err)
		}
	}

	// 老库升级：activity_segments 在历史版本没有 summary 列。
	// SQLite 的 ALTER TABLE ADD COLUMN 不支持 IF NOT EXISTS，
	// 用 pragma_table_info 检测，列已存在则跳过，否则补列。
	if err := db.ensureColumn("activity_segments", "summary", "TEXT"); err != nil {
		return fmt.Errorf("迁移 activity_segments.summary 列失败: %w", err)
	}

	return nil
}

// ensureColumn 幂等地确保表里存在指定列。
// 列声明 decl 是该列的类型与约束（如 "TEXT" 或 "TEXT NOT NULL DEFAULT ''"）。
// 新库的 CREATE TABLE 通常已含该列，此时跳过；老库则通过 ALTER TABLE ADD COLUMN 补齐。
func (db *DB) ensureColumn(table, col, decl string) error {
	var name string
	err := db.QueryRow(
		`SELECT name FROM pragma_table_info(?) WHERE name = ?`, table, col,
	).Scan(&name)
	if err == nil {
		return nil // 列已存在
	}
	if err != sql.ErrNoRows {
		return err
	}
	_, err = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, col, decl))
	return err
}

// toUnix 把 time.Time 转成整数秒。
func toUnix(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

// fromUnix 把整数秒转成 time.Time（UTC）。
func fromUnix(sec int64) time.Time {
	if sec == 0 {
		return time.Time{}
	}
	return time.Unix(sec, 0).UTC()
}
