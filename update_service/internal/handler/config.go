package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"shadow-worker/update_service/internal/config"
	"shadow-worker/update_service/internal/github"
)

// ConfigHandler 处理 GitHub 配置的查看（GET）与热加载（PUT）。
// 支持热加载的字段：github_owner / github_repo / github_token /
// github_cache_ttl / asset_name_template。
// 不支持热加载的字段（listen_addr / jwt_secret / admin_*）请改 config.yaml 后重启。
type ConfigHandler struct {
	cfg     *config.Config
	ghc     *github.Client
}

// NewConfigHandler 创建 ConfigHandler。
// ghc 用于热加载（改 owner/repo/token 后调 UpdateSource 即时生效）。
func NewConfigHandler(cfg *config.Config, ghc *github.Client) *ConfigHandler {
	return &ConfigHandler{cfg: cfg, ghc: ghc}
}

// configResponse 是 GET 返回的配置视图（token 脱敏）。
type configResponse struct {
	GitHubOwner       string `json:"github_owner"`
	GitHubRepo        string `json:"github_repo"`
	GitHubTokenMasked string `json:"github_token_masked"` // 已配置返回 ****xxxx，未配置返回空
	GitHubCacheTTL    string `json:"github_cache_ttl"`
	AssetNameTemplate string `json:"asset_name_template"`
}

// configRequest 是 PUT 提交的新配置。
// 所有字段可选：为空字符串表示"不修改"（token 特殊：空串=不清空，要清空用字面 "none"）。
type configRequest struct {
	GitHubOwner       string `json:"github_owner"`
	GitHubRepo        string `json:"github_repo"`
	GitHubToken       string `json:"github_token"`        // 空串=不修改；字面值修改
	GitHubCacheTTL    string `json:"github_cache_ttl"`    // 例 "5m" / "30s"
	AssetNameTemplate string `json:"asset_name_template"`
}

// maskToken 把 token 脱敏成 ****xxxx（末 4 位），空 token 返回空串。
func maskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 4 {
		return "****"
	}
	return "****" + token[len(token)-4:]
}

// ServeHTTP 按 method 分发 GET / PUT。
func (h *ConfigHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.get(w, r)
	case http.MethodPut:
		h.update(w, r)
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *ConfigHandler) get(w http.ResponseWriter, r *http.Request) {
	resp := configResponse{
		GitHubOwner:       h.cfg.GitHubOwner,
		GitHubRepo:        h.cfg.GitHubRepo,
		GitHubTokenMasked: maskToken(h.cfg.GitHubToken),
		GitHubCacheTTL:    h.cfg.GitHubCacheTTL.String(),
		AssetNameTemplate: h.cfg.AssetNameTemplate,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *ConfigHandler) update(w http.ResponseWriter, r *http.Request) {
	var req configRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	// 逐字段更新：空串表示不修改
	if req.GitHubOwner != "" {
		h.cfg.GitHubOwner = req.GitHubOwner
	}
	if req.GitHubRepo != "" {
		h.cfg.GitHubRepo = req.GitHubRepo
	}
	if req.GitHubToken != "" {
		h.cfg.GitHubToken = req.GitHubToken
	}
	if req.GitHubCacheTTL != "" {
		d, err := time.ParseDuration(req.GitHubCacheTTL)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"invalid github_cache_ttl: %s"}`, err.Error()), http.StatusBadRequest)
			return
		}
		if d <= 0 {
			http.Error(w, `{"error":"github_cache_ttl must be positive"}`, http.StatusBadRequest)
			return
		}
		h.cfg.GitHubCacheTTL = d
	}
	if req.AssetNameTemplate != "" {
		h.cfg.AssetNameTemplate = req.AssetNameTemplate
	}

	// 校验必填
	if h.cfg.GitHubOwner == "" || h.cfg.GitHubRepo == "" {
		http.Error(w, `{"error":"github_owner and github_repo are required"}`, http.StatusBadRequest)
		return
	}

	// 1) 热加载到 GitHub client（立即生效：下次 CheckUpdate 用新配置）
	h.ghc.UpdateSource(h.cfg.GitHubOwner, h.cfg.GitHubRepo, h.cfg.GitHubToken, h.cfg.GitHubCacheTTL)

	// 2) 持久化到 config.yaml（重启不丢）
	if err := h.cfg.Save(); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"persist config failed: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// 返回更新后的视图
	h.get(w, r)
}
