package configgen

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"singbox-web/internal/configref"
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
		// reference field. Other read failures stop before a partial candidate.
		if kind == "inbound" && h.inboundSource != nil {
			value, err := h.inboundSource.Get(ctx, id)
			if err == nil {
				snapshot.Inbounds[id] = value
			} else if !errors.Is(err, inbounds.ErrNotFound) {
				return ResolvedReferences{}, configref.Error("reference_source_unavailable", referenceSourcePath(model, key, draft.SchemaVersion), "入站来源暂时无法读取，请重试")
			}
		}
		if kind == "subscription" && h.subscriptionSource != nil {
			value, err := h.subscriptionSource.Get(ctx, id)
			if err == nil {
				snapshot.Subscriptions[id] = value
			} else if !errors.Is(err, subscriptions.ErrNotFound) {
				return ResolvedReferences{}, configref.Error("reference_source_unavailable", referenceSourcePath(model, key, draft.SchemaVersion), "订阅来源暂时无法读取，请重试")
			}
		}
	}
	return ResolveReferences(draft, snapshot)
}

func referenceSourcePath(model configref.Model, key string, schema int) string {
	if strings.HasPrefix(key, "inbound:") && model.Inbounds != nil {
		for i, b := range model.Inbounds.ResourceBindings {
			if "inbound:"+b.ResourceID == key {
				if schema == 2 {
					return fmt.Sprintf("references.inbounds[%d]", i)
				}
				return fmt.Sprintf("editorState.inbounds.resourceBindings[%d]", i)
			}
		}
	}
	if strings.HasPrefix(key, "subscription:") && model.Outbounds != nil {
		for i, g := range model.Outbounds.Groups {
			for j, r := range g.References {
				if "subscription:"+r.SubscriptionID == key {
					if schema == 2 {
						return fmt.Sprintf("references.outbounds[%d].references[%d]", i, j)
					}
					return fmt.Sprintf("editorState.outbounds.groups[%d].references[%d]", i, j)
				}
			}
		}
	}
	return "references"
}
