// Package github 提供 GitHub Releases 读取能力。
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Release 是从 GitHub API 解析出的 release。
type Release struct {
	TagName     string
	Name        string
	Body        string // release 的 Markdown 更新日志（GitHub API body 字段）
	Prerelease  bool
	Draft       bool
	PublishedAt time.Time
	HTMLURL     string
	Assets      []Asset
}

// Asset 是 release 里的安装包资源。
type Asset struct {
	Name               string
	Size               int64
	BrowserDownloadURL string
}

// Client 用于查询 GitHub Releases，带本地缓存。
type Client struct {
	owner  string
	repo   string
	token  string
	ttl    time.Duration
	client *http.Client

	mu        sync.RWMutex
	cached    []Release
	cachedAt  time.Time
	cachedErr error
}

// NewClient 创建 GitHub 客户端。ttl 控制缓存有效期，<=0 则每次请求都回源。
func NewClient(owner, repo, token string, ttl time.Duration) *Client {
	return &Client{
		owner: owner,
		repo:  repo,
		token: token,
		ttl:   ttl,
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

type ghAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type ghRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	Prerelease  bool      `json:"prerelease"`
	Draft       bool      `json:"draft"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
	Assets      []ghAsset `json:"assets"`
}

// ListReleases 返回该仓库的 releases（默认最新 100 条）。结果会按 ttl 缓存。
func (c *Client) ListReleases(ctx context.Context) ([]Release, error) {
	c.mu.RLock()
	if c.cachedErr != nil && time.Since(c.cachedAt) < c.ttl {
		defer c.mu.RUnlock()
		return nil, c.cachedErr
	}
	if len(c.cached) > 0 && time.Since(c.cachedAt) < c.ttl {
		defer c.mu.RUnlock()
		return c.copyCache(), nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// 双重检查，防止多个请求同时回源。
	if c.cachedErr != nil && time.Since(c.cachedAt) < c.ttl {
		return nil, c.cachedErr
	}
	if len(c.cached) > 0 && time.Since(c.cachedAt) < c.ttl {
		return c.copyCache(), nil
	}

	releases, err := c.fetch(ctx)
	c.cachedAt = time.Now()
	c.cachedErr = err
	if err != nil {
		c.cached = nil
		return nil, err
	}
	c.cached = releases
	return c.copyCache(), nil
}

func (c *Client) copyCache() []Release {
	out := make([]Release, len(c.cached))
	copy(out, c.cached)
	return out
}

func (c *Client) fetch(ctx context.Context) ([]Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=100", c.owner, c.repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github api request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned %d", resp.StatusCode)
	}

	var raw []ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode github response: %w", err)
	}

	out := make([]Release, 0, len(raw))
	for _, r := range raw {
		assets := make([]Asset, 0, len(r.Assets))
		for _, a := range r.Assets {
			assets = append(assets, Asset{
				Name:               a.Name,
				Size:               a.Size,
				BrowserDownloadURL: a.BrowserDownloadURL,
			})
		}
		out = append(out, Release{
			TagName:     r.TagName,
			Name:        r.Name,
			Body:        r.Body,
			Prerelease:  r.Prerelease,
			Draft:       r.Draft,
			PublishedAt: r.PublishedAt,
			HTMLURL:     r.HTMLURL,
			Assets:      assets,
		})
	}
	return out, nil
}

// LatestRelease 按 channel 取最新 release。
//   - stable: 最新的非 draft、非 prerelease release
//   - beta:   最新的非 draft release（允许 prerelease）
func (c *Client) LatestRelease(ctx context.Context, channel string) (*Release, error) {
	releases, err := c.ListReleases(ctx)
	if err != nil {
		return nil, err
	}
	for i := range releases {
		r := &releases[i]
		if r.Draft {
			continue
		}
		if channel == "stable" && r.Prerelease {
			continue
		}
		return r, nil
	}
	return nil, nil
}

// AssetName 根据版本号与模板生成 asset 文件名。
func AssetName(version, template string) string {
	v := strings.TrimPrefix(version, "v")
	return strings.ReplaceAll(template, "{version}", v)
}

// FindAsset 在 release 中查找指定文件名的 asset。
func FindAsset(r *Release, filename string) *Asset {
	for i := range r.Assets {
		if r.Assets[i].Name == filename {
			return &r.Assets[i]
		}
	}
	return nil
}
