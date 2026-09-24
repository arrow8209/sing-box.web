// Command harness hosts an isolated visual-test facade. It never reads the
// repository data directory: all temporary state belongs to one MkdirTemp dir.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	"singbox-web/internal/app"
	"singbox-web/internal/auth"
	"singbox-web/internal/paths"
)

func main() {
	harnessAddress := "127.0.0.1:4173"
	if port := os.Getenv("SINGBOX_WEB_E2E_PORT"); port != "" {
		harnessAddress = net.JoinHostPort("127.0.0.1", port)
	}
	workDir, err := os.MkdirTemp("", "singbox-web-e2e-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(workDir)
	if err := installFakeCore(workDir); err != nil {
		panic(err)
	}
	layout, err := paths.Resolve(workDir)
	if err != nil {
		panic(err)
	}
	if err := auth.NewStore(layout.AuthFile, nil).SetPassword([]byte("correct horse battery staple")); err != nil {
		panic(err)
	}
	appListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	appServer, err := app.NewServer(workDir, app.Dependencies{Listener: appListener, RuleHTTPTransport: rulesetFixtureTransport(), EnableServerReferenceResolution: true})
	if err != nil {
		_ = appListener.Close()
		panic(err)
	}
	appContext, cancelApp := context.WithCancel(context.Background())
	appDone := make(chan struct{})
	var appRunErr error
	go func() {
		appRunErr = appServer.Run(appContext)
		close(appDone)
	}()
	defer func() {
		cancelApp()
		<-appDone
		if appRunErr != nil {
			panic(appRunErr)
		}
	}()
	appURL := "http://" + appListener.Addr().String()
	if err := waitFor(appURL+"/api/v1/auth/session", appDone, func() error { return appRunErr }); err != nil {
		panic(err)
	}

	apiTarget, _ := url.Parse(appURL)
	appProxy := httputil.NewSingleHostReverseProxy(apiTarget)
	appProxy.Transport = &http.Transport{Proxy: nil}
	// The browser remains on :4173 while this proxy reaches the isolated app.
	// Preserve the public Host so production same-origin checks see the exact
	// Origin that the browser sent.
	proxyDirector := appProxy.Director
	appProxy.Director = func(request *http.Request) {
		proxyDirector(request)
		request.Host = harnessAddress
	}
	var coreUpdateState atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/core/releases/latest" {
			writeJSON(w, map[string]any{"version": "1.0.0", "arch": "amd64"})
			return
		}
		if r.URL.Path == "/api/v1/core/update" {
			if r.Method == http.MethodPost {
				coreUpdateState.Store(1)
				writeJSON(w, map[string]any{"operationId": "e2e-core-update"})
				return
			}
			if r.Method == http.MethodDelete && coreUpdateState.Load() == 1 {
				coreUpdateState.Store(2)
				writeJSON(w, map[string]any{"operationId": "e2e-core-update"})
				return
			}
			if coreUpdateState.Load() == 1 {
				writeJSON(w, map[string]any{
					"operationId": "e2e-core-update", "targetVersion": "1.0.0", "stage": "downloading",
					"startedAt": "2026-08-27T00:00:00Z", "downloadedBytes": 8 << 20,
					"totalBytes": 32 << 20, "bytesPerSecond": 4 << 20,
				})
				return
			}
			if coreUpdateState.Load() == 2 {
				writeJSON(w, map[string]any{
					"operationId": "e2e-core-update", "targetVersion": "1.0.0", "stage": "cancelled",
					"startedAt": "2026-08-27T00:00:00Z", "finishedAt": "2026-08-27T00:00:01Z",
					"message": "下载已停止", "downloadedBytes": 8 << 20, "totalBytes": 32 << 20,
				})
				return
			}
			writeJSON(w, map[string]any{"operationId": "", "stage": "idle", "startedAt": ""})
			return
		}
		appProxy.ServeHTTP(w, r)
	})
	server := &http.Server{Addr: harnessAddress, Handler: handler}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signals
		cancelApp()
		_ = server.Shutdown(context.Background())
	}()
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

func waitFor(endpoint string, serviceDone <-chan struct{}, serviceErr func() error) error {
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if serviceDone != nil {
			select {
			case <-serviceDone:
				err := serviceErr()
				if err == nil {
					return errors.New("E2E harness service exited before becoming ready")
				}
				return err
			default:
			}
		}
		response, err := (&http.Client{Transport: &http.Transport{Proxy: nil}}).Get(endpoint) // #nosec G107 -- fixed local test endpoint
		if err == nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("E2E harness dependency did not start within 20 seconds")
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func fixturePath(workDir, name string) string { return filepath.Join(workDir, name) }
