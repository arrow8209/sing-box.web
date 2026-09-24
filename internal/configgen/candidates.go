package configgen

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"

	"singbox-web/internal/configref"
	"singbox-web/internal/profile"
)

// CandidateItem is a display view. Text and RouteText are JSON strings so
// browsers can pass the exact candidate back without a JS number round-trip.
type CandidateItem struct {
	Module           string      `json:"module"`
	Tag              string      `json:"tag"`
	Type             string      `json:"type"`
	SourceID         string      `json:"sourceId,omitempty"`
	SourceName       string      `json:"sourceName,omitempty"`
	Independent      bool        `json:"independent"`
	Generated        bool        `json:"generated,omitempty"`
	Members          []string    `json:"members,omitempty"`
	Listen           string      `json:"listen,omitempty"`
	ListenPort       json.Number `json:"listenPort,omitempty"`
	Sniff            bool        `json:"sniff,omitempty"`
	Text             string      `json:"text,omitempty"`
	RouteText        string      `json:"routeText,omitempty"`
	InboundStateText string      `json:"inboundStateText,omitempty"`
	Token            string      `json:"token,omitempty"`
}

type CandidateView struct {
	ProfileID       string            `json:"profileId"`
	Revision        uint64            `json:"revision"`
	Digest          string            `json:"digest"`
	ResourceDigests map[string]string `json:"resourceDigests"`
	Items           []CandidateItem   `json:"items"`
}

func newCandidateKey() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic("candidate signing key unavailable")
	}
	return key
}

func (h *Handler) candidateView(draft profile.Draft, output Output) *CandidateView {
	if !h.resolveReferences {
		return nil
	}
	var model configref.Model
	if draft.ResolvedModel != nil {
		model = *draft.ResolvedModel
	} else {
		var err error
		model, err = configref.DecodeLegacy(draft.EditorState)
		if err != nil {
			return nil
		}
	}
	view := &CandidateView{ProfileID: draft.ProfileID, Revision: draft.Revision, Digest: output.Digest, ResourceDigests: output.ResourceDigests, Items: []CandidateItem{}}
	bindings := map[string]configref.InboundBinding{}
	sniff := map[string]bool{}
	if model.Inbounds != nil {
		for _, b := range model.Inbounds.ResourceBindings {
			bindings[b.Tag] = b
		}
		for _, rule := range model.Inbounds.ManagedSniffRules {
			sniff[rule.Tag] = true
		}
	}
	generated := map[string]bool{}
	groups := map[string]configref.Group{}
	if model.Outbounds != nil {
		for _, tag := range model.Outbounds.GeneratedTags {
			generated[tag] = true
		}
		for _, g := range model.Outbounds.Groups {
			groups[g.Tag] = g
		}
	}
	for _, module := range []string{"inbounds", "outbounds"} {
		items, _ := referenceArray(draft.Config[module], module)
		for _, raw := range items {
			fields := object(raw)
			tag := rawString(fields["tag"])
			if tag == "" {
				continue
			}
			item := CandidateItem{Module: module, Tag: tag, Type: rawString(fields["type"]), Independent: true, Generated: generated[tag]}
			if module == "inbounds" {
				item.Listen = rawString(fields["listen"])
				if v := fields["listen_port"]; len(v) > 0 {
					item.ListenPort = json.Number(v)
				}
				if b, ok := bindings[tag]; ok {
					item.Independent = false
					item.SourceID = b.ResourceID
					item.SourceName = b.SourceTag
					item.Sniff = sniff[tag]
					item.Text = string(raw)
					item.RouteText = string(draft.Config["route"])
					if draft.SchemaVersion == 2 {
						item.InboundStateText = string(rawValue(model.Inbounds))
					} else {
						item.InboundStateText = string(draft.EditorState["inbounds"])
					}
				}
			} else if g, ok := groups[tag]; ok {
				item.Members = slices.Clone(g.RuntimeTags)
			}
			if !item.Independent || item.Generated {
				item.Token = h.candidateToken(view.ProfileID, view.Revision, view.Digest, view.ResourceDigests, item)
			}
			view.Items = append(view.Items, item)
		}
	}
	return view
}

func (h *Handler) candidateToken(id string, revision uint64, digest string, resources map[string]string, item CandidateItem) string {
	mac := hmac.New(sha256.New, h.candidateKey)
	for _, value := range []string{id, fmt.Sprint(revision), digest, item.Module, item.Tag, item.Text, item.RouteText, item.InboundStateText} {
		mac.Write([]byte(value))
		mac.Write([]byte{0})
	}
	encoded, _ := json.Marshal(resources)
	mac.Write(encoded)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyFreeze is a pure signature check. SaveDraft enforces If-Match while
// applying the already displayed text; the source is never fetched again.
func (h *Handler) VerifyFreeze(profileID string, revision uint64, operation profile.FreezeOperation) error {
	if !h.resolveReferences || operation.Module != "inbounds" || operation.Tag == "" || operation.Digest == "" || operation.Text == "" || operation.InboundStateText == "" {
		return configref.Error("candidate_invalid", "freeze", "候选内容无效，请重新预览")
	}
	item := CandidateItem{Module: operation.Module, Tag: operation.Tag, Text: operation.Text, RouteText: operation.RouteText, InboundStateText: operation.InboundStateText}
	current := h.candidateToken(profileID, revision, operation.Digest, operation.ResourceDigests, item)
	provided, err := hex.DecodeString(operation.Token)
	expected, _ := hex.DecodeString(current)
	if err != nil || !hmac.Equal(provided, expected) {
		return configref.Error("candidate_mismatch", "freeze.token", "候选已失效，请重新预览")
	}
	return nil
}
