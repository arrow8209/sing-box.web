package auth

import (
	"sync"
	"time"
)

// LoginLimiter records failed password attempts per request source. It stores
// no account identifiers because this application has exactly one account.
type LoginLimiter struct {
	mu              sync.Mutex
	limit           int
	window          time.Duration
	entries         map[string]*limiterEntry
	nextReservation uint64
}

type limiterEntry struct {
	failures     []time.Time
	reservations map[uint64]time.Time
}

// LoginReservation occupies one failure slot while an expensive password
// verification is in progress. It must be completed or cancelled exactly once.
type LoginReservation struct {
	source string
	entry  *limiterEntry
	id     uint64
	at     time.Time
}

func NewLoginLimiter(limit int, window time.Duration) *LoginLimiter {
	if limit <= 0 {
		limit = 5
	}
	if window <= 0 {
		window = 10 * time.Minute
	}
	return &LoginLimiter{limit: limit, window: window, entries: make(map[string]*limiterEntry)}
}

// Reserve atomically checks the current window and reserves one allowed
// failure slot. Holding the reservation through password verification prevents
// concurrent requests from all observing the same pre-limit failure count.
func (l *LoginLimiter) Reserve(source string, now time.Time) (*LoginReservation, time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entryLocked(source, now)
	if len(entry.failures)+len(entry.reservations) >= l.limit {
		return nil, l.retryLocked(entry, now), true
	}
	l.nextReservation++
	entry.reservations[l.nextReservation] = now
	return &LoginReservation{source: source, entry: entry, id: l.nextReservation, at: now}, 0, false
}

// Complete turns a reservation into a recorded failure, or clears all source
// state after a successful login. Reservations that predate such a successful
// login are intentionally ignored when they later finish.
func (l *LoginLimiter) Complete(reservation *LoginReservation, success bool) {
	if reservation == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, exists := l.entries[reservation.source]
	if !exists || entry != reservation.entry {
		return
	}
	if _, exists := entry.reservations[reservation.id]; !exists {
		return
	}
	delete(entry.reservations, reservation.id)
	if success {
		delete(l.entries, reservation.source)
		return
	}
	entry.failures = append(entry.failures, reservation.at)
}

// Cancel releases a reservation for a request that never reached a password
// outcome, such as a transient session-issuance failure.
func (l *LoginLimiter) Cancel(reservation *LoginReservation) {
	if reservation == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, exists := l.entries[reservation.source]
	if !exists || entry != reservation.entry {
		return
	}
	delete(entry.reservations, reservation.id)
	if len(entry.failures) == 0 && len(entry.reservations) == 0 {
		delete(l.entries, reservation.source)
	}
}

// Limited is retained for callers that only need to inspect the rate limit.
// Login handlers should use Reserve so check and accounting remain atomic.
func (l *LoginLimiter) Limited(source string, now time.Time) (retry time.Duration, limited bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entryLocked(source, now)
	if len(entry.failures)+len(entry.reservations) < l.limit {
		return 0, false
	}
	return l.retryLocked(entry, now), true
}

func (l *LoginLimiter) Failure(source string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entryLocked(source, now)
	entry.failures = append(entry.failures, now)
}

func (l *LoginLimiter) Success(source string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, source)
}

func (l *LoginLimiter) entryLocked(source string, now time.Time) *limiterEntry {
	entry := l.entries[source]
	if entry == nil {
		entry = &limiterEntry{reservations: make(map[uint64]time.Time)}
		l.entries[source] = entry
	}
	cutoff := now.Add(-l.window)
	first := 0
	for first < len(entry.failures) && !entry.failures[first].After(cutoff) {
		first++
	}
	entry.failures = entry.failures[first:]
	return entry
}

func (l *LoginLimiter) retryLocked(entry *limiterEntry, now time.Time) time.Duration {
	oldest := time.Time{}
	for _, at := range entry.failures {
		if oldest.IsZero() || at.Before(oldest) {
			oldest = at
		}
	}
	for _, at := range entry.reservations {
		if oldest.IsZero() || at.Before(oldest) {
			oldest = at
		}
	}
	retry := oldest.Add(l.window).Sub(now)
	if retry < time.Second {
		return time.Second
	}
	return retry
}
