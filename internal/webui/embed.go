package webui

import (
	"embed"
	"errors"
	"io/fs"
	"net/http"
)

// Files contains the Vue build output and the version-controlled fallback page.
//
//go:embed all:assets all:fallback
var Files embed.FS

// Handler serves the Vue index page when it is embedded, otherwise it serves
// the version-controlled fallback page.
func Handler() http.Handler {
	return newHandler(Files)
}

func newHandler(files fs.FS) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		page, err := fs.ReadFile(files, "assets/index.html")
		if errors.Is(err, fs.ErrNotExist) {
			page, err = fs.ReadFile(files, "fallback/index.html")
		}
		if err != nil {
			http.Error(writer, "嵌入前端资源不可用", http.StatusInternalServerError)
			return
		}

		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = writer.Write(page)
	})
}
