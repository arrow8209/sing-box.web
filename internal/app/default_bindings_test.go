package app

import (
	"bytes"
	"context"
	"testing"

	"singbox-web/internal/paths"
	"singbox-web/internal/rules"
	"singbox-web/internal/templates"
)

func TestDefaultBindingSourceCreatesConcreteBindings(t *testing.T) {
	factory, err := templates.NewFactory(bytes.NewReader(bytes.Repeat([]byte{0x11}, 256)))
	if err != nil {
		t.Fatal(err)
	}
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store, err := rules.OpenStore(layout.RulesFile, layout, nil)
	if err != nil {
		t.Fatal(err)
	}
	randomIDs := make([]byte, 0, 16*6)
	for value := byte(1); value <= 6; value++ {
		randomIDs = append(randomIDs, bytes.Repeat([]byte{value}, 16)...)
	}
	bindings, err := NewDefaultBindingSource(factory, store, bytes.NewReader(randomIDs)).DefaultBindings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 6 {
		t.Fatalf("bindings=%d", len(bindings))
	}
	for _, binding := range bindings {
		if binding.ResourceID == "" {
			t.Fatal("binding lacks concrete resource ID")
		}
	}
}
