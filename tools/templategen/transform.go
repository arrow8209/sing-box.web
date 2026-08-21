package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type Result struct {
	Profile   []byte
	Resources []byte
	Bindings  []byte
}

type resourceTemplate struct {
	TemplateKey    string `json:"templateKey"`
	Name           string `json:"name"`
	URL            string `json:"url"`
	Format         string `json:"format"`
	UpdateInterval string `json:"updateInterval"`
}

type bindingTemplate struct {
	TemplateKey string `json:"templateKey"`
	Tag         string `json:"tag"`
	Enabled     bool   `json:"enabled"`
}

func Transform(input []byte) (Result, error) {
	var config map[string]json.RawMessage
	if err := json.Unmarshal(input, &config); err != nil {
		return Result{}, err
	}
	if config == nil {
		return Result{}, fmt.Errorf("顶层 JSON 必须是对象")
	}
	route := make(map[string]json.RawMessage)
	if raw, ok := config["route"]; ok {
		var err error
		route, err = decodeObject(raw)
		if err != nil {
			return Result{}, fmt.Errorf("route: %w", err)
		}
	}
	remote, local, err := splitRuleSets(route["rule_set"])
	if err != nil {
		return Result{}, err
	}
	localJSON, _ := json.Marshal(local)
	route["rule_set"] = localJSON
	routeJSON, _ := json.Marshal(route)
	config["route"] = routeJSON
	if err := setTemplateString(config, []string{"experimental", "clash_api", "secret"}, ""); err != nil {
		return Result{}, err
	}
	if err := setTemplateString(config, []string{"experimental", "cache_file", "cache_id"}, "ID_template"); err != nil {
		return Result{}, err
	}
	if err := rejectNonEmptyCredentialFields(config, ""); err != nil {
		return Result{}, err
	}
	resources, bindings := makeTemplates(remote)
	return Result{Profile: stableIndent(config), Resources: stableIndent(resources), Bindings: stableIndent(bindings)}, nil
}

func decodeObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("缺少对象")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	if object == nil {
		return nil, fmt.Errorf("必须是对象")
	}
	return object, nil
}

func splitRuleSets(raw json.RawMessage) ([]map[string]json.RawMessage, []json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, nil, nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, nil, fmt.Errorf("route.rule_set: %w", err)
	}
	remote := make([]map[string]json.RawMessage, 0)
	local := make([]json.RawMessage, 0, len(items))
	for i, item := range items {
		object, err := decodeObject(item)
		if err != nil {
			return nil, nil, fmt.Errorf("route.rule_set[%d]: %w", i, err)
		}
		var kind string
		_ = json.Unmarshal(object["type"], &kind)
		if kind != "remote" {
			local = append(local, item)
			continue
		}
		for _, field := range []string{"tag", "url", "format"} {
			var value string
			if err := json.Unmarshal(object[field], &value); err != nil || value == "" {
				return nil, nil, fmt.Errorf("route.rule_set[%d].%s: 缺失或无效", i, field)
			}
		}
		var format string
		_ = json.Unmarshal(object["format"], &format)
		if format != "source" && format != "binary" {
			return nil, nil, fmt.Errorf("route.rule_set[%d].format: 不支持 %q", i, format)
		}
		remote = append(remote, object)
	}
	return remote, local, nil
}

func makeTemplates(remote []map[string]json.RawMessage) ([]resourceTemplate, []bindingTemplate) {
	resources := make([]resourceTemplate, 0, len(remote))
	bindings := make([]bindingTemplate, 0, len(remote))
	for _, item := range remote {
		var tag, url, format, interval string
		_ = json.Unmarshal(item["tag"], &tag)
		_ = json.Unmarshal(item["url"], &url)
		_ = json.Unmarshal(item["format"], &format)
		_ = json.Unmarshal(item["update_interval"], &interval)
		digest := sha256.Sum256([]byte(tag))
		key := hex.EncodeToString(digest[:])[:16]
		resources = append(resources, resourceTemplate{TemplateKey: key, Name: tag, URL: url, Format: format, UpdateInterval: interval})
		bindings = append(bindings, bindingTemplate{TemplateKey: key, Tag: tag, Enabled: true})
	}
	return resources, bindings
}

func setTemplateString(config map[string]json.RawMessage, path []string, value string) error {
	current := config
	for _, part := range path[:len(path)-1] {
		next, err := decodeObject(current[part])
		if err != nil {
			return fmt.Errorf("%s: %w", strings.Join(path, "."), err)
		}
		current = next
		encoded, _ := json.Marshal(current)
		// Parent is re-encoded by the caller's parent only after mutation; rebuild below.
		_ = encoded
	}
	// Rebuild recursively to retain changes in RawMessage maps.
	return setNested(config, path, value)
}

func setNested(current map[string]json.RawMessage, path []string, value string) error {
	if len(path) == 1 {
		raw, _ := json.Marshal(value)
		current[path[0]] = raw
		return nil
	}
	child, err := decodeObject(current[path[0]])
	if err != nil {
		return err
	}
	if err := setNested(child, path[1:], value); err != nil {
		return err
	}
	raw, _ := json.Marshal(child)
	current[path[0]] = raw
	return nil
}

func rejectNonEmptyCredentialFields(value any, path string) error {
	switch item := value.(type) {
	case map[string]json.RawMessage:
		for key, raw := range item {
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			if isCredentialKey(key) && !isAllowedEmptyCredential(raw) {
				return fmt.Errorf("%s: 不允许嵌入凭据", childPath)
			}
			var child any
			if err := json.Unmarshal(raw, &child); err != nil {
				return err
			}
			if err := rejectNonEmptyCredentialFields(child, childPath); err != nil {
				return err
			}
		}
	case map[string]any:
		for key, child := range item {
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			if isCredentialKey(key) && child != nil {
				if text, ok := child.(string); !ok || text != "" {
					return fmt.Errorf("%s: 不允许嵌入凭据", childPath)
				}
			}
			if err := rejectNonEmptyCredentialFields(child, childPath); err != nil {
				return err
			}
		}
	case []any:
		for i, child := range item {
			if err := rejectNonEmptyCredentialFields(child, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

func isCredentialKey(key string) bool {
	switch key {
	case "password", "uuid", "private_key", "token", "client_secret", "secret":
		return true
	}
	return false
}

func isAllowedEmptyCredential(raw json.RawMessage) bool {
	if string(raw) == "null" {
		return true
	}
	var value string
	return json.Unmarshal(raw, &value) == nil && value == ""
}
func stableIndent(value any) []byte {
	result, _ := json.MarshalIndent(value, "", "  ")
	return append(result, '\n')
}
