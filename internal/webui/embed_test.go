package webui

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestHandlerUsesFallbackWhenBuiltIndexIsMissing(t *testing.T) {
	handler := newHandler(fstest.MapFS{
		"fallback/index.html": &fstest.MapFile{Data: []byte("fallback")},
	})
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Body.String(); got != "fallback" {
		t.Fatalf("body = %q, want fallback", got)
	}
}

func TestHandlerPrefersBuiltIndex(t *testing.T) {
	handler := newHandler(fstest.MapFS{
		"assets/index.html":   &fstest.MapFile{Data: []byte("built")},
		"fallback/index.html": &fstest.MapFile{Data: []byte("fallback")},
	})
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/profiles", nil))

	if got := recorder.Body.String(); got != "built" {
		t.Fatalf("body = %q, want built", got)
	}
}
