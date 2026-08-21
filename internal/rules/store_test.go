package rules

import (
	"bytes"
	"context"
	"testing"
	"time"

	"singbox-web/internal/paths"
)

func fixedClock(now time.Time) func() time.Time { return func() time.Time { return now } }

func TestEnsureTemplateResourcesAssignsRandomStableIDs(t *testing.T) {
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(layout.RulesFile, layout, fixedClock(time.Unix(1, 0)))
	if err != nil {
		t.Fatal(err)
	}
	templates := []ResourceTemplate{{TemplateKey: "template-cn", Name: "GeoSite-CN", URL: "https://example.test/cn.srs", Format: FormatBinary}}
	first, err := store.EnsureTemplateResources(context.Background(), templates, bytes.NewReader(bytes.Repeat([]byte{0x11}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(layout.RulesFile, layout, fixedClock(time.Unix(2, 0)))
	if err != nil {
		t.Fatal(err)
	}
	second, err := reopened.EnsureTemplateResources(context.Background(), templates, bytes.NewReader(bytes.Repeat([]byte{0x22}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	if first["template-cn"] == "" || first["template-cn"] != second["template-cn"] {
		t.Fatalf("ids first=%v second=%v", first, second)
	}
}
