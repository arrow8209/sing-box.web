package subscriptions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"singbox-web/internal/paths"
	"singbox-web/internal/storage"
)

func TestServicePersistsCompleteOutboundsAndReturnsCredentialFreeSummaries(t *testing.T) {
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.August, 24, 9, 30, 0, 0, time.UTC)
	service, err := Open(layout, func() time.Time { return now }, bytes.NewReader(bytes.Repeat([]byte{0x2a}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	summary, err := service.Create(context.Background(), Input{
		Name: "  家庭节点  ",
		Outbounds: []json.RawMessage{
			json.RawMessage(`{"type":"shadowsocks","tag":" 香港 01 ","server":"hk.example.test","password":"keep-me","plugin_options":{"mode":"fast"}}`),
			json.RawMessage(`{"type":"trojan","tag":"新加坡 01","server":"sg.example.test","password":"also-keep"}`),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if summary.ID != "2a2a2a2a2a2a2a2a2a2a2a2a2a2a2a2a" || summary.Name != "家庭节点" || summary.NodeCount != 2 {
		t.Fatalf("summary=%+v", summary)
	}
	if len(summary.Protocols) != 2 || summary.Protocols[0] != "shadowsocks" || summary.Protocols[1] != "trojan" {
		t.Fatalf("protocols=%v", summary.Protocols)
	}
	detail, err := service.Get(context.Background(), summary.ID)
	if err != nil {
		t.Fatal(err)
	}
	var outbound struct {
		Tag           string            `json:"tag"`
		Password      string            `json:"password"`
		PluginOptions map[string]string `json:"plugin_options"`
	}
	if err := json.Unmarshal(detail.Outbounds[0], &outbound); err != nil {
		t.Fatal(err)
	}
	if outbound.Password != "keep-me" || outbound.PluginOptions["mode"] != "fast" {
		t.Fatalf("outbound fields were lost: %+v", outbound)
	}
	if outbound.Tag != "香港 01" {
		t.Fatalf("tag was not normalized: %+v", outbound)
	}
	mode, err := os.Stat(layout.SubscriptionsFile)
	if err != nil {
		t.Fatal(err)
	}
	if mode.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%o", mode.Mode().Perm())
	}
	reopened, err := Open(layout, func() time.Time { return now }, nil)
	if err != nil {
		t.Fatal(err)
	}
	items, err := reopened.List(context.Background())
	if err != nil || len(items) != 1 || !reflect.DeepEqual(items[0], summary) {
		t.Fatalf("items=%+v err=%v", items, err)
	}
}

func TestSubscriptionChangeHookRunsAfterUnlockAndUsesOrderedNodes(t *testing.T) {
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service, err := Open(layout, nil, bytes.NewReader(bytes.Repeat([]byte{0x43}, 16)))
	if err != nil {
		t.Fatal(err)
	}
	var observed []map[string]string
	service.SetOnChange(func(ctx context.Context, _ string) {
		digests, e := service.CurrentDigests(ctx)
		if e != nil {
			t.Error(e)
		}
		observed = append(observed, digests)
	})
	nodes := []json.RawMessage{json.RawMessage(`{"type":"direct","tag":"a"}`), json.RawMessage(`{"type":"block","tag":"b"}`)}
	created, err := service.Create(context.Background(), Input{Name: "source", Outbounds: nodes})
	if err != nil {
		t.Fatal(err)
	}
	key := "subscription:" + created.ID
	if _, err := service.Replace(context.Background(), created.ID, Input{Name: "renamed", Outbounds: nodes}); err != nil {
		t.Fatal(err)
	}
	if len(observed) != 2 || observed[0][key] != observed[1][key] {
		t.Fatal("display name changed digest", observed)
	}
	if _, err := service.Replace(context.Background(), created.ID, Input{Name: "renamed", Outbounds: []json.RawMessage{nodes[1], nodes[0]}}); err != nil {
		t.Fatal(err)
	}
	if len(observed) != 3 || observed[1][key] == observed[2][key] {
		t.Fatal("node order omitted", observed)
	}
	if err := service.Delete(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	if len(observed) != 4 || observed[3][key] != "" {
		t.Fatal("deletion digest remains", observed)
	}
}

func TestServiceReplacesAndDeletesWholeSubscription(t *testing.T) {
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(10, 0).UTC()
	service, err := Open(layout, func() time.Time { return now }, bytes.NewReader(bytes.Repeat([]byte{0x11}, 16)))
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.Create(context.Background(), Input{Name: "旧订阅", Outbounds: []json.RawMessage{json.RawMessage(`{"type":"direct","tag":"旧节点"}`)}})
	if err != nil {
		t.Fatal(err)
	}
	now = time.Unix(20, 0).UTC()
	replaced, err := service.Replace(context.Background(), created.ID, Input{Name: "新订阅", Outbounds: []json.RawMessage{json.RawMessage(`{"type":"vless","tag":"新节点","flow":"xtls-rprx-vision"}`)}})
	if err != nil {
		t.Fatal(err)
	}
	if replaced.Name != "新订阅" || replaced.NodeCount != 1 || !replaced.UpdatedAt.Equal(now) {
		t.Fatalf("replaced=%+v", replaced)
	}
	if err := service.Delete(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Get(context.Background(), created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get after delete err=%v", err)
	}
}

func TestServiceRejectsInvalidOrDuplicateNodeTagsWithoutWriting(t *testing.T) {
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service, err := Open(layout, nil, bytes.NewReader(bytes.Repeat([]byte{0x33}, 16)))
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []Input{
		{Name: "空节点", Outbounds: nil},
		{Name: "缺少标签", Outbounds: []json.RawMessage{json.RawMessage(`{"type":"trojan"}`)}},
		{Name: "重复标签", Outbounds: []json.RawMessage{json.RawMessage(`{"type":"trojan","tag":"同名"}`), json.RawMessage(`{"type":"vless","tag":" 同名 "}`)}},
	} {
		if _, err := service.Create(context.Background(), input); err == nil {
			t.Fatalf("accepted input=%+v", input)
		}
	}
	if _, err := os.Stat(layout.SubscriptionsFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid input created state file: %v", err)
	}
}

func TestValidateManagedStateReportsSubscriptionFileCorruption(t *testing.T) {
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.WriteFile(layout.SubscriptionsFile, []byte(`{"version":1,"subscriptions":null}`), 0o600); err != nil {
		t.Fatal(err)
	}
	err = ValidateManagedState(layout)
	var corrupt *storage.CorruptFileError
	if !errors.As(err, &corrupt) || corrupt.Path != layout.SubscriptionsFile {
		t.Fatalf("err=%v", err)
	}
}
