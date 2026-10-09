// Package webui раздаёт собранный React-фронтенд как SPA.
package webui

import (
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// Default раздаёт фронтенд, встроенный в бинарь (build tag embedweb) или лежащий на диске.
func Default() http.Handler { return New(assets()) }

// New раздаёт файлы из fsys; неизвестные пути получают index.html, чтобы работали клиентские маршруты.
func New(fsys fs.FS) http.Handler {
	files := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" {
			if info, err := fs.Stat(fsys, name); err == nil && !info.IsDir() {
				files.ServeHTTP(w, r)
				return
			}
		}
		index, err := fs.ReadFile(fsys, "index.html")
		if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, "фронтенд не собран: выполните make web", http.StatusServiceUnavailable)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(index)
	})
}
