package httpapi

import (
	"bytes"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const exactCSP = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'"

func TestSecurityHeadersAreExactAndIdempotent(t *testing.T) {
	inner := http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) { response.WriteHeader(http.StatusNoContent) })
	handler := WithSecurityHeaders(WithSecurityHeaders(inner))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/test", nil))
	tests := map[string]string{
		"Content-Security-Policy": exactCSP,
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "no-referrer",
		"Cache-Control":           "no-store",
	}
	for name, want := range tests {
		if got := response.Header().Values(name); len(got) != 1 || got[0] != want {
			t.Errorf("%s = %#v", name, got)
		}
	}
}

func TestEveryResponseGetsSecurityHeaders(t *testing.T) {
	handler := New(Dependencies{})
	for _, path := range []string{"/healthz", "/missing", "/api/v1/missing"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Header().Get("Content-Security-Policy") != exactCSP || response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("Referrer-Policy") != "no-referrer" {
			t.Errorf("%s headers=%v", path, response.Header())
		}
		if strings.HasPrefix(path, "/api/v1/") && response.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s cache=%q", path, response.Header().Get("Cache-Control"))
		}
	}
}

func TestOversizedBodyReturns413BeforeAuthenticationOrJSON(t *testing.T) {
	handler := New(Dependencies{})
	request := httptest.NewRequest(http.MethodPut, "/api/v1/auth/password", bytes.NewReader(bytes.Repeat([]byte("x"), maxBodyBytes+1)))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestPanicRecoveryKeepsSecurityHeadersAndSafeBody(t *testing.T) {
	handler := recoveryMiddleware(requestIDMiddleware(WithSecurityHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("secret")
	}))))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/test", nil))
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "secret") || response.Header().Get("Content-Security-Policy") != exactCSP || response.Header().Get("X-Request-ID") == "" {
		t.Fatalf("status=%d body=%q headers=%v", response.Code, response.Body.String(), response.Header())
	}
}

func TestForwardedAddressIsTrustedOnlyFromLoopbackOrConfiguredProxy(t *testing.T) {
	tests := []struct {
		name        string
		remote      string
		trusted     bool
		wantClient  string
		wantForward string
	}{
		{name: "untrusted", remote: "198.51.100.7:1234", wantClient: "198.51.100.7"},
		{name: "loopback", remote: "127.0.0.1:1234", wantClient: "203.0.113.9", wantForward: "203.0.113.9"},
		{name: "configured", remote: "198.51.100.7:1234", trusted: true, wantClient: "203.0.113.9", wantForward: "203.0.113.9"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &server{deps: Dependencies{TrustedProxy: func(_ net.IP) bool { return test.trusted }}}
			var gotClient, gotForward string
			handler := service.trustedProxyMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
				gotClient = normalizedClientIP(request.Context())
				gotForward = request.Header.Get("X-Forwarded-For")
			}))
			request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			request.RemoteAddr = test.remote
			request.Header.Set("X-Forwarded-For", "203.0.113.9")
			handler.ServeHTTP(httptest.NewRecorder(), request)
			if gotClient != test.wantClient || gotForward != test.wantForward {
				t.Fatalf("client=%q forwarded=%q", gotClient, gotForward)
			}
		})
	}
}

func TestMiddlewareOrderBodyOriginAuthenticationCSRF(t *testing.T) {
	service := &server{deps: Dependencies{PublicOrigin: "https://admin.example"}}
	var handler http.Handler = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	handler = service.csrfMiddleware(handler)
	handler = service.authenticationMiddleware(handler)
	handler = service.originMiddleware(handler)
	handler = service.trustedProxyMiddleware(handler)
	handler = bodyLimitMiddleware(handler)
	handler = WithSecurityHeaders(handler)
	handler = requestIDMiddleware(handler)
	handler = recoveryMiddleware(handler)

	oversized := httptest.NewRequest(http.MethodPut, "/api/v1/auth/password", bytes.NewReader(bytes.Repeat([]byte("x"), maxBodyBytes+1)))
	oversized.Header.Set("Origin", "https://evil.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, oversized)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized foreign unauthenticated status=%d", response.Code)
	}

	foreign := httptest.NewRequest(http.MethodPut, "/api/v1/auth/password", strings.NewReader("{}"))
	foreign.Header.Set("Origin", "https://evil.example")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, foreign)
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "forbidden_origin") {
		t.Fatalf("foreign unauthenticated status=%d body=%q", response.Code, response.Body.String())
	}

	unauthenticated := httptest.NewRequest(http.MethodPut, "/api/v1/auth/password", strings.NewReader("{}"))
	unauthenticated.Header.Set("Origin", "https://admin.example")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, unauthenticated)
	if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), "unauthorized") {
		t.Fatalf("unauthenticated status=%d body=%q", response.Code, response.Body.String())
	}
}
