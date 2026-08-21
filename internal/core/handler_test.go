package core

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"singbox-web/internal/paths"
)

func TestHandlerReturnsCoreStatusAndRejectsWrites(t *testing.T) {
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(NewManagerWithFactory(layout, nil, nil, nil, &fakeFactory{}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/core", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "not_installed") {
		t.Fatalf("response=%d %s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/core", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("response=%d", response.Code)
	}
}
