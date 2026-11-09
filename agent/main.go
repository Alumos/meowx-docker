// meowx-agent：MeowX Docker 版的控制服务（网页界面 + 管理内核 / 喵窝），容器里的 PID 1。
package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Alumos/meowx-docker/agent/internal/api"
	"github.com/Alumos/meowx-docker/agent/internal/app"
	"github.com/Alumos/meowx-docker/agent/internal/update"
)

var version = "dev"

//go:embed web
var webFS embed.FS

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	a, err := app.New(version, envOr("MEOWX_DATA", "/data"), envOr("MEOWX_CORE", "/usr/bin/meowx-core"),
		envOr("MEOWX_TAILSCALE_DIR", "/usr/bin"), envOr("MEOWX_DASHBOARD", ""))
	if err != nil {
		log.Fatal(err)
	}
	a.Log.Add("agent", "MeowX "+version+" 启动")
	// 检查更新：读 GitHub Releases（MEOWX_UPDATE_REPO 可改成自己的仓库）
	a.Update = &update.Updater{
		Version: version, Repo: os.Getenv("MEOWX_UPDATE_REPO"),
		Client: &http.Client{Timeout: 20 * time.Second},
		Log:    func(s string) { a.Log.Add("agent", s) },
	}
	a.Mesh.Cleanup() // 上次没收干净（容器被强杀）留下的防火墙规则

	web, _ := fs.Sub(webFS, "web")
	srv := &http.Server{
		Addr:              ":" + envOr("MEOWX_UI_PORT", "7899"),
		Handler:           (&api.Server{App: a, Web: web}).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	go a.Loop(ctx)
	go func() {
		if a.Settings.Get().Enabled {
			_ = a.Apply(ctx)
		}
		if a.Settings.Get().Mesh.Enabled {
			_ = a.Mesh.Apply(ctx)
		}
	}()
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	// 容器停止 / 卸载：先停内核（它退出时撤掉自己加的路由），再关网页
	a.Shutdown()
	shut, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shut)
}
