package httpapi

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"singbox-web/internal/auth"
	"singbox-web/internal/capabilities"
	"singbox-web/internal/profile"
)

const sessionCookieName = "sb_session"

type AuthService interface {
	Login(username string, password []byte) (auth.Session, error)
	Validate(token string) (auth.Claims, error)
	Refresh(token string) (auth.Session, bool, error)
	ChangePassword(password []byte) error
}

type CapabilityService interface {
	Snapshot(context.Context) capabilities.Snapshot
}

type ProfileService interface {
	Create(context.Context, profile.CreateInput) (profile.Profile, profile.Draft, error)
	List(context.Context) ([]profile.Profile, error)
	Get(context.Context, string) (profile.Profile, error)
	Rename(context.Context, string, string) (profile.Profile, error)
	GetDraft(context.Context, string) (profile.Draft, error)
	SaveDraft(context.Context, string, uint64, profile.DraftContent) (profile.Draft, error)
	DraftStatus(context.Context, string) profile.DraftStatus
	RecordValidation(context.Context, string, uint64, profile.ValidationSummary) (profile.Profile, error)
}

type Dependencies struct {
	Auth         AuthService
	Capabilities CapabilityService
	Profiles     ProfileService
	ConfigRoot   string
	PublicOrigin string
	TrustedProxy func(net.IP) bool
	Now          func() time.Time
}

type server struct {
	deps    Dependencies
	limiter *loginLimiter
}

func New(deps Dependencies) http.Handler {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	service := &server{deps: deps, limiter: newLoginLimiter(1024)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", service.health)
	mux.HandleFunc("POST /api/v1/auth/login", service.login)
	mux.HandleFunc("POST /api/v1/auth/logout", service.logout)
	mux.HandleFunc("GET /api/v1/auth/session", service.session)
	mux.HandleFunc("PUT /api/v1/auth/password", service.changePassword)
	mux.HandleFunc("GET /api/v1/capabilities", service.capabilities)
	mux.HandleFunc("POST /api/v1/profiles", service.createProfile)
	mux.HandleFunc("GET /api/v1/profiles", service.listProfiles)
	mux.HandleFunc("PATCH /api/v1/profiles/{id}", service.renameProfile)
	mux.HandleFunc("GET /api/v1/profiles/{id}/draft", service.getDraft)
	mux.HandleFunc("PUT /api/v1/profiles/{id}/draft", service.saveDraft)
	mux.HandleFunc("/api/v1/profiles/", service.profileRouteNotFound)

	var handler http.Handler = mux
	handler = service.csrfMiddleware(handler)
	handler = service.authenticationMiddleware(handler)
	handler = service.originMiddleware(handler)
	handler = service.trustedProxyMiddleware(handler)
	handler = bodyLimitMiddleware(handler)
	handler = WithSecurityHeaders(handler)
	handler = requestIDMiddleware(handler)
	handler = recoveryMiddleware(handler)
	return handler
}

func (service *server) health(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func (service *server) capabilities(response http.ResponseWriter, request *http.Request) {
	if service.deps.Capabilities == nil {
		writeAPIError(response, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}
	writeJSON(response, http.StatusOK, service.deps.Capabilities.Snapshot(request.Context()))
}

type limiterEntry struct {
	failures     []time.Time
	blockedUntil time.Time
	lastSeen     time.Time
}

type loginLimiter struct {
	mu      sync.Mutex
	entries map[string]limiterEntry
	maximum int
}

func newLoginLimiter(maximum int) *loginLimiter {
	return &loginLimiter{entries: make(map[string]limiterEntry), maximum: maximum}
}

func (limiter *loginLimiter) limited(key string, now time.Time) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	entry, ok := limiter.entries[key]
	if !ok {
		return false
	}
	if !entry.blockedUntil.IsZero() {
		if now.Before(entry.blockedUntil) {
			entry.lastSeen = now
			limiter.entries[key] = entry
			return true
		}
		delete(limiter.entries, key)
		return false
	}
	return false
}

func (limiter *loginLimiter) fail(key string, now time.Time) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	entry := limiter.entries[key]
	cutoff := now.Add(-10 * time.Minute)
	kept := entry.failures[:0]
	for _, failure := range entry.failures {
		if failure.After(cutoff) {
			kept = append(kept, failure)
		}
	}
	entry.failures = append(kept, now)
	entry.lastSeen = now
	if len(entry.failures) >= 5 {
		entry.blockedUntil = now.Add(10 * time.Minute)
	}
	if _, exists := limiter.entries[key]; !exists && len(limiter.entries) >= limiter.maximum {
		limiter.evictOldest()
	}
	limiter.entries[key] = entry
}

func (limiter *loginLimiter) clear(key string) {
	limiter.mu.Lock()
	delete(limiter.entries, key)
	limiter.mu.Unlock()
}

func (limiter *loginLimiter) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	for key, entry := range limiter.entries {
		if oldestKey == "" || entry.lastSeen.Before(oldestTime) {
			oldestKey, oldestTime = key, entry.lastSeen
		}
	}
	if oldestKey != "" {
		delete(limiter.entries, oldestKey)
	}
}
