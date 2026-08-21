package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLoginAcceptsOnlyPasswordAndReturnsStrictCookie(t *testing.T) {
	handler := newAuthHandler(t, "correct horse battery staple")
	response := postJSON(handler, "/api/v1/auth/login", `{"password":"correct horse battery staple"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies=%d", len(cookies))
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Secure || cookie.MaxAge != 86400 {
		t.Fatalf("cookie=%+v", cookie)
	}
	if !bytes.Contains(response.Body.Bytes(), []byte("csrfToken")) {
		t.Fatalf("missing csrf token: %s", response.Body.String())
	}

	rejected := postJSON(handler, "/api/v1/auth/login", `{"username":"admin","password":"correct horse battery staple"}`)
	if rejected.Code != http.StatusBadRequest {
		t.Fatalf("username status=%d", rejected.Code)
	}
}

func TestWriteRequiresSessionAndMatchingCSRF(t *testing.T) {
	handler := newAuthHandler(t, "correct horse battery staple")
	login := postJSON(handler, "/api/v1/auth/login", `{"password":"correct horse battery staple"}`)
	cookie := login.Result().Cookies()[0]
	csrf := jsonField(t, login.Body.Bytes(), "csrfToken")

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("missing csrf status=%d", response.Code)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	request.AddCookie(cookie)
	request.Header.Set("X-CSRF-Token", csrf)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("logout status=%d", response.Code)
	}
}

func TestLoginRateLimitAndSuccessReset(t *testing.T) {
	handler := newAuthHandler(t, "correct horse battery staple")
	for range 5 {
		response := postJSON(handler, "/api/v1/auth/login", `{"password":"wrong password"}`)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("failure status=%d", response.Code)
		}
	}
	limited := postJSON(handler, "/api/v1/auth/login", `{"password":"wrong password"}`)
	if limited.Code != http.StatusTooManyRequests || limited.Header().Get("Retry-After") == "" {
		t.Fatalf("limited status=%d retry=%q", limited.Code, limited.Header().Get("Retry-After"))
	}
}

func TestConcurrentLoginReservesAtMostFivePasswordChecks(t *testing.T) {
	const attempts = 8
	handler := newAuthHandler(t, "correct horse battery staple")
	enteredVerification := make(chan struct{}, 5)
	releaseVerification := make(chan struct{})
	var verificationCalls atomic.Int32
	handler.verify = func(_ []byte, _ string) (bool, error) {
		verificationCalls.Add(1)
		enteredVerification <- struct{}{}
		<-releaseVerification
		return false, nil
	}

	start := make(chan struct{})
	statuses := make(chan int, attempts)
	var requests sync.WaitGroup
	for range attempts {
		requests.Add(1)
		go func() {
			defer requests.Done()
			<-start
			request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"password":"wrong password"}`))
			request.RemoteAddr = "192.0.2.1:43210"
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			statuses <- response.Code
		}()
	}
	close(start)
	for range 5 {
		select {
		case <-enteredVerification:
		case <-time.After(5 * time.Second):
			t.Fatal("five password checks did not start")
		}
	}
	if got := verificationCalls.Load(); got != 5 {
		t.Fatalf("password checks=%d, want 5", got)
	}

	for range attempts - 5 {
		select {
		case status := <-statuses:
			if status != http.StatusTooManyRequests {
				t.Fatalf("concurrent status=%d, want 429", status)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("sixth or later concurrent request did not receive 429")
		}
	}
	close(releaseVerification)
	requests.Wait()
	close(statuses)
	unauthorized := 0
	for status := range statuses {
		if status != http.StatusUnauthorized {
			t.Fatalf("completed verification status=%d, want 401", status)
		}
		unauthorized++
	}
	if unauthorized != 5 {
		t.Fatalf("completed password failures=%d, want 5", unauthorized)
	}
}

func TestProtectedWriteRejectsMismatchedOriginAndOldSessionAfterPasswordChange(t *testing.T) {
	handler := newAuthHandler(t, "correct horse battery staple")
	login := postJSON(handler, "/api/v1/auth/login", `{"password":"correct horse battery staple"}`)
	cookie := login.Result().Cookies()[0]
	csrf := jsonField(t, login.Body.Bytes(), "csrfToken")

	wrongOrigin := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	wrongOrigin.Host = "console.example.test"
	wrongOrigin.Header.Set("Origin", "https://attacker.example.test")
	wrongOrigin.AddCookie(cookie)
	wrongOrigin.Header.Set("X-CSRF-Token", csrf)
	wrongOriginResponse := httptest.NewRecorder()
	handler.ServeHTTP(wrongOriginResponse, wrongOrigin)
	if wrongOriginResponse.Code != http.StatusForbidden {
		t.Fatalf("origin status=%d", wrongOriginResponse.Code)
	}

	change := httptest.NewRequest(http.MethodPut, "/api/v1/auth/password", bytes.NewBufferString(`{"currentPassword":"correct horse battery staple","newPassword":"new correct horse battery staple"}`))
	change.AddCookie(cookie)
	change.Header.Set("X-CSRF-Token", csrf)
	changeResponse := httptest.NewRecorder()
	handler.ServeHTTP(changeResponse, change)
	if changeResponse.Code != http.StatusNoContent {
		t.Fatalf("password status=%d body=%s", changeResponse.Code, changeResponse.Body.String())
	}

	oldSession := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	oldSession.AddCookie(cookie)
	oldSession.Header.Set("X-CSRF-Token", csrf)
	oldSessionResponse := httptest.NewRecorder()
	handler.ServeHTTP(oldSessionResponse, oldSession)
	if oldSessionResponse.Code != http.StatusUnauthorized {
		t.Fatalf("old session status=%d", oldSessionResponse.Code)
	}
}

func newAuthHandler(t *testing.T, password string) *Handler {
	t.Helper()
	store := NewStore(filepath.Join(t.TempDir(), "auth.json"), nil)
	if err := store.SetPassword([]byte(password)); err != nil {
		t.Fatal(err)
	}
	return NewHandler(store)
}

func postJSON(handler http.Handler, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func jsonField(t *testing.T, body []byte, name string) string {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatal(err)
	}
	result, ok := value[name].(string)
	if !ok || result == "" {
		t.Fatalf("field %s is not a non-empty string in %s", name, body)
	}
	return result
}
