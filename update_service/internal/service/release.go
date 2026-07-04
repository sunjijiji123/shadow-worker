// Package service 处理版本业务，数据源为 GitHub Releases。
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"shadow-worker/update_service/internal/config"
	"shadow-worker/update_service/internal/github"
	"shadow-worker/update_service/internal/version"
)

// ReleaseInfo 是对外统一的 release 视图。
type ReleaseInfo struct {
	ID              int64
	Version         string
	Channel         string
	MinVersion      string
	PackageFilename string
	PackageSize     int64
	PackageSHA256   string
	ChangelogURL    string
	Body            string // Markdown 更新日志原文
	DownloadURL     string
	PublishedAt     time.Time
	CreatedAt       time.Time
}

// ReleaseService 处理版本业务。
type ReleaseService struct {
	gh       *github.Client
	cfg      *config.Config
}

// NewReleaseService 创建 ReleaseService。
func NewReleaseService(gh *github.Client, cfg *config.Config) *ReleaseService {
	return &ReleaseService{gh: gh, cfg: cfg}
}

// CheckUpdate 检查是否有新版本。
func (s *ReleaseService) CheckUpdate(ctx context.Context, currentVersion, channel string) (*ReleaseInfo, bool, error) {
	current, err := version.Parse(currentVersion)
	if err != nil {
		return nil, false, fmt.Errorf("invalid current version: %w", err)
	}

	latest, err := s.gh.LatestRelease(ctx, channel)
	if err != nil {
		return nil, false, fmt.Errorf("query github: %w", err)
	}
	if latest == nil {
		return nil, false, nil
	}

	latestVer, err := version.Parse(strings.TrimPrefix(latest.TagName, "v"))
	if err != nil {
		return nil, false, fmt.Errorf("invalid latest version: %w", err)
	}
	if latestVer.Compare(current) <= 0 {
		return nil, false, nil
	}

	info, err := s.mapRelease(latest)
	if err != nil {
		return nil, false, err
	}
	return info, true, nil
}

// List 返回版本列表。
func (s *ReleaseService) List(ctx context.Context, channel string) ([]ReleaseInfo, error) {
	releases, err := s.gh.ListReleases(ctx)
	if err != nil {
		return nil, fmt.Errorf("query github: %w", err)
	}

	out := make([]ReleaseInfo, 0, len(releases))
	for i := range releases {
		r := &releases[i]
		if r.Draft {
			continue
		}
		relChannel := "stable"
		if r.Prerelease {
			relChannel = "beta"
		}
		if channel != "" && channel != relChannel {
			continue
		}
		info, err := s.mapRelease(r)
		if err != nil {
			// 跳过没有对应 asset 的 release，不打断整个列表
			continue
		}
		info.Channel = relChannel
		out = append(out, *info)
	}
	return out, nil
}

func (s *ReleaseService) mapRelease(r *github.Release) (*ReleaseInfo, error) {
	version := strings.TrimPrefix(r.TagName, "v")
	filename := github.AssetName(r.TagName, s.cfg.AssetNameTemplate)
	asset := github.FindAsset(r, filename)
	if asset == nil {
		return nil, fmt.Errorf("asset %s not found in release %s", filename, r.TagName)
	}
	return &ReleaseInfo{
		ID:              0,
		Version:         version,
		PackageFilename: asset.Name,
		PackageSize:     asset.Size,
		PackageSHA256:   "",
		ChangelogURL:    r.HTMLURL,
		Body:            r.Body,
		DownloadURL:     asset.BrowserDownloadURL,
		PublishedAt:     r.PublishedAt,
		CreatedAt:       r.PublishedAt,
	}, nil
}
