// Package service 处理业务逻辑。
package service

import (
	"crypto/subtle"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"shadow-worker/update_service/internal/config"
)

// AuthService 处理登录、JWT、改密码。
//
// 密码以 bcrypt 哈希存在 config.yaml（admin_password_hash 字段），
// 启动时加载到内存。改密码时更新内存 + 写回 config.yaml（cfg.Save）。
// 不再依赖 SQLite。
type AuthService struct {
	cfg        *config.Config // 持有指针，改密码时同步更新 + Save
	jwtSecret  []byte

	// passwordHash 受 mu 保护（并发读写：Login 读 / ChangePassword 写）
	mu           sync.RWMutex
	passwordHash []byte
}

// NewAuthService 创建 AuthService。cfg 必须已通过 Load 完成（含 hash 迁移）。
func NewAuthService(cfg *config.Config, jwtSecret string) *AuthService {
	return &AuthService{
		cfg:          cfg,
		jwtSecret:    []byte(jwtSecret),
		passwordHash: []byte(cfg.AdminPasswordHash),
	}
}

// Login 验证账号密码并返回 JWT。
func (s *AuthService) Login(username, password string) (string, error) {
	// 用户名常量时间比较（避免枚举）
	if subtle.ConstantTimeCompare([]byte(username), []byte(s.cfg.AdminUsername)) != 1 {
		return "", fmt.Errorf("invalid credentials")
	}

	s.mu.RLock()
	hash := s.passwordHash
	s.mu.RUnlock()
	if err := bcrypt.CompareHashAndPassword(hash, []byte(password)); err != nil {
		return "", fmt.Errorf("invalid credentials")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": username,
		"iat": time.Now().UTC().Unix(),
		"exp": time.Now().UTC().Add(24 * time.Hour).Unix(),
	})
	return token.SignedString(s.jwtSecret)
}

// ChangePassword 验证旧密码，更新成新密码（bcrypt 哈希），
// 同步更新内存 + config.yaml（持久化）。
func (s *AuthService) ChangePassword(username, oldPassword, newPassword string) error {
	// 先验证旧密码（复用 Login 的校验逻辑，失败则拒绝）
	if _, err := s.Login(username, oldPassword); err != nil {
		return fmt.Errorf("invalid old password")
	}
	if len(newPassword) < 6 {
		return fmt.Errorf("new password too short (min 6 chars)")
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}

	// 更新内存
	s.mu.Lock()
	s.passwordHash = newHash
	s.mu.Unlock()

	// 同步到 cfg 并持久化
	s.cfg.AdminPasswordHash = string(newHash)
	s.cfg.AdminPassword = "" // 明文字段保持空
	if err := s.cfg.Save(); err != nil {
		return fmt.Errorf("persist config: %w", err)
	}
	return nil
}

// ValidateToken 验证 JWT 并返回用户名。
func (s *AuthService) ValidateToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return "", fmt.Errorf("parse token: %w", err)
	}
	if !token.Valid {
		return "", fmt.Errorf("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid claims")
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", fmt.Errorf("missing subject")
	}
	return sub, nil
}

// ConstantTimeCompare 提供常量时间字符串比较（包装 subtle）。
func ConstantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
