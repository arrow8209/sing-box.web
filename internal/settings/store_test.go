package settings

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"singbox-web/internal/storage"
)

func TestStorePersistsNormalizedProxyURL(t *testing.T) {
	path := t.TempDir() + "/settings.json"
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := store.Get()
	if err != nil || initial.ProxyURL != "" {
		t.Fatalf("initial=%+v err=%v", initial, err)
	}
	coreSettings, err := store.GetCore()
	if err != nil || !coreSettings.AutoStart || coreSettings.AutoReload {
		t.Fatalf("coreSettings=%+v err=%v", coreSettings, err)
	}
	updatedCore, err := store.UpdateCore(CoreSettings{AutoStart: false, AutoReload: true})
	if err != nil || updatedCore.AutoStart || !updatedCore.AutoReload {
		t.Fatalf("updatedCore=%+v err=%v", updatedCore, err)
	}
	updated, err := store.UpdateProxy("  HTTPS://proxy.example.test:8443/  ")
	if err != nil {
		t.Fatal(err)
	}
	if updated.ProxyURL != "https://proxy.example.test:8443" {
		t.Fatalf("proxyUrl=%q", updated.ProxyURL)
	}
	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := reopened.Get()
	if err != nil || persisted != updated {
		t.Fatalf("persisted=%+v updated=%+v err=%v", persisted, updated, err)
	}
	persistedCore, err := reopened.GetCore()
	if err != nil || persistedCore != updatedCore {
		t.Fatalf("persistedCore=%+v updatedCore=%+v err=%v", persistedCore, updatedCore, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
}

func TestOpenStoreKeepsLegacyCoreBehaviorDefaults(t *testing.T) {
	path := t.TempDir() + "/settings.json"
	if err := storage.WriteFile(path, []byte(`{"version":1,"proxyUrl":""}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	value, err := store.GetCore()
	if err != nil || !value.AutoStart || value.AutoReload {
		t.Fatalf("value=%+v err=%v", value, err)
	}
	ports, err := store.GetServer()
	if err != nil || ports.WebPort != 50022 || ports.DefaultMixedPort != 50023 || ports.DefaultAPIPort != 50024 || ports.DefaultAPIController != "127.0.0.1:50024" {
		t.Fatalf("ports=%+v err=%v", ports, err)
	}
}

func TestServerPortsSurviveOtherSettingsUpdates(t *testing.T) {
	path := t.TempDir() + "/settings.json"
	if err := storage.WriteFile(path, []byte(`{"version":1,"proxyUrl":"","webPort":50015,"defaultMixedPort":50016,"defaultApiPort":50017}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateProxy("http://proxy.example.test:8080"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateCore(CoreSettings{AutoStart: false, AutoReload: true}); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	ports, err := reopened.GetServer()
	if err != nil || ports.WebPort != 50015 || ports.DefaultMixedPort != 50016 || ports.DefaultAPIPort != 50017 || ports.DefaultAPIController != "127.0.0.1:50017" {
		t.Fatalf("ports=%+v err=%v", ports, err)
	}
}

func TestOpenStoreRejectsInvalidServerPorts(t *testing.T) {
	for _, field := range []string{"webPort", "defaultMixedPort", "defaultApiPort"} {
		for _, value := range []string{"0", "-1", "65536", "50018.5", `"50018"`, "true"} {
			t.Run(field+"/"+value, func(t *testing.T) {
				path := t.TempDir() + "/settings.json"
				body := fmt.Sprintf(`{"version":1,"proxyUrl":"",%q:%s}`, field, value)
				if err := storage.WriteFile(path, []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
				if _, err := OpenStore(path); err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), field) {
					t.Fatalf("body=%s err=%v", body, err)
				}
			})
		}
	}
	path := t.TempDir() + "/settings.json"
	if err := storage.WriteFile(path, []byte(`{"version":1,"proxyUrl":"","webPort":50019,"defaultMixedPort":50019}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenStore(path); err == nil || !strings.Contains(err.Error(), "不能相同") {
		t.Fatalf("same ports err=%v", err)
	}
	if err := storage.WriteFile(path, []byte(`{"version":1,"proxyUrl":"","webPort":50018,"defaultMixedPort":50019,"defaultApiPort":50019}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenStore(path); err == nil || !strings.Contains(err.Error(), "不能相同") {
		t.Fatalf("same API and Mixed ports err=%v", err)
	}
}

func TestStoreRejectsInvalidProxyURLsWithoutReplacingCurrentValue(t *testing.T) {
	path := t.TempDir() + "/settings.json"
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateProxy("http://proxy.example.test:8080"); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{
		"proxy.example.test:8080",
		"ftp://proxy.example.test:21",
		"http://",
		"http://user:password@proxy.example.test:8080",
		"http://proxy.example.test:8080/path",
		"http://proxy.example.test:8080?mode=fast",
		"http://proxy.example.test:99999",
	} {
		if _, err := store.UpdateProxy(value); !errors.Is(err, ErrInvalidProxyURL) {
			t.Fatalf("value=%q err=%v", value, err)
		}
	}
	current, err := store.Get()
	if err != nil || current.ProxyURL != "http://proxy.example.test:8080" {
		t.Fatalf("current=%+v err=%v", current, err)
	}
}

func TestOpenStoreRejectsUnknownOrNonCanonicalManagedState(t *testing.T) {
	for _, body := range []string{
		`{"version":1,"proxyUrl":"","extra":true}`,
		`{"version":2,"proxyUrl":""}`,
		`{"version":1,"proxyUrl":"HTTP://proxy.example.test/"}`,
	} {
		path := t.TempDir() + "/settings.json"
		if err := storage.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := OpenStore(path); err == nil || !strings.Contains(err.Error(), path) {
			t.Fatalf("body=%s err=%v", body, err)
		}
	}
}

func TestDownloadTransportAppliesProxyDynamicallyToHTTPAndHTTPS(t *testing.T) {
	store, err := OpenStore(t.TempDir() + "/settings.json")
	if err != nil {
		t.Fatal(err)
	}
	var proxyRequests atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyRequests.Add(1)
		if !r.URL.IsAbs() || r.URL.Host != "download.example.test" {
			t.Errorf("proxy request URL=%s", r.URL)
		}
		_, _ = io.WriteString(w, "proxied")
	}))
	defer proxy.Close()
	if _, err := store.UpdateProxy(proxy.URL); err != nil {
		t.Fatal(err)
	}
	transport := NewDownloadTransport(store)
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	response, err := client.Get("http://download.example.test/file")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil || string(payload) != "proxied" || proxyRequests.Load() != 1 {
		t.Fatalf("payload=%q proxyRequests=%d err=%v", payload, proxyRequests.Load(), err)
	}
	for _, endpoint := range []string{"http://download.example.test/file", "https://download.example.test/file"} {
		request, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			t.Fatal(err)
		}
		selected, err := transport.Proxy(request)
		if err != nil || selected == nil || selected.String() != proxy.URL {
			t.Fatalf("endpoint=%s selected=%v err=%v", endpoint, selected, err)
		}
	}
	if _, err := store.UpdateProxy(""); err != nil {
		t.Fatal(err)
	}
	direct := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "direct")
	}))
	defer direct.Close()
	response, err = client.Get(direct.URL)
	if err != nil {
		t.Fatal(err)
	}
	payload, err = io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil || string(payload) != "direct" || proxyRequests.Load() != 1 {
		t.Fatalf("payload=%q proxyRequests=%d err=%v", payload, proxyRequests.Load(), err)
	}
}
