package grpcapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"shadow-worker/backend/internal/config"
)

// UpdateServer 实现 UpdateService：把客户端的更新检查请求转发到独立升级服务器。
type UpdateServer struct {
	UnimplementedUpdateServiceServer
	cfg     *config.Config
	version string
	client  *http.Client
	logger  *slog.Logger
}

// NewUpdateServer 创建 UpdateServer。
func NewUpdateServer(cfg *config.Config, version string, logger *slog.Logger) *UpdateServer {
	return &UpdateServer{
		cfg:     cfg,
		version: version,
		client:  &http.Client{Timeout: 10 * time.Second},
		logger:  logger,
	}
}

// updateServerInfo 是升级服务器返回的 JSON 结构。
type updateServerInfo struct {
	Available     bool   `json:"available"`
	LatestVersion string `json:"latest_version"`
	DownloadURL   string `json:"download_url"`
	ChangelogURL  string `json:"changelog_url"`
	Changelog     string `json:"changelog"`
	PackageSize   int64  `json:"package_size"`
	PackageSHA256 string `json:"package_sha256"`
	PublishedAt   string `json:"published_at"`
	Error         string `json:"error"`
}

// CheckUpdate 转发检查更新请求到独立升级服务器。
func (s *UpdateServer) CheckUpdate(ctx context.Context, req *CheckUpdateRequest) (*CheckUpdateResponse, error) {
	resp := &CheckUpdateResponse{Available: false}

	serverURL := s.cfg.Update.ServerURL
	if serverURL == "" {
		resp.Error = "更新服务器未配置"
		return resp, nil
	}

	current := req.CurrentVersion
	if current == "" {
		current = s.version
	}

	query := url.Values{}
	query.Set("version", current)
	query.Set("channel", s.cfg.Update.Channel)

	u, err := url.Parse(serverURL)
	if err != nil {
		s.logger.Error("解析更新服务器地址失败", "url", serverURL, "err", err)
		resp.Error = "更新服务器地址无效"
		return resp, nil
	}
	u = u.JoinPath("/api/update/check")
	u.RawQuery = query.Encode()

	s.logger.Info("检查更新", "version", current, "channel", s.cfg.Update.Channel, "url", u.String())

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		s.logger.Error("构造更新请求失败", "err", err)
		resp.Error = "构造更新请求失败"
		return resp, nil
	}
	httpReq.Header.Set("Accept", "application/json")

	httpResp, err := s.client.Do(httpReq)
	if err != nil {
		s.logger.Error("连接更新服务器失败", "err", err)
		resp.Error = "更新服务不可用"
		return resp, nil
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		s.logger.Error("读取更新响应失败", "err", err)
		resp.Error = "读取更新响应失败"
		return resp, nil
	}

	if httpResp.StatusCode != http.StatusOK {
		s.logger.Error("更新服务器返回非 200", "status", httpResp.StatusCode, "body", string(body))
		resp.Error = fmt.Sprintf("更新服务不可用 (HTTP %d)", httpResp.StatusCode)
		return resp, nil
	}

	var info updateServerInfo
	if err := json.Unmarshal(body, &info); err != nil {
		s.logger.Error("解析更新响应失败", "err", err, "body", string(body))
		resp.Error = "解析更新响应失败"
		return resp, nil
	}

	resp.Available = info.Available
	resp.LatestVersion = info.LatestVersion
	resp.DownloadUrl = info.DownloadURL
	resp.ChangelogUrl = info.ChangelogURL
	resp.Changelog = info.Changelog
	resp.PackageSize = info.PackageSize
	resp.PackageSha256 = info.PackageSHA256
	resp.PublishedAt = info.PublishedAt
	resp.Error = info.Error

	if resp.Available {
		s.logger.Info("发现新版本", "latest", resp.LatestVersion)
	} else {
		s.logger.Info("当前已是最新版本")
	}
	return resp, nil
}
