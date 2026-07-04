package handler

import (
	"encoding/json"
	"net/http"
	"os"

	"shadow-worker/update_service/internal/config"
)

// StatusHandler 返回服务运行状态与配置（敏感字段不泄露）。
type StatusHandler struct {
	cfg *config.Config
}

// NewStatusHandler 创建 StatusHandler。
func NewStatusHandler(cfg *config.Config) *StatusHandler {
	return &StatusHandler{cfg: cfg}
}

type statusResponse struct {
	ListenAddr          string `json:"listen_addr"`
	GitHubOwner         string `json:"github_owner"`
	GitHubRepo          string `json:"github_repo"`
	GitHubTokenConfigured bool   `json:"github_token_configured"`
	GitHubCacheTTL      string `json:"github_cache_ttl"`
	AssetNameTemplate   string `json:"asset_name_template"`
}

// Status 返回当前配置状态。
func (h *StatusHandler) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// token 是否配置：文件配置或环境变量都算。
	tokenConfigured := h.cfg.GitHubToken != "" || os.Getenv("UPDATE_GITHUB_TOKEN") != ""

	resp := statusResponse{
		ListenAddr:          h.cfg.ListenAddr,
		GitHubOwner:         h.cfg.GitHubOwner,
		GitHubRepo:          h.cfg.GitHubRepo,
		GitHubTokenConfigured: tokenConfigured,
		GitHubCacheTTL:      h.cfg.GitHubCacheTTL.String(),
		AssetNameTemplate:   h.cfg.AssetNameTemplate,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
