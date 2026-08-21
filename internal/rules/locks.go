package rules

import (
	"context"
	"sync"
)

// resourceLocks serializes every operation that can replace or move a single
// fixed rule-set file. Different resources remain independent.
type resourceLocks struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func newResourceLocks() *resourceLocks { return &resourceLocks{locks: make(map[string]*sync.Mutex)} }

func (l *resourceLocks) Lock(ctx context.Context, id string) (func(), error) {
	l.mu.Lock()
	lock := l.locks[id]
	if lock == nil {
		lock = &sync.Mutex{}
		l.locks[id] = lock
	}
	l.mu.Unlock()
	acquired := make(chan struct{}, 1)
	go func() { lock.Lock(); acquired <- struct{}{} }()
	select {
	case <-ctx.Done():
		go func() { <-acquired; lock.Unlock() }()
		return nil, ctx.Err()
	case <-acquired:
		return lock.Unlock, nil
	}
}
