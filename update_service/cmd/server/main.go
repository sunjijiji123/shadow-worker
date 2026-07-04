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
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"shadow-worker/update_service/internal/config"
	"shadow-worker/update_service/internal/github"
	"shadow-worker/update_service/internal/handler"
	"shadow-worker/update_service/internal/middleware"
	"shadow-worker/update_service/internal/service"
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

	// 已去掉 SQLite：admin 密码以 bcrypt 哈希存在 config.yaml（admin_password_hash），
	// 启动时 config.Load 已完成明文→hash 迁移。AuthService 直接从 cfg 加载。
	authSvc := service.NewAuthService(cfg, cfg.JWTSecret)

	ghClient := github.NewClient(cfg.GitHubOwner, cfg.GitHubRepo, cfg.GitHubToken, cfg.GitHubCacheTTL)
	releaseSvc := service.NewReleaseService(ghClient, cfg)

	authHandler := handler.NewAuthHandler(authSvc)
	releaseHandler := handler.NewReleaseHandler(releaseSvc)
	updateHandler := handler.NewUpdateHandler(releaseSvc)
	statusHandler := handler.NewStatusHandler(cfg)
	configHandler := handler.NewConfigHandler(cfg, ghClient)
	accountHandler := handler.NewAccountHandler(authSvc)

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

	// index.html 用专用 handler：把 ?v=__APP_VERSION__ 占位符替换成启动时间戳，
	// 这样每次重启服务，app.js 的 URL 版本号就变，浏览器自动失效缓存。
	// 开发期改 app.js 后只需重启服务即可，无需让用户手动硬刷新。
	startEpoch := fmt.Sprintf("%d", time.Now().Unix())
	indexHandler := func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(filepath.Join(webRoot, "index.html"))
		if err != nil {
			http.Error(w, "index.html not found", http.StatusInternalServerError)
			return
		}
		rendered := strings.ReplaceAll(string(data), "__APP_VERSION__", startEpoch)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(rendered))
	}
	mux.HandleFunc("/admin/", func(w http.ResponseWriter, r *http.Request) {
		// 根路径 /admin/ 和 /admin/index.html 走模板注入；其它静态文件走 FileServer
		if r.URL.Path == "/admin/" || r.URL.Path == "/admin/index.html" {
			indexHandler(w, r)
			return
		}
		http.StripPrefix("/admin", http.FileServer(http.Dir(webRoot))).ServeHTTP(w, r)
	})

	// 管理 API（需 JWT）
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/releases", releaseHandler.List)
	adminMux.HandleFunc("/status", statusHandler.Status)
	adminMux.HandleFunc("/config", configHandler.ServeHTTP)
	adminMux.HandleFunc("/password", accountHandler.ChangePassword)

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
