package profile

import (
	"encoding/json"
	"slices"
	"unicode/utf8"

	"singbox-web/internal/configref"
)

// applyFreeze patches the saved raw buffer under the same lock and ETag as
// SaveDraft. The candidate is signed by configgen; source services are not read.
func applyFreeze(draft Draft, operation FreezeOperation, verifier FreezeVerifier) (Draft, error) {
	bad := func(path, message string) error { return configref.Error("candidate_invalid", path, message) }
	if operation.Module != "inbounds" || operation.Tag == "" {
		return Draft{}, bad("freeze", "仅支持指定入站的候选冻结")
	}
	if operation.Action != "detach" && operation.Action != "copy" && operation.Action != "restore_saved" {
		return Draft{}, bad("freeze.action", "冻结操作无效")
	}
	if operation.Action != "restore_saved" {
		if err := verifier.VerifyFreeze(draft.ProfileID, draft.Revision, operation); err != nil {
			return Draft{}, err
		}
	}
	model, err := configref.DecodeLegacy(draft.EditorState)
	if err != nil {
		return Draft{}, err
	}
	if model.Inbounds == nil {
		return Draft{}, bad("editorState.inbounds", "入站没有来源关联")
	}
	linked := false
	for _, binding := range model.Inbounds.ResourceBindings {
		if binding.Tag == operation.Tag {
			linked = true
			break
		}
	}
	if !linked {
		return Draft{}, bad("freeze.tag", "入站已解除关联，请刷新档案")
	}
	items, err := freezeArray(draft.Buffers["inbounds"])
	if err != nil {
		return Draft{}, err
	}
	index := -1
	used := map[string]bool{}
	for i, item := range items {
		fields := freezeObject(item)
		tag := freezeString(fields["tag"])
		if tag == operation.Tag {
			index = i
		}
		used[tag] = true
	}
	if index < 0 {
		return Draft{}, bad("freeze.tag", "关联入站不存在")
	}
	if operation.Action == "copy" {
		if operation.NewTag == "" || utf8.RuneCountInString(operation.NewTag) > 128 || used[operation.NewTag] {
			return Draft{}, bad("freeze.newTag", "新入站名称为空、过长或已被使用")
		}
	}
	result := cloneDraft(draft)
	if result.EditorState == nil {
		result.EditorState = map[string]json.RawMessage{}
	}
	if result.Buffers == nil {
		result.Buffers = map[string]string{}
	}
	item := items[index]
	if operation.Action != "restore_saved" {
		if !json.Valid([]byte(operation.Text)) {
			return Draft{}, bad("freeze.text", "候选入站 JSON 无效")
		}
		fields := freezeObject(json.RawMessage(operation.Text))
		if fields == nil || freezeString(fields["tag"]) != operation.Tag {
			return Draft{}, bad("freeze.text", "候选入站名称不匹配")
		}
		item = json.RawMessage(operation.Text)
		var candidateState configref.Inbounds
		if err := json.Unmarshal([]byte(operation.InboundStateText), &candidateState); err != nil || candidateState.Version != 1 || candidateState.ManagedSniffRules == nil {
			return Draft{}, bad("freeze.inboundStateText", "候选入站归属无效")
		}
		model.Inbounds = &candidateState
		if operation.RouteText != "" {
			if !json.Valid([]byte(operation.RouteText)) {
				return Draft{}, bad("freeze.routeText", "候选路由无效")
			}
			result.Buffers["route"] = operation.RouteText
		}
	}
	if operation.Action == "copy" {
		fields := freezeObject(item)
		fields["tag"], _ = json.Marshal(operation.NewTag)
		copyText, _ := json.Marshal(fields)
		items = slices.Insert(items, index+1, json.RawMessage(copyText))
		if err := copyManagedSniff(&result, model.Inbounds, operation.Tag, operation.NewTag); err != nil {
			return Draft{}, err
		}
	} else {
		items[index] = item
		model.Inbounds.ResourceBindings = slices.DeleteFunc(model.Inbounds.ResourceBindings, func(binding configref.InboundBinding) bool { return binding.Tag == operation.Tag })
	}
	encoded, _ := json.Marshal(items)
	result.Buffers["inbounds"] = string(encoded)
	result.EditorState["inbounds"], _ = json.Marshal(model.Inbounds)
	return result, nil
}

func freezeArray(text string) ([]json.RawMessage, error) {
	var items []json.RawMessage
	if json.Unmarshal([]byte(text), &items) != nil || items == nil {
		return nil, configref.Error("candidate_invalid", "inbounds", "入站模块 JSON 无效")
	}
	return items, nil
}
func freezeObject(raw json.RawMessage) map[string]json.RawMessage {
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	return fields
}
func freezeString(raw json.RawMessage) string {
	var value string
	_ = json.Unmarshal(raw, &value)
	return value
}

func copyManagedSniff(draft *Draft, state *configref.Inbounds, original, newTag string) error {
	var managed *configref.SniffRule
	for i := range state.ManagedSniffRules {
		if state.ManagedSniffRules[i].Tag == original {
			managed = &state.ManagedSniffRules[i]
			break
		}
	}
	if managed == nil {
		return nil
	}
	rule := freezeObject(managed.Rule)
	if rule == nil {
		return configref.Error("candidate_invalid", "freeze.inboundStateText", "托管嗅探规则无效")
	}
	rule["inbound"], _ = json.Marshal(newTag)
	newRule, _ := json.Marshal(rule)
	route := freezeObject(json.RawMessage(draft.Buffers["route"]))
	if route == nil {
		return configref.Error("candidate_invalid", "route", "无法保存复制入站的嗅探规则")
	}
	var rules []json.RawMessage
	if raw, ok := route["rules"]; ok && json.Unmarshal(raw, &rules) != nil {
		return configref.Error("candidate_invalid", "route.rules", "路由规则必须是数组")
	}
	for _, existing := range rules {
		obj := freezeObject(existing)
		if freezeString(obj["action"]) == "sniff" && freezeString(obj["inbound"]) == newTag {
			return configref.Error("reference_conflict", "route.rules", "复制入站的嗅探规则已存在")
		}
	}
	rules = append([]json.RawMessage{newRule}, rules...)
	route["rules"], _ = json.Marshal(rules)
	encoded, _ := json.Marshal(route)
	draft.Buffers["route"] = string(encoded)
	state.ManagedSniffRules = append(state.ManagedSniffRules, configref.SniffRule{Tag: newTag, Rule: newRule})
	if raw := draft.EditorState["route"]; len(raw) > 0 {
		editor := freezeObject(raw)
		var insertion int
		_ = json.Unmarshal(editor["insertionPosition"], &insertion)
		editor["insertionPosition"], _ = json.Marshal(insertion + 1)
		var disabled []map[string]json.RawMessage
		_ = json.Unmarshal(editor["disabledRules"], &disabled)
		for _, d := range disabled {
			var position int
			_ = json.Unmarshal(d["position"], &position)
			d["position"], _ = json.Marshal(position + 1)
		}
		if disabled != nil {
			editor["disabledRules"], _ = json.Marshal(disabled)
		}
		draft.EditorState["route"], _ = json.Marshal(editor)
	}
	return nil
}
