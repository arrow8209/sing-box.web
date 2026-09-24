package runtime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"singbox-web/internal/core"
)

func TestModeChangesReachCoreAndFreshServiceReadsActualMode(t *testing.T) {
	var mode atomic.Value
	mode.Store("Rule")
	var writes atomic.Int32
	controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/configs" || r.Header.Get("Authorization") != "Bearer controller-only-secret" {
			t.Error("incorrect controller path or credentials")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Method == http.MethodPatch {
			var input map[string]string
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil || len(input) != 1 || !validMode(input["mode"]) {
				t.Errorf("unexpected mode update: %+v, error=%v", input, err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			mode.Store(strings.ToUpper(input["mode"]))
			writes.Add(1)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"mode": mode.Load(), "mode-list": []string{"Rule", "Global", "Direct", "custom", "GLOBAL"},
			"secret": "must-not-be-exposed", "mixed-port": 50016,
		})
	}))
	defer controller.Close()
	layout := writeControllerConfig(t, strings.TrimPrefix(controller.URL, "http://"), "controller-only-secret")
	request := func(method, body string) ModeSnapshot {
		t.Helper()
		// Recreating the service proves the mode is not merely kept by the Web app.
		handler := NewHandler(NewService(layout, fixedCoreStatus{state: core.ProcessRunning}))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(method, modePath, strings.NewReader(body)))
		if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "secret") || strings.Contains(response.Body.String(), "mixed-port") {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
		var snapshot ModeSnapshot
		if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(snapshot.AvailableModes, []string{"rule", "global", "direct"}) {
			t.Fatalf("available modes=%v", snapshot.AvailableModes)
		}
		return snapshot
	}
	if snapshot := request(http.MethodGet, ""); snapshot.Mode != "rule" {
		t.Fatalf("initial mode=%s", snapshot.Mode)
	}
	for _, target := range []string{"global", "direct", "rule"} {
		if snapshot := request(http.MethodPut, `{"mode":"`+target+`"}`); snapshot.Mode != target {
			t.Fatalf("confirmed mode=%s, want %s", snapshot.Mode, target)
		}
		if snapshot := request(http.MethodGet, ""); snapshot.Mode != target {
			t.Fatalf("refreshed mode=%s, want %s", snapshot.Mode, target)
		}
	}
	request(http.MethodPut, `{"mode":"rule"}`)
	if writes.Load() != 3 {
		t.Fatalf("mode writes=%d, want 3 (unchanged mode must not be patched)", writes.Load())
	}
}

func TestModeRejectsInvalidUnavailableAndUnconfirmedChanges(t *testing.T) {
	for _, test := range []struct {
		name, body, upstreamBody, code string
		readStatus, patchStatus, want  int
	}{
		{name: "invalid JSON", body: `{"mode":`, want: 400, code: "invalid_json"},
		{name: "unknown field", body: `{"mode":"global","allow-lan":true}`, want: 400, code: "invalid_json"},
		{name: "missing mode", body: `{}`, want: 400, code: "invalid_runtime_mode"},
		{name: "unknown mode", body: `{"mode":"custom"}`, want: 400, code: "invalid_runtime_mode"},
		{name: "malformed controller response", upstreamBody: `{"mode":`, want: 502, code: "clash_api_unavailable"},
		{name: "missing current mode", upstreamBody: `{}`, want: 502, code: "clash_api_unavailable"},
		{name: "unavailable mode", upstreamBody: `{"mode":"rule","mode-list":["rule"]}`, want: 409, code: "runtime_mode_unavailable"},
		{name: "missing available modes", upstreamBody: `{"mode":"rule"}`, want: 409, code: "runtime_mode_unavailable"},
		{name: "controller unavailable", readStatus: 401, want: 502, code: "clash_api_unavailable"},
		{name: "patch rejected", patchStatus: 400, want: 409, code: "runtime_mode_change_failed"},
		{name: "patch failed", patchStatus: 500, want: 502, code: "clash_api_unavailable"},
		{name: "acknowledged but not changed", patchStatus: 204, want: 409, code: "runtime_mode_change_failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var writes atomic.Int32
			controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPatch {
					writes.Add(1)
					if test.patchStatus == 0 {
						w.WriteHeader(500)
					} else {
						w.WriteHeader(test.patchStatus)
					}
					return
				}
				if test.readStatus != 0 {
					w.WriteHeader(test.readStatus)
					return
				}
				body := test.upstreamBody
				if body == "" {
					body = `{"mode":"rule","mode-list":["rule","global","direct"]}`
				}
				_, _ = w.Write([]byte(body))
			}))
			defer controller.Close()
			layout := writeControllerConfig(t, strings.TrimPrefix(controller.URL, "http://"), "")
			handler := NewHandler(NewService(layout, fixedCoreStatus{state: core.ProcessRunning}))
			body := test.body
			if body == "" {
				body = `{"mode":"global"}`
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodPut, modePath, strings.NewReader(body)))
			if response.Code != test.want || !strings.Contains(response.Body.String(), test.code) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if test.patchStatus == 0 && writes.Load() != 0 {
				t.Fatalf("unexpected mode writes=%d", writes.Load())
			}
		})
	}
}

func TestModeRejectsStoppedCoreAndUnsupportedMethods(t *testing.T) {
	layout := writeControllerConfig(t, "127.0.0.1:50017", "")
	client := &recordingClient{}
	handler := NewHandler(NewServiceWithClient(layout, fixedCoreStatus{state: core.ProcessStopped}, client))
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(method, modePath, strings.NewReader(`{"mode":"global"}`)))
		if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "core_not_running") || client.calls != 0 {
			t.Fatalf("%s status=%d body=%s calls=%d", method, response.Code, response.Body.String(), client.calls)
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, modePath, nil))
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != "GET, PUT" {
		t.Fatal("unexpected methods")
	}
}
