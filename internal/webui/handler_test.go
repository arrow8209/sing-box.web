package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"singbox-web/internal/httpapi"
)

const exactCSP = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'"

func TestNewHandlerServingAndFallback(t *testing.T) {
	files := fstest.MapFS{
		"index.html":      &fstest.MapFile{Data: []byte("<html><body>index</body></html>")},
		"assets/app.js":   &fstest.MapFile{Data: []byte("console.log('app')")},
		"assets/app.css":  &fstest.MapFile{Data: []byte("body { color: black; }")},
		"placeholder.txt": &fstest.MapFile{Data: []byte("singbox-web embedded UI build sentinel\n")},
	}

	handler := httpapi.WithSecurityHeaders(NewHandler(files))

	tests := []struct {
		name        string
		path        string
		wantStatus  int
		wantBody    string
		cache       string
		contentType string
	}{
		{name: "asset", path: "/assets/app.js", wantStatus: http.StatusOK, wantBody: "console.log('app')", cache: assetCacheControlHint, contentType: "text/javascript"},
		{name: "stylesheet", path: "/assets/app.css", wantStatus: http.StatusOK, wantBody: "body { color: black; }", cache: assetCacheControlHint, contentType: "text/css"},
		{name: "spa fallback", path: "/config/id/general", wantStatus: http.StatusOK, wantBody: "<html><body>index</body></html>", cache: indexCacheControl},
		{name: "missing asset", path: "/assets/missing.js", wantStatus: http.StatusNotFound},
		{name: "placeholder", path: "/placeholder.txt", wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := httptest.NewRecorder()
			handler.ServeHTTP(record, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if record.Code != tt.wantStatus {
				t.Fatalf("status=%d want=%d", record.Code, tt.wantStatus)
			}
			if tt.cache != "" && record.Header().Get("Cache-Control") != tt.cache {
				t.Fatalf("cache=%q want=%q", record.Header().Get("Cache-Control"), tt.cache)
			}
			if tt.contentType != "" && !strings.HasPrefix(record.Header().Get("Content-Type"), tt.contentType) {
				t.Fatalf("content-type=%q want prefix=%q", record.Header().Get("Content-Type"), tt.contentType)
			}
			if record.Header().Get("Content-Security-Policy") != exactCSP {
				t.Fatalf("missing CSP")
			}
			if record.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatalf("missing X-Content-Type-Options")
			}
			if record.Header().Get("Referrer-Policy") != "no-referrer" {
				t.Fatalf("missing Referrer-Policy")
			}
			if tt.wantBody != "" {
				if body := record.Body.String(); body != tt.wantBody {
					t.Fatalf("body=%q want=%q", body, tt.wantBody)
				}
			}
		})
	}
}

func TestNewHandlerPathTraversalAndAPIFallback(t *testing.T) {
	files := fstest.MapFS{
		"index.html":    &fstest.MapFile{Data: []byte("<html><body>index</body></html>")},
		"assets/app.js": &fstest.MapFile{Data: []byte("console.log('app')")},
	}
	handler := httpapi.WithSecurityHeaders(NewHandler(files))

	mux := http.NewServeMux()
	mux.Handle("/api/", http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNotFound)
		_, _ = response.Write([]byte("api-miss"))
	}))
	mux.Handle("/", httpapi.WithSecurityHeaders(NewHandler(files)))

	for _, path := range []string{"/%2e%2e/%2e%2e/assets/app.js", "/assets/../app.js", "/config/../../assets/app.js"} {
		record := httptest.NewRecorder()
		handler.ServeHTTP(record, httptest.NewRequest(http.MethodGet, path, nil))
		if record.Code != http.StatusBadRequest {
			t.Fatalf("path=%q status=%d", path, record.Code)
		}
	}

	record := httptest.NewRecorder()
	mux.ServeHTTP(record, httptest.NewRequest(http.MethodGet, "/api/v1/unknown", nil))
	if record.Code != http.StatusNotFound {
		t.Fatalf("status=%d want=404", record.Code)
	}
	if body := record.Body.String(); body != "api-miss" {
		t.Fatalf("body=%q", body)
	}
}
