package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"singbox-web/internal/capabilities"
	"singbox-web/internal/httpapi"
	"singbox-web/internal/webui"
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

func TestRouterAuthenticatesSubscriptionRequestsAndRequiresCSRFWrites(t *testing.T) {
	subscriptions := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
	router := httpapi.NewRouter(httpapi.Dependencies{SubscriptionHandler: subscriptions, Authenticate: authenticated, RequireCSRF: csrf})
	for _, test := range []struct {
		method  string
		session string
		csrf    string
		want    int
	}{
		{method: http.MethodGet, want: http.StatusUnauthorized},
		{method: http.MethodGet, session: "ok", want: http.StatusNoContent},
		{method: http.MethodPost, session: "ok", want: http.StatusForbidden},
		{method: http.MethodPost, session: "ok", csrf: "ok", want: http.StatusNoContent},
	} {
		request := httptest.NewRequest(test.method, "/api/v1/subscriptions", nil)
		request.Header.Set("X-Test-Session", test.session)
		request.Header.Set("X-Test-CSRF", test.csrf)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != test.want {
			t.Fatalf("method=%s status=%d want=%d", test.method, response.Code, test.want)
		}
	}
}

func TestRouterAuthenticatesInboundResourceRequestsAndRequiresCSRFWrites(t *testing.T) {
	inbounds := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
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
	router := httpapi.NewRouter(httpapi.Dependencies{InboundHandler: inbounds, Authenticate: authenticated, RequireCSRF: csrf})
	for _, test := range []struct {
		method, session, csrf string
		want                  int
	}{
		{method: http.MethodGet, want: http.StatusUnauthorized},
		{method: http.MethodGet, session: "ok", want: http.StatusNoContent},
		{method: http.MethodPost, session: "ok", want: http.StatusForbidden},
		{method: http.MethodPost, session: "ok", csrf: "ok", want: http.StatusNoContent},
	} {
		request := httptest.NewRequest(test.method, "/api/v1/inbound-resources", nil)
		request.Header.Set("X-Test-Session", test.session)
		request.Header.Set("X-Test-CSRF", test.csrf)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != test.want {
			t.Fatalf("method=%s status=%d want=%d", test.method, response.Code, test.want)
		}
	}
}

func TestRouterKeepsAPIsOutOfSPAFallback(t *testing.T) {
	router := httpapi.NewRouter(httpapi.Dependencies{
		CapabilityHandler: capabilities.NewHandler(capabilities.PhaseOne),
		WebUI:             webui.Handler(),
	})
	spa := httptest.NewRecorder()
	router.ServeHTTP(spa, httptest.NewRequest(http.MethodGet, "/profiles/abc/config", nil))
	if spa.Code != http.StatusOK || !strings.Contains(spa.Body.String(), "id=\"app\"") {
		t.Fatalf("spa=%d %s", spa.Code, spa.Body.String())
	}
	for _, path := range []string{"/api", "/api/", "/api/v1/missing"} {
		api := httptest.NewRecorder()
		router.ServeHTTP(api, httptest.NewRequest(http.MethodGet, path, nil))
		if api.Code != http.StatusNotFound || strings.Contains(api.Body.String(), "<html") {
			t.Fatalf("path=%s api=%d %s", path, api.Code, api.Body.String())
		}
		if api.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("path=%s api cache=%q", path, api.Header().Get("Cache-Control"))
		}
	}
}

func TestRouterAuthenticatesCoreControlAndRequiresCSRFWrites(t *testing.T) {
	authenticated := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Test-Session") != "ok" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	core := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost && r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	csrf := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Test-CSRF") != "ok" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	router := httpapi.NewRouter(httpapi.Dependencies{CoreHandler: core, Authenticate: authenticated, RequireCSRF: csrf})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/core", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", response.Code)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/core", nil)
	request.Header.Set("X-Test-Session", "ok")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d", response.Code)
	}
	request = httptest.NewRequest(http.MethodDelete, "/api/v1/core", nil)
	request.Header.Set("X-Test-Session", "ok")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d", response.Code)
	}
	request.Header.Set("X-Test-CSRF", "ok")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d", response.Code)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/v1/core", nil)
	request.Header.Set("X-Test-Session", "ok")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d", response.Code)
	}
	request.Header.Set("X-Test-CSRF", "ok")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d", response.Code)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/v1/core/restart", nil)
	request.Header.Set("X-Test-Session", "ok")
	request.Header.Set("X-Test-CSRF", "ok")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d", response.Code)
	}
}

func TestRouterAuthenticatesRuntimeAndRequiresCSRFWrites(t *testing.T) {
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
	runtimeData := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	router := httpapi.NewRouter(httpapi.Dependencies{RuntimeHandler: runtimeData, Authenticate: authenticated, RequireCSRF: csrf})

	for _, test := range []struct {
		method, session, csrf string
		want                  int
	}{
		{method: http.MethodGet, want: http.StatusUnauthorized},
		{method: http.MethodGet, session: "ok", want: http.StatusNoContent},
		{method: http.MethodDelete, session: "ok", want: http.StatusForbidden},
		{method: http.MethodDelete, session: "ok", csrf: "ok", want: http.StatusNoContent},
		{method: http.MethodPut, want: http.StatusUnauthorized},
		{method: http.MethodPut, session: "ok", want: http.StatusForbidden},
		{method: http.MethodPut, session: "ok", csrf: "ok", want: http.StatusNoContent},
	} {
		for _, path := range []string{"/api/v1/runtime/connections/connection-1", "/api/v1/runtime/proxy-groups"} {
			request := httptest.NewRequest(test.method, path, nil)
			request.Header.Set("X-Test-Session", test.session)
			request.Header.Set("X-Test-CSRF", test.csrf)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("path=%s method=%s status=%d want=%d", path, test.method, response.Code, test.want)
			}
		}
	}
}

func TestRouterAuthenticatesSettingsAndRequiresCSRFWrites(t *testing.T) {
	settings := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
	router := httpapi.NewRouter(httpapi.Dependencies{SettingsHandler: settings, Authenticate: authenticated, RequireCSRF: csrf})
	for _, path := range []string{"/api/v1/settings/proxy", "/api/v1/settings/core"} {
		for _, test := range []struct {
			method  string
			session string
			csrf    string
			want    int
		}{
			{method: http.MethodGet, want: http.StatusUnauthorized},
			{method: http.MethodGet, session: "ok", want: http.StatusNoContent},
			{method: http.MethodPut, session: "ok", want: http.StatusForbidden},
			{method: http.MethodPut, session: "ok", csrf: "ok", want: http.StatusNoContent},
		} {
			request := httptest.NewRequest(test.method, path, nil)
			request.Header.Set("X-Test-Session", test.session)
			request.Header.Set("X-Test-CSRF", test.csrf)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("path=%s method=%s status=%d want=%d", path, test.method, response.Code, test.want)
			}
		}
	}
}
