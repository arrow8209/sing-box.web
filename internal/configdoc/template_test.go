package configdoc

import (
	"bytes"
	"encoding/json"
	"testing"

	"singbox-web/internal/assets"
)

func TestTemplateNewPreservesConfigAndRotatesCredentials(t *testing.T) {
	template, err := LoadTemplate(assets.DefaultConfig)
	if err != nil {
		t.Fatal(err)
	}
	first, err := template.New(bytes.NewReader(bytes.Repeat([]byte{1}, 128)))
	if err != nil {
		t.Fatal(err)
	}
	second, err := template.New(bytes.NewReader(bytes.Repeat([]byte{2}, 128)))
	if err != nil {
		t.Fatal(err)
	}
	var a, b map[string]any
	if err := json.Unmarshal(first, &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(second, &b); err != nil {
		t.Fatal(err)
	}
	aClash := a["experimental"].(map[string]any)["clash_api"].(map[string]any)
	bClash := b["experimental"].(map[string]any)["clash_api"].(map[string]any)
	aCache := a["experimental"].(map[string]any)["cache_file"].(map[string]any)
	bCache := b["experimental"].(map[string]any)["cache_file"].(map[string]any)
	if aClash["secret"] == bClash["secret"] || len(aClash["secret"].(string)) != 64 {
		t.Fatalf("secrets were not independently rotated")
	}
	if aCache["cache_id"] == bCache["cache_id"] {
		t.Fatalf("cache IDs were not independently rotated")
	}
	for _, key := range []string{"log", "experimental", "inbounds", "outbounds", "route", "dns"} {
		if a[key] == nil {
			t.Fatalf("missing top-level key %q", key)
		}
	}
}
