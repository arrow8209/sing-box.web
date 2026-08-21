package auth

import (
	"sync"
	"time"
)

// LoginLimiter records failed password attempts per request source. It stores
// no account identifiers because this application has exactly one account.
type LoginLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	failures map[string][]time.Time
}

func NewLoginLimiter(limit int, window time.Duration) *LoginLimiter {
	if limit <= 0 {
		limit = 5
	}
	if window <= 0 {
		window = 10 * time.Minute
	}
	return &LoginLimiter{limit: limit, window: window, failures: make(map[string][]time.Time)}
}

// Limited reports whether a source has exhausted its current failure window.
// retry is zero unless the source is limited.
func (l *LoginLimiter) Limited(source string, now time.Time) (retry time.Duration, limited bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	failures := l.pruneLocked(source, now)
	if len(failures) < l.limit {
		return 0, false
	}
	retry = failures[0].Add(l.window).Sub(now)
	if retry < time.Second {
		retry = time.Second
	}
	return retry, true
}

func (l *LoginLimiter) Failure(source string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	failures := l.pruneLocked(source, now)
	l.failures[source] = append(failures, now)
}

func (l *LoginLimiter) Success(source string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, source)
}

func (l *LoginLimiter) pruneLocked(source string, now time.Time) []time.Time {
	failures := l.failures[source]
	cutoff := now.Add(-l.window)
	first := 0
	for first < len(failures) && !failures[first].After(cutoff) {
		first++
	}
	failures = failures[first:]
	if len(failures) == 0 {
		delete(l.failures, source)
		return nil
	}
	l.failures[source] = failures
	return failures
}
