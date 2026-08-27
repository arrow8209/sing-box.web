package inbounds

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"singbox-web/internal/paths"
	"singbox-web/internal/storage"
)

func TestServicePersistsCompleteInboundAndReturnsCredentialFreeSummary(t *testing.T) {
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.August, 25, 10, 30, 0, 0, time.UTC)
	service, err := Open(layout, func() time.Time { return now }, bytes.NewReader(bytes.Repeat([]byte{0x2a}, 16)))
	if err != nil {
		t.Fatal(err)
	}
	summary, err := service.Create(context.Background(), Input{
		Mode:    ModeStructured,
		Config:  json.RawMessage(`{"type":" mixed ","tag":" 家庭入口 ","listen":"0.0.0.0","listen_port":7891,"users":[{"username":"admin","password":"keep-me"}],"future":{"enabled":true}}`),
		Routing: &RoutingIntent{Sniff: true, SniffTimeout: " 750ms "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if summary.ID != "2a2a2a2a2a2a2a2a2a2a2a2a2a2a2a2a" || summary.Tag != "家庭入口" || summary.Type != "mixed" || summary.Endpoint != "0.0.0.0:7891" {
		t.Fatalf("summary=%+v", summary)
	}
	detail, err := service.Get(context.Background(), summary.ID)
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Type  string `json:"type"`
		Tag   string `json:"tag"`
		Users []struct {
			Password string `json:"password"`
		} `json:"users"`
		Future map[string]bool `json:"future"`
	}
	if err := json.Unmarshal(detail.Config, &config); err != nil {
		t.Fatal(err)
	}
	if config.Type != "mixed" || config.Tag != "家庭入口" || config.Users[0].Password != "keep-me" || !config.Future["enabled"] {
		t.Fatalf("config fields were lost: %+v", config)
	}
	if detail.Routing == nil || !detail.Routing.Sniff || detail.Routing.SniffTimeout != "750ms" {
		t.Fatalf("routing intent was lost: %+v", detail.Routing)
	}
	info, err := os.Stat(layout.InboundResourcesFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
}

func TestServiceEnforcesUniqueTagsAndStructuredTypes(t *testing.T) {
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service, err := Open(layout, nil, bytes.NewReader(bytes.Repeat([]byte{0x31}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.Create(context.Background(), Input{Mode: ModeStructured, Config: json.RawMessage(`{"type":"tun","tag":"tun-in","address":["172.18.0.1/30"]}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(context.Background(), Input{Mode: ModeCustom, Config: json.RawMessage(`{"type":"shadowtls","tag":" tun-in "}`)}); !errors.Is(err, ErrDuplicateTag) {
		t.Fatalf("duplicate err=%v", err)
	}
	if _, err := service.Replace(context.Background(), created.ID, Input{Mode: ModeStructured, Config: json.RawMessage(`{"type":"shadowtls","tag":"other"}`)}); err == nil {
		t.Fatal("unsupported structured type was accepted")
	}
	if _, err := service.Replace(context.Background(), created.ID, Input{Mode: ModeCustom, Config: json.RawMessage(`{"type":"shadowtls","tag":"other"}`), Routing: &RoutingIntent{Sniff: true}}); err == nil {
		t.Fatal("custom resource routing intent was accepted")
	}
	replaced, err := service.Replace(context.Background(), created.ID, Input{Mode: ModeCustom, Config: json.RawMessage(`{"type":"shadowtls","tag":"other","unknown":900}`)})
	if err != nil || replaced.Type != "shadowtls" || replaced.Mode != ModeCustom {
		t.Fatalf("replaced=%+v err=%v", replaced, err)
	}
	detail, err := service.Get(context.Background(), created.ID)
	if err != nil || detail.Routing != nil {
		t.Fatalf("routing intent survived replacement: detail=%+v err=%v", detail, err)
	}
	if err := service.Delete(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Get(context.Background(), created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get after delete err=%v", err)
	}
}

func TestValidateManagedStateReportsInboundFileCorruption(t *testing.T) {
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.WriteFile(layout.InboundResourcesFile, []byte(`{"version":1,"inbounds":null}`), 0o600); err != nil {
		t.Fatal(err)
	}
	err = ValidateManagedState(layout)
	var corrupt *storage.CorruptFileError
	if !errors.As(err, &corrupt) || corrupt.Path != layout.InboundResourcesFile {
		t.Fatalf("err=%v", err)
	}
}
