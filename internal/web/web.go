package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed dist
var assets embed.FS

func Handler(admin http.Handler) http.Handler {
	dist, err := fs.Sub(assets, "dist")
	if err != nil {
		panic(err)
	}

	files := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/proto.") {
			admin.ServeHTTP(w, r)
			return
		}

		if r.URL.Path == "/admin-api" || strings.HasPrefix(r.URL.Path, "/admin-api/") {
			req := r.Clone(r.Context())
			req.URL.Path = strings.TrimPrefix(r.URL.Path, "/admin-api")
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
			admin.ServeHTTP(w, req)
			return
		}

		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "." {
			name = "index.html"
		}
		if info, err := fs.Stat(dist, name); err == nil && !info.IsDir() {
			files.ServeHTTP(w, r)
			return
		}

		req := r.Clone(r.Context())
		req.URL.Path = "/"
		files.ServeHTTP(w, req)
	})
}
