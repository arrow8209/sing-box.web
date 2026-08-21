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
	ProfileHandler      http.Handler
	RuleResourceHandler http.Handler
	ConfigHandler       http.Handler
	ApplicationHandler  http.Handler
	CoreHandler         http.Handler
	UpdaterHandler      http.Handler
	Authenticate        Middleware
	RequireCSRF         Middleware
	AuthHandler         http.Handler
	CapabilityHandler   http.Handler
	WebUI               http.Handler
}

// NewRouter exposes API routes and hands all non-API GET/HEAD requests to the
// embedded SPA. Unknown API paths always receive a JSON error, never HTML.
func NewRouter(deps Dependencies) http.Handler {
	profiles := deps.ProfileHandler
	rules := deps.RuleResourceHandler
	configuration := deps.ConfigHandler
	application := deps.ApplicationHandler
	core := deps.CoreHandler
	updater := deps.UpdaterHandler
	if profiles == nil || deps.Authenticate == nil || deps.RequireCSRF == nil {
		profiles = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			apierror.Write(w, http.StatusInternalServerError, apierror.Error{Code: "router_unavailable", Message: "API 路由不可用"})
		})
	} else {
		profiles = csrfForWrites(profiles, deps.RequireCSRF)
		profiles = deps.Authenticate(profiles)
	}
	if configuration == nil || deps.Authenticate == nil || deps.RequireCSRF == nil {
		configuration = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			apierror.Write(w, http.StatusInternalServerError, apierror.Error{Code: "router_unavailable", Message: "API 路由不可用"})
		})
	} else {
		configuration = csrfForWrites(configuration, deps.RequireCSRF)
		configuration = deps.Authenticate(configuration)
	}
	if application == nil || deps.Authenticate == nil || deps.RequireCSRF == nil {
		application = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			apierror.Write(w, http.StatusInternalServerError, apierror.Error{Code: "router_unavailable", Message: "API 路由不可用"})
		})
	} else {
		application = csrfForWrites(application, deps.RequireCSRF)
		application = deps.Authenticate(application)
	}
	if rules == nil || deps.Authenticate == nil || deps.RequireCSRF == nil {
		rules = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			apierror.Write(w, http.StatusInternalServerError, apierror.Error{Code: "router_unavailable", Message: "API 路由不可用"})
		})
	} else {
		rules = csrfForWrites(rules, deps.RequireCSRF)
		rules = deps.Authenticate(rules)
	}
	if core == nil || deps.Authenticate == nil {
		core = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			apierror.Write(w, http.StatusInternalServerError, apierror.Error{Code: "router_unavailable", Message: "API 路由不可用"})
		})
	} else {
		core = deps.Authenticate(core)
	}
	if updater == nil || deps.Authenticate == nil || deps.RequireCSRF == nil {
		updater = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			apierror.Write(w, http.StatusInternalServerError, apierror.Error{Code: "router_unavailable", Message: "API 路由不可用"})
		})
	} else {
		updater = csrfForWrites(updater, deps.RequireCSRF)
		updater = deps.Authenticate(updater)
	}
	return SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/profiles" || strings.HasPrefix(r.URL.Path, "/api/v1/profiles/"):
			if strings.HasSuffix(r.URL.Path, "/apply") {
				application.ServeHTTP(w, r)
			} else if strings.HasSuffix(r.URL.Path, "/preview") || strings.HasSuffix(r.URL.Path, "/validate") {
				configuration.ServeHTTP(w, r)
			} else {
				profiles.ServeHTTP(w, r)
			}
		case r.URL.Path == "/api/v1/rule-resources" || strings.HasPrefix(r.URL.Path, "/api/v1/rule-resources/"):
			rules.ServeHTTP(w, r)
		case strings.HasPrefix(r.URL.Path, "/api/v1/auth/") && deps.AuthHandler != nil:
			deps.AuthHandler.ServeHTTP(w, r)
		case r.URL.Path == "/api/v1/capabilities" && deps.CapabilityHandler != nil:
			deps.CapabilityHandler.ServeHTTP(w, r)
		case r.URL.Path == "/api/v1/core":
			core.ServeHTTP(w, r)
		case r.URL.Path == "/api/v1/core/releases/latest" || r.URL.Path == "/api/v1/core/update":
			updater.ServeHTTP(w, r)
		case r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/"):
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
