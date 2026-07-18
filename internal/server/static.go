package server

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/deng23yu/stockagent/web"
)

// staticHandler 服务内嵌的前端静态资源，未知路径回退到 index.html (SPA)。
func staticHandler() http.Handler {
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		return staticUnavailable(err)
	}
	index, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		return staticUnavailable(err)
	}
	fileSrv := http.FileServerFS(dist)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p != "" && !strings.Contains(p, "..") {
			if f, err := fs.Stat(dist, p); err == nil && !f.IsDir() {
				fileSrv.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
	})
}

// staticUnavailable 在前端未构建时给出可操作的提示。
func staticUnavailable(err error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Web UI 不可用 (前端未构建): cd web && npm run build\n"+err.Error(), http.StatusNotImplemented)
	})
}
