package profile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"

	"singbox-web/internal/configref"
)

// migrateLegacyDraft converts only entries with explicit ownership. Unknown
// JSON tokens and standalone entries remain raw input. It is called on save or
// copy, never while opening a profile.
func migrateLegacyDraft(source Draft) (Draft, error) {
	if source.SchemaVersion == 2 {
		return cloneDraft(source), nil
	}
	model, err := configref.DecodeLegacy(source.EditorState)
	if err != nil {
		return Draft{}, err
	}
	result := cloneDraft(source)
	positions := map[string]int{}
	if model.Outbounds != nil {
		items, err := migrationArray(result.Buffers["outbounds"], "outbounds")
		if err != nil {
			return Draft{}, err
		}
		generated := map[string]bool{}
		for _, tag := range model.Outbounds.GeneratedTags {
			generated[tag] = true
		}
		groups := []configref.Group{}
		uiGroups := []map[string]any{}
		for _, group := range model.Outbounds.Groups {
			index := migrationTag(items, group.Tag)
			if index < 0 {
				continue
			}
			fields := migrationObject(items[index])
			if migrationString(fields["type"]) != group.Type {
				return Draft{}, configref.Error("reference_conflict", fmt.Sprintf("outbounds[%d].type", index), "策略组类型与引用不一致")
			}
			var members []string
			_ = json.Unmarshal(fields["outbounds"], &members)
			if !slices.Equal(members, group.RuntimeTags) {
				return Draft{}, configref.Error("reference_conflict", fmt.Sprintf("outbounds[%d].outbounds", index), "策略组成员与引用不一致")
			}
			for _, ref := range group.References {
				if ref.Kind == "subscription" || ref.Kind == "subscription-node" {
					for _, tag := range ref.ResolvedTags {
						if migrationTag(items, tag) >= 0 && !generated[tag] {
							return Draft{}, configref.Error("reference_ownership_missing", "editorState.outbounds.generatedTags", "订阅节点缺少生成归属")
						}
					}
				}
			}
			delete(fields, "outbounds")
			items[index] = migrationRaw(fields)
			groups = append(groups, group)
			uiGroups = append(uiGroups, map[string]any{"id": group.ID, "tag": group.Tag, "hidden": group.Hidden, "icon": group.Icon})
		}
		model.Outbounds.Groups = groups
		items = slices.DeleteFunc(items, func(raw json.RawMessage) bool { return generated[migrationString(migrationObject(raw)["tag"])] })
		result.Buffers["outbounds"] = string(migrationRaw(items))
		result.EditorState["outbounds"] = migrationRaw(map[string]any{"version": 2, "groups": uiGroups})
	}
	if model.Inbounds != nil {
		items, err := migrationArray(result.Buffers["inbounds"], "inbounds")
		if err != nil {
			return Draft{}, err
		}
		linked := map[string]bool{}
		for _, binding := range model.Inbounds.ResourceBindings {
			if index := migrationTag(items, binding.Tag); index >= 0 {
				positions[binding.Tag] = index
				linked[binding.Tag] = true
			}
		}
		model.Inbounds.ResourceBindings = slices.DeleteFunc(model.Inbounds.ResourceBindings, func(b configref.InboundBinding) bool { return !linked[b.Tag] })
		items = slices.DeleteFunc(items, func(raw json.RawMessage) bool { return linked[migrationString(migrationObject(raw)["tag"])] })
		result.Buffers["inbounds"] = string(migrationRaw(items))
		managedLinked := slices.DeleteFunc(slices.Clone(model.Inbounds.ManagedSniffRules), func(rule configref.SniffRule) bool { return !linked[rule.Tag] })
		if len(managedLinked) > 0 {
			route := migrationObject(json.RawMessage(result.Buffers["route"]))
			if route == nil {
				return Draft{}, configref.Error("reference_conflict", "route", "路由结构无效，不能迁移托管嗅探")
			}
			var rules []json.RawMessage
			if err := json.Unmarshal(route["rules"], &rules); err != nil {
				return Draft{}, configref.Error("reference_conflict", "route.rules", "路由规则结构无效")
			}
			removed := []int{}
			for _, managed := range managedLinked {
				matches := []int{}
				for i, rule := range rules {
					if migrationEqual(rule, managed.Rule) {
						matches = append(matches, i)
					}
				}
				if len(matches) != 1 {
					return Draft{}, configref.Error("reference_ownership_missing", "editorState.inbounds.managedSniffRules", "托管嗅探规则归属无法确认")
				}
				removed = append(removed, matches[0])
			}
			slices.Sort(removed)
			if routeState := result.EditorState["route"]; len(routeState) > 0 {
				result.EditorState["route"] = migrationRemoveRoutePositions(routeState, len(rules), removed)
			}
			for i := len(removed) - 1; i >= 0; i-- {
				rules = slices.Delete(rules, removed[i], removed[i]+1)
			}
			route["rules"] = migrationRaw(rules)
			result.Buffers["route"] = string(migrationRaw(route))
		}
		delete(result.EditorState, "inbounds")
	}
	persisted := configref.FromLegacy(model, positions)
	if err := persisted.Validate(); err != nil {
		return Draft{}, err
	}
	result.SchemaVersion = 2
	result.References = &persisted
	config, err := reconcileBuffers(result.Config, result.Buffers)
	if err != nil {
		return Draft{}, err
	}
	result.Config = config
	return result, nil
}

func migrationRemoveRoutePositions(raw json.RawMessage, activeCount int, removed []int) json.RawMessage {
	state := migrationObject(raw)
	if state == nil {
		return raw
	}
	var disabled []map[string]json.RawMessage
	if json.Unmarshal(state["disabledRules"], &disabled) != nil {
		return raw
	}
	for i := len(removed) - 1; i >= 0; i-- {
		index := removed[i]
		disabledPositions := map[int]bool{}
		for _, entry := range disabled {
			var position int
			_ = json.Unmarshal(entry["position"], &position)
			disabledPositions[position] = true
		}
		combined, seen := 0, 0
		for ; combined < activeCount+len(disabled); combined++ {
			if disabledPositions[combined] {
				continue
			}
			if seen == index {
				break
			}
			seen++
		}
		var cursor int
		_ = json.Unmarshal(state["insertionPosition"], &cursor)
		if combined < cursor {
			cursor--
		}
		state["insertionPosition"] = migrationRaw(cursor)
		for _, entry := range disabled {
			var position int
			_ = json.Unmarshal(entry["position"], &position)
			if position > combined {
				entry["position"] = migrationRaw(position - 1)
			}
		}
		activeCount--
	}
	state["disabledRules"] = migrationRaw(disabled)
	return migrationRaw(state)
}

// applyV2Override accepts an explicit user edit performed on a v1 draft. The
// old metadata identifies removable copies; the submitted v2 model becomes
// authoritative after those copies have been removed.
func applyV2Override(draft *Draft, submitted configref.Persisted) error {
	if err := submitted.Validate(); err != nil {
		return err
	}
	for _, module := range []string{"inbounds", "outbounds"} {
		items, err := migrationArray(draft.Buffers[module], module)
		if err != nil {
			return err
		}
		if module == "inbounds" {
			linked := map[string]bool{}
			for _, b := range submitted.Inbounds {
				linked[b.Tag] = true
			}
			items = slices.DeleteFunc(items, func(raw json.RawMessage) bool { return linked[migrationString(migrationObject(raw)["tag"])] })
		} else {
			groups := map[string]bool{}
			for _, g := range submitted.Outbounds {
				groups[g.Tag] = true
			}
			for i, raw := range items {
				fields := migrationObject(raw)
				if groups[migrationString(fields["tag"])] {
					delete(fields, "outbounds")
					items[i] = migrationRaw(fields)
				}
			}
		}
		draft.Buffers[module] = string(migrationRaw(items))
	}
	copy := submitted.Clone()
	draft.References = &copy
	config, err := reconcileBuffers(draft.Config, draft.Buffers)
	if err != nil {
		return err
	}
	draft.Config = config
	return nil
}

func migrationArray(text, path string) ([]json.RawMessage, error) {
	var items []json.RawMessage
	if text == "" {
		return []json.RawMessage{}, nil
	}
	if json.Unmarshal([]byte(text), &items) != nil || items == nil {
		return nil, configref.Error("reference_conflict", path, "模块必须是数组")
	}
	return items, nil
}
func migrationObject(raw json.RawMessage) map[string]json.RawMessage {
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	return fields
}
func migrationString(raw json.RawMessage) string {
	var value string
	_ = json.Unmarshal(raw, &value)
	return value
}
func migrationTag(items []json.RawMessage, tag string) int {
	for i, item := range items {
		if migrationString(migrationObject(item)["tag"]) == tag {
			return i
		}
	}
	return -1
}
func migrationRaw(value any) json.RawMessage { raw, _ := json.Marshal(value); return raw }
func migrationEqual(left, right json.RawMessage) bool {
	decode := func(raw json.RawMessage) any {
		d := json.NewDecoder(bytes.NewReader(raw))
		d.UseNumber()
		var value any
		_ = d.Decode(&value)
		return value
	}
	return reflect.DeepEqual(decode(left), decode(right))
}
