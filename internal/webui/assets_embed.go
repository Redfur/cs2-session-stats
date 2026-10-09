//go:build embedweb

package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

func assets() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
