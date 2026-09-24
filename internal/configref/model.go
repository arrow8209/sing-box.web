// Package configref defines reference intent independently of resource services
// and the config generator. Legacy editor metadata is adapted without guessing.
package configref

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"

	"singbox-web/internal/apierror"
)

func InboundDigest(mode string, config json.RawMessage) string {
	h := sha256.New()
	h.Write([]byte(mode))
	h.Write([]byte{0})
	h.Write(config)
	return hex.EncodeToString(h.Sum(nil))
}

func SubscriptionDigest(outbounds []json.RawMessage) string {
	h := sha256.New()
	for _, node := range outbounds {
		h.Write(node)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

const Stage = "resolve_references"

type Reference struct {
	Kind           string   `json:"kind"`
	Tag            string   `json:"tag,omitempty"`
	OutboundID     string   `json:"outboundId,omitempty"`
	SubscriptionID string   `json:"subscriptionId,omitempty"`
	NodeTag        string   `json:"nodeTag,omitempty"`
	ResolvedTags   []string `json:"resolvedTags,omitempty"`
}

type Group struct {
	ID          string      `json:"id"`
	Tag         string      `json:"tag"`
	Type        string      `json:"type"`
	Include     string      `json:"include"`
	Exclude     string      `json:"exclude"`
	References  []Reference `json:"references"`
	RuntimeTags []string    `json:"runtimeTags"`
	Hidden      bool        `json:"hidden"`
	Icon        string      `json:"icon"`
}

type Outbounds struct {
	Version       int      `json:"version"`
	Groups        []Group  `json:"groups"`
	GeneratedTags []string `json:"generatedTags"`
}

type InboundBinding struct {
	Tag        string `json:"tag"`
	ResourceID string `json:"resourceId"`
	SourceTag  string `json:"sourceTag"`
	Position   int    `json:"position,omitempty"`
}

type SniffRule struct {
	Tag  string          `json:"tag"`
	Rule json.RawMessage `json:"rule"`
}

type Inbounds struct {
	Version           int              `json:"version"`
	ManagedSniffRules []SniffRule      `json:"managedSniffRules"`
	ResourceBindings  []InboundBinding `json:"resourceBindings,omitempty"`
}

type Model struct {
	Outbounds *Outbounds `json:"outbounds,omitempty"`
	Inbounds  *Inbounds  `json:"inbounds,omitempty"`
}

func Error(code, path, message string) error {
	return apierror.Field(code, message, Stage, path, nil)
}

var resourceID = regexp.MustCompile(`^[a-f0-9]{32}$`)

func DecodeLegacy(state map[string]json.RawMessage) (Model, error) {
	var model Model
	for _, key := range []string{"inbounds", "outbounds"} {
		raw, exists := state[key]
		if !exists {
			continue
		}
		path := "editorState." + key
		var envelope struct {
			Version int `json:"version"`
		}
		if json.Unmarshal(raw, &envelope) != nil || string(raw) == "null" {
			return Model{}, Error("reference_metadata_invalid", path, "引用元数据损坏，请重新确认关联")
		}
		if envelope.Version != 1 {
			return Model{}, Error("reference_version_unsupported", path+".version", "引用版本不受支持，请更新页面或修复档案")
		}
		if key == "outbounds" {
			var value Outbounds
			if json.Unmarshal(raw, &value) != nil || value.Groups == nil {
				return Model{}, Error("reference_metadata_invalid", path, "出站引用结构无效")
			}
			if value.GeneratedTags == nil {
				return Model{}, Error("reference_ownership_missing", path+".generatedTags", "缺少生成节点归属，不能安全重建")
			}
			ids, tags := map[string]bool{}, map[string]bool{}
			for i, g := range value.Groups {
				gp := fmt.Sprintf("%s.groups[%d]", path, i)
				if g.ID == "" || g.Tag == "" || ids[g.ID] || tags[g.Tag] || g.References == nil || g.RuntimeTags == nil || (g.Type != "direct" && g.Type != "block" && g.Type != "selector" && g.Type != "urltest") {
					return Model{}, Error("reference_metadata_invalid", gp, "策略组引用结构无效")
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
						return Model{}, Error("reference_metadata_invalid", fmt.Sprintf("%s.references[%d]", gp, j), "节点引用结构无效")
					}
				}
			}
			seen := map[string]bool{}
			for _, tag := range value.GeneratedTags {
				if tag == "" || seen[tag] || tags[tag] {
					return Model{}, Error("reference_ownership_missing", path+".generatedTags", "生成节点归属冲突")
				}
				seen[tag] = true
			}
			model.Outbounds = &value
		} else {
			var value Inbounds
			if json.Unmarshal(raw, &value) != nil || value.ManagedSniffRules == nil {
				return Model{}, Error("reference_metadata_invalid", path, "入站引用结构无效")
			}
			seen := map[string]bool{}
			for i, binding := range value.ResourceBindings {
				if binding.Tag == "" || !resourceID.MatchString(binding.ResourceID) || seen[binding.Tag] {
					return Model{}, Error("reference_metadata_invalid", fmt.Sprintf("%s.resourceBindings[%d]", path, i), "入站来源结构无效")
				}
				seen[binding.Tag] = true
			}
			seen = map[string]bool{}
			for _, managed := range value.ManagedSniffRules {
				var rule struct {
					Action  string `json:"action"`
					Inbound string `json:"inbound"`
				}
				if managed.Tag == "" || seen[managed.Tag] || json.Unmarshal(managed.Rule, &rule) != nil || rule.Action != "sniff" || rule.Inbound != managed.Tag {
					return Model{}, Error("reference_metadata_invalid", path+".managedSniffRules", "托管嗅探归属无效")
				}
				seen[managed.Tag] = true
			}
			model.Inbounds = &value
		}
	}
	return model, nil
}

func (m Model) ResourceKeys() []string {
	keys := []string{}
	seen := map[string]bool{}
	add := func(key string) {
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	if m.Inbounds != nil {
		for _, b := range m.Inbounds.ResourceBindings {
			add("inbound:" + b.ResourceID)
		}
	}
	if m.Outbounds != nil {
		for _, g := range m.Outbounds.Groups {
			for _, r := range g.References {
				if r.Kind == "subscription" || r.Kind == "subscription-node" {
					add("subscription:" + r.SubscriptionID)
				}
			}
		}
	}
	return keys
}
