package core

import (
	"context"
	"strings"
	"sync"
	"time"
)

const maxOperationErrorBytes = 64 << 10

// OperationLock serializes all operations that can change the managed core or
// its fixed configuration directory.
type OperationLock struct {
	token  chan struct{}
	mu     sync.RWMutex
	status OperationStatus
	clock  func() time.Time
}

func NewOperationLock() *OperationLock {
	return newOperationLock(time.Now)
}

func newOperationLock(clock func() time.Time) *OperationLock {
	if clock == nil {
		clock = time.Now
	}
	lock := &OperationLock{token: make(chan struct{}, 1), clock: clock}
	lock.token <- struct{}{}
	return lock
}

// Enter waits for the single global operation slot and honours ctx while
// waiting. release is idempotent, so deferred cleanup is safe.
func (l *OperationLock) Enter(ctx context.Context, kind OperationKind) (*Operation, func(), error) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case <-l.token:
	}
	started := l.clock().UTC()
	l.mu.Lock()
	l.status = OperationStatus{Active: true, Kind: string(kind), StartedAt: &started}
	l.mu.Unlock()
	operation := &Operation{lock: l}
	var once sync.Once
	release := func() {
		once.Do(func() {
			l.mu.Lock()
			l.status = OperationStatus{}
			l.mu.Unlock()
			l.token <- struct{}{}
		})
	}
	return operation, release, nil
}

// EnterGate adapts this lock to configgen's deliberately small gate.
func (l *OperationLock) EnterGate(ctx context.Context, kind string) (func(), error) {
	_, release, err := l.Enter(ctx, OperationKind(kind))
	return release, err
}

func (l *OperationLock) Status() OperationStatus {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return cloneOperationStatus(l.status)
}

// Operation represents the holder of the currently active operation slot.
type Operation struct{ lock *OperationLock }

func (o *Operation) SetStage(stage string) {
	if o == nil || o.lock == nil {
		return
	}
	o.lock.mu.Lock()
	defer o.lock.mu.Unlock()
	if o.lock.status.Active {
		o.lock.status.Stage = stage
	}
}

// SetError publishes a bounded, non-sensitive summary for polling clients.
func (o *Operation) SetError(message string) {
	if o == nil || o.lock == nil {
		return
	}
	o.lock.mu.Lock()
	defer o.lock.mu.Unlock()
	if o.lock.status.Active {
		o.lock.status.Error = truncateOperationError(message)
	}
}

func cloneOperationStatus(status OperationStatus) OperationStatus {
	if status.StartedAt != nil {
		started := *status.StartedAt
		status.StartedAt = &started
	}
	return status
}

func truncateOperationError(message string) string {
	if len(message) <= maxOperationErrorBytes {
		return message
	}
	message = message[:maxOperationErrorBytes]
	return strings.ToValidUTF8(message, "")
}
