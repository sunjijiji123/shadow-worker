// Package config 加载升级服务配置。
//
// config.yaml 是唯一配置真相源（已去掉 SQLite）。admin 密码以 bcrypt 哈希
// 存 admin_password_hash 字段；前端改密码 / GitHub 配置后，热加载到内存 +
// 调 Save() 写回 config.yaml，无需重启。
package config

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

// Config 是升级服务配置。
type Config struct {
	ListenAddr string `yaml:"listen_addr"`

	AdminUsername     string `yaml:"admin_username"`
	AdminPassword     string `yaml:"admin_password,omitempty"`      // 明文，仅旧配置兼容；启动时迁移成 hash 后清空
	AdminPasswordHash string `yaml:"admin_password_hash,omitempty"` // bcrypt 哈希，运行期真相源
	JWTSecret         string `yaml:"jwt_secret"`

	// GitHub Releases 源配置（前端可热加载）
	GitHubOwner       string        `yaml:"github_owner"`
	GitHubRepo        string        `yaml:"github_repo"`
	GitHubToken       string        `yaml:"github_token,omitempty"` // 可选，私有仓库或提高 rate limit
	GitHubCacheTTL    time.Duration `yaml:"github_cache_ttl"`
	AssetNameTemplate string        `yaml:"asset_name_template"`

	// 配置文件路径（Save 用），非 yaml 字段
	configPath string `yaml:"-"`
}

// Default 返回默认配置。
func Default() *Config {
	return &Config{
		ListenAddr:        "0.0.0.0:8080",
		AdminUsername:     "admin",
		GitHubOwner:       "sunjijiji123",
		GitHubRepo:        "shadow-worker",
		GitHubCacheTTL:    5 * time.Minute,
		AssetNameTemplate: "shadow-worker-{version}-setup.exe",
	}
}

// Load 从文件加载配置，并叠加环境变量。
func Load(path string) (*Config, error) {
	cfg := Default()
	cfg.configPath = path
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
	if v := os.Getenv("UPDATE_ADMIN_USERNAME"); v != "" {
		cfg.AdminUsername = v
	}
	if v := os.Getenv("UPDATE_ADMIN_PASSWORD"); v != "" {
		cfg.AdminPassword = v
	}
	if v := os.Getenv("UPDATE_ADMIN_PASSWORD_HASH"); v != "" {
		cfg.AdminPasswordHash = v
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

	// 兼容旧配置：明文 admin_password 迁移成 hash。
	// 启动时若 hash 空但明文非空，算哈希存 hash 字段，清空明文，写回文件。
	if cfg.AdminPasswordHash == "" && cfg.AdminPassword != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("hash admin password: %w", err)
		}
		cfg.AdminPasswordHash = string(hash)
		cfg.AdminPassword = ""
		// 静默写回（迁移），失败不致命（内存值已正确）
		_ = cfg.Save()
	}

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("jwt_secret is required")
	}
	if cfg.AdminPasswordHash == "" {
		return nil, fmt.Errorf("admin_password or admin_password_hash is required")
	}
	if cfg.AdminUsername == "" {
		cfg.AdminUsername = "admin"
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

// Save 把当前配置写回 config.yaml（热加载持久化用）。
// admin_password 明文字段已清空（迁移后），只写 hash。
func (c *Config) Save() error {
	if c.configPath == "" {
		return fmt.Errorf("config path not set")
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	// 保留 0600 权限（含密码哈希 + token，敏感）
	if err := os.WriteFile(c.configPath, data, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// ConfigPath 返回配置文件路径（Save 用）。
func (c *Config) ConfigPath() string {
	return c.configPath
}
