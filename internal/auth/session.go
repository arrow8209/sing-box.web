package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

const Username = "admin"

const (
	sessionIdleDuration    = 24 * time.Hour
	sessionRefreshInterval = 15 * time.Minute
	csrfTokenLength        = 32
)

var ErrInvalidSession = errors.New("invalid session")

type Claims struct {
	Username  string `json:"username"`
	Epoch     uint64 `json:"epoch"`
	IssuedAt  int64  `json:"issuedAt"`
	ExpiresAt int64  `json:"expiresAt"`
	CSRFToken string `json:"csrfToken"`
}

type Session struct {
	Token     string    `json:"-"`
	CSRFToken string    `json:"csrfToken"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type SessionManager struct {
	store  *FileStore
	now    func() time.Time
	random io.Reader
}

func NewSessionManager(store *FileStore, now func() time.Time, random io.Reader) *SessionManager {
	return &SessionManager{store: store, now: now, random: random}
}

func (manager *SessionManager) Login(username string, password []byte) (Session, error) {
	if username != Username {
		return Session{}, ErrInvalidSession
	}
	record, ok, err := manager.store.Authenticate(password)
	if err != nil || !ok {
		return Session{}, ErrInvalidSession
	}
	csrfBytes := make([]byte, csrfTokenLength)
	defer clear(csrfBytes)
	if _, err := io.ReadFull(manager.random, csrfBytes); err != nil {
		return Session{}, err
	}
	now := manager.now()
	issuedAt := quantizedIssuedAt(now)
	claims := Claims{
		Username:  Username,
		Epoch:     record.SessionEpoch,
		IssuedAt:  issuedAt,
		ExpiresAt: issuedAt + int64(sessionIdleDuration/time.Second),
		CSRFToken: base64.RawURLEncoding.EncodeToString(csrfBytes),
	}
	return sessionFromClaims(claims, record.SessionKey)
}

func (manager *SessionManager) Validate(token string) (Claims, error) {
	claims, err := manager.validate(token)
	if err != nil {
		return Claims{}, ErrInvalidSession
	}
	return claims, nil
}

func (manager *SessionManager) Refresh(token string) (Session, bool, error) {
	claims, err := manager.Validate(token)
	if err != nil {
		return Session{}, false, ErrInvalidSession
	}
	now := manager.now()
	if now.Before(time.Unix(claims.IssuedAt, 0).Add(sessionRefreshInterval)) {
		return Session{}, false, nil
	}
	record, err := manager.store.Load()
	if err != nil || record.SessionEpoch != claims.Epoch {
		return Session{}, false, ErrInvalidSession
	}
	claims.IssuedAt = quantizedIssuedAt(now)
	claims.ExpiresAt = claims.IssuedAt + int64(sessionIdleDuration/time.Second)
	session, err := sessionFromClaims(claims, record.SessionKey)
	if err != nil {
		return Session{}, false, ErrInvalidSession
	}
	return session, true, nil
}

func (manager *SessionManager) validate(token string) (Claims, error) {
	payloadText, signatureText, ok := strings.Cut(token, ".")
	if !ok || payloadText == "" || signatureText == "" || strings.Contains(signatureText, ".") {
		return Claims{}, ErrInvalidSession
	}
	payload, err := base64.RawURLEncoding.Strict().DecodeString(payloadText)
	if err != nil {
		return Claims{}, ErrInvalidSession
	}
	signature, err := base64.RawURLEncoding.Strict().DecodeString(signatureText)
	if err != nil || len(signature) != sha256.Size {
		return Claims{}, ErrInvalidSession
	}
	record, err := manager.store.Load()
	if err != nil {
		return Claims{}, ErrInvalidSession
	}
	key, err := decodeSessionKey(record.SessionKey)
	if err != nil {
		return Claims{}, ErrInvalidSession
	}
	defer clear(key)
	expected := signPayload(payloadText, key)
	if !hmac.Equal(signature, expected) {
		return Claims{}, ErrInvalidSession
	}
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Claims{}, ErrInvalidSession
	}
	now := manager.now().Unix()
	if claims.Username != Username ||
		claims.Epoch != record.SessionEpoch ||
		claims.IssuedAt < 0 ||
		claims.IssuedAt > now+1 ||
		claims.ExpiresAt <= claims.IssuedAt ||
		claims.ExpiresAt-claims.IssuedAt != int64(sessionIdleDuration/time.Second) ||
		now >= claims.ExpiresAt {
		return Claims{}, ErrInvalidSession
	}
	return claims, nil
}

func quantizedIssuedAt(value time.Time) int64 {
	issuedAt := value.Unix()
	if value.Nanosecond() != 0 {
		issuedAt++
	}
	return issuedAt
}

func sessionFromClaims(claims Claims, encodedKey string) (Session, error) {
	payload, err := json.Marshal(claims)
	if err != nil {
		return Session{}, err
	}
	payloadText := base64.RawURLEncoding.EncodeToString(payload)
	key, err := decodeSessionKey(encodedKey)
	if err != nil {
		return Session{}, err
	}
	defer clear(key)
	signature := signPayload(payloadText, key)
	token := payloadText + "." + base64.RawURLEncoding.EncodeToString(signature)
	return Session{
		Token:     token,
		CSRFToken: claims.CSRFToken,
		ExpiresAt: time.Unix(claims.ExpiresAt, 0).UTC(),
	}, nil
}

func decodeSessionKey(encoded string) ([]byte, error) {
	key, err := hex.DecodeString(encoded)
	if err != nil || len(key) != sha256.Size {
		clear(key)
		return nil, ErrInvalidSession
	}
	return key, nil
}

func signPayload(payload string, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(payload))
	return mac.Sum(nil)
}
