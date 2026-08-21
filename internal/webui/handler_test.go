package webui

import (
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"testing/fstest"
)

var builtAssetReferencePattern = regexp.MustCompile(`(?:src|href)="(/assets/[^"]+)"`)

func TestHandlerServesAssetsAndRejectsTraversal(t *testing.T) {
	handler := newHandler(fstest.MapFS{
		"assets/index.html":             &fstest.MapFile{Data: []byte("built")},
		"assets/assets/app.1234abcd.js": &fstest.MapFile{Data: []byte("console.log(1)")},
	})

	asset := httptest.NewRecorder()
	handler.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/assets/app.1234abcd.js", nil))
	if asset.Code != http.StatusOK || asset.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("asset status=%d headers=%v", asset.Code, asset.Header())
	}
	traversal := httptest.NewRecorder()
	handler.ServeHTTP(traversal, httptest.NewRequest(http.MethodGet, "/assets/%2e%2e/data/auth.json", nil))
	if traversal.Code != http.StatusBadRequest && traversal.Code != http.StatusNotFound {
		t.Fatalf("traversal=%d", traversal.Code)
	}
	nonAssetTraversal := httptest.NewRecorder()
	handler.ServeHTTP(nonAssetTraversal, httptest.NewRequest(http.MethodGet, "/profiles/%2e%2e/config", nil))
	if nonAssetTraversal.Code != http.StatusBadRequest && nonAssetTraversal.Code != http.StatusNotFound {
		t.Fatalf("non-asset traversal=%d", nonAssetTraversal.Code)
	}
}

func TestEmbeddedBuildReferencesResolvableAssets(t *testing.T) {
	index, err := fs.ReadFile(Files, builtIndexPath)
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip("前端尚未构建")
	}
	if err != nil {
		t.Fatal(err)
	}
	references := builtAssetReferencePattern.FindAllSubmatch(index, -1)
	if len(references) == 0 {
		t.Fatal("构建后的 index.html 未引用静态资源")
	}

	handler := Handler()
	for _, reference := range references {
		requestPath := string(reference[1])
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, requestPath, nil))
		if recorder.Code != http.StatusOK {
			t.Errorf("asset %s status=%d body=%q", requestPath, recorder.Code, recorder.Body.String())
		}
	}
}
