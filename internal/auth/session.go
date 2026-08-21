package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	SessionCookieName = "singbox_session"
	sessionLifetime   = 24 * time.Hour
	refreshInterval   = 5 * time.Minute
)

var errInvalidSessionKey = errors.New("无效的会话签名密钥")

// Claims are the signed, non-secret contents of a browser session cookie.
type Claims struct {
	Generation uint64 `json:"generation"`
	IssuedAt   int64  `json:"issuedAt"`
	LastSeenAt int64  `json:"lastSeenAt"`
	ExpiresAt  int64  `json:"expiresAt"`
	Nonce      string `json:"nonce"`
}

// SessionManager signs and verifies stateless browser sessions.
type SessionManager struct {
	random io.Reader
}

func NewSessionManager(randomSource io.Reader) *SessionManager {
	if randomSource == nil {
		randomSource = rand.Reader
	}
	return &SessionManager{random: randomSource}
}

func (s *SessionManager) Issue(state State, now time.Time) (token, csrf string, err error) {
	nonce, err := randomToken(s.random, 16)
	if err != nil {
		return "", "", err
	}
	claims := Claims{
		Generation: state.SessionGeneration,
		IssuedAt:   now.Unix(),
		LastSeenAt: now.Unix(),
		ExpiresAt:  now.Add(sessionLifetime).Unix(),
		Nonce:      nonce,
	}
	token, err = s.sign(claims, state.SessionKey)
	if err != nil {
		return "", "", err
	}
	csrf, err = deriveCSRF(claims.Nonce, claims.Generation, state.SessionKey)
	if err != nil {
		return "", "", err
	}
	return token, csrf, nil
}

// Validate checks signature, generation, and the 24-hour idle window.
func (s *SessionManager) Validate(token string, state State, now time.Time) (Claims, string, bool) {
	payload, signature, ok := splitToken(token)
	if !ok {
		return Claims{}, "", false
	}
	key, ok := decodeSessionKey(state.SessionKey)
	if !ok {
		return Claims{}, "", false
	}
	expected := sign(payload, key)
	if subtle.ConstantTimeCompare(signature, expected) != 1 {
		return Claims{}, "", false
	}
	var claims Claims
	if json.Unmarshal(payload, &claims) != nil || !validClaims(claims, state, now) {
		return Claims{}, "", false
	}
	csrf, err := deriveCSRF(claims.Nonce, claims.Generation, state.SessionKey)
	if err != nil {
		return Claims{}, "", false
	}
	return claims, csrf, true
}

// Refresh creates a new signed token after the throttled refresh interval,
// preserving nonce and generation so the existing CSRF value remains valid.
func (s *SessionManager) Refresh(state State, claims Claims, now time.Time) (token, csrf string, err error) {
	claims.LastSeenAt = now.Unix()
	claims.ExpiresAt = now.Add(sessionLifetime).Unix()
	token, err = s.sign(claims, state.SessionKey)
	if err != nil {
		return "", "", err
	}
	csrf, err = deriveCSRF(claims.Nonce, claims.Generation, state.SessionKey)
	if err != nil {
		return "", "", err
	}
	return token, csrf, nil
}

func (s *SessionManager) ShouldRefresh(claims Claims, now time.Time) bool {
	return now.Unix()-claims.LastSeenAt >= int64(refreshInterval/time.Second)
}

// Cookie returns the sole session-cookie policy used by the API. Secure is
// deliberately false because the initial local listener is plain HTTP; the
// cookie remains HttpOnly and SameSite=Strict in every environment.
func Cookie(token string, now time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  now.Add(sessionLifetime).UTC(),
		MaxAge:   int(sessionLifetime / time.Second),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
}

func ExpiredCookie() *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(1, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
}

func (s *SessionManager) sign(claims Claims, encodedKey string) (string, error) {
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	key, ok := decodeSessionKey(encodedKey)
	if !ok {
		return "", errInvalidSessionKey
	}
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(sign(payload, key)), nil
}

func randomToken(source io.Reader, size int) (string, error) {
	value := make([]byte, size)
	if _, err := io.ReadFull(source, value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func deriveCSRF(nonce string, generation uint64, encodedKey string) (string, error) {
	key, ok := decodeSessionKey(encodedKey)
	if !ok {
		return "", errInvalidSessionKey
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("singbox-web csrf v1\x00"))
	_, _ = mac.Write([]byte(nonce))
	_, _ = mac.Write([]byte("\x00"))
	_, _ = mac.Write([]byte(strconv.FormatUint(generation, 10)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func splitToken(token string) ([]byte, []byte, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, nil, false
	}
	payload, payloadErr := base64.RawURLEncoding.DecodeString(parts[0])
	signature, signatureErr := base64.RawURLEncoding.DecodeString(parts[1])
	if payloadErr != nil || signatureErr != nil || len(signature) != sha256.Size || len(payload) == 0 {
		return nil, nil, false
	}
	return payload, signature, true
}

func sign(payload, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(payload)
	return mac.Sum(nil)
}

func decodeSessionKey(encoded string) ([]byte, bool) {
	key, err := base64.RawStdEncoding.DecodeString(encoded)
	return key, err == nil && len(key) == sha256.Size
}

func validClaims(claims Claims, state State, now time.Time) bool {
	return claims.Generation != 0 && claims.Generation == state.SessionGeneration && claims.IssuedAt > 0 && claims.LastSeenAt >= claims.IssuedAt && claims.ExpiresAt > claims.LastSeenAt && claims.Nonce != "" && now.Unix() < claims.ExpiresAt
}

type sessionContextKey struct{}

type sessionContext struct {
	claims Claims
	csrf   string
}

func sessionFromContext(ctx context.Context) (Claims, string, bool) {
	value, ok := ctx.Value(sessionContextKey{}).(sessionContext)
	return value.claims, value.csrf, ok
}

// ClaimsFromContext exposes the verified claims to protected handlers.
func ClaimsFromContext(ctx context.Context) (Claims, bool) {
	claims, _, ok := sessionFromContext(ctx)
	return claims, ok
}
