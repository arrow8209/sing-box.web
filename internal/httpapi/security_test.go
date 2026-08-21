package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"singbox-web/internal/httpapi"
)

func TestSecurityHeaders(t *testing.T) {
	recorder := httptest.NewRecorder()
	httpapi.SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Header().Get("X-Content-Type-Options") != "nosniff" ||
		recorder.Header().Get("Referrer-Policy") != "no-referrer" ||
		recorder.Header().Get("Content-Security-Policy") == "" ||
		recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("headers=%v", recorder.Header())
	}
}
