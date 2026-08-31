package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"singbox-web/internal/core"
	"singbox-web/internal/paths"
)

type fixedCoreStatus struct{ state core.ProcessState }

func (status fixedCoreStatus) Status(context.Context) core.Status {
	return core.Status{State: status.state}
}

type recordingClient struct{ calls int }

func (client *recordingClient) Do(*http.Request) (*http.Response, error) {
	client.calls++
	return nil, nil
}

func TestHandlerReadsAndClosesConnectionsThroughAppliedController(t *testing.T) {
	var methods []string
	controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method+" "+r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer private-controller-secret" {
			t.Fatalf("authorization header not forwarded")
		}
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_ = json.NewEncoder(w).Encode(ConnectionsSnapshot{
			Memory: 2048, UploadTotal: 100, DownloadTotal: 200,
			Connections: []Connection{{
				ID: "connection-1", Metadata: map[string]any{"host": "example.test", "network": "tcp"},
				Upload: 10, Download: 20, Start: "2026-08-31T06:00:00Z", Chains: []string{"proxy-a"}, Rule: "final",
			}},
		})
	}))
	defer controller.Close()

	layout := writeControllerConfig(t, strings.TrimPrefix(controller.URL, "http://"), "private-controller-secret")
	handler := NewHandler(NewService(layout, fixedCoreStatus{state: core.ProcessRunning}))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, connectionsPath, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET status=%d body=%s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	var snapshot ConnectionsSnapshot
	if err := json.Unmarshal([]byte(body), &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Connections) != 1 || snapshot.Connections[0].ID != "connection-1" {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	if strings.Contains(body, "private-controller-secret") {
		t.Fatal("controller secret leaked in response")
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, connectionsPath+"/connection-1", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("DELETE status=%d body=%s", response.Code, response.Body.String())
	}
	if strings.Join(methods, ",") != "GET /connections,DELETE /connections/connection-1" {
		t.Fatalf("controller requests=%v", methods)
	}
}

func TestHandlerReportsStoppedCoreWithoutContactingController(t *testing.T) {
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	client := &recordingClient{}
	handler := NewHandler(NewServiceWithClient(layout, fixedCoreStatus{state: core.ProcessStopped}, client))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, connectionsPath, nil))
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "core_not_running") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if client.calls != 0 {
		t.Fatalf("controller calls=%d", client.calls)
	}
}

func TestHandlerRefusesNonLocalAppliedController(t *testing.T) {
	layout := writeControllerConfig(t, "192.0.2.10:9090", "do-not-leak")
	client := &recordingClient{}
	handler := NewHandler(NewServiceWithClient(layout, fixedCoreStatus{state: core.ProcessRunning}, client))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, connectionsPath, nil))
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "clash_api_not_configured") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "do-not-leak") {
		t.Fatal("controller secret leaked in error")
	}
	if client.calls != 0 {
		t.Fatalf("controller calls=%d", client.calls)
	}
}

func TestHandlerRejectsInvalidConnectionIDAndMethods(t *testing.T) {
	response := httptest.NewRecorder()
	NewHandler(&Service{}).ServeHTTP(response, httptest.NewRequest(http.MethodDelete, connectionsPath+"/invalid%20id", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid id status=%d", response.Code)
	}
	response = httptest.NewRecorder()
	NewHandler(&Service{}).ServeHTTP(response, httptest.NewRequest(http.MethodPost, connectionsPath, nil))
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != "GET, DELETE" {
		t.Fatalf("method status=%d allow=%q", response.Code, response.Header().Get("Allow"))
	}
}

func TestLocalControllerAddressNormalizesOnlyLocalListeners(t *testing.T) {
	tests := map[string]string{
		"127.0.0.1:9090": "127.0.0.1:9090",
		"localhost:9090": "127.0.0.1:9090",
		"0.0.0.0:9090":   "127.0.0.1:9090",
		"[::1]:9090":     "[::1]:9090",
		"[::]:9090":      "[::1]:9090",
	}
	for input, want := range tests {
		got, err := localControllerAddress(input)
		if err != nil || got != want {
			t.Fatalf("input=%q got=%q err=%v", input, got, err)
		}
	}
	for _, input := range []string{"", "http://127.0.0.1:9090", "example.test:9090", "127.0.0.1:0"} {
		if _, err := localControllerAddress(input); err == nil {
			t.Fatalf("input=%q unexpectedly accepted", input)
		}
	}
}

func writeControllerConfig(t *testing.T, controller, secret string) paths.Layout {
	t.Helper()
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(layout.ConfigDir, 0o700); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{"experimental": map[string]any{"clash_api": map[string]any{
		"external_controller": controller, "secret": secret,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(layout.ConfigDir, "99-experimental.json"), payload, 0o600); err != nil {
		t.Fatal(err)
	}
	return layout
}
