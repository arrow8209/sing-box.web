package httpapi

import (
	"net/http"
	"strings"

	"singbox-web/internal/apierror"
)

// Middleware wraps a request handler. It is deliberately structural so the
// router can consume auth middleware without importing the auth package.
type Middleware func(http.Handler) http.Handler

// Dependencies supplies independently constructed endpoints. Keeping auth and
// profile concrete types out of this package prevents an import cycle while
// still making the top-level route composition explicit.
type Dependencies struct {
	ProfileHandler    http.Handler
	Authenticate      Middleware
	RequireCSRF       Middleware
	AuthHandler       http.Handler
	CapabilityHandler http.Handler
	WebUI             http.Handler
}

// NewRouter exposes API routes and hands all non-API GET/HEAD requests to the
// embedded SPA. Unknown API paths always receive a JSON error, never HTML.
func NewRouter(deps Dependencies) http.Handler {
	profiles := deps.ProfileHandler
	if profiles == nil || deps.Authenticate == nil || deps.RequireCSRF == nil {
		profiles = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			apierror.Write(w, http.StatusInternalServerError, apierror.Error{Code: "router_unavailable", Message: "API 路由不可用"})
		})
	} else {
		profiles = csrfForWrites(profiles, deps.RequireCSRF)
		profiles = deps.Authenticate(profiles)
	}
	return SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/profiles" || strings.HasPrefix(r.URL.Path, "/api/v1/profiles/"):
			profiles.ServeHTTP(w, r)
		case strings.HasPrefix(r.URL.Path, "/api/v1/auth/") && deps.AuthHandler != nil:
			deps.AuthHandler.ServeHTTP(w, r)
		case r.URL.Path == "/api/v1/capabilities" && deps.CapabilityHandler != nil:
			deps.CapabilityHandler.ServeHTTP(w, r)
		case strings.HasPrefix(r.URL.Path, "/api/"):
			apierror.Write(w, http.StatusNotFound, apierror.Error{Code: "not_found", Message: "请求的资源不存在"})
		case deps.WebUI != nil:
			deps.WebUI.ServeHTTP(w, r)
		default:
			http.Error(w, "前端资源不可用", http.StatusInternalServerError)
		}
	}))
}

func csrfForWrites(next http.Handler, requireCSRF Middleware) http.Handler {
	protected := requireCSRF(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			protected.ServeHTTP(w, r)
		default:
			next.ServeHTTP(w, r)
		}
	})
}
