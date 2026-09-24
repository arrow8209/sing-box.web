package templates

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func newTestFactory(t *testing.T) *Factory {
	t.Helper()
	random := append(bytes.Repeat([]byte{0x11}, 48), bytes.Repeat([]byte{0x22}, 48)...)
	factory, err := NewFactory(bytes.NewReader(random))
	if err != nil {
		t.Fatal(err)
	}
	return factory
}

func TestFactoryCreatesUniqueSecretsAndCacheIDs(t *testing.T) {
	factory := newTestFactory(t)
	first, err := factory.NewProfileSeed(time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	second, err := factory.NewProfileSeed(time.Unix(2, 0))
	if err != nil {
		t.Fatal(err)
	}
	if first.ClashSecret == second.ClashSecret || len(first.ClashSecret) != 64 {
		t.Fatalf("secrets are not unique 32-byte hex values")
	}
	if first.CacheID == second.CacheID || len(first.CacheID) != 35 || first.CacheID[:3] != "ID_" {
		t.Fatalf("cache IDs are not unique: %q %q", first.CacheID, second.CacheID)
	}
}

func TestDefaultProfileUsesClashAPIPort50024(t *testing.T) {
	seed, err := newTestFactory(t).NewProfileSeed(time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	var experimental struct {
		ClashAPI struct {
			ExternalController string `json:"external_controller"`
		} `json:"clash_api"`
	}
	if err := json.Unmarshal(seed.Config["experimental"], &experimental); err != nil {
		t.Fatal(err)
	}
	if experimental.ClashAPI.ExternalController != "127.0.0.1:50024" {
		t.Fatalf("default Clash API controller=%q, want 127.0.0.1:50024", experimental.ClashAPI.ExternalController)
	}
}

func TestFactoryDoesNotReadTempAtRuntime(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	if err := os.Chdir(directory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	factory, err := NewFactory(bytes.NewReader(bytes.Repeat([]byte{0x22}, 48)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := factory.NewProfileSeed(time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultProfileIncludesProtectedFakeIPRule(t *testing.T) {
	seed, err := newTestFactory(t).NewProfileSeed(time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	var dns struct {
		Rules []struct {
			Action string           `json:"action"`
			Type   string           `json:"type"`
			Mode   string           `json:"mode"`
			Server string           `json:"server"`
			Rules  []map[string]any `json:"rules"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(seed.Config["dns"], &dns); err != nil {
		t.Fatal(err)
	}
	if len(dns.Rules) != 5 {
		t.Fatalf("default DNS rules=%d, want 5", len(dns.Rules))
	}
	fakeIP := dns.Rules[3]
	if fakeIP.Action != "route" || fakeIP.Type != "logical" || fakeIP.Mode != "and" || fakeIP.Server != "Fake-IP" || len(fakeIP.Rules) != 2 {
		t.Fatalf("invalid default Fake-IP rule: %+v", fakeIP)
	}
	domainSuffixes, ok := fakeIP.Rules[0]["domain_suffix"].([]any)
	if !ok || len(domainSuffixes) != 10 || fakeIP.Rules[0]["invert"] != true {
		t.Fatalf("invalid Fake-IP exclusions: %+v", fakeIP.Rules[0])
	}
}
