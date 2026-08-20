package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"io"
	"net"
	"net/http"
	"strings"

	"singbox-web/internal/auth"
)

const (
	maxBodyBytes          = 2_000_000
	contentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'"
)

type contextKey uint8

const (
	claimsContextKey contextKey = iota
	clientIPContextKey
	requestIDContextKey
)

func WithSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Security-Policy", contentSecurityPolicy)
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("Referrer-Policy", "no-referrer")
		if request.URL.Path == "/api/v1" || strings.HasPrefix(request.URL.Path, "/api/v1/") {
			response.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(response, request)
	})
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		defer func() {
			if recover() != nil {
				writeAPIError(response, http.StatusInternalServerError, "internal_error", "服务器内部错误")
			}
		}()
		next.ServeHTTP(response, request)
	})
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		identifier := make([]byte, 16)
		if _, err := io.ReadFull(rand.Reader, identifier); err != nil {
			identifier = []byte("request-id-unavailable")
		}
		requestID := hex.EncodeToString(identifier)
		response.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(request.Context(), requestIDContextKey, requestID)
		next.ServeHTTP(response, request.WithContext(ctx))
	})
}

func bodyLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.ContentLength > maxBodyBytes {
			writeAPIError(response, http.StatusRequestEntityTooLarge, "request_too_large", "请求体过大")
			return
		}
		if request.Body == nil {
			next.ServeHTTP(response, request)
			return
		}
		body, err := io.ReadAll(io.LimitReader(request.Body, maxBodyBytes+1))
		_ = request.Body.Close()
		if err != nil {
			writeAPIError(response, http.StatusBadRequest, "invalid_request", "请求无效")
			return
		}
		if len(body) > maxBodyBytes {
			writeAPIError(response, http.StatusRequestEntityTooLarge, "request_too_large", "请求体过大")
			return
		}
		request.Body = io.NopCloser(bytes.NewReader(body))
		request.ContentLength = int64(len(body))
		next.ServeHTTP(response, request)
	})
}

func (service *server) trustedProxyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		peer := remoteIP(request.RemoteAddr)
		trusted := peer != nil && peer.IsLoopback()
		if !trusted && peer != nil && service.deps.TrustedProxy != nil {
			trusted = service.deps.TrustedProxy(peer)
		}
		client := peer
		if trusted {
			if forwarded := firstForwardedIP(request.Header.Values("X-Forwarded-For")); forwarded != nil {
				client = forwarded
			}
		} else {
			for name := range request.Header {
				lowerName := strings.ToLower(name)
				if lowerName == "forwarded" || strings.HasPrefix(lowerName, "x-forwarded-") {
					delete(request.Header, name)
				}
			}
		}
		ctx := context.WithValue(request.Context(), clientIPContextKey, client)
		next.ServeHTTP(response, request.WithContext(ctx))
	})
}

func remoteIP(address string) net.IP {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}
	return net.ParseIP(strings.Trim(host, "[]"))
}

func firstForwardedIP(values []string) net.IP {
	if len(values) != 1 {
		return nil
	}
	first := strings.TrimSpace(strings.Split(values[0], ",")[0])
	return net.ParseIP(strings.Trim(first, "[]"))
}

func isUnsafeAPIRequest(request *http.Request) bool {
	if !(request.URL.Path == "/api/v1" || strings.HasPrefix(request.URL.Path, "/api/v1/")) {
		return false
	}
	return request.Method != http.MethodGet && request.Method != http.MethodHead && request.Method != http.MethodOptions
}

func (service *server) authenticationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if !isProtectedAPI(request.URL.Path) {
			next.ServeHTTP(response, request)
			return
		}
		if service.deps.Auth == nil {
			writeAPIError(response, http.StatusUnauthorized, "unauthorized", "未认证")
			return
		}
		cookies := request.CookiesNamed(sessionCookieName)
		if len(cookies) != 1 || cookies[0].Value == "" {
			writeAPIError(response, http.StatusUnauthorized, "unauthorized", "未认证")
			return
		}
		claims, err := service.deps.Auth.Validate(cookies[0].Value)
		if err != nil {
			writeAPIError(response, http.StatusUnauthorized, "unauthorized", "未认证")
			return
		}
		refreshed, renewed, err := service.deps.Auth.Refresh(cookies[0].Value)
		if err != nil {
			writeAPIError(response, http.StatusUnauthorized, "unauthorized", "未认证")
			return
		}
		if renewed {
			setSessionCookie(response, refreshed)
			claims.CSRFToken = refreshed.CSRFToken
			claims.ExpiresAt = refreshed.ExpiresAt.UnixNano()
		}
		ctx := context.WithValue(request.Context(), claimsContextKey, claims)
		next.ServeHTTP(response, request.WithContext(ctx))
	})
}

func isProtectedAPI(path string) bool {
	if !(path == "/api/v1" || strings.HasPrefix(path, "/api/v1/")) {
		return false
	}
	return path != "/api/v1/auth/login"
}

func (service *server) csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if isProtectedAPI(request.URL.Path) && isUnsafeAPIRequest(request) {
			claims, ok := claimsFromContext(request.Context())
			token := request.Header.Get("X-CSRF-Token")
			if !ok || token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(claims.CSRFToken)) != 1 {
				writeAPIError(response, http.StatusForbidden, "invalid_csrf", "CSRF 令牌无效")
				return
			}
		}
		next.ServeHTTP(response, request)
	})
}

func claimsFromContext(ctx context.Context) (auth.Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey).(auth.Claims)
	return claims, ok
}

func normalizedClientIP(ctx context.Context) string {
	ip, _ := ctx.Value(clientIPContextKey).(net.IP)
	if ip == nil {
		return "unknown"
	}
	return ip.String()
}
