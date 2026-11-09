package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"0.1.1", "0.1.0", true},
		{"0.2.0", "0.1.9", true},
		{"0.1.0", "0.1.0", false},
		{"0.1.0", "0.1.1", false},
		{"1.0", "0.9.9", true},
		{"0.1.0.1", "0.1.0", true},
		{"v0.1.2", "0.1.1", true},
		{"0.1.2", "dev", false},
		{"", "0.1.0", false},
	}
	for _, c := range cases {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/Alumos/meowx-docker/releases/latest" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"tag_name":"v0.3.0","html_url":"https://example.com/r","published_at":"2026-11-09T00:00:00Z",
			"assets":[{"name":"docker-compose.yaml","browser_download_url":"https://example.com/c","size":1},
			{"name":"meowx-docker-0.3.0-amd64.tar.gz","browser_download_url":"https://example.com/t","size":42}]}`))
	}))
	defer srv.Close()
	u := &Updater{Version: "0.2.0", API: srv.URL}
	info := u.Check(context.Background(), true)
	if info.Error != "" || !info.HasUpdate || info.Latest != "0.3.0" || info.DownloadURL != "https://example.com/t" || info.Size != 42 {
		t.Fatalf("%+v", info)
	}
	if info.CanInstall || info.Command == "" || info.Image != "ghcr.io/alumos/meowx-docker" {
		t.Fatalf("%+v", info)
	}
	u = &Updater{Version: "0.3.0", API: srv.URL}
	if info := u.Check(context.Background(), true); info.HasUpdate {
		t.Fatalf("同版本不算新：%+v", info)
	}
	u = &Updater{Version: "0.3.0", API: srv.URL, Repo: "x/y"}
	if info := u.Check(context.Background(), true); info.Error == "" {
		t.Fatal("没有发布时应当报错")
	}
}
