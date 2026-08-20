package webui

import (
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

const (
	placeholderFile       = "placeholder.txt"
	indexCacheControl     = "no-cache"
	assetCacheControlHint = "public, max-age=31536000, immutable"
	assetsPrefix          = "assets/"
)

func NewHandler(files fs.FS) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		relPath, ok := normalizePath(request.URL.Path)
		if !ok {
			response.WriteHeader(http.StatusBadRequest)
			return
		}

		if relPath == "" || relPath == "index.html" {
			serveIndex(response, files)
			return
		}

		if relPath == placeholderFile {
			response.WriteHeader(http.StatusNotFound)
			return
		}

		if strings.Contains(relPath, ".") {
			if cacheControl := assetCacheControl(relPath); cacheControl != "" {
				response.Header().Set("Cache-Control", cacheControl)
			}
			serveAsset(response, files, relPath)
			return
		}

		if strings.HasPrefix(relPath, assetsPrefix) {
			response.WriteHeader(http.StatusNotFound)
			return
		}

		serveIndex(response, files)
	})
}

func normalizePath(raw string) (string, bool) {
	unescaped, err := url.PathUnescape(raw)
	if err != nil {
		return "", false
	}
	for _, part := range strings.Split(unescaped, "/") {
		if part == ".." {
			return "", false
		}
	}
	clean := path.Clean("/" + unescaped)
	if strings.HasPrefix(clean, "/../") {
		return "", false
	}
	return strings.TrimPrefix(clean, "/"), true
}

func serveAsset(response http.ResponseWriter, files fs.FS, filePath string) {
	data, err := fs.ReadFile(files, filePath)
	if err != nil {
		response.WriteHeader(http.StatusNotFound)
		return
	}
	if contentType := mime.TypeByExtension(filepath.Ext(filePath)); contentType != "" {
		response.Header().Set("Content-Type", contentType)
	}
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(data)
}

func serveIndex(response http.ResponseWriter, files fs.FS) {
	response.Header().Set("Cache-Control", indexCacheControl)
	serveAsset(response, files, "index.html")
}

func assetCacheControl(path string) string {
	if strings.HasPrefix(path, assetsPrefix) {
		return assetCacheControlHint
	}
	return ""
}
