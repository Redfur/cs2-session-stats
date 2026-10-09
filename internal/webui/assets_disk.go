//go:build !embedweb

package webui

import (
	"io/fs"
	"os"
)

// Без тега embedweb (разработка, тесты) статика читается с диска, поэтому
// go build и go test работают без сборки фронтенда.
func assets() fs.FS {
	dir := os.Getenv("STATIC_DIR")
	if dir == "" {
		dir = "internal/webui/dist"
	}
	return os.DirFS(dir)
}
