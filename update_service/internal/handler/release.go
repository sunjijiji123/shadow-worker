// Package handler 提供 HTTP handler。
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"shadow-worker/update_service/internal/service"
)

// ReleaseHandler 处理版本管理请求。
type ReleaseHandler struct {
	releaseSvc *service.ReleaseService
}

// NewReleaseHandler 创建 ReleaseHandler。
func NewReleaseHandler(releaseSvc *service.ReleaseService) *ReleaseHandler {
	return &ReleaseHandler{releaseSvc: releaseSvc}
}

type releaseResponse struct {
	ID              int64  `json:"id"`
	Version         string `json:"version"`
	Channel         string `json:"channel"`
	MinVersion      string `json:"min_version"`
	PackageFilename string `json:"package_filename"`
	PackageSize     int64  `json:"package_size"`
	PackageSHA256   string `json:"package_sha256"`
	ChangelogURL    string `json:"changelog_url"`
	Changelog       string `json:"changelog"` // Markdown 更新日志原文
	DownloadURL     string `json:"download_url"`
	PublishedAt     string `json:"published_at"`
	CreatedAt       string `json:"created_at"`
}

// List 返回版本列表（从 GitHub Releases 同步）。
func (h *ReleaseHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	channel := r.URL.Query().Get("channel")

	list, err := h.releaseSvc.List(r.Context(), channel)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	resp := make([]releaseResponse, 0, len(list))
	for _, item := range list {
		resp = append(resp, releaseResponse{
			ID:              item.ID,
			Version:         item.Version,
			Channel:         item.Channel,
			MinVersion:      item.MinVersion,
			PackageFilename: item.PackageFilename,
			PackageSize:     item.PackageSize,
			PackageSHA256:   item.PackageSHA256,
			ChangelogURL:    item.ChangelogURL,
			Changelog:       item.Body,
			DownloadURL:     item.DownloadURL,
			PublishedAt:     item.PublishedAt.Format(time.RFC3339),
			CreatedAt:       item.CreatedAt.Format(time.RFC3339),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"releases": resp})
}
