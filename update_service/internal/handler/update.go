package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"shadow-worker/update_service/internal/service"
)

// UpdateHandler 处理公开的更新检查请求。
type UpdateHandler struct {
	releaseSvc *service.ReleaseService
}

// NewUpdateHandler 创建 UpdateHandler。
func NewUpdateHandler(releaseSvc *service.ReleaseService) *UpdateHandler {
	return &UpdateHandler{releaseSvc: releaseSvc}
}

type updateInfoResponse struct {
	Available     bool   `json:"available"`
	LatestVersion string `json:"latest_version"`
	DownloadURL   string `json:"download_url"`
	ChangelogURL  string `json:"changelog_url"`
	Changelog     string `json:"changelog"` // Markdown 更新日志原文
	PackageSize   int64  `json:"package_size"`
	PackageSHA256 string `json:"package_sha256"`
	PublishedAt   string `json:"published_at"`
}

// Check 检查更新。
func (h *UpdateHandler) Check(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	current := r.URL.Query().Get("version")
	channel := r.URL.Query().Get("channel")
	if channel == "" {
		channel = "stable"
	}

	latest, available, err := h.releaseSvc.CheckUpdate(r.Context(), current, channel)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	resp := updateInfoResponse{Available: false}
	if available && latest != nil {
		resp.Available = true
		resp.LatestVersion = latest.Version
		resp.DownloadURL = latest.DownloadURL
		resp.ChangelogURL = latest.ChangelogURL
		resp.Changelog = latest.Body
		resp.PackageSize = latest.PackageSize
		resp.PackageSHA256 = latest.PackageSHA256
		resp.PublishedAt = latest.PublishedAt.Format(time.RFC3339)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
