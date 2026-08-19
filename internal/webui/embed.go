package webui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var embedded embed.FS

func Handler() http.Handler {
	files, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err)
	}
	return NewHandler(files)
}
