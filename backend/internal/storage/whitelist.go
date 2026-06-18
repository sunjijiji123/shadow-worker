package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// AppCategory 对应 app_categories 表。
type AppCategory struct {
	Path     string
	Name     string
	Category string
	IconPath string
	AddedAt  time.Time
}

// AddAppCategory 添加一条白名单应用。path 为主键，重复则更新。
func (db *DB) AddAppCategory(app AppCategory) error {
	if app.Path == "" || app.Name == "" || app.Category == "" {
		return fmt.Errorf("path/name/category 不能为空")
	}
	if app.AddedAt.IsZero() {
		app.AddedAt = time.Now().UTC()
	}

	const q = `INSERT INTO app_categories(path, name, category, icon_path, added_at)
		VALUES(?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			name=excluded.name,
			category=excluded.category,
			icon_path=excluded.icon_path,
			added_at=excluded.added_at`

	_, err := db.Exec(q, app.Path, app.Name, app.Category, app.IconPath, toUnix(app.AddedAt))
	if err != nil {
		return fmt.Errorf("添加白名单应用失败: %w", err)
	}
	return nil
}

// RemoveAppCategory 按 path 删除白名单应用。
func (db *DB) RemoveAppCategory(path string) error {
	_, err := db.Exec("DELETE FROM app_categories WHERE path = ?", path)
	if err != nil {
		return fmt.Errorf("删除白名单应用失败: %w", err)
	}
	return nil
}

// UpdateAppCategory 更新白名单应用的分类/名称/图标。
func (db *DB) UpdateAppCategory(path, name, category, iconPath string) error {
	if path == "" {
		return fmt.Errorf("path 不能为空")
	}
	_, err := db.Exec(
		"UPDATE app_categories SET name = ?, category = ?, icon_path = ? WHERE path = ?",
		name, category, iconPath, path,
	)
	if err != nil {
		return fmt.Errorf("更新白名单应用失败: %w", err)
	}
	return nil
}

// GetAppCategory 按 path 查询单条白名单应用。
func (db *DB) GetAppCategory(path string) (*AppCategory, error) {
	row := db.QueryRow(
		"SELECT path, name, category, icon_path, added_at FROM app_categories WHERE path = ?",
		path,
	)
	return scanAppCategory(row)
}

// ListAppCategories 列出所有白名单应用，按添加时间升序。
func (db *DB) ListAppCategories() ([]AppCategory, error) {
	rows, err := db.Query("SELECT path, name, category, icon_path, added_at FROM app_categories ORDER BY added_at")
	if err != nil {
		return nil, fmt.Errorf("列出白名单应用失败: %w", err)
	}
	defer rows.Close()

	var apps []AppCategory
	for rows.Next() {
		app, err := scanAppCategory(rows)
		if err != nil {
			return nil, err
		}
		apps = append(apps, *app)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历白名单应用失败: %w", err)
	}
	return apps, nil
}

// scanAppCategory 从 sql.Row/sql.Rows 扫描 AppCategory。
func scanAppCategory(sc interface {
	Scan(dest ...any) error
}) (*AppCategory, error) {
	var app AppCategory
	var addedAt int64
	if err := sc.Scan(&app.Path, &app.Name, &app.Category, &app.IconPath, &addedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("扫描白名单应用失败: %w", err)
	}
	app.AddedAt = fromUnix(addedAt)
	return &app, nil
}
