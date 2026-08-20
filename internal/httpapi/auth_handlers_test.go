package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"singbox-web/internal/auth"
)

type clearingLimiterAuth struct{ now time.Time }

func (service clearingLimiterAuth) Login(_ string, password []byte) (auth.Session, error) {
	if string(password) != "correct" {
		return auth.Session{}, errors.New("invalid credentials")
	}
	return auth.Session{Token: "token", CSRFToken: "csrf", ExpiresAt: service.now.Add(24 * time.Hour)}, nil
}

func (clearingLimiterAuth) Validate(string) (auth.Claims, error) {
	return auth.Claims{}, errors.New("unused")
}

func (clearingLimiterAuth) Refresh(string) (auth.Session, bool, error) {
	return auth.Session{}, false, errors.New("unused")
}

func (clearingLimiterAuth) ChangePassword([]byte) error { return errors.New("unused") }

type authHarness struct {
	handler http.Handler
	now     *time.Time
}

func newAuthHarness(t *testing.T) authHarness {
	t.Helper()
	current := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	random := bytes.NewReader(bytes.Repeat([]byte{41}, 16384))
	store := auth.NewFileStore(filepath.Join(t.TempDir(), "auth.json"), func() time.Time { return current }, random)
	if err := store.SetPassword([]byte("a-strong-password")); err != nil {
		t.Fatal(err)
	}
	manager := auth.NewSessionManager(store, func() time.Time { return current }, random)
	return authHarness{
		handler: New(Dependencies{Auth: manager, Now: func() time.Time { return current }}),
		now:     &current,
	}
}

func performJSON(handler http.Handler, method, path, body string, cookie *http.Cookie, csrf string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		request.AddCookie(cookie)
	}
	if csrf != "" {
		request.Header.Set("X-CSRF-Token", csrf)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func login(t *testing.T, harness authHarness) (*http.Cookie, sessionResponse) {
	t.Helper()
	response := performJSON(harness.handler, http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":"a-strong-password"}`, nil, "")
	if response.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%q", response.Code, response.Body.String())
	}
	var session sessionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies=%v", cookies)
	}
	cookie := cookies[0]
	if cookie.Name != sessionCookieName || cookie.Path != "/" || cookie.Domain != "" || !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || !cookie.Expires.Equal(session.ExpiresAt) || session.Username != auth.Username || session.CSRFToken == "" {
		t.Fatalf("cookie=%#v session=%#v", cookie, session)
	}
	return cookie, session
}

func TestLoginDoesNotRequireOrigin(t *testing.T) {
	harness := newAuthHarness(t)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"admin","password":"a-strong-password"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	harness.handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("login without Origin status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestLoginRefreshAndLogoutCookies(t *testing.T) {
	harness := newAuthHarness(t)
	cookie, session := login(t, harness)
	*harness.now = harness.now.Add(14 * time.Minute)
	unchanged := performJSON(harness.handler, http.MethodGet, "/api/v1/auth/session", "", cookie, "")
	if unchanged.Code != http.StatusOK || len(unchanged.Result().Cookies()) != 0 {
		t.Fatalf("14m status=%d cookies=%v", unchanged.Code, unchanged.Result().Cookies())
	}
	*harness.now = harness.now.Add(2 * time.Minute)
	refreshed := performJSON(harness.handler, http.MethodGet, "/api/v1/auth/session", "", cookie, "")
	if refreshed.Code != http.StatusOK || len(refreshed.Result().Cookies()) != 1 {
		t.Fatalf("16m status=%d cookies=%v body=%q", refreshed.Code, refreshed.Result().Cookies(), refreshed.Body.String())
	}
	var refreshedSession sessionResponse
	if err := json.Unmarshal(refreshed.Body.Bytes(), &refreshedSession); err != nil {
		t.Fatal(err)
	}
	refreshedCookie := refreshed.Result().Cookies()[0]
	if refreshedSession.CSRFToken != session.CSRFToken || !refreshedCookie.Expires.Equal(harness.now.Add(24*time.Hour)) {
		t.Fatalf("session=%#v cookie=%#v", refreshedSession, refreshedCookie)
	}
	logout := performJSON(harness.handler, http.MethodPost, "/api/v1/auth/logout", "", refreshedCookie, session.CSRFToken)
	if logout.Code != http.StatusNoContent || len(logout.Result().Cookies()) != 1 {
		t.Fatalf("logout status=%d cookies=%v", logout.Code, logout.Result().Cookies())
	}
	expired := logout.Result().Cookies()[0]
	if expired.Name != sessionCookieName || expired.Path != "/" || expired.Domain != "" || !expired.Secure || !expired.HttpOnly || expired.SameSite != http.SameSiteStrictMode || expired.MaxAge >= 0 || !expired.Expires.Before(*harness.now) {
		t.Fatalf("expired cookie=%#v", expired)
	}
}

func TestPasswordChangeExpiresCookieAndRevokesOldToken(t *testing.T) {
	harness := newAuthHarness(t)
	cookie, session := login(t, harness)
	changed := performJSON(harness.handler, http.MethodPut, "/api/v1/auth/password", `{"password":"a-new-strong-password"}`, cookie, session.CSRFToken)
	if changed.Code != http.StatusNoContent || len(changed.Result().Cookies()) != 1 || changed.Result().Cookies()[0].MaxAge >= 0 {
		t.Fatalf("change status=%d cookies=%v body=%q", changed.Code, changed.Result().Cookies(), changed.Body.String())
	}
	old := performJSON(harness.handler, http.MethodGet, "/api/v1/auth/session", "", cookie, "")
	if old.Code != http.StatusUnauthorized {
		t.Fatalf("old token status=%d body=%q", old.Code, old.Body.String())
	}
}

func TestInvalidCredentialResponsesAreIndistinguishable(t *testing.T) {
	harness := newAuthHarness(t)
	badUser := performJSON(harness.handler, http.MethodPost, "/api/v1/auth/login", `{"username":"other","password":"a-strong-password"}`, nil, "")
	badPassword := performJSON(harness.handler, http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":"wrong-password"}`, nil, "")
	if badUser.Code != http.StatusUnauthorized || badPassword.Code != http.StatusUnauthorized || badUser.Body.Len() != badPassword.Body.Len() || badUser.Body.String() != badPassword.Body.String() || !strings.Contains(badUser.Body.String(), `"code":"invalid_credentials"`) {
		t.Fatalf("bad user=%d %q bad password=%d %q", badUser.Code, badUser.Body.String(), badPassword.Code, badPassword.Body.String())
	}
}

func TestCSRFAndLoginRateLimit(t *testing.T) {
	harness := newAuthHarness(t)
	cookie, _ := login(t, harness)
	missingCSRF := performJSON(harness.handler, http.MethodPost, "/api/v1/auth/logout", "", cookie, "")
	if missingCSRF.Code != http.StatusForbidden {
		t.Fatalf("missing csrf status=%d", missingCSRF.Code)
	}
	for index := 0; index < 5; index++ {
		response := performJSON(harness.handler, http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":"wrong-password"}`, nil, "")
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status=%d", index+1, response.Code)
		}
	}
	limited := performJSON(harness.handler, http.MethodPost, "/api/v1/auth/login", `{"username":"someone-else","password":"wrong-password"}`, nil, "")
	if limited.Code != http.StatusTooManyRequests || !strings.Contains(limited.Body.String(), `"code":"rate_limited"`) {
		t.Fatalf("limited status=%d body=%q", limited.Code, limited.Body.String())
	}
	*harness.now = harness.now.Add(10*time.Minute + time.Nanosecond)
	cleared := performJSON(harness.handler, http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":"a-strong-password"}`, nil, "")
	if cleared.Code != http.StatusOK {
		t.Fatalf("after limit status=%d body=%q", cleared.Code, cleared.Body.String())
	}
}

func TestSuccessfulLoginClearsPreviousFailures(t *testing.T) {
	current := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	handler := New(Dependencies{
		Auth: clearingLimiterAuth{now: current},
		Now:  func() time.Time { return current },
	})
	for index := 0; index < 4; index++ {
		response := performJSON(handler, http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":"wrong"}`, nil, "")
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("initial failure %d status=%d", index+1, response.Code)
		}
	}
	success := performJSON(handler, http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":"correct"}`, nil, "")
	if success.Code != http.StatusOK {
		t.Fatalf("success status=%d body=%q", success.Code, success.Body.String())
	}
	for index := 0; index < 5; index++ {
		response := performJSON(handler, http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":"wrong"}`, nil, "")
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("post-success failure %d status=%d", index+1, response.Code)
		}
	}
	limited := performJSON(handler, http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":"wrong"}`, nil, "")
	if limited.Code != http.StatusTooManyRequests {
		t.Fatalf("post-success limit status=%d body=%q", limited.Code, limited.Body.String())
	}
}
