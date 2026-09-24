package configref

import (
	"encoding/json"
	"fmt"
)

// Persisted is the version-2 authoritative business input. It contains no
// generated member list, generated JSON, or pure presentation fields.
type Persisted struct {
	Version   int              `json:"version"`
	Outbounds []OutboundInput  `json:"outbounds"`
	Inbounds  []InboundBinding `json:"inbounds"`
}

type OutboundInput struct {
	ID         string      `json:"id"`
	Tag        string      `json:"tag"`
	Type       string      `json:"type"`
	Include    string      `json:"include,omitempty"`
	Exclude    string      `json:"exclude,omitempty"`
	References []Reference `json:"references"`
}

func (p Persisted) Validate() error {
	if p.Version != 2 || p.Outbounds == nil || p.Inbounds == nil {
		return Error("reference_version_unsupported", "references.version", "引用格式版本不受支持")
	}
	ids, tags := map[string]bool{}, map[string]bool{}
	for i, g := range p.Outbounds {
		path := fmt.Sprintf("references.outbounds[%d]", i)
		if g.ID == "" || g.Tag == "" || ids[g.ID] || tags[g.Tag] || g.References == nil || (g.Type != "direct" && g.Type != "block" && g.Type != "selector" && g.Type != "urltest") {
			return Error("reference_metadata_invalid", path, "策略组引用结构无效")
		}
		ids[g.ID], tags[g.Tag] = true, true
		for j, r := range g.References {
			valid := false
			switch r.Kind {
			case "builtin":
				valid = r.Tag == "direct" || r.Tag == "block"
			case "outbound":
				valid = r.OutboundID != "" && r.Tag != ""
			case "subscription":
				valid = resourceID.MatchString(r.SubscriptionID)
			case "subscription-node":
				valid = resourceID.MatchString(r.SubscriptionID) && r.NodeTag != ""
			case "raw", "missing":
				valid = r.Tag != ""
			}
			if !valid {
				return Error("reference_metadata_invalid", fmt.Sprintf("%s.references[%d]", path, j), "节点引用结构无效")
			}
		}
	}
	seen := map[string]bool{}
	for i, b := range p.Inbounds {
		if b.Tag == "" || !resourceID.MatchString(b.ResourceID) || b.Position < 0 || seen[b.Tag] {
			return Error("reference_metadata_invalid", fmt.Sprintf("references.inbounds[%d]", i), "入站引用结构无效")
		}
		seen[b.Tag] = true
	}
	return nil
}

func (p Persisted) ToModel() (Model, error) {
	if err := p.Validate(); err != nil {
		return Model{}, err
	}
	model := Model{Outbounds: &Outbounds{Version: 1, Groups: []Group{}, GeneratedTags: []string{}}, Inbounds: &Inbounds{Version: 1, ManagedSniffRules: []SniffRule{}, ResourceBindings: []InboundBinding{}}}
	for _, g := range p.Outbounds {
		references := make([]Reference, len(g.References))
		for i, r := range g.References {
			r.ResolvedTags = nil
			references[i] = r
		}
		model.Outbounds.Groups = append(model.Outbounds.Groups, Group{ID: g.ID, Tag: g.Tag, Type: g.Type, Include: g.Include, Exclude: g.Exclude, References: references, RuntimeTags: []string{}})
	}
	model.Inbounds.ResourceBindings = append(model.Inbounds.ResourceBindings, p.Inbounds...)
	return model, nil
}

func (p Persisted) ResourceKeys() []string {
	m, err := p.ToModel()
	if err != nil {
		return []string{"invalid:references"}
	}
	return m.ResourceKeys()
}

func FromLegacy(model Model, inboundPositions map[string]int) Persisted {
	p := Persisted{Version: 2, Outbounds: []OutboundInput{}, Inbounds: []InboundBinding{}}
	if model.Outbounds != nil {
		for _, g := range model.Outbounds.Groups {
			references := make([]Reference, len(g.References))
			for i, r := range g.References {
				r.ResolvedTags = nil
				references[i] = r
			}
			p.Outbounds = append(p.Outbounds, OutboundInput{ID: g.ID, Tag: g.Tag, Type: g.Type, Include: g.Include, Exclude: g.Exclude, References: references})
		}
	}
	if model.Inbounds != nil {
		for _, b := range model.Inbounds.ResourceBindings {
			b.Position = inboundPositions[b.Tag]
			p.Inbounds = append(p.Inbounds, b)
		}
	}
	return p
}

func (p Persisted) Clone() Persisted {
	raw, _ := json.Marshal(p)
	var copy Persisted
	_ = json.Unmarshal(raw, &copy)
	return copy
}
