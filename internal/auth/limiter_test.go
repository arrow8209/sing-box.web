package auth

import (
	"testing"
	"time"
)

func TestLimiterBlocksSixthFailureAndClearsOnSuccess(t *testing.T) {
	limiter := NewLoginLimiter(5, 10*time.Minute)
	now := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
	for range 5 {
		limiter.Failure("192.0.2.1", now)
	}
	if retry, limited := limiter.Limited("192.0.2.1", now); !limited || retry != 10*time.Minute {
		t.Fatalf("limited=%v retry=%s", limited, retry)
	}
	limiter.Success("192.0.2.1")
	if _, limited := limiter.Limited("192.0.2.1", now); limited {
		t.Fatal("successful login did not clear failures")
	}
}
