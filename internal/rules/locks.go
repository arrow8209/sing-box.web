package rules

import (
	"context"
	"sync"
	"sync/atomic"
)

// ResourceLocker is deliberately small so snapshots can lock several rule
// files in a globally stable order and tests can observe that order.
type ResourceLocker interface {
	Lock(ctx context.Context, resourceID string) (release func(), err error)
}

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
		var released atomic.Bool
		return func() {
			if released.CompareAndSwap(false, true) {
				lock.Unlock()
			}
		}, nil
	}
}
