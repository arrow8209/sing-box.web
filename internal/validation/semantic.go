package validation

import (
	"bytes"
	"encoding/json"
)

var semanticMessages = map[string]string{
	"duplicate_tag":      "标签重复",
	"missing_outbound":   "引用的出站不存在",
	"missing_dns_server": "引用的 DNS 服务器不存在",
	"missing_rule_set":   "引用的规则集不存在",
}

func ValidateSemantics(config json.RawMessage) []Issue {
	decoder := json.NewDecoder(bytes.NewReader(config))
	decoder.UseNumber()
	var root map[string]any
	if err := decoder.Decode(&root); err != nil {
		return nil
	}

	issues := make([]Issue, 0)
	collectTags(root["inbounds"], "/inbounds", &issues)
	outboundTags := collectTags(root["outbounds"], "/outbounds", &issues)

	dns, _ := root["dns"].(map[string]any)
	dnsTags := collectTags(dns["servers"], "/dns/servers", &issues)
	route, _ := root["route"].(map[string]any)
	ruleSetTags := collectTags(route["rule_set"], "/route/rule_set", &issues)

	checkOutboundReferences(root["outbounds"], outboundTags, &issues)
	checkStringReference(route["final"], "/route/final", outboundTags, "missing_outbound", &issues)
	checkRouteRules(route["rules"], outboundTags, ruleSetTags, &issues)
	checkStringReference(dns["final"], "/dns/final", dnsTags, "missing_dns_server", &issues)
	checkDNSRules(dns["rules"], dnsTags, &issues)
	checkDomainResolvers(dns, "/dns", dnsTags, &issues)
	checkDomainResolver(route["default_domain_resolver"], "/route/default_domain_resolver", dnsTags, &issues)

	sortIssues(issues)
	return issues
}

func collectTags(value any, basePath string, issues *[]Issue) map[string]struct{} {
	tags := make(map[string]struct{})
	items, _ := value.([]any)
	for index, item := range items {
		object, _ := item.(map[string]any)
		tag, _ := object["tag"].(string)
		if tag == "" {
			continue
		}
		if _, exists := tags[tag]; exists {
			addSemanticIssue(issues, basePath+"/"+indexString(index)+"/tag", "duplicate_tag")
			continue
		}
		tags[tag] = struct{}{}
	}
	return tags
}

func checkOutboundReferences(value any, tags map[string]struct{}, issues *[]Issue) {
	items, _ := value.([]any)
	for index, item := range items {
		object, _ := item.(map[string]any)
		basePath := "/outbounds/" + indexString(index)
		outboundType, _ := object["type"].(string)
		if outboundType == "selector" || outboundType == "urltest" {
			members, _ := object["outbounds"].([]any)
			for memberIndex, member := range members {
				checkStringReference(member, basePath+"/outbounds/"+indexString(memberIndex), tags, "missing_outbound", issues)
			}
		}
		checkStringReference(object["detour"], basePath+"/detour", tags, "missing_outbound", issues)
	}
}

func checkRouteRules(value any, outboundTags, ruleSetTags map[string]struct{}, issues *[]Issue) {
	rules, _ := value.([]any)
	for index, item := range rules {
		rule, _ := item.(map[string]any)
		basePath := "/route/rules/" + indexString(index)
		checkStringReference(rule["outbound"], basePath+"/outbound", outboundTags, "missing_outbound", issues)
		checkReferenceList(rule["rule_set"], basePath+"/rule_set", ruleSetTags, "missing_rule_set", issues)
	}
}

func checkDNSRules(value any, dnsTags map[string]struct{}, issues *[]Issue) {
	rules, _ := value.([]any)
	for index, item := range rules {
		rule, _ := item.(map[string]any)
		checkReferenceList(rule["server"], "/dns/rules/"+indexString(index)+"/server", dnsTags, "missing_dns_server", issues)
	}
}

func checkDomainResolvers(value any, path string, dnsTags map[string]struct{}, issues *[]Issue) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			childPath := path + "/" + escapeJSONPointerToken(key)
			if key == "domain_resolver" {
				checkDomainResolver(child, childPath, dnsTags, issues)
				continue
			}
			checkDomainResolvers(child, childPath, dnsTags, issues)
		}
	case []any:
		for index, child := range typed {
			checkDomainResolvers(child, path+"/"+indexString(index), dnsTags, issues)
		}
	}
}

func checkDomainResolver(value any, path string, dnsTags map[string]struct{}, issues *[]Issue) {
	switch typed := value.(type) {
	case string:
		checkStringReference(typed, path, dnsTags, "missing_dns_server", issues)
	case map[string]any:
		checkStringReference(typed["server"], path+"/server", dnsTags, "missing_dns_server", issues)
	}
}

func checkReferenceList(value any, path string, tags map[string]struct{}, code string, issues *[]Issue) {
	switch typed := value.(type) {
	case string:
		checkStringReference(typed, path, tags, code, issues)
	case []any:
		for index, item := range typed {
			checkStringReference(item, path+"/"+indexString(index), tags, code, issues)
		}
	}
}

func checkStringReference(value any, path string, tags map[string]struct{}, code string, issues *[]Issue) {
	reference, ok := value.(string)
	if !ok || reference == "" {
		return
	}
	if _, exists := tags[reference]; !exists {
		addSemanticIssue(issues, path, code)
	}
}

func addSemanticIssue(issues *[]Issue, path, code string) {
	*issues = append(*issues, Issue{Stage: "semantic", Path: path, Code: code, Message: semanticMessages[code]})
}

func indexString(index int) string {
	const digits = "0123456789"
	if index == 0 {
		return "0"
	}
	var buffer [20]byte
	position := len(buffer)
	for index > 0 {
		position--
		buffer[position] = digits[index%10]
		index /= 10
	}
	return string(buffer[position:])
}
