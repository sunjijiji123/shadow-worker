// Package storage 提供数据库访问。
package storage

import (
	"database/sql"
	"fmt"
	"time"

	"shadow-worker/update_service/internal/model"
)

// UserStorage 是用户表操作。
type UserStorage struct {
	db *sql.DB
}

// NewUserStorage 创建 UserStorage。
func NewUserStorage(db *sql.DB) *UserStorage {
	return &UserStorage{db: db}
}

// Create 创建用户。
func (s *UserStorage) Create(u *model.User) error {
	now := time.Now().UTC().Unix()
	res, err := s.db.Exec(
		`INSERT INTO users (username, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		u.Username, u.PasswordHash, now, now,
	)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	u.ID, _ = res.LastInsertId()
	u.CreatedAt = time.Unix(now, 0).UTC()
	u.UpdatedAt = u.CreatedAt
	return nil
}

// GetByUsername 按用户名查询。
func (s *UserStorage) GetByUsername(username string) (*model.User, error) {
	row := s.db.QueryRow(
		`SELECT id, username, password_hash, created_at, updated_at FROM users WHERE username = ?`,
		username,
	)
	u := &model.User{}
	var createdAt, updatedAt int64
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("select user: %w", err)
	}
	u.CreatedAt = time.Unix(createdAt, 0).UTC()
	u.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return u, nil
}

// Count 返回用户数。
func (s *UserStorage) Count() (int64, error) {
	var n int64
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return n, nil
}
