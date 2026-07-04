// Package main 是 Shadow Worker 升级服务器入口。
//
// 升级服务完全独立于主后端，数据源使用 GitHub Releases：
//   - 对外提供检查更新 API
//   - 管理员在 GitHub 上发布 Release 后，本服务自动同步
//   - 安装包由 GitHub 托管并下载
//
// 启动：
//
//	go run ./cmd/server
package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"shadow-worker/update_service/internal/config"
	"shadow-worker/update_service/internal/db"
	"shadow-worker/update_service/internal/github"
	"shadow-worker/update_service/internal/handler"
	"shadow-worker/update_service/internal/middleware"
	"shadow-worker/update_service/internal/service"
	"shadow-worker/update_service/internal/storage"
)

func main() {
	cfgPath := os.Getenv("UPDATE_CONFIG")
	if cfgPath == "" {
		cfgPath = "config.yaml"
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	sqlDB, err := db.Open(cfg.DBPath())
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer sqlDB.Close()

	userStore := storage.NewUserStorage(sqlDB)

	authSvc := service.NewAuthService(userStore, cfg.JWTSecret)
	if err := authSvc.EnsureAdmin(cfg.AdminUsername, cfg.AdminPassword); err != nil {
		log.Fatalf("初始化管理员账号失败: %v", err)
	}

	ghClient := github.NewClient(cfg.GitHubOwner, cfg.GitHubRepo, cfg.GitHubToken, cfg.GitHubCacheTTL)
	releaseSvc := service.NewReleaseService(ghClient, cfg)

	authHandler := handler.NewAuthHandler(authSvc)
	releaseHandler := handler.NewReleaseHandler(releaseSvc)
	updateHandler := handler.NewUpdateHandler(releaseSvc)
	statusHandler := handler.NewStatusHandler(cfg)

	mux := http.NewServeMux()

	// 公开 API
	mux.HandleFunc("/api/update/check", updateHandler.Check)

	// 认证 API
	mux.HandleFunc("/api/auth/login", authHandler.Login)
	mux.HandleFunc("/api/auth/logout", authHandler.Logout)

	// 静态管理后台（公开，由前端页面自行处理登录态）
	webRoot := os.Getenv("UPDATE_WEB_ROOT")
	if webRoot == "" {
		webRoot = filepath.Join(filepath.Dir(os.Args[0]), "web", "admin")
		// 开发期 fallback：从工作目录找 web/admin
		if _, err := os.Stat(webRoot); os.IsNotExist(err) {
			webRoot = "web/admin"
		}
	}
	mux.Handle("/admin/", http.StripPrefix("/admin", http.FileServer(http.Dir(webRoot))))

	// 管理 API（需 JWT）
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/releases", releaseHandler.List)
	adminMux.HandleFunc("/status", statusHandler.Status)

	mux.Handle("/admin/api/", http.StripPrefix("/admin/api", middleware.Auth(authSvc)(adminMux)))

	// 根路径重定向到管理后台登录页
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusTemporaryRedirect)
	})

	log.Printf("Shadow Worker 升级服务已启动: %s", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, mux); err != nil {
		log.Fatalf("服务退出: %v", err)
	}
}
