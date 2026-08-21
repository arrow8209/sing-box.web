package configgen

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"singbox-web/internal/apierror"
	"singbox-web/internal/profile"
	"singbox-web/internal/rules"
)

// Generator produces only the fixed modules listed in modules.
type Generator struct{}

func NewGenerator() *Generator { return &Generator{} }

// Generate creates stable JSON files from an immutable profile draft and a
// rules snapshot. The snapshot must remain locked by the caller until this
// method returns.
func (g *Generator) Generate(ctx context.Context, draft profile.Draft, snapshot rules.Snapshot) (Output, error) {
	if err := ctx.Err(); err != nil {
		return Output{}, err
	}
	config := cloneRawMap(draft.Config)
	route, err := injectLocalRuleSets(config["route"], draft.Bindings, snapshot)
	if err != nil {
		return Output{}, err
	}
	if route != nil {
		config["route"] = route
	}

	files := make([]File, 0, len(modules))
	for _, module := range modules {
		if err := ctx.Err(); err != nil {
			return Output{}, err
		}
		value, exists := config[module.Key]
		if !exists || isJSONNull(value) {
			continue
		}
		content, err := stableModuleJSON(module.Key, value)
		if err != nil {
			return Output{}, apierror.Field("invalid_module_json", "模块 JSON 无效", "generate", module.Key, err)
		}
		files = append(files, File{Name: module.FileName, Content: content})
	}
	digests := maps.Clone(snapshot.Digests)
	if digests == nil {
		digests = map[string]string{}
	}
	return Output{Files: files, Digest: digestOutput(files, digests), ResourceDigests: digests}, nil
}

func cloneRawMap(source map[string]json.RawMessage) map[string]json.RawMessage {
	result := make(map[string]json.RawMessage, len(source))
	for key, value := range source {
		result[key] = append(json.RawMessage(nil), value...)
	}
	return result
}

func isJSONNull(value json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(value), []byte("null"))
}

func stableModuleJSON(key string, raw json.RawMessage) (json.RawMessage, error) {
	value, err := decodeJSON(raw)
	if err != nil {
		return nil, err
	}
	encoded, err := json.MarshalIndent(map[string]any{key: value}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func decodeJSON(raw []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("JSON 包含多个值")
		}
		return nil, err
	}
	return value, nil
}

func injectLocalRuleSets(raw json.RawMessage, bindings []profile.RuleBinding, snapshot rules.Snapshot) (json.RawMessage, error) {
	hasEnabledBinding := false
	for _, binding := range bindings {
		if binding.Enabled {
			hasEnabledBinding = true
			break
		}
	}
	if len(raw) == 0 || isJSONNull(raw) {
		if !hasEnabledBinding {
			return nil, nil
		}
		raw = json.RawMessage(`{}`)
	}
	routeValue, err := decodeJSON(raw)
	if err != nil {
		return nil, apierror.Field("invalid_route_json", "路由模块 JSON 无效", "generate", "route", err)
	}
	route, ok := routeValue.(map[string]any)
	if !ok {
		return nil, apierror.Field("invalid_route_json", "路由模块必须是对象", "generate", "route", nil)
	}

	knownTags := make(map[string]struct{})
	if existing, exists := route["rule_set"]; exists && existing != nil {
		items, ok := existing.([]any)
		if !ok {
			return nil, apierror.Field("invalid_rule_set", "route.rule_set 必须是数组", "generate", "route.rule_set", nil)
		}
		for index, item := range items {
			ruleSet, ok := item.(map[string]any)
			if !ok {
				return nil, apierror.Field("invalid_rule_set", "规则集必须是对象", "generate", fmt.Sprintf("route.rule_set[%d]", index), nil)
			}
			typeValue, ok := ruleSet["type"].(string)
			if !ok {
				return nil, apierror.Field("invalid_rule_set", "规则集类型无效", "generate", fmt.Sprintf("route.rule_set[%d].type", index), nil)
			}
			if typeValue == "remote" {
				return nil, apierror.Field("remote_rule_set_forbidden", "不允许使用远程规则集", "generate", fmt.Sprintf("route.rule_set[%d].type", index), nil)
			}
			if typeValue != "local" && typeValue != "inline" {
				return nil, apierror.Field("invalid_rule_set", "规则集类型必须是 local 或 inline", "generate", fmt.Sprintf("route.rule_set[%d].type", index), nil)
			}
			tag, ok := ruleSet["tag"].(string)
			if !ok || tag == "" {
				return nil, apierror.Field("invalid_rule_set", "规则集 tag 无效", "generate", fmt.Sprintf("route.rule_set[%d].tag", index), nil)
			}
			if _, duplicate := knownTags[tag]; duplicate {
				return nil, apierror.Field("duplicate_rule_set_tag", "规则集 tag 重复", "generate", fmt.Sprintf("route.rule_set[%d].tag", index), nil)
			}
			knownTags[tag] = struct{}{}
		}
	}

	items, _ := route["rule_set"].([]any)
	for index, binding := range bindings {
		if !binding.Enabled {
			continue
		}
		if binding.Tag == "" {
			return nil, apierror.Field("invalid_rule_binding", "规则绑定 tag 无效", "generate", fmt.Sprintf("bindings[%d].tag", index), nil)
		}
		if _, duplicate := knownTags[binding.Tag]; duplicate {
			return nil, apierror.Field("duplicate_rule_set_tag", "规则集 tag 重复", "generate", fmt.Sprintf("bindings[%d].tag", index), nil)
		}
		resource, exists := snapshot.Resources[binding.ResourceID]
		if !exists {
			return nil, apierror.Field("rule_resource_not_found", "规则绑定资源不在应用快照中", "generate", fmt.Sprintf("bindings[%d].resourceId", index), nil)
		}
		path, err := expectedRulePath(resource)
		if err != nil {
			return nil, apierror.Field("invalid_rule_resource_path", "规则资源路径与快照不匹配", "generate", fmt.Sprintf("bindings[%d].resourceId", index), err)
		}
		items = append(items, map[string]any{"type": "local", "tag": binding.Tag, "format": string(resource.Format), "path": path})
		knownTags[binding.Tag] = struct{}{}
	}
	if len(items) > 0 {
		route["rule_set"] = items
	}
	encoded, err := json.Marshal(route)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func expectedRulePath(resource rules.Resource) (string, error) {
	extension := ".srs"
	if resource.Format == rules.FormatSource {
		extension = ".json"
	} else if resource.Format != rules.FormatBinary {
		return "", fmt.Errorf("规则资源格式无效")
	}
	if len(resource.ID) != 32 {
		return "", fmt.Errorf("规则资源 ID 无效")
	}
	for _, character := range resource.ID {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return "", fmt.Errorf("规则资源 ID 无效")
		}
	}
	expected := "data/rulesets/" + resource.ID + extension
	actual := filepath.ToSlash(filepath.Clean(resource.LocalPath))
	if actual != expected && (!filepath.IsAbs(resource.LocalPath) || !strings.HasSuffix(actual, "/"+expected)) {
		return "", fmt.Errorf("规则资源路径不是固定路径")
	}
	return expected, nil
}

func digestOutput(files []File, digests map[string]string) string {
	hash := sha256.New()
	ordered := append([]File(nil), files...)
	slices.SortFunc(ordered, func(left, right File) int { return strings.Compare(left.Name, right.Name) })
	for _, file := range ordered {
		_, _ = hash.Write([]byte(file.Name))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write(file.Content)
		_, _ = hash.Write([]byte{0})
	}
	ids := make([]string, 0, len(digests))
	for id := range digests {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		_, _ = hash.Write([]byte(id))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(digests[id]))
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}
