package runtime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"singbox-web/internal/core"
)

func TestProxySelectionReachesCoreAndSurvivesNewService(t *testing.T) {
	const tag = "🚀 策略/组 ?#%"
	const first = "🇭🇰 香港 01"
	const target = "🇯🇵 日本 / 02 [0.3X]"
	selected := first
	var writes int
	controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer controller-only-secret" || r.URL.RawQuery != "" {
			t.Errorf("controller authentication or escaped path is incorrect")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.EscapedPath() != "/proxies" && r.URL.EscapedPath() != "/proxies/"+url.PathEscape(tag) {
			t.Errorf("unexpected path: %s", r.URL.EscapedPath())
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method == http.MethodPut {
			if r.Header.Get("Content-Type") != "application/json" {
				t.Error("missing JSON content type")
			}
			var input struct {
				Name string `json:"name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Name != target {
				t.Errorf("unexpected selection: %q, error: %v", input.Name, err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			selected = input.Name
			writes++
			w.WriteHeader(http.StatusNoContent)
			return
		}
		group := clashProxy{Type: "Selector", Now: selected, All: []string{first, target}}
		if r.URL.Path != "/proxies" {
			_ = json.NewEncoder(w).Encode(group)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"proxies": map[string]any{
			"GLOBAL": clashProxy{Type: "Fallback", Now: tag, All: []string{tag, first, target, "自动选择"}},
			tag:      group,
			first:    map[string]any{"type": "AnyTLS", "secret": "must-not-be-exposed"},
			target:   clashProxy{Type: "Trojan"},
			"自动选择":   clashProxy{Type: "URLTest", Now: target, All: []string{first, target}},
		}})
	}))
	defer controller.Close()
	layout := writeControllerConfig(t, strings.TrimPrefix(controller.URL, "http://"), "controller-only-secret")
	newHandler := func() *Handler { return NewHandler(NewService(layout, fixedCoreStatus{state: core.ProcessRunning})) }
	read := func(handler *Handler) []ProxyGroup {
		t.Helper()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, proxyGroupsPath, nil))
		if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "secret") {
			t.Fatalf("GET status=%d body=%s", response.Code, response.Body.String())
		}
		var groups []ProxyGroup
		if err := json.Unmarshal(response.Body.Bytes(), &groups); err != nil {
			t.Fatal(err)
		}
		if len(groups) != 2 || groups[0].Tag != tag || groups[1].Type != "URLTest" || groups[0].Members[1].Type != "Trojan" {
			t.Fatalf("unexpected groups: %+v", groups)
		}
		return groups
	}
	if read(newHandler())[0].Selected != first {
		t.Fatal("initial selection is incorrect")
	}
	payload, _ := json.Marshal(ProxySelection{Tag: tag, Selected: target})
	response := httptest.NewRecorder()
	newHandler().ServeHTTP(response, httptest.NewRequest(http.MethodPut, proxyGroupsPath, strings.NewReader(string(payload))))
	if response.Code != http.StatusOK || writes != 1 || selected != target {
		t.Fatalf("PUT status=%d writes=%d selected=%q body=%s", response.Code, writes, selected, response.Body.String())
	}
	if read(newHandler())[0].Selected != target {
		t.Fatal("new service lost the core's selection")
	}
}

func TestProxySelectionRejectsInvalidReadOnlyMissingAndUnconfirmedChanges(t *testing.T) {
	for _, test := range []struct {
		name, body, groupType, selected, code string
		upstreamStatus, writeStatus, want     int
	}{
		{name: "invalid JSON", body: `{"tag":`, want: 400, code: "invalid_json"},
		{name: "unknown field", body: `{"tag":"group","selected":"second","secret":"x"}`, want: 400, code: "invalid_json"},
		{name: "empty tag", body: `{"tag":"","selected":"second"}`, want: 400, code: "invalid_proxy_selection"},
		{name: "control character", body: `{"tag":"group\n","selected":"second"}`, want: 400, code: "invalid_proxy_selection"},
		{name: "read only", groupType: "URLTest", want: 409, code: "proxy_group_read_only"},
		{name: "node missing", groupType: "Selector", selected: "missing", want: 400, code: "proxy_node_not_found"},
		{name: "group missing", upstreamStatus: 404, want: 404, code: "proxy_group_not_found"},
		{name: "controller unavailable", upstreamStatus: 401, want: 502, code: "clash_api_unavailable"},
		{name: "write rejected", groupType: "Selector", writeStatus: 400, want: 409, code: "proxy_selection_failed"},
		{name: "write failed", groupType: "Selector", writeStatus: 500, want: 502, code: "clash_api_unavailable"},
		{name: "acknowledged but not changed", groupType: "Selector", writeStatus: 204, want: 409, code: "proxy_selection_failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			writes := 0
			controller := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPut {
					writes++
					w.WriteHeader(test.writeStatus)
					return
				}
				if test.upstreamStatus != 0 {
					w.WriteHeader(test.upstreamStatus)
					return
				}
				_ = json.NewEncoder(w).Encode(clashProxy{Type: test.groupType, Now: "first", All: []string{"first", "second"}})
			}))
			defer controller.Close()
			layout := writeControllerConfig(t, strings.TrimPrefix(controller.URL, "http://"), "")
			handler := NewHandler(NewService(layout, fixedCoreStatus{state: core.ProcessRunning}))
			body := test.body
			if body == "" {
				selected := test.selected
				if selected == "" {
					selected = "second"
				}
				payload, _ := json.Marshal(ProxySelection{Tag: "group", Selected: selected})
				body = string(payload)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodPut, proxyGroupsPath, strings.NewReader(body)))
			if response.Code != test.want || !strings.Contains(response.Body.String(), test.code) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if test.writeStatus == 0 && writes != 0 {
				t.Fatalf("unexpected writes=%d", writes)
			}
		})
	}
}

func TestProxyGroupsRejectStoppedCoreAndUnsupportedMethods(t *testing.T) {
	layout := writeControllerConfig(t, "127.0.0.1:9090", "")
	client := &recordingClient{}
	handler := NewHandler(NewServiceWithClient(layout, fixedCoreStatus{state: core.ProcessStopped}, client))
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(method, proxyGroupsPath, strings.NewReader(`{"tag":"group","selected":"second"}`)))
		if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "core_not_running") || client.calls != 0 {
			t.Fatalf("%s status=%d body=%s calls=%d", method, response.Code, response.Body.String(), client.calls)
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, proxyGroupsPath, nil))
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != "GET, PUT" {
		t.Fatal("unexpected methods")
	}
}
