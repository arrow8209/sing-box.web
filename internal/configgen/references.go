package configgen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"singbox-web/internal/configref"
	"singbox-web/internal/inbounds"
	"singbox-web/internal/profile"
	"singbox-web/internal/subscriptions"
)

type ReferenceSnapshot struct {
	Inbounds      map[string]inbounds.Inbound
	Subscriptions map[string]subscriptions.Subscription
}

type ResolvedReferences struct {
	Draft           profile.Draft
	Model           configref.Model
	ResourceDigests map[string]string
}

type rawObject = map[string]json.RawMessage

func rawValue(value any) json.RawMessage   { raw, _ := json.Marshal(value); return raw }
func rawString(raw json.RawMessage) string { var s string; _ = json.Unmarshal(raw, &s); return s }
func rawEqual(a, b json.RawMessage) bool {
	av, ae := decodeJSON(a)
	bv, be := decodeJSON(b)
	return ae == nil && be == nil && reflect.DeepEqual(av, bv)
}
func object(raw json.RawMessage) rawObject { var o rawObject; _ = json.Unmarshal(raw, &o); return o }

func InboundDigest(source inbounds.Inbound) string {
	return configref.InboundDigest(string(source.Mode), source.Config)
}
func SubscriptionDigest(source subscriptions.Subscription) string {
	return configref.SubscriptionDigest(source.Outbounds)
}

// ReferenceInput copies the draft and makes its raw user buffers authoritative.
// Every consumer then sees the same precision-preserving candidate.
func ReferenceInput(input profile.Draft) (profile.Draft, configref.Model, error) {
	draft := input
	draft.Config = cloneRawMap(input.Config)
	draft.Buffers = maps.Clone(input.Buffers)
	draft.EditorState = cloneRawMap(input.EditorState)
	for key, text := range draft.Buffers {
		if text == "" {
			delete(draft.Config, key)
			continue
		}
		if !json.Valid([]byte(text)) {
			return profile.Draft{}, configref.Model{}, configref.Error("invalid_module_json", key, "模块 JSON 无效，请先修正输入")
		}
		draft.Config[key] = json.RawMessage(text)
	}
	model, err := configref.DecodeLegacy(draft.EditorState)
	if err != nil {
		return profile.Draft{}, configref.Model{}, err
	}
	// A deleted local entry must not resurrect itself through stale metadata.
	if model.Inbounds != nil {
		items, err := referenceArray(draft.Config["inbounds"], "inbounds")
		if err != nil {
			return profile.Draft{}, configref.Model{}, err
		}
		model.Inbounds.ResourceBindings = slices.DeleteFunc(model.Inbounds.ResourceBindings, func(b configref.InboundBinding) bool { return findTag(items, b.Tag) < 0 })
	}
	return draft, model, nil
}

func ResolveReferences(input profile.Draft, snapshot ReferenceSnapshot) (ResolvedReferences, error) {
	draft, model, err := ReferenceInput(input)
	if err != nil {
		return ResolvedReferences{}, err
	}
	digests := map[string]string{}
	if model.Outbounds != nil {
		if err := resolveOutbounds(&draft, model.Outbounds, snapshot, digests); err != nil {
			return ResolvedReferences{}, err
		}
		draft.EditorState["outbounds"] = rawValue(model.Outbounds)
	}
	if model.Inbounds != nil {
		if err := resolveInbounds(&draft, model.Inbounds, snapshot, digests); err != nil {
			return ResolvedReferences{}, err
		}
		draft.EditorState["inbounds"] = rawValue(model.Inbounds)
	}
	return ResolvedReferences{Draft: draft, Model: model, ResourceDigests: digests}, nil
}

func referenceArray(raw json.RawMessage, path string) ([]json.RawMessage, error) {
	if len(raw) == 0 {
		return []json.RawMessage{}, nil
	}
	var items []json.RawMessage
	if json.Unmarshal(raw, &items) != nil || items == nil {
		return nil, configref.Error("reference_conflict", path, "引用模块必须是数组")
	}
	seen := map[string]bool{}
	for i, item := range items {
		tag := rawString(object(item)["tag"])
		if tag == "" {
			continue
		}
		if seen[tag] {
			return nil, configref.Error("reference_conflict", fmt.Sprintf("%s[%d].tag", path, i), "名称重复，不能确定引用条目")
		}
		seen[tag] = true
	}
	return items, nil
}
func findTag(items []json.RawMessage, tag string) int {
	for i, item := range items {
		if rawString(object(item)["tag"]) == tag {
			return i
		}
	}
	return -1
}
func setModule(draft *profile.Draft, key string, raw json.RawMessage) {
	draft.Config[key] = raw
	draft.Buffers[key] = string(raw)
}
func uniqueTag(seed string, used map[string]bool) string {
	if !used[seed] {
		return seed
	}
	for i := 2; ; i++ {
		tag := fmt.Sprintf("%s-%d", seed, i)
		if !used[tag] {
			return tag
		}
	}
}

func resolveOutbounds(draft *profile.Draft, state *configref.Outbounds, snapshot ReferenceSnapshot, digests map[string]string) error {
	items, err := referenceArray(draft.Config["outbounds"], "outbounds")
	if err != nil {
		return err
	}
	generatedSet, groupByID, groupByTag := map[string]bool{}, map[string]*configref.Group{}, map[string]*configref.Group{}
	for _, tag := range state.GeneratedTags {
		generatedSet[tag] = true
	}
	active := []configref.Group{}
	for _, g := range state.Groups {
		index := findTag(items, g.Tag)
		if index < 0 {
			continue
		}
		config := object(items[index])
		if rawString(config["type"]) != g.Type {
			return configref.Error("reference_conflict", fmt.Sprintf("outbounds[%d].type", index), "策略组类型与引用信息不一致")
		}
		var tags []string
		_ = json.Unmarshal(config["outbounds"], &tags)
		if !slices.Equal(tags, g.RuntimeTags) {
			return configref.Error("reference_conflict", fmt.Sprintf("outbounds[%d].outbounds", index), "策略组成员与引用信息不一致")
		}
		for _, r := range g.References {
			if r.Kind == "subscription" || r.Kind == "subscription-node" {
				for _, tag := range r.ResolvedTags {
					if findTag(items, tag) >= 0 && !generatedSet[tag] {
						return configref.Error("reference_ownership_missing", "editorState.outbounds.generatedTags", "引用节点缺少明确生成归属")
					}
				}
			}
		}
		active = append(active, g)
	}
	state.Groups = active
	used := map[string]bool{}
	for i := range state.Groups {
		g := &state.Groups[i]
		groupByID[g.ID] = g
		groupByTag[g.Tag] = g
		used[g.Tag] = true
	}
	for _, item := range items {
		tag := rawString(object(item)["tag"])
		if !generatedSet[tag] {
			used[tag] = true
		}
	}
	generated := []json.RawMessage{}
	generatedSources := []string{}
	allocated, mappings := map[string]string{}, map[string]map[string]string{}
	addNode := func(source subscriptions.Subscription, node json.RawMessage, index int) string {
		key := fmt.Sprintf("%s:%d", source.ID, index)
		if tag := allocated[key]; tag != "" {
			return tag
		}
		config := object(node)
		original := rawString(config["tag"])
		tag := uniqueTag(original, used)
		config["tag"] = rawValue(tag)
		generated = append(generated, rawValue(config))
		generatedSources = append(generatedSources, source.ID)
		used[tag] = true
		allocated[key] = tag
		if mappings[source.ID] == nil {
			mappings[source.ID] = map[string]string{}
		}
		if mappings[source.ID][original] == "" {
			mappings[source.ID][original] = tag
		}
		return tag
	}
	for gi := range state.Groups {
		g := &state.Groups[gi]
		gp := fmt.Sprintf("editorState.outbounds.groups[%d]", gi)
		include, e := referenceFilter(g.Include, gp+".include")
		if e != nil {
			return e
		}
		exclude, e := referenceFilter(g.Exclude, gp+".exclude")
		if e != nil {
			return e
		}
		tags := []string{}
		seen := map[string]bool{}
		for ri := range g.References {
			r := &g.References[ri]
			rp := fmt.Sprintf("%s.references[%d]", gp, ri)
			resolved := []string{}
			if g.Type != "selector" && g.Type != "urltest" {
				continue
			}
			switch r.Kind {
			case "subscription", "subscription-node":
				source, ok := snapshot.Subscriptions[r.SubscriptionID]
				if !ok {
					return configref.Error("reference_source_missing", rp, "订阅来源不存在，请重新关联或转为独立配置")
				}
				digests["subscription:"+source.ID] = SubscriptionDigest(source)
				found := r.Kind == "subscription"
				for ni, node := range source.Outbounds {
					config := object(node)
					tag := rawString(config["tag"])
					if tag == "" || rawString(config["type"]) == "" {
						return configref.Error("reference_metadata_invalid", rp, "订阅节点结构无效")
					}
					if r.Kind == "subscription-node" && r.NodeTag != tag {
						continue
					}
					found = true
					inc, e := include(tag)
					if e != nil {
						return e
					}
					exc := false
					if g.Exclude != "" {
						exc, e = exclude(tag)
						if e != nil {
							return e
						}
					}
					if inc && !exc {
						resolved = append(resolved, addNode(source, node, ni))
					}
					if r.Kind == "subscription-node" {
						break
					}
				}
				if !found {
					return configref.Error("reference_node_missing", rp, "订阅节点已不存在，请重新选择引用")
				}
				if r.Kind == "subscription" {
					r.Tag = source.Name
				}
			case "builtin":
				if !used[r.Tag] {
					generated = append(generated, rawValue(map[string]string{"type": r.Tag, "tag": r.Tag}))
					generatedSources = append(generatedSources, "")
					used[r.Tag] = true
				}
				resolved = []string{r.Tag}
			case "outbound":
				if target := groupByID[r.OutboundID]; target != nil && target.ID != g.ID {
					r.Tag = target.Tag
					resolved = []string{target.Tag}
				} else {
					resolved = []string{r.Tag}
				}
			default:
				resolved = slices.Clone(r.ResolvedTags)
				if len(resolved) == 0 {
					resolved = []string{r.Tag}
				}
			}
			r.ResolvedTags = resolved
			for _, tag := range resolved {
				if tag != "" && tag != g.Tag && !seen[tag] {
					tags = append(tags, tag)
					seen[tag] = true
				}
			}
		}
		g.RuntimeTags = tags
		index := findTag(items, g.Tag)
		config := object(items[index])
		for _, k := range []string{"hidden", "icon", "include", "exclude"} {
			delete(config, k)
		}
		if g.Type == "direct" || g.Type == "block" {
			for _, k := range []string{"outbounds", "interrupt_exist_connections", "url", "interval", "tolerance"} {
				delete(config, k)
			}
		} else {
			config["outbounds"] = rawValue(tags)
			if !bytes.Equal(config["interrupt_exist_connections"], []byte("false")) && !bytes.Equal(config["interrupt_exist_connections"], []byte("true")) {
				config["interrupt_exist_connections"] = rawValue(true)
			}
			if def := rawString(config["default"]); def != "" && !slices.Contains(tags, def) {
				return configref.Error("reference_conflict", fmt.Sprintf("outbounds[%d].default", index), "默认节点已不在策略组成员中，请重新选择")
			}
			if g.Type == "urltest" {
				if rawString(config["url"]) == "" {
					config["url"] = rawValue("https://www.gstatic.com/generate_204")
				}
				if rawString(config["interval"]) == "" {
					config["interval"] = rawValue("3m")
				}
				if len(config["tolerance"]) == 0 {
					config["tolerance"] = rawValue(150)
				}
			} else {
				delete(config, "url")
				delete(config, "interval")
				delete(config, "tolerance")
			}
		}
		items[index] = rawValue(config)
	}
	state.GeneratedTags = []string{}
	for i, node := range generated {
		config := object(node)
		mapping := mappings[generatedSources[i]]
		for _, key := range []string{"detour", "default"} {
			if tag := mapping[rawString(config[key])]; tag != "" {
				config[key] = rawValue(tag)
			}
		}
		var refs []string
		if json.Unmarshal(config["outbounds"], &refs) == nil && refs != nil {
			for j, tag := range refs {
				if v := mapping[tag]; v != "" {
					refs[j] = v
				}
			}
			config["outbounds"] = rawValue(refs)
		}
		generated[i] = rawValue(config)
		state.GeneratedTags = append(state.GeneratedTags, rawString(config["tag"]))
	}
	result := []json.RawMessage{}
	for _, item := range items {
		if !generatedSet[rawString(object(item)["tag"])] {
			result = append(result, item)
		}
	}
	result = append(result, generated...)
	if len(items) > 0 || len(result) > 0 {
		setModule(draft, "outbounds", rawValue(result))
	}
	return nil
}

func resolveInbounds(draft *profile.Draft, state *configref.Inbounds, snapshot ReferenceSnapshot, digests map[string]string) error {
	items, err := referenceArray(draft.Config["inbounds"], "inbounds")
	if err != nil {
		return err
	}
	for i := range state.ResourceBindings {
		b := &state.ResourceBindings[i]
		path := fmt.Sprintf("editorState.inbounds.resourceBindings[%d]", i)
		source, ok := snapshot.Inbounds[b.ResourceID]
		if !ok {
			return configref.Error("reference_source_missing", path, "入站来源不存在，请重新关联或明确恢复此前副本")
		}
		digests["inbound:"+source.ID] = InboundDigest(source)
		config := object(source.Config)
		if config == nil {
			return configref.Error("reference_metadata_invalid", path, "入站来源结构无效")
		}
		b.SourceTag = rawString(config["tag"])
		config["tag"] = rawValue(b.Tag)
		sniff := source.Mode == inbounds.ModeStructured && bytes.Equal(config["sniff"], []byte("true"))
		timeout := rawString(config["sniff_timeout"])
		if source.Mode == inbounds.ModeStructured {
			for _, key := range []string{"sniff", "sniff_timeout", "sniff_override_destination", "domain_strategy", "udp_disable_domain_unmapping"} {
				delete(config, key)
			}
		}
		index := findTag(items, b.Tag)
		if index < 0 {
			continue
		}
		items[index] = rawValue(config)
		if err := applySniff(draft, state, b.Tag, sniff, timeout); err != nil {
			return err
		}
	}
	if len(state.ResourceBindings) > 0 {
		setModule(draft, "inbounds", rawValue(items))
	}
	return nil
}

func applySniff(draft *profile.Draft, state *configref.Inbounds, tag string, enabled bool, timeout string) error {
	managedIndex := -1
	for i, m := range state.ManagedSniffRules {
		if m.Tag == tag {
			managedIndex = i
			break
		}
	}
	if !enabled && managedIndex < 0 {
		return nil
	}
	route := object(draft.Config["route"])
	if route == nil {
		if len(draft.Config["route"]) > 0 {
			return configref.Error("reference_conflict", "route", "路由必须是对象")
		}
		route = rawObject{}
	}
	var rules []json.RawMessage
	if raw, ok := route["rules"]; ok && json.Unmarshal(raw, &rules) != nil {
		return configref.Error("reference_conflict", "route.rules", "路由规则必须是数组")
	}
	index := -1
	if managedIndex >= 0 {
		for i, rule := range rules {
			if rawEqual(rule, state.ManagedSniffRules[managedIndex].Rule) {
				index = i
				break
			}
		}
	}
	conflict := false
	for i, rule := range rules {
		if i == index {
			continue
		}
		r := object(rule)
		if rawString(r["action"]) != "sniff" {
			continue
		}
		v, exists := r["inbound"]
		var tags []string
		_ = json.Unmarshal(v, &tags)
		if !exists || rawString(v) == tag || slices.Contains(tags, tag) {
			conflict = true
		}
	}
	if managedIndex >= 0 {
		state.ManagedSniffRules = append(state.ManagedSniffRules[:managedIndex], state.ManagedSniffRules[managedIndex+1:]...)
	}
	if enabled && !conflict {
		if strings.TrimSpace(timeout) == "" {
			timeout = "300ms"
		}
		desired := rawValue(map[string]string{"action": "sniff", "inbound": tag, "timeout": strings.TrimSpace(timeout)})
		if index >= 0 {
			rules[index] = desired
		} else {
			adjustRoutePositions(draft, len(rules), 0, true)
			rules = append([]json.RawMessage{desired}, rules...)
		}
		state.ManagedSniffRules = append(state.ManagedSniffRules, configref.SniffRule{Tag: tag, Rule: desired})
	} else if index >= 0 {
		adjustRoutePositions(draft, len(rules), index, false)
		rules = append(rules[:index], rules[index+1:]...)
	}
	if rules == nil {
		rules = []json.RawMessage{}
	}
	route["rules"] = rawValue(rules)
	setModule(draft, "route", rawValue(route))
	return nil
}

func adjustRoutePositions(draft *profile.Draft, activeCount, index int, insert bool) {
	state := object(draft.EditorState["route"])
	if state == nil {
		return
	}
	var disabled []rawObject
	if json.Unmarshal(state["disabledRules"], &disabled) != nil {
		return
	}
	positions := map[int]bool{}
	for _, r := range disabled {
		var p int
		_ = json.Unmarshal(r["position"], &p)
		positions[p] = true
	}
	position, seen := 0, 0
	for ; position < activeCount+len(disabled); position++ {
		if positions[position] {
			continue
		}
		if seen == index {
			break
		}
		seen++
	}
	var cursor int
	_ = json.Unmarshal(state["insertionPosition"], &cursor)
	if insert && position <= cursor {
		cursor++
	}
	if !insert && position < cursor {
		cursor--
	}
	state["insertionPosition"] = rawValue(cursor)
	for _, r := range disabled {
		var p int
		_ = json.Unmarshal(r["position"], &p)
		if insert && p >= position {
			p++
		}
		if !insert && p > position {
			p--
		}
		r["position"] = rawValue(p)
	}
	state["disabledRules"] = rawValue(disabled)
	draft.EditorState["route"] = rawValue(state)
}
