// Package update：检查新版本（GitHub Releases）。Docker 版不自己换镜像：网页显示更新命令和离线镜像包的下载地址，
// 由用户在 NAS 上 `docker compose pull && docker compose up -d`（或在 UGOS 的 Docker 应用里重新拉取 / 导入镜像）。
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	DefaultRepo = "Alumos/meowx-docker"
	cacheFor    = time.Hour
	// UpdateCommand：在 docker-compose.yaml 所在目录执行
	UpdateCommand = "docker compose pull && docker compose up -d"
)

type Updater struct {
	Version string // 当前版本
	Repo    string // owner/name，缺省 DefaultRepo
	API     string // 只给测试用，缺省 https://api.github.com
	Client  *http.Client
	Log     func(string)

	mu        sync.Mutex
	cached    *Info
	checkedAt time.Time
}

// Info：给网页的检查结果（字段沿用飞牛版，can_install 恒为 false）
type Info struct {
	Current     string `json:"current"`
	Latest      string `json:"latest,omitempty"`
	HasUpdate   bool   `json:"has_update"`
	ReleasedAt  string `json:"released_at,omitempty"`
	NotesURL    string `json:"notes_url,omitempty"`
	DownloadURL string `json:"download_url,omitempty"` // 离线镜像包（docker load 用）
	Size        int64  `json:"size,omitempty"`
	CanInstall  bool   `json:"can_install"`
	Command     string `json:"command"`
	Image       string `json:"image"`
	Error       string `json:"error,omitempty"`
	CheckedAt   int64  `json:"checked_at"`
}

func (u *Updater) repo() string {
	if u.Repo == "" {
		return DefaultRepo
	}
	return u.Repo
}

// Image：镜像地址（GHCR 的仓库名一律小写）
func (u *Updater) Image() string { return "ghcr.io/" + strings.ToLower(u.repo()) }

// Check：缓存一小时，force 时重新取
func (u *Updater) Check(ctx context.Context, force bool) Info {
	u.mu.Lock()
	if !force && u.cached != nil && time.Since(u.checkedAt) < cacheFor {
		info := *u.cached
		u.mu.Unlock()
		return info
	}
	u.mu.Unlock()

	info := Info{Current: u.Version, Command: UpdateCommand, Image: u.Image(), CheckedAt: time.Now().Unix()}
	if err := u.fetch(ctx, &info); err != nil {
		info.Error = err.Error()
		return info
	}
	u.mu.Lock()
	u.cached, u.checkedAt = &info, time.Now()
	u.mu.Unlock()
	return info
}

func (u *Updater) fetch(ctx context.Context, info *Info) error {
	api := u.API
	if api == "" {
		api = "https://api.github.com"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api+"/repos/"+u.repo()+"/releases/latest", nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "MeowX/"+u.Version+" (Docker)")
	req.Header.Set("Accept", "application/vnd.github+json")
	client := u.Client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("取不到发布信息：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return errors.New("还没有发布过正式版本")
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("取不到发布信息：HTTP %d", resp.StatusCode)
	}
	var rel struct {
		TagName     string `json:"tag_name"`
		HTMLURL     string `json:"html_url"`
		PublishedAt string `json:"published_at"`
		Assets      []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
			Size int64  `json:"size"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&rel); err != nil {
		return errors.New("发布信息格式不对")
	}
	latest := strings.TrimPrefix(rel.TagName, "v")
	if latest == "" {
		return errors.New("发布信息里没有版本号")
	}
	info.Latest, info.ReleasedAt, info.NotesURL = latest, rel.PublishedAt, rel.HTMLURL
	info.HasUpdate = Newer(latest, u.Version)
	for _, a := range rel.Assets {
		if strings.HasSuffix(a.Name, "-amd64.tar.gz") {
			info.DownloadURL, info.Size = a.URL, a.Size
			break
		}
	}
	return nil
}

// Newer：a 比 b 新（按点分的数字逐段比，段里数字后面的后缀不算；b 不是版本号时不算新）
func Newer(a, b string) bool {
	pa, okA := parseVersion(a)
	pb, okB := parseVersion(b)
	if !okA || !okB {
		return false
	}
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}

func parseVersion(v string) ([]int, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if v == "" {
		return nil, false
	}
	var out []int
	for _, part := range strings.Split(v, ".") {
		n := 0
		digits := 0
		for _, c := range part {
			if c < '0' || c > '9' {
				break
			}
			n = n*10 + int(c-'0')
			digits++
		}
		if digits == 0 {
			return nil, false
		}
		out = append(out, n)
	}
	return out, true
}
