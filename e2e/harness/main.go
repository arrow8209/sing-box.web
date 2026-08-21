// Command harness hosts an isolated visual-test facade. It never reads the
// repository data directory: all temporary state belongs to one MkdirTemp dir.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

const visualSessionCookie = "singbox_web_e2e_session"
const visualCSRFToken = "visual-csrf-token"

func main() {
	workDir, err := os.MkdirTemp("", "singbox-web-e2e-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(workDir)
	if err := installFakeCore(workDir); err != nil {
		panic(err)
	}

	vite := exec.Command("npm", "--prefix", "web", "run", "dev", "--", "--port", "5173")
	vite.Stdout, vite.Stderr = os.Stdout, os.Stderr
	if err := vite.Start(); err != nil {
		panic(err)
	}
	defer func() { _ = vite.Process.Signal(syscall.SIGTERM); _, _ = vite.Process.Wait() }()
	if err := waitForVite(); err != nil {
		panic(err)
	}

	target, _ := url.Parse("http://127.0.0.1:5173")
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{Proxy: nil}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/session" {
			if hasVisualSession(r) {
				writeJSON(w, map[string]any{"valid": true, "expiresAt": "2099-01-01T00:00:00Z", "csrfToken": visualCSRFToken})
			} else {
				writeJSON(w, map[string]any{"valid": false, "expiresAt": nil})
			}
			return
		}
		if r.URL.Path == "/api/v1/auth/login" {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: visualSessionCookie, Value: "1", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
			writeJSON(w, map[string]any{"csrfToken": visualCSRFToken})
			return
		}
		if r.URL.Path == "/api/v1/profiles" {
			writeJSON(w, []any{})
			return
		}
		if r.URL.Path == "/api/v1/rule-resources" {
			writeJSON(w, []any{})
			return
		}
		if r.URL.Path == "/api/v1/core" {
			writeJSON(w, map[string]any{"state": "not_installed", "operation": map[string]any{"active": false, "stage": "idle"}})
			return
		}
		proxy.ServeHTTP(w, r)
	})
	server := &http.Server{Addr: "127.0.0.1:4173", Handler: handler}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() { <-signals; _ = server.Shutdown(context.Background()) }()
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

func hasVisualSession(r *http.Request) bool {
	cookie, err := r.Cookie(visualSessionCookie)
	return err == nil && cookie.Value == "1"
}

func waitForVite() error {
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		response, err := (&http.Client{Transport: &http.Transport{Proxy: nil}}).Get("http://127.0.0.1:5173") // #nosec G107 -- fixed local test endpoint
		if err == nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("Vite visual harness did not start within 20 seconds")
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func fixturePath(workDir, name string) string { return filepath.Join(workDir, name) }
