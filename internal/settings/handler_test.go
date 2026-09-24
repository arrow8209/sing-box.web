package settings

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerReadsAndUpdatesProxySettings(t *testing.T) {
	path := t.TempDir() + "/settings.json"
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(store)
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/v1/settings/proxy", nil))
	if get.Code != http.StatusOK || strings.TrimSpace(get.Body.String()) != `{"proxyUrl":""}` {
		t.Fatalf("get=%d %s", get.Code, get.Body.String())
	}
	put := httptest.NewRecorder()
	handler.ServeHTTP(put, httptest.NewRequest(http.MethodPut, "/api/v1/settings/proxy", strings.NewReader(`{"proxyUrl":"http://127.0.0.1:7890"}`)))
	if put.Code != http.StatusOK || strings.TrimSpace(put.Body.String()) != `{"proxyUrl":"http://127.0.0.1:7890"}` {
		t.Fatalf("put=%d %s", put.Code, put.Body.String())
	}
	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	value, err := reopened.Get()
	if err != nil || value.ProxyURL != "http://127.0.0.1:7890" {
		t.Fatalf("value=%+v err=%v", value, err)
	}
}

func TestHandlerReadsAndUpdatesCoreSettings(t *testing.T) {
	path := t.TempDir() + "/settings.json"
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(store)
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/v1/settings/core", nil))
	if get.Code != http.StatusOK || strings.TrimSpace(get.Body.String()) != `{"autoStart":true,"autoReload":false}` {
		t.Fatalf("get=%d %s", get.Code, get.Body.String())
	}
	put := httptest.NewRecorder()
	handler.ServeHTTP(put, httptest.NewRequest(http.MethodPut, "/api/v1/settings/core", strings.NewReader(`{"autoStart":false,"autoReload":true}`)))
	if put.Code != http.StatusOK || strings.TrimSpace(put.Body.String()) != `{"autoStart":false,"autoReload":true}` {
		t.Fatalf("put=%d %s", put.Code, put.Body.String())
	}
	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	value, err := reopened.GetCore()
	if err != nil || value.AutoStart || !value.AutoReload {
		t.Fatalf("value=%+v err=%v", value, err)
	}
}

func TestServerSettingsAreReadOnly(t *testing.T) {
	store, err := OpenStore(t.TempDir() + "/settings.json")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(store)
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(method, "/api/v1/settings/server", nil))
		if method != http.MethodGet {
			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("%s status=%d", method, response.Code)
			}
			continue
		}
		var ports ServerSettings
		if err := json.Unmarshal(response.Body.Bytes(), &ports); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusOK || ports.WebPort != 50022 || ports.DefaultMixedPort != 50023 || ports.DefaultAPIPort != 50024 || ports.DefaultAPIController != "127.0.0.1:50024" {
			t.Fatalf("get=%d ports=%+v", response.Code, ports)
		}
	}
}

func TestHandlerReportsInvalidProxyURLField(t *testing.T) {
	store, err := OpenStore(t.TempDir() + "/settings.json")
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	NewHandler(store).ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/v1/settings/proxy", strings.NewReader(`{"proxyUrl":"socks5://127.0.0.1:1080"}`)))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Error struct {
			Code      string `json:"code"`
			FieldPath string `json:"fieldPath"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "invalid_proxy_url" || body.Error.FieldPath != "proxyUrl" {
		t.Fatalf("body=%+v", body)
	}
}
