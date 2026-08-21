package webui

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestHandlerServesAssetsAndRejectsTraversal(t *testing.T) {
	handler := newHandler(fstest.MapFS{
		"assets/index.html":      &fstest.MapFile{Data: []byte("built")},
		"assets/app.1234abcd.js": &fstest.MapFile{Data: []byte("console.log(1)")},
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
