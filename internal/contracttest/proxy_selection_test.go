//go:build contract

package contracttest

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"singbox-web/internal/core"
	"singbox-web/internal/paths"
	runtimeapi "singbox-web/internal/runtime"
)

type runningContractCore struct{}

func (runningContractCore) Status(context.Context) core.Status {
	return core.Status{State: core.ProcessRunning}
}

func TestRuntimeControlsAndMetricsUseActualCore(t *testing.T) {
	binary := os.Getenv("SINGBOX_CONTRACT_BINARY")
	selected := readSelection(t)
	if binary == "" || sha256File(t, binary) != selected.BinarySHA256 {
		t.Fatal("verified SINGBOX_CONTRACT_BINARY is required")
	}
	workDir := t.TempDir()
	layout, err := paths.Resolve(workDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(layout.ConfigDir, 0o700); err != nil {
		t.Fatal(err)
	}
	unusedAddress := func() string {
		t.Helper()
		listener, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		address := listener.Addr().String()
		listener.Close()
		return address
	}
	controllerAddress := unusedAddress()
	inboundAddress := unusedAddress()
	_, portString, _ := net.SplitHostPort(inboundAddress)
	var port int
	if err := json.Unmarshal([]byte(portString), &port); err != nil {
		t.Fatal(err)
	}
	const groupTag = "🚀 节点/选择 #%"
	writeConfig := func(name string, value any) {
		t.Helper()
		payload, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(layout.ConfigDir, name), payload, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeConfig("99-experimental.json", map[string]any{"experimental": map[string]any{
		"clash_api":  map[string]any{"external_controller": controllerAddress, "secret": "contract-controller-only", "default_mode": "rule"},
		"cache_file": map[string]any{"enabled": true, "path": filepath.Join(workDir, "cache.db")},
	}})
	writeConfig("01-routing.json", map[string]any{
		"log":      map[string]any{"disabled": true},
		"inbounds": []any{map[string]any{"type": "mixed", "tag": "test-in", "listen": "127.0.0.1", "listen_port": port}},
		"outbounds": []any{
			map[string]any{"type": "selector", "tag": groupTag, "outbounds": []string{"出口一", "出口二"}, "default": "出口一"},
			map[string]any{"type": "direct", "tag": "出口一", "inet4_bind_address": "127.0.0.1"},
			map[string]any{"type": "direct", "tag": "出口二", "inet4_bind_address": "127.0.0.2"},
			map[string]any{"type": "direct", "tag": "直连出口", "inet4_bind_address": "127.0.0.3"},
			map[string]any{"type": "direct", "tag": "全局出口", "inet4_bind_address": "127.0.0.4"},
		},
		"route": map[string]any{"final": groupTag, "rules": []any{
			map[string]any{"clash_mode": "direct", "action": "route", "outbound": "直连出口"},
			map[string]any{"clash_mode": "global", "action": "route", "outbound": "全局出口"},
		}},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "run", "-C", layout.ConfigDir)
	command.Dir = workDir
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	defer func() {
		_ = command.Process.Signal(os.Interrupt)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = command.Process.Kill()
			<-done
		}
	}()
	service := runtimeapi.NewService(layout, runningContractCore{})
	deadline := time.Now().Add(5 * time.Second)
	for {
		groups, err := service.ProxyGroups(ctx)
		if err == nil && len(groups) == 1 && groups[0].Selected == "出口一" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("core not ready: groups=%+v error=%v", groups, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	metricsStream := make(chan struct{})
	releaseMetricsStream := sync.OnceFunc(func() { close(metricsStream) })
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics-transfer" {
			_, _ = io.Copy(io.Discard, r.Body)
			_, _ = io.WriteString(w, strings.Repeat("d", 64<<10))
			w.(http.Flusher).Flush()
			select {
			case <-metricsStream:
			case <-r.Context().Done():
			}
			return
		}
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		_, _ = io.WriteString(w, host)
	}))
	defer origin.Close()
	defer releaseMetricsStream()
	proxyURL, _ := url.Parse("http://" + inboundAddress)
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	checkSource := func(want string) {
		t.Helper()
		response, err := client.Get(origin.URL)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil || string(body) != want {
			t.Fatalf("actual traffic source=%q, want %s, error=%v", body, want, err)
		}
	}
	checkSource("127.0.0.1")
	result, err := service.SelectProxy(ctx, runtimeapi.ProxySelection{Tag: groupTag, Selected: "出口二"})
	if err != nil || result.Selected != "出口二" {
		t.Fatalf("selection=%+v error=%v", result, err)
	}
	checkSource("127.0.0.2")
	groups, err := runtimeapi.NewService(layout, runningContractCore{}).ProxyGroups(ctx)
	if err != nil || len(groups) != 1 || groups[0].Selected != "出口二" {
		t.Fatalf("refreshed groups=%+v error=%v", groups, err)
	}
	for _, test := range []struct{ mode, source string }{
		{mode: "direct", source: "127.0.0.3"},
		{mode: "global", source: "127.0.0.4"},
		{mode: "rule", source: "127.0.0.2"},
	} {
		result, err := service.SetMode(ctx, runtimeapi.ModeSelection{Mode: test.mode})
		if err != nil || result.Mode != test.mode {
			t.Fatalf("mode=%+v want=%s error=%v", result, test.mode, err)
		}
		checkSource(test.source)
		refreshed, err := runtimeapi.NewService(layout, runningContractCore{}).Mode(ctx)
		if err != nil || refreshed.Mode != test.mode {
			t.Fatalf("refreshed mode=%+v error=%v", refreshed, err)
		}
	}
	before, err := service.Metrics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	transfer, err := client.Post(origin.URL+"/metrics-transfer", "application/octet-stream", strings.NewReader(strings.Repeat("u", 64<<10)))
	if err != nil {
		t.Fatal(err)
	}
	defer transfer.Body.Close()
	if _, err := io.CopyN(io.Discard, transfer.Body, 64<<10); err != nil {
		t.Fatal(err)
	}
	metrics, err := service.Metrics(ctx)
	if err != nil || metrics.Memory <= 0 || metrics.ConnectionCount < 1 || metrics.UploadTotal-before.UploadTotal < 64<<10 || metrics.DownloadTotal-before.DownloadTotal < 64<<10 || !metrics.ObservedAt.After(before.ObservedAt) {
		t.Fatalf("live transfer metrics=%+v before=%+v error=%v", metrics, before, err)
	}
	releaseMetricsStream()
	_ = transfer.Body.Close()
	deadline = time.Now().Add(2 * time.Second)
	for {
		metrics, err = service.Metrics(ctx)
		if err == nil && metrics.ConnectionCount == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("closed connection metrics=%+v error=%v", metrics, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Logf("sing-box %s: node and mode changes routed real traffic; live transfers increased upload/download counters, reported memory and active connections, and closed connections disappeared", selected.Version)
}
