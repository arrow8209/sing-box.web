//go:build contract

package contracttest

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"singbox-web/internal/paths"
	runtimeapi "singbox-web/internal/runtime"
)

func TestProxyDelayUsesActualCoreAndIsolatedHTTPSOrigin(t *testing.T) {
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

	// Only the child core trusts this temporary certificate. The synthetic
	// HTTP proxy below tunnels exclusively to a loopback HTTPS origin, so
	// the production test URL never causes a public network request here.
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), DNSNames: []string{"www.gstatic.com"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:        true, BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certificatePath := filepath.Join(workDir, "test-ca.pem")
	if err := os.WriteFile(certificatePath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	var mode atomic.Int32
	var heads atomic.Int32
	origin := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead || r.Host != "www.gstatic.com" || r.URL.Path != "/generate_204" {
			t.Errorf("unexpected probe: %s %s %s", r.Method, r.Host, r.URL)
		}
		heads.Add(1)
		if mode.Load() == 1 {
			select {
			case <-r.Context().Done():
			case <-time.After(6 * time.Second):
			}
		} else {
			time.Sleep(30 * time.Millisecond)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	origin.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}, MinVersion: tls.VersionTLS12}
	origin.StartTLS()
	defer origin.Close()
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect || r.Host != "www.gstatic.com:443" {
			t.Errorf("unexpected proxy target: %s %s", r.Method, r.Host)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if mode.Load() == 2 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		upstream, err := net.DialTimeout("tcp", origin.Listener.Addr().String(), time.Second)
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer upstream.Close()
		client, buffered, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer client.Close()
		_, _ = buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
		_ = buffered.Flush()
		copied := make(chan struct{})
		go func() { _, _ = io.Copy(upstream, buffered); close(copied) }()
		_, _ = io.Copy(client, upstream)
		_ = client.Close()
		_ = upstream.Close()
		<-copied
	}))
	defer proxy.Close()
	_, portString, _ := net.SplitHostPort(proxy.Listener.Addr().String())
	proxyPort, _ := strconv.Atoi(portString)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	controllerAddress := listener.Addr().String()
	_ = listener.Close()
	const nodeTag = "🇯🇵 测试/出口 ?#%"
	config := map[string]any{
		"log": map[string]any{"disabled": true},
		"outbounds": []any{
			map[string]any{"type": "selector", "tag": "测试组", "outbounds": []string{"direct", nodeTag}, "default": "direct"},
			map[string]any{"type": "direct", "tag": "direct"},
			map[string]any{"type": "http", "tag": nodeTag, "server": "127.0.0.1", "server_port": proxyPort},
		},
		"experimental": map[string]any{"clash_api": map[string]any{"external_controller": controllerAddress, "secret": "isolated-delay-secret"}},
	}
	payload, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(layout.ConfigDir, "99-experimental.json"), payload, 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "run", "-C", layout.ConfigDir)
	command.Dir = workDir
	command.Env = append(os.Environ(), "SSL_CERT_FILE="+certificatePath, "SSL_CERT_DIR="+workDir)
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
	var groups []runtimeapi.ProxyGroup
	deadline := time.Now().Add(5 * time.Second)
	for {
		groups, err = service.ProxyGroups(ctx)
		if err == nil && len(groups) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("core not ready: %v", err)
		}
		time.Sleep(30 * time.Millisecond)
	}
	request := runtimeapi.ProxyDelayRequest{Tag: nodeTag, RuntimeKey: groups[0].RuntimeKey}
	result, err := service.TestProxyDelay(ctx, request)
	if err != nil || result.Status != "ok" || result.Delay < 25 || heads.Load() != 1 {
		t.Fatalf("actual delay result=%+v heads=%d err=%v", result, heads.Load(), err)
	}
	groups, err = service.ProxyGroups(ctx)
	if err != nil || len(groups) != 1 || groups[0].Selected != "direct" || groups[0].Members[1].History == nil || groups[0].Members[1].History.Delay != result.Delay {
		t.Fatalf("selection/history after measurement: %+v err=%v", groups, err)
	}
	mode.Store(1)
	result, err = service.TestProxyDelay(ctx, request)
	if err != nil || result.Status != "timeout" || result.Delay != 0 {
		t.Fatalf("actual timeout=%+v err=%v", result, err)
	}
	mode.Store(2)
	result, err = service.TestProxyDelay(ctx, request)
	if err != nil || result.Status != "failed" || result.Delay != 0 {
		t.Fatalf("actual failure=%+v err=%v", result, err)
	}
	groups, err = service.ProxyGroups(ctx)
	if err != nil || len(groups) != 1 || groups[0].Members[1].History != nil {
		t.Fatalf("failure retained success history: %+v err=%v", groups, err)
	}
	t.Logf("sing-box %s: isolated HTTPS HEAD through %s, real delay/history, 5s timeout and proxy connection failure verified", selected.Version, strings.TrimSpace(nodeTag))
}
