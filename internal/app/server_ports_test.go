package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"singbox-web/internal/auth"
	"singbox-web/internal/paths"
	"singbox-web/internal/profile"
	"singbox-web/internal/settings"
	"singbox-web/internal/storage"
)

func TestConfiguredPortsReachServerAndNewProfilesWithoutRewritingExistingProfiles(t *testing.T) {
	workDir := t.TempDir()
	layout, err := paths.Resolve(workDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.NewStore(layout.AuthFile, nil).SetPassword([]byte("correct horse battery staple")); err != nil {
		t.Fatal(err)
	}
	var existingID string
	for _, ports := range []settings.ServerSettings{
		{WebPort: 50018, DefaultMixedPort: 50019},
		{WebPort: 50015, DefaultMixedPort: 50016},
	} {
		body := fmt.Sprintf(`{"version":1,"proxyUrl":"","autoStart":false,"webPort":%d,"defaultMixedPort":%d}`, ports.WebPort, ports.DefaultMixedPort)
		if err := storage.WriteFile(layout.SettingsFile, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		server, err := NewServer(workDir, Dependencies{})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = server.Close(context.Background()) })
		if server.http.Addr != fmt.Sprintf("0.0.0.0:%d", ports.WebPort) {
			t.Fatalf("listen=%s", server.http.Addr)
		}
		unauthorized := httptest.NewRecorder()
		server.http.Handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/settings/server", nil))
		if unauthorized.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated settings status=%d", unauthorized.Code)
		}
		login := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"password":"correct horse battery staple"}`))
		request.Header.Set("Content-Type", "application/json")
		server.http.Handler.ServeHTTP(login, request)
		if login.Code != http.StatusOK {
			t.Fatalf("login=%d %s", login.Code, login.Body.String())
		}
		var session struct {
			CSRFToken string `json:"csrfToken"`
		}
		if err := json.Unmarshal(login.Body.Bytes(), &session); err != nil {
			t.Fatal(err)
		}
		send := func(method, path, body string) *httptest.ResponseRecorder {
			t.Helper()
			request := httptest.NewRequest(method, path, strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-CSRF-Token", session.CSRFToken)
			for _, cookie := range login.Result().Cookies() {
				request.AddCookie(cookie)
			}
			response := httptest.NewRecorder()
			server.http.Handler.ServeHTTP(response, request)
			if response.Code < 200 || response.Code >= 300 {
				t.Fatalf("%s %s: %d %s", method, path, response.Code, response.Body.String())
			}
			return response
		}
		response := send(http.MethodGet, "/api/v1/settings/server", "")
		var returned settings.ServerSettings
		if err := json.Unmarshal(response.Body.Bytes(), &returned); err != nil || returned != ports {
			t.Fatalf("ports=%+v err=%v", returned, err)
		}
		created := send(http.MethodPost, "/api/v1/profiles", `{"name":"端口验证"}`)
		var meta profile.Meta
		if err := json.Unmarshal(created.Body.Bytes(), &meta); err != nil {
			t.Fatal(err)
		}
		assertPort := func(id string, want int) {
			t.Helper()
			response := send(http.MethodGet, "/api/v1/profiles/"+id+"/draft", "")
			var draft profile.Draft
			if err := json.Unmarshal(response.Body.Bytes(), &draft); err != nil {
				t.Fatal(err)
			}
			var inbounds []struct {
				Type string `json:"type"`
				Port int    `json:"listen_port"`
			}
			if err := json.Unmarshal(draft.Config["inbounds"], &inbounds); err != nil {
				t.Fatal(err)
			}
			if len(inbounds) != 1 || inbounds[0].Type != "mixed" || inbounds[0].Port != want {
				t.Fatalf("inbounds=%+v, want mixed port %d", inbounds, want)
			}
		}
		assertPort(meta.ID, ports.DefaultMixedPort)
		if existingID != "" {
			assertPort(existingID, 50019)
		} else {
			existingID = meta.ID
		}
		if err := server.Close(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
}
