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
		{WebPort: 50015, DefaultMixedPort: 50016, DefaultAPIPort: 50017, DefaultAPIController: "127.0.0.1:50017"},
		{WebPort: 50022, DefaultMixedPort: 50023, DefaultAPIPort: 50024, DefaultAPIController: "127.0.0.1:50024"},
	} {
		body := fmt.Sprintf(`{"version":1,"proxyUrl":"","autoStart":false,"webPort":%d,"defaultMixedPort":%d,"defaultApiPort":%d}`, ports.WebPort, ports.DefaultMixedPort, ports.DefaultAPIPort)
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
		assertPorts := func(id string, wantMixedPort, wantAPIPort int) {
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
			if len(inbounds) != 1 || inbounds[0].Type != "mixed" || inbounds[0].Port != wantMixedPort {
				t.Fatalf("inbounds=%+v, want mixed port %d", inbounds, wantMixedPort)
			}
			var experimental struct {
				ClashAPI struct {
					ExternalController string `json:"external_controller"`
				} `json:"clash_api"`
			}
			if err := json.Unmarshal(draft.Config["experimental"], &experimental); err != nil {
				t.Fatal(err)
			}
			if want := fmt.Sprintf("127.0.0.1:%d", wantAPIPort); experimental.ClashAPI.ExternalController != want {
				t.Fatalf("API controller=%q, want %q", experimental.ClashAPI.ExternalController, want)
			}
		}
		assertPorts(meta.ID, ports.DefaultMixedPort, ports.DefaultAPIPort)
		if existingID != "" {
			assertPorts(existingID, 50016, 50017)
		} else {
			existingID = meta.ID
		}
		if err := server.Close(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
}
