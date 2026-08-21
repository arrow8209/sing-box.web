package webui

import (
	"errors"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

const (
	assetPrefix       = "/assets/"
	immutableCache    = "public, max-age=31536000, immutable"
	htmlCache         = "no-cache"
	builtIndexPath    = "assets/index.html"
	fallbackIndexPath = "fallback/index.html"
)

// Handler serves only embedded UI files. It never falls back to the host
// filesystem.
func Handler() http.Handler {
	return newHandler(Files)
}

func newHandler(files fs.FS) http.Handler {
	assets, assetsErr := fs.Sub(files, "assets/assets")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "请求方法不被允许", http.StatusMethodNotAllowed)
			return
		}
		if invalidRequestPath(r.URL.Path) {
			http.Error(w, "无效静态资源路径", http.StatusBadRequest)
			return
		}
		if strings.HasPrefix(r.URL.Path, assetPrefix) {
			serveAsset(w, r, assets, assetsErr)
			return
		}
		serveIndex(w, r, files)
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request, files fs.FS) {
	file, err := openEmbeddedFile(files, builtIndexPath)
	if errors.Is(err, fs.ErrNotExist) {
		file, err = openEmbeddedFile(files, fallbackIndexPath)
	}
	if err != nil {
		http.Error(w, "嵌入前端资源不可用", http.StatusInternalServerError)
		return
	}
	defer file.Close()
	w.Header().Set("Cache-Control", htmlCache)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodHead {
		return
	}
	_, _ = io.Copy(w, file)
}

func serveAsset(w http.ResponseWriter, r *http.Request, assets fs.FS, assetsErr error) {
	name, ok := assetName(r.URL.Path)
	if !ok {
		http.Error(w, "无效静态资源路径", http.StatusBadRequest)
		return
	}
	if assetsErr != nil {
		http.NotFound(w, r)
		return
	}
	file, err := openEmbeddedFile(assets, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	if isContentHashed(name) {
		w.Header().Set("Cache-Control", immutableCache)
	} else {
		w.Header().Set("Cache-Control", htmlCache)
	}
	if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	if r.Method == http.MethodHead {
		return
	}
	_, _ = io.Copy(w, file)
}

func openEmbeddedFile(files fs.FS, name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, fs.ErrInvalid
	}
	return files.Open(name)
}

func assetName(requestPath string) (string, bool) {
	if !strings.HasPrefix(requestPath, assetPrefix) {
		return "", false
	}
	name := strings.TrimPrefix(requestPath, assetPrefix)
	if invalidRequestPath(name) || !fs.ValidPath(name) {
		return "", false
	}
	return name, true
}

func invalidRequestPath(value string) bool {
	if strings.ContainsAny(value, "\\\x00") {
		return true
	}
	for _, component := range strings.Split(value, "/") {
		if component == ".." {
			return true
		}
	}
	return false
}

func isContentHashed(name string) bool {
	for _, part := range strings.Split(path.Base(name), ".") {
		if len(part) < 8 {
			continue
		}
		valid := true
		for _, char := range part {
			if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
				valid = false
				break
			}
		}
		if valid {
			return true
		}
	}
	return false
}
