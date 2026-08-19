package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"singbox-web/internal/audit"
	"singbox-web/internal/auth"
)

type sessionResponse struct {
	Username  string    `json:"username"`
	CSRFToken string    `json:"csrfToken"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type passwordRequest struct {
	Password string `json:"password"`
}

func (service *server) login(response http.ResponseWriter, request *http.Request) {
	result := "invalid_credentials"
	defer func() { service.recordAudit(request.Context(), audit.Event{Operation: "auth.login", Result: result}) }()
	if service.deps.Auth == nil {
		writeAPIError(response, http.StatusUnauthorized, "invalid_credentials", "用户名或密码错误")
		return
	}
	key := normalizedClientIP(request.Context()) + "\x00" + auth.Username
	now := service.deps.Now()
	if service.limiter.limited(key, now) {
		result = "rate_limited"
		writeAPIError(response, http.StatusTooManyRequests, "rate_limited", "登录尝试过多，请稍后再试")
		return
	}
	var input loginRequest
	if err := decodeRequest(request, &input); err != nil {
		result = "invalid_request"
		writeAPIError(response, http.StatusBadRequest, "invalid_request", "请求无效")
		return
	}
	password := []byte(input.Password)
	defer clear(password)
	session, err := service.deps.Auth.Login(input.Username, password)
	if err != nil {
		service.limiter.fail(key, now)
		writeAPIError(response, http.StatusUnauthorized, "invalid_credentials", "用户名或密码错误")
		return
	}
	service.limiter.clear(key)
	result = "ok"
	setSessionCookie(response, session)
	writeJSON(response, http.StatusOK, sessionResponse{Username: auth.Username, CSRFToken: session.CSRFToken, ExpiresAt: session.ExpiresAt})
}

func (service *server) logout(response http.ResponseWriter, request *http.Request) {
	service.recordAudit(request.Context(), audit.Event{Operation: "auth.logout", Result: "ok"})
	expireSessionCookie(response)
	response.WriteHeader(http.StatusNoContent)
}

func (service *server) session(response http.ResponseWriter, request *http.Request) {
	result := "unauthorized"
	defer func() { service.recordAudit(request.Context(), audit.Event{Operation: "auth.session", Result: result}) }()
	claims, ok := claimsFromContext(request.Context())
	if !ok {
		writeAPIError(response, http.StatusUnauthorized, "unauthorized", "未认证")
		return
	}
	result = "ok"
	writeJSON(response, http.StatusOK, sessionResponse{
		Username:  auth.Username,
		CSRFToken: claims.CSRFToken,
		ExpiresAt: time.Unix(0, claims.ExpiresAt).UTC(),
	})
}

func (service *server) changePassword(response http.ResponseWriter, request *http.Request) {
	result := "internal_error"
	defer func() {
		service.recordAudit(request.Context(), audit.Event{Operation: "auth.password", Result: result})
	}()
	var input passwordRequest
	if err := decodeRequest(request, &input); err != nil {
		result = "invalid_request"
		writeAPIError(response, http.StatusBadRequest, "invalid_request", "请求无效")
		return
	}
	password := []byte(input.Password)
	defer clear(password)
	if err := service.deps.Auth.ChangePassword(password); err != nil {
		if errors.Is(err, auth.ErrWeakPassword) {
			result = "weak_password"
			writeAPIError(response, http.StatusBadRequest, "weak_password", "密码至少需要 12 个字符")
			return
		}
		writeAPIError(response, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}
	result = "ok"
	expireSessionCookie(response)
	response.WriteHeader(http.StatusNoContent)
}

func decodeRequest(request *http.Request, destination any) error {
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values")
	}
	return nil
}

func setSessionCookie(response http.ResponseWriter, session auth.Session) {
	http.SetCookie(response, &http.Cookie{
		Name:     sessionCookieName,
		Value:    session.Token,
		Path:     "/",
		Expires:  session.ExpiresAt,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

func expireSessionCookie(response http.ResponseWriter) {
	response.Header().Del("Set-Cookie")
	http.SetCookie(response, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(1, 0).UTC(),
		MaxAge:   -1,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}
