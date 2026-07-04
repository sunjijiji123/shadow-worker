// Package model 定义升级服务的数据模型。
package model

import "time"

// User 是管理员账号。
type User struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Release 是一个已发布版本。
type Release struct {
	ID               int64
	Version          string
	Channel          string
	MinVersion       string
	PackageFilename  string
	PackageSize      int64
	PackageSHA256    string
	ChangelogURL     string
	PublishedAt      time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
