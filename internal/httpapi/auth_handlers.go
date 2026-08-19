package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

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
	if service.deps.Auth == nil {
		writeAPIError(response, http.StatusUnauthorized, "invalid_credentials", "用户名或密码错误")
		return
	}
	key := normalizedClientIP(request.Context()) + "\x00" + auth.Username
	now := service.deps.Now()
	if service.limiter.limited(key, now) {
		writeAPIError(response, http.StatusTooManyRequests, "rate_limited", "登录尝试过多，请稍后再试")
		return
	}
	var input loginRequest
	if err := decodeRequest(request, &input); err != nil {
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
	setSessionCookie(response, session)
	writeJSON(response, http.StatusOK, sessionResponse{Username: auth.Username, CSRFToken: session.CSRFToken, ExpiresAt: session.ExpiresAt})
}

func (service *server) logout(response http.ResponseWriter, _ *http.Request) {
	expireSessionCookie(response)
	response.WriteHeader(http.StatusNoContent)
}

func (service *server) session(response http.ResponseWriter, request *http.Request) {
	claims, ok := claimsFromContext(request.Context())
	if !ok {
		writeAPIError(response, http.StatusUnauthorized, "unauthorized", "未认证")
		return
	}
	writeJSON(response, http.StatusOK, sessionResponse{
		Username:  auth.Username,
		CSRFToken: claims.CSRFToken,
		ExpiresAt: time.Unix(0, claims.ExpiresAt).UTC(),
	})
}

func (service *server) changePassword(response http.ResponseWriter, request *http.Request) {
	var input passwordRequest
	if err := decodeRequest(request, &input); err != nil {
		writeAPIError(response, http.StatusBadRequest, "invalid_request", "请求无效")
		return
	}
	password := []byte(input.Password)
	defer clear(password)
	if err := service.deps.Auth.ChangePassword(password); err != nil {
		if errors.Is(err, auth.ErrWeakPassword) {
			writeAPIError(response, http.StatusBadRequest, "weak_password", "密码至少需要 12 个字符")
			return
		}
		writeAPIError(response, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}
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
