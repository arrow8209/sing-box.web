package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"singbox-web/internal/httpapi"
)

func TestRouterAuthenticatesAllProfileRequestsAndRequiresCSRFWrites(t *testing.T) {
	profiles := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	authenticated := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Test-Session") != "ok" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	csrf := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Test-CSRF") != "ok" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	router := httpapi.NewRouter(httpapi.Dependencies{ProfileHandler: profiles, Authenticate: authenticated, RequireCSRF: csrf})

	for _, test := range []struct {
		name   string
		method string
		sess   string
		csrf   string
		want   int
	}{
		{name: "read requires session", method: http.MethodGet, want: http.StatusUnauthorized},
		{name: "read with session", method: http.MethodGet, sess: "ok", want: http.StatusNoContent},
		{name: "write requires csrf", method: http.MethodPost, sess: "ok", want: http.StatusForbidden},
		{name: "write with csrf", method: http.MethodPost, sess: "ok", csrf: "ok", want: http.StatusNoContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, "/api/v1/profiles", nil)
			request.Header.Set("X-Test-Session", test.sess)
			request.Header.Set("X-Test-CSRF", test.csrf)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status=%d want=%d", response.Code, test.want)
			}
		})
	}
}
