package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"singbox-web/internal/capabilities"
)

type staticCapabilities struct{ value capabilities.Snapshot }

func (service staticCapabilities) Snapshot(context.Context) capabilities.Snapshot {
	return service.value
}

func TestHealthAndCapabilitiesRoutes(t *testing.T) {
	wantCapabilities := capabilities.Snapshot{Capabilities: map[string]bool{"config.manage": true}}
	handler := New(Dependencies{Capabilities: staticCapabilities{wantCapabilities}})

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusOK || health.Body.String() != "{\"status\":\"ok\"}\n" {
		t.Fatalf("health: status=%d body=%q", health.Code, health.Body.String())
	}

	capability := httptest.NewRecorder()
	handler.ServeHTTP(capability, httptest.NewRequest(http.MethodGet, "/api/v1/capabilities", nil))
	if capability.Code != http.StatusUnauthorized || capability.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("capabilities without auth: status=%d headers=%v", capability.Code, capability.Header())
	}
}

func TestErrorEnvelope(t *testing.T) {
	handler := New(Dependencies{})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil))
	var body ErrorBody
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Error.Code != "unauthorized" || body.Error.Message == "" {
		t.Fatalf("body=%q decoded=%#v err=%v", response.Body.String(), body, err)
	}
}
