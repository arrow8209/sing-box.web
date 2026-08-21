package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestTransformExtractsRemoteRuleSets(t *testing.T) {
	input := []byte(`{"route":{"rule_set":[{"type":"remote","tag":"GeoSite-CN","format":"binary","url":"https://example.test/cn.srs","download_detour":"direct"}],"rules":[{"rule_set":"GeoSite-CN","outbound":"direct"}]},"experimental":{"clash_api":{"secret":"old"},"cache_file":{"cache_id":"old"}}}`)
	result, err := Transform(input)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(result.Profile, []byte(`"type": "remote"`)) {
		t.Fatal("remote rule leaked into profile")
	}
	if bytes.Contains(result.Resources, []byte("download_detour")) {
		t.Fatal("download_detour leaked into resource template")
	}
	if bytes.Contains(result.Profile, []byte(`"secret": "old"`)) || bytes.Contains(result.Profile, []byte(`"cache_id": "old"`)) {
		t.Fatal("source identifiers leaked into tracked template")
	}
	assertJSONPath(t, result.Resources, "0.url", "https://example.test/cn.srs")
	assertJSONPath(t, result.Bindings, "0.tag", "GeoSite-CN")
}

func TestTransformRejectsEmbeddedCredentialsOutsideGeneratedFields(t *testing.T) {
	input := []byte(`{"outbounds":[{"type":"test","tag":"node-a","password":"real-value-must-not-be-embedded"}],"experimental":{"clash_api":{"secret":"old"},"cache_file":{"cache_id":"old"}}}`)
	if _, err := Transform(input); err == nil || !strings.Contains(err.Error(), "outbounds[0].password") {
		t.Fatalf("err=%v", err)
	}
}

func assertJSONPath(t *testing.T, payload []byte, path, want string) {
	t.Helper()
	var values []map[string]any
	if err := json.Unmarshal(payload, &values); err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(path, ".")
	if len(parts) != 2 || parts[0] != "0" {
		t.Fatalf("unsupported path: %s", path)
	}
	index := 0
	key := parts[1]
	if got, _ := values[index][key].(string); got != want {
		t.Fatalf("%s=%q want %q", path, got, want)
	}
}
