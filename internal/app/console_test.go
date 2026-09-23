package app

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"singbox-web/internal/auth"
	"singbox-web/internal/core"
	"singbox-web/internal/paths"
	"singbox-web/internal/storage"
)

type consoleBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *consoleBuffer) Write(payload []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(payload)
}

func (b *consoleBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

type readyListener struct {
	net.Listener
	ready chan struct{}
	once  sync.Once
}

func (l *readyListener) Accept() (net.Conn, error) {
	l.once.Do(func() { close(l.ready) })
	return l.Listener.Accept()
}

func consoleTestLayout(t *testing.T) paths.Layout {
	t.Helper()
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.NewStore(layout.AuthFile, nil).SetPassword([]byte("console test password")); err != nil {
		t.Fatal(err)
	}
	return layout
}

func TestServerPrintsBoundAddressAndConfiguredMixedPort(t *testing.T) {
	for _, mixedPort := range []int{50016, 50019} {
		t.Run(fmt.Sprint(mixedPort), func(t *testing.T) {
			layout := consoleTestLayout(t)
			settings := fmt.Sprintf(`{"version":1,"proxyUrl":"http://proxy.example.test:7890","autoStart":false,"webPort":%d,"defaultMixedPort":%d}`, mixedPort-1, mixedPort)
			if err := storage.WriteFile(layout.SettingsFile, []byte(settings), 0o600); err != nil {
				t.Fatal(err)
			}
			bound, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer bound.Close()
			listener := &readyListener{Listener: bound, ready: make(chan struct{})}
			var output consoleBuffer
			server, err := NewServer(layout.WorkDir, Dependencies{Output: &output, Listener: listener})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = server.Close(context.Background()) })
			if strings.Contains(output.String(), "服务已启动") {
				t.Fatal("reported listening before Run")
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			finished := make(chan error, 1)
			go func() { finished <- server.Run(ctx) }()
			select {
			case <-listener.ready:
			case <-time.After(5 * time.Second):
				t.Fatal("server did not start serving")
			}
			transport := &http.Transport{Proxy: nil}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
			response, err := client.Get("http://" + bound.Addr().String() + "/api/v1/settings/server")
			if err != nil {
				t.Fatal(err)
			}
			_ = response.Body.Close()
			if response.StatusCode != http.StatusUnauthorized {
				t.Fatalf("HTTP status=%d", response.StatusCode)
			}
			cancel()
			select {
			case err := <-finished:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("server did not stop")
			}
			for _, expected := range []string{
				"[Web] 服务已启动", "监听地址：http://" + bound.Addr().String(),
				"本机访问：http://" + bound.Addr().String(), "工作目录：" + layout.WorkDir,
				"配置目录：" + layout.ConfigDir, fmt.Sprintf("默认 Mixed 入站端口：%d", mixedPort),
				"核心日志：" + layout.CoreLog, "核心状态：未安装", "服务已停止",
			} {
				if !strings.Contains(output.String(), expected) {
					t.Fatalf("output missing %q: %s", expected, output.String())
				}
			}
			if strings.Contains(output.String(), "proxy.example.test") || strings.Contains(output.String(), "console test password") {
				t.Fatal("startup output included settings beyond the runtime summary")
			}
		})
	}
}

func TestServerDoesNotReportStartupWhenListenFails(t *testing.T) {
	layout := consoleTestLayout(t)
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	var output consoleBuffer
	server, err := NewServer(layout.WorkDir, Dependencies{Output: &output, ListenAddress: occupied.Addr().String()})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close(context.Background())
	if err := server.Run(context.Background()); err == nil {
		t.Fatal("expected bind failure")
	}
	if strings.Contains(output.String(), "服务已启动") || strings.Contains(output.String(), "监听地址：") {
		t.Fatalf("reported startup after bind failure: %s", output.String())
	}
}

func TestServerMirrorsCoreStdoutAndStderrAndFlushesOnShutdown(t *testing.T) {
	layout := consoleTestLayout(t)
	program := `#!/bin/sh
if [ "$1" = version ]; then
  printf 'sing-box version 1.0.0\n'
  exit 0
fi
printf 'core stdout\n'
printf 'core stderr\n' >&2
trap 'printf "core shutdown tail"; exit 0' TERM INT
while :; do sleep 1; done
`
	if err := storage.WriteFile(layout.CoreBinary, []byte(program), 0o700); err != nil {
		t.Fatal(err)
	}
	var output consoleBuffer
	server, err := NewServer(layout.WorkDir, Dependencies{Output: &output})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.core.Start(ctx, core.StartRequest{}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"[核心] core stdout\n", "[核心] core stderr\n"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("missing live core output %q: %s", expected, output.String())
		}
	}
	if err := server.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "[核心] core shutdown tail\n") {
		t.Fatalf("partial shutdown line was lost: %s", output.String())
	}
	data, err := os.ReadFile(layout.CoreLog)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"core stdout\n", "core stderr\n", "core shutdown tail\n"} {
		if !strings.Contains(string(data), expected) {
			t.Fatalf("missing retained core output %q: %q", expected, data)
		}
	}
}
