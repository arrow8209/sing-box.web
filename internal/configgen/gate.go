package configgen

import "context"

// OperationGate serializes operations that touch a temporary configuration or
// the managed core. The core package replaces this implementation with its
// shared operation lock in the next phase without changing Handler.
type OperationGate interface {
	EnterGate(ctx context.Context, kind string) (release func(), err error)
}

type mutexGate struct{ token chan struct{} }

func NewOperationGate() OperationGate {
	gate := &mutexGate{token: make(chan struct{}, 1)}
	gate.token <- struct{}{}
	return gate
}

func (g *mutexGate) EnterGate(ctx context.Context, _ string) (func(), error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-g.token:
	}
	return func() { g.token <- struct{}{} }, nil
}
