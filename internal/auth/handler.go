package auth

import (
	"context"
	"crypto/subtle"
	"net"
	"net/http"
	"strconv"
	"time"

	"singbox-web/internal/apierror"
	"singbox-web/internal/httpapi"
)

const authRequestLimit = 16 << 10

type loginRequest struct {
	Password string `json:"password"`
}

type passwordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// Handler serves only the four authentication endpoints. The top-level API
// router may mount it directly or register its routes with Register.
type Handler struct {
	store    *Store
	sessions *SessionManager
	limiter  *LoginLimiter
	now      func() time.Time
	verify   func(password []byte, encoded string) (bool, error)
}

func NewHandler(store *Store) *Handler {
	return NewHandlerWithDependencies(store, NewSessionManager(nil), NewLoginLimiter(5, 10*time.Minute), time.Now)
}

// NewHandlerWithDependencies exists for deterministic callers and tests.
func NewHandlerWithDependencies(store *Store, sessions *SessionManager, limiter *LoginLimiter, clock func() time.Time) *Handler {
	if sessions == nil {
		sessions = NewSessionManager(nil)
	}
	if limiter == nil {
		limiter = NewLoginLimiter(5, 10*time.Minute)
	}
	if clock == nil {
		clock = time.Now
	}
	return &Handler{store: store, sessions: sessions, limiter: limiter, now: clock, verify: Verify}
}

// Register registers all authentication endpoints on a standard ServeMux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/auth/login", h)
	mux.Handle("POST /api/v1/auth/logout", h)
	mux.Handle("GET /api/v1/auth/session", h)
	mux.Handle("PUT /api/v1/auth/password", h)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/login":
		h.login(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/logout":
		h.Middleware(RequireCSRF(http.HandlerFunc(h.logout))).ServeHTTP(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/auth/session":
		h.session(w, r)
	case r.Method == http.MethodPut && r.URL.Path == "/api/v1/auth/password":
		h.Middleware(RequireCSRF(http.HandlerFunc(h.changePassword))).ServeHTTP(w, r)
	case r.URL.Path == "/api/v1/auth/login":
		httpapi.MethodNotAllowed(w, http.MethodPost)
	case r.URL.Path == "/api/v1/auth/logout":
		httpapi.MethodNotAllowed(w, http.MethodPost)
	case r.URL.Path == "/api/v1/auth/session":
		httpapi.MethodNotAllowed(w, http.MethodGet)
	case r.URL.Path == "/api/v1/auth/password":
		httpapi.MethodNotAllowed(w, http.MethodPut)
	default:
		apierror.Write(w, http.StatusNotFound, apierror.Error{Code: "not_found", Message: "请求的资源不存在"})
	}
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	if !httpapi.IsSameOrigin(r) {
		forbidden(w)
		return
	}
	now := h.now().UTC()
	source := sourceIP(r)
	var input loginRequest
	if err := httpapi.DecodeJSON(w, r, &input, authRequestLimit); err != nil {
		apierror.WriteError(w, err)
		return
	}
	state, err := h.loadState()
	if err != nil {
		apierror.WriteError(w, err)
		return
	}
	reservation, retry, limited := h.limiter.Reserve(source, now)
	if limited {
		w.Header().Set("Retry-After", strconv.FormatInt(int64(retry.Round(time.Second)/time.Second), 10))
		apierror.Write(w, http.StatusTooManyRequests, apierror.Error{Code: "login_rate_limited", Message: "登录尝试过于频繁"})
		return
	}
	valid, err := h.verify([]byte(input.Password), state.PasswordHash)
	if err != nil || !valid {
		h.limiter.Complete(reservation, false)
		invalidCredentials(w)
		return
	}
	token, csrf, err := h.sessions.Issue(state, now)
	if err != nil {
		h.limiter.Cancel(reservation)
		apierror.WriteError(w, err)
		return
	}
	h.limiter.Complete(reservation, true)
	http.SetCookie(w, Cookie(token, now))
	httpapi.WriteJSON(w, http.StatusOK, struct {
		CSRFToken string `json:"csrfToken"`
	}{CSRFToken: csrf})
}

func (h *Handler) logout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, ExpiredCookie())
	httpapi.WriteJSON(w, http.StatusNoContent, nil)
}

func (h *Handler) session(w http.ResponseWriter, r *http.Request) {
	claims, csrf, state, ok := h.authenticate(r, h.now().UTC())
	if !ok {
		httpapi.WriteJSON(w, http.StatusOK, struct {
			Valid     bool       `json:"valid"`
			ExpiresAt *time.Time `json:"expiresAt"`
		}{Valid: false, ExpiresAt: nil})
		return
	}
	now := h.now().UTC()
	if h.sessions.ShouldRefresh(claims, now) {
		token, refreshedCSRF, err := h.sessions.Refresh(state, claims, now)
		if err == nil {
			http.SetCookie(w, Cookie(token, now))
			csrf = refreshedCSRF
			claims.LastSeenAt = now.Unix()
			claims.ExpiresAt = now.Add(sessionLifetime).Unix()
		}
	}
	expiresAt := time.Unix(claims.ExpiresAt, 0).UTC()
	httpapi.WriteJSON(w, http.StatusOK, struct {
		Valid     bool      `json:"valid"`
		ExpiresAt time.Time `json:"expiresAt"`
		CSRFToken string    `json:"csrfToken"`
	}{Valid: true, ExpiresAt: expiresAt, CSRFToken: csrf})
}

func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	var input passwordRequest
	if err := httpapi.DecodeJSON(w, r, &input, authRequestLimit); err != nil {
		apierror.WriteError(w, err)
		return
	}
	state, err := h.loadState()
	if err != nil {
		apierror.WriteError(w, err)
		return
	}
	valid, err := Verify([]byte(input.CurrentPassword), state.PasswordHash)
	if err != nil || !valid {
		invalidCredentials(w)
		return
	}
	if err := validatePassword([]byte(input.NewPassword)); err != nil {
		apierror.Write(w, http.StatusBadRequest, apierror.Error{Code: "invalid_password", Message: "新密码不符合要求"})
		return
	}
	if err := h.store.SetPassword([]byte(input.NewPassword)); err != nil {
		apierror.WriteError(w, err)
		return
	}
	http.SetCookie(w, ExpiredCookie())
	httpapi.WriteJSON(w, http.StatusNoContent, nil)
}

func (h *Handler) loadState() (State, error) {
	if h.store == nil {
		return State{}, apierror.New(http.StatusInternalServerError, "auth_unavailable", "认证服务不可用", nil)
	}
	state, err := h.store.Load()
	if err != nil {
		return State{}, apierror.New(http.StatusInternalServerError, "auth_unavailable", "认证服务不可用", err)
	}
	return state, nil
}

// Middleware requires a valid session cookie and adds verified claims to the
// request context. It performs throttled sliding expiration renewal.
func (h *Handler) Middleware(next http.Handler) http.Handler {
	return Middleware(h.store, h.sessions, h.now, next)
}

// Middleware is the reusable authentication middleware for non-auth APIs.
func Middleware(store *Store, sessions *SessionManager, clock func() time.Time, next http.Handler) http.Handler {
	if sessions == nil {
		sessions = NewSessionManager(nil)
	}
	if clock == nil {
		clock = time.Now
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			unauthorized(w)
			return
		}
		state, err := store.Load()
		if err != nil {
			unauthorized(w)
			return
		}
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil {
			unauthorized(w)
			return
		}
		now := clock().UTC()
		claims, csrf, ok := sessions.Validate(cookie.Value, state, now)
		if !ok {
			unauthorized(w)
			return
		}
		if sessions.ShouldRefresh(claims, now) {
			token, _, err := sessions.Refresh(state, claims, now)
			if err == nil {
				http.SetCookie(w, Cookie(token, now))
			}
		}
		ctx := context.WithValue(r.Context(), sessionContextKey{}, sessionContext{claims: claims, csrf: csrf})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireCSRF rejects cross-origin writes and missing or mismatched CSRF
// tokens. It must be used behind Middleware.
func RequireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !httpapi.IsSameOrigin(r) {
			forbidden(w)
			return
		}
		_, csrf, ok := sessionFromContext(r.Context())
		if !ok || csrf == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(csrf)) != 1 {
			forbidden(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) authenticate(r *http.Request, now time.Time) (Claims, string, State, bool) {
	state, err := h.loadState()
	if err != nil {
		return Claims{}, "", State{}, false
	}
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return Claims{}, "", State{}, false
	}
	claims, csrf, ok := h.sessions.Validate(cookie.Value, state, now)
	return claims, csrf, state, ok
}

func sourceIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}
	return "unknown"
}

func invalidCredentials(w http.ResponseWriter) {
	apierror.Write(w, http.StatusUnauthorized, apierror.Error{Code: "invalid_credentials", Message: "密码无效"})
}

func unauthorized(w http.ResponseWriter) {
	apierror.Write(w, http.StatusUnauthorized, apierror.Error{Code: "authentication_required", Message: "需要有效会话"})
}

func forbidden(w http.ResponseWriter) {
	apierror.Write(w, http.StatusForbidden, apierror.Error{Code: "csrf_forbidden", Message: "请求来源或 CSRF Token 无效"})
}
