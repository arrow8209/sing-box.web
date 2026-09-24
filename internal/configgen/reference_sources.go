package configgen

import (
	"context"
	"strings"

	"singbox-web/internal/inbounds"
	"singbox-web/internal/profile"
	"singbox-web/internal/subscriptions"
)

type InboundSource interface {
	Get(context.Context, string) (inbounds.Inbound, error)
}
type SubscriptionSource interface {
	Get(context.Context, string) (subscriptions.Subscription, error)
}
type HandlerOption func(*Handler)

// WithReferenceSources opts every operation into the same resolution path.
// Configure only while constructing the handler, before it serves requests.
func WithReferenceSources(inbound InboundSource, subscription SubscriptionSource) HandlerOption {
	return func(h *Handler) {
		h.inboundSource = inbound
		h.subscriptionSource = subscription
		h.resolveReferences = true
	}
}

func (h *Handler) ServerReferenceResolution() bool { return h != nil && h.resolveReferences }

func (h *Handler) resolveSnapshot(ctx context.Context, draft profile.Draft) (ResolvedReferences, error) {
	_, model, err := ReferenceInput(draft)
	if err != nil {
		return ResolvedReferences{}, err
	}
	snapshot := ReferenceSnapshot{Inbounds: map[string]inbounds.Inbound{}, Subscriptions: map[string]subscriptions.Subscription{}}
	for _, key := range model.ResourceKeys() {
		if err := ctx.Err(); err != nil {
			return ResolvedReferences{}, err
		}
		kind, id, _ := strings.Cut(key, ":")
		// Missing sources remain absent; the pure resolver supplies the exact
		// reference field, without copying transport errors or secrets to clients.
		if kind == "inbound" && h.inboundSource != nil {
			value, err := h.inboundSource.Get(ctx, id)
			if err == nil {
				snapshot.Inbounds[id] = value
			}
		}
		if kind == "subscription" && h.subscriptionSource != nil {
			value, err := h.subscriptionSource.Get(ctx, id)
			if err == nil {
				snapshot.Subscriptions[id] = value
			}
		}
	}
	return ResolveReferences(draft, snapshot)
}
