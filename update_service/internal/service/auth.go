// Package service 处理业务逻辑。
package service

import (
	"crypto/subtle"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"shadow-worker/update_service/internal/model"
	"shadow-worker/update_service/internal/storage"
)

// AuthService 处理登录与 JWT。
type AuthService struct {
	users     *storage.UserStorage
	jwtSecret []byte
}

// NewAuthService 创建 AuthService。
func NewAuthService(users *storage.UserStorage, jwtSecret string) *AuthService {
	return &AuthService{
		users:     users,
		jwtSecret: []byte(jwtSecret),
	}
}

// EnsureAdmin 在数据库为空时创建初始管理员账号。
func (s *AuthService) EnsureAdmin(username, password string) error {
	count, err := s.users.Count()
	if err != nil {
		return fmt.Errorf("count users: %w", err)
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return s.users.Create(&model.User{
		Username:     username,
		PasswordHash: string(hash),
	})
}

// Login 验证账号密码并返回 JWT。
func (s *AuthService) Login(username, password string) (string, error) {
	u, err := s.users.GetByUsername(username)
	if err != nil {
		return "", fmt.Errorf("lookup user: %w", err)
	}
	if u == nil {
		return "", fmt.Errorf("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return "", fmt.Errorf("invalid credentials")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": u.Username,
		"iat": time.Now().UTC().Unix(),
		"exp": time.Now().UTC().Add(24 * time.Hour).Unix(),
	})
	return token.SignedString(s.jwtSecret)
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
