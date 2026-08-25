package configgen

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"singbox-web/internal/apierror"
	"singbox-web/internal/profile"
	"singbox-web/internal/rules"
)

const semanticStage = "semantic_check"

var objectModules = map[string]bool{
	"log": true, "dns": true, "ntp": true, "certificate": true,
	"route": true, "experimental": true,
}

var managedModules = func() map[string]bool {
	result := make(map[string]bool, len(modules)+1)
	result["$schema"] = true
	for _, module := range modules {
		result[module.Key] = true
	}
	return result
}()

// ValidateSemantic checks the small, stable subset of relationships that the
// application owns. Protocol-specific validation remains sing-box's job.
func ValidateSemantic(draft profile.Draft, snapshot rules.Snapshot) []apierror.Error {
	collector := &semanticCollector{}
	parsed := parseDraft(collector, draft)
	validateContainers(collector, parsed)
	tags := collectTags(collector, parsed)
	validateOutboundReferences(collector, parsed, tags.outbounds)
	validateDNSReferences(collector, parsed, tags.dnsServers)
	validateRuleSetReferences(collector, parsed, draft.Bindings, snapshot, tags.ruleSets)
	return collector.sorted()
}

type semanticCollector struct{ errors []apierror.Error }

func (c *semanticCollector) add(code, message, path string) {
	c.errors = append(c.errors, apierror.Error{Code: code, Message: message, Stage: semanticStage, FieldPath: path})
}

func (c *semanticCollector) sorted() []apierror.Error {
	sort.SliceStable(c.errors, func(i, j int) bool {
		if c.errors[i].FieldPath != c.errors[j].FieldPath {
			return c.errors[i].FieldPath < c.errors[j].FieldPath
		}
		return c.errors[i].Code < c.errors[j].Code
	})
	return c.errors
}

func parseDraft(c *semanticCollector, draft profile.Draft) map[string]any {
	parsed := make(map[string]any, len(draft.Config))
	for key, raw := range draft.Config {
		if !managedModules[key] || !json.Valid(raw) {
			continue
		}
		var value any
		if json.Unmarshal(raw, &value) == nil {
			parsed[key] = value
		}
	}
	for key, text := range draft.Buffers {
		if !managedModules[key] {
			c.add("unknown_module", "不支持的配置模块", key)
			continue
		}
		if strings.TrimSpace(text) == "" {
			delete(parsed, key)
			continue
		}
		var value any
		if err := json.Unmarshal([]byte(text), &value); err != nil {
			c.add("invalid_json_buffer", "配置缓冲不是有效 JSON", key)
			continue
		}
		parsed[key] = value
	}
	return parsed
}

func validateContainers(c *semanticCollector, parsed map[string]any) {
	for key, value := range parsed {
		if key == "$schema" || value == nil {
			continue
		}
		if objectModules[key] {
			if _, ok := value.(map[string]any); !ok {
				c.add("invalid_module_container", "模块必须是对象", key)
			}
			continue
		}
		if _, ok := value.([]any); !ok {
			c.add("invalid_module_container", "模块必须是数组", key)
		}
	}
}

type collectedTags struct {
	outbounds  map[string]struct{}
	dnsServers map[string]struct{}
	ruleSets   map[string]struct{}
}

func collectTags(c *semanticCollector, parsed map[string]any) collectedTags {
	tags := collectedTags{outbounds: map[string]struct{}{}, dnsServers: map[string]struct{}{}, ruleSets: map[string]struct{}{}}
	collectArrayTags(c, parsed["inbounds"], "inbounds", nil)
	collectArrayTags(c, parsed["outbounds"], "outbounds", tags.outbounds)
	collectArrayTags(c, parsed["services"], "services", nil)
	if dns, ok := parsed["dns"].(map[string]any); ok {
		collectArrayTags(c, dns["servers"], "dns.servers", tags.dnsServers)
	}
	if route, ok := parsed["route"].(map[string]any); ok {
		collectRuleSetTags(c, route["rule_set"], tags.ruleSets)
	}
	return tags
}

func collectArrayTags(c *semanticCollector, value any, path string, target map[string]struct{}) {
	items, ok := value.([]any)
	if !ok {
		return
	}
	seen := map[string]struct{}{}
	for i, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			c.add("invalid_item", "数组项目必须是对象", indexed(path, i))
			continue
		}
		validateTypeAndTag(c, object, indexed(path, i))
		tag, ok := object["tag"].(string)
		if !ok || tag == "" {
			continue
		}
		if _, duplicate := seen[tag]; duplicate {
			c.add("duplicate_tag", "tag 不能重复", indexed(path, i)+".tag")
			continue
		}
		seen[tag] = struct{}{}
		if target != nil {
			target[tag] = struct{}{}
		}
	}
}

func collectRuleSetTags(c *semanticCollector, value any, target map[string]struct{}) {
	items, ok := value.([]any)
	if !ok {
		return
	}
	for i, item := range items {
		path := indexed("route.rule_set", i)
		object, ok := item.(map[string]any)
		if !ok {
			c.add("invalid_item", "规则集项目必须是对象", path)
			continue
		}
		validateTypeAndTag(c, object, path)
		typeName, _ := object["type"].(string)
		if typeName == "remote" {
			c.add("remote_rule_set_forbidden", "不允许使用远程规则集", path+".type")
		}
		tag, _ := object["tag"].(string)
		if tag == "" {
			continue
		}
		if _, duplicate := target[tag]; duplicate {
			c.add("duplicate_tag", "规则集 tag 不能重复", path+".tag")
			continue
		}
		target[tag] = struct{}{}
	}
}

func validateTypeAndTag(c *semanticCollector, object map[string]any, path string) {
	if value, ok := object["type"].(string); !ok || value == "" {
		c.add("required_field", "type 为必填字段", path+".type")
	}
	if value, ok := object["tag"].(string); !ok || value == "" {
		c.add("required_field", "tag 为必填字段", path+".tag")
	}
}

func validateOutboundReferences(c *semanticCollector, parsed map[string]any, outbounds map[string]struct{}) {
	if route, ok := parsed["route"].(map[string]any); ok {
		validateOutbound(c, route["final"], "route.final", outbounds)
		if rules, ok := route["rules"].([]any); ok {
			for i, rule := range rules {
				if object, ok := rule.(map[string]any); ok {
					validateOutbound(c, object["outbound"], indexed("route.rules", i)+".outbound", outbounds)
				}
			}
		}
	}
	if list, ok := parsed["outbounds"].([]any); ok {
		for i, item := range list {
			if object, ok := item.(map[string]any); ok {
				validateOutbound(c, object["detour"], indexed("outbounds", i)+".detour", outbounds)
				if nested, ok := object["outbounds"].([]any); ok {
					for j, tag := range nested {
						validateOutbound(c, tag, fmt.Sprintf("outbounds[%d].outbounds[%d]", i, j), outbounds)
					}
				}
			}
		}
	}
	if list, ok := parsed["inbounds"].([]any); ok {
		for i, item := range list {
			if object, ok := item.(map[string]any); ok {
				validateOutbound(c, object["detour"], indexed("inbounds", i)+".detour", outbounds)
			}
		}
	}
}

func validateOutbound(c *semanticCollector, value any, path string, known map[string]struct{}) {
	tag, exists := value.(string)
	if !exists || tag == "" {
		return
	}
	if _, ok := known[tag]; !ok {
		c.add("unknown_outbound", "引用的出站不存在", path)
	}
}

func validateDNSReferences(c *semanticCollector, parsed map[string]any, servers map[string]struct{}) {
	dns, ok := parsed["dns"].(map[string]any)
	if !ok {
		return
	}
	validateDNSServer(c, dns["final"], "dns.final", servers)
	rules, ok := dns["rules"].([]any)
	if !ok {
		return
	}
	for i, rule := range rules {
		object, ok := rule.(map[string]any)
		if !ok {
			continue
		}
		path := indexed("dns.rules", i)
		validateDNSServer(c, object["server"], path+".server", servers)
		if action, ok := object["action"].(map[string]any); ok {
			validateDNSServer(c, action["server"], path+".action.server", servers)
		}
	}
}

func validateDNSServer(c *semanticCollector, value any, path string, known map[string]struct{}) {
	tag, exists := value.(string)
	if !exists || tag == "" {
		return
	}
	if _, ok := known[tag]; !ok {
		c.add("unknown_dns_server", "引用的 DNS 服务器不存在", path)
	}
}

func validateRuleSetReferences(c *semanticCollector, parsed map[string]any, bindings []profile.RuleBinding, snapshot rules.Snapshot, ruleSets map[string]struct{}) {
	available := make(map[string]struct{}, len(ruleSets)+len(bindings))
	for tag := range ruleSets {
		available[tag] = struct{}{}
	}
	seen := map[string]struct{}{}
	for i, binding := range bindings {
		if !binding.Enabled {
			continue
		}
		path := indexed("bindings", i)
		if binding.Tag == "" {
			c.add("required_field", "tag 为必填字段", path+".tag")
		} else {
			if _, duplicate := seen[binding.Tag]; duplicate {
				c.add("duplicate_tag", "规则绑定 tag 不能重复", path+".tag")
			}
			if _, duplicate := ruleSets[binding.Tag]; duplicate {
				c.add("duplicate_tag", "规则绑定 tag 与草稿规则集重复", path+".tag")
			}
			seen[binding.Tag] = struct{}{}
			available[binding.Tag] = struct{}{}
		}
		resource, exists := snapshot.Resources[binding.ResourceID]
		if !exists || snapshot.Digests[binding.ResourceID] == "" || resource.LastSuccessAt == nil {
			c.add("unknown_rule_resource", "规则绑定资源不在有效快照中", path+".resourceId")
		}
	}
	route, ok := parsed["route"].(map[string]any)
	if !ok {
		return
	}
	rules, ok := route["rules"].([]any)
	if !ok {
		return
	}
	for i, rule := range rules {
		object, ok := rule.(map[string]any)
		if !ok {
			continue
		}
		path := indexed("route.rules", i) + ".rule_set"
		switch value := object["rule_set"].(type) {
		case string:
			validateRuleSet(c, value, path, available)
		case []any:
			for j, tag := range value {
				if value, ok := tag.(string); ok {
					validateRuleSet(c, value, fmt.Sprintf("%s[%d]", path, j), available)
				}
			}
		}
	}
}

func validateRuleSet(c *semanticCollector, tag, path string, known map[string]struct{}) {
	if tag == "" {
		return
	}
	if _, ok := known[tag]; !ok {
		c.add("unknown_rule_set", "引用的规则集不存在", path)
	}
}

func indexed(path string, index int) string { return fmt.Sprintf("%s[%d]", path, index) }
