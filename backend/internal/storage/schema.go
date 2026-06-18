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
			state           TEXT NOT NULL
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
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("执行建表语句失败: %w", err)
		}
	}
	return nil
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
