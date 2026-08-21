package httpapi

import (
	"net/http"

	"singbox-web/internal/apierror"
)

// Middleware wraps a request handler. It is deliberately structural so the
// router can consume auth middleware without importing the auth package.
type Middleware func(http.Handler) http.Handler

// Dependencies supplies the profile endpoint and the protections it requires.
// Additional application composition belongs to the later server task.
type Dependencies struct {
	ProfileHandler http.Handler
	Authenticate   Middleware
	RequireCSRF    Middleware
}

// NewRouter exposes the foundational profile API. Every profile request is
// authenticated; unsafe requests are additionally protected by CSRF.
func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	profiles := deps.ProfileHandler
	if profiles == nil || deps.Authenticate == nil || deps.RequireCSRF == nil {
		profiles = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			apierror.Write(w, http.StatusInternalServerError, apierror.Error{Code: "router_unavailable", Message: "API 路由不可用"})
		})
	} else {
		profiles = csrfForWrites(profiles, deps.RequireCSRF)
		profiles = deps.Authenticate(profiles)
	}
	mux.Handle("/api/v1/profiles", profiles)
	mux.Handle("/api/v1/profiles/", profiles)
	return mux
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
