// Package config 加载升级服务配置。
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 是升级服务配置。
type Config struct {
	ListenAddr string `yaml:"listen_addr"`
	DataDir    string `yaml:"data_dir"`

	AdminUsername string `yaml:"admin_username"`
	AdminPassword string `yaml:"admin_password"`
	JWTSecret     string `yaml:"jwt_secret"`

	// GitHub Releases 源配置
	GitHubOwner       string        `yaml:"github_owner"`
	GitHubRepo        string        `yaml:"github_repo"`
	GitHubToken       string        `yaml:"github_token"`        // 可选，私有仓库或需要更高 rate limit 时填写
	GitHubCacheTTL    time.Duration `yaml:"github_cache_ttl"`    // 默认 5m，公有仓库建议开启缓存避免 rate limit
	AssetNameTemplate string        `yaml:"asset_name_template"` // 例如 shadow-worker-{version}-setup.exe
}

// Default 返回默认配置。
func Default() *Config {
	return &Config{
		ListenAddr:        "0.0.0.0:8080",
		DataDir:           "./data",
		AdminUsername:     "admin",
		AdminPassword:     "",
		JWTSecret:         "",
		GitHubOwner:       "sunjijiji123",
		GitHubRepo:        "shadow-worker",
		GitHubCacheTTL:    5 * time.Minute,
		AssetNameTemplate: "shadow-worker-{version}-setup.exe",
	}
}

// Load 从文件加载配置，并叠加环境变量。
func Load(path string) (*Config, error) {
	cfg := Default()
	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read config: %w", err)
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}

	// 环境变量覆盖，便于部署时不改文件。
	if v := os.Getenv("UPDATE_LISTEN_ADDR"); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv("UPDATE_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("UPDATE_ADMIN_USERNAME"); v != "" {
		cfg.AdminUsername = v
	}
	if v := os.Getenv("UPDATE_ADMIN_PASSWORD"); v != "" {
		cfg.AdminPassword = v
	}
	if v := os.Getenv("UPDATE_JWT_SECRET"); v != "" {
		cfg.JWTSecret = v
	}
	if v := os.Getenv("UPDATE_GITHUB_OWNER"); v != "" {
		cfg.GitHubOwner = v
	}
	if v := os.Getenv("UPDATE_GITHUB_REPO"); v != "" {
		cfg.GitHubRepo = v
	}
	if v := os.Getenv("UPDATE_GITHUB_TOKEN"); v != "" {
		cfg.GitHubToken = v
	}
	if v := os.Getenv("UPDATE_GITHUB_CACHE_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.GitHubCacheTTL = d
		}
	}
	if v := os.Getenv("UPDATE_ASSET_NAME_TEMPLATE"); v != "" {
		cfg.AssetNameTemplate = v
	}

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("jwt_secret is required")
	}
	if cfg.AdminPassword == "" {
		return nil, fmt.Errorf("admin_password is required")
	}
	if cfg.GitHubOwner == "" || cfg.GitHubRepo == "" {
		return nil, fmt.Errorf("github_owner and github_repo are required")
	}
	if cfg.GitHubCacheTTL <= 0 {
		cfg.GitHubCacheTTL = 5 * time.Minute
	}
	if cfg.AssetNameTemplate == "" {
		cfg.AssetNameTemplate = "shadow-worker-{version}-setup.exe"
	}

	return cfg, nil
}

// DBPath 返回 SQLite 数据库路径。
func (c *Config) DBPath() string {
	return filepath.Join(c.DataDir, "update.db")
}
