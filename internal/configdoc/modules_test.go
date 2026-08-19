package configdoc

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestSplitMergeAndDigestAreDeterministic(t *testing.T) {
	input := json.RawMessage(`{"outbounds":[{"type":"direct","tag":"direct"}],"log":{"level":"info"},"future":{"enabled":true}}`)
	files, err := Split(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 || files[0].Name != "10-log.json" || files[1].Name != "70-outbounds.json" || files[2].Name != "98-future-ebb3de8a.json" {
		t.Fatalf("files = %#v", files)
	}
	merged, err := Merge(files)
	if err != nil {
		t.Fatal(err)
	}
	if !jsonSemanticEqual(input, merged) {
		t.Fatalf("merged = %s", merged)
	}
	again, err := Split(merged)
	if err != nil {
		t.Fatal(err)
	}
	if Digest(files) != Digest(again) {
		t.Fatal("digest changed across round trip")
	}
}

func TestSplitUnknownNamesAreCollisionSafeAndSortedByOriginalKey(t *testing.T) {
	files, err := Split(json.RawMessage(`{"z":{"n":1},"A!":true,"a?":false,"日本語":null,"abcdefghijklmnopqrstuvwxyz0123456789-more":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 5 {
		t.Fatalf("got %d files", len(files))
	}
	wantKeys := []string{"A!", "a?", "abcdefghijklmnopqrstuvwxyz0123456789-more", "z", "日本語"}
	for i, want := range wantKeys {
		if files[i].TopKey != want {
			t.Fatalf("files[%d].TopKey = %q, want %q", i, files[i].TopKey, want)
		}
		if !bytes.HasSuffix(files[i].Content, []byte("\n")) {
			t.Fatalf("files[%d] does not have final newline", i)
		}
	}
	if files[0].Name == files[1].Name || files[3].Name != "98-z-594e519a.json" || files[4].Name != "98-module-77710aed.json" {
		t.Fatalf("unexpected unknown names: %#v", files)
	}
	if files[2].Name != "98-abcdefghijklmnopqrstuvwxyz012345-d2aa446a.json" {
		t.Fatalf("long name = %q", files[2].Name)
	}
}

func TestSplitRejectsInvalidAndNonObjectInput(t *testing.T) {
	for _, input := range []json.RawMessage{json.RawMessage(``), json.RawMessage(`[]`), json.RawMessage(`null`), json.RawMessage(`{} {}`)} {
		if files, err := Split(input); err == nil || files != nil {
			t.Fatalf("Split(%q) = %#v, %v", input, files, err)
		}
	}
}

func TestSplitAndMergeRejectDuplicateTopLevelKeys(t *testing.T) {
	duplicate := json.RawMessage(`{"log":{},"log":{}}`)
	if files, err := Split(duplicate); err == nil || files != nil {
		t.Fatalf("Split accepted duplicate key: %#v, %v", files, err)
	}
	if merged, err := Merge([]ModuleFile{{Content: duplicate}}); err == nil || merged != nil {
		t.Fatalf("Merge accepted duplicate key: %s, %v", merged, err)
	}
}

func TestMergeRejectsTrailingSecondJSONValue(t *testing.T) {
	files := []ModuleFile{{Content: []byte(`{"log":{}} {}`)}}
	if merged, err := Merge(files); err == nil || merged != nil {
		t.Fatalf("Merge accepted trailing JSON value: %s, %v", merged, err)
	}
}

func TestMergeUsesContentAndRejectsInvalidModuleFiles(t *testing.T) {
	merged, err := Merge([]ModuleFile{{Name: "untrusted.json", TopKey: "wrong", Content: []byte("{\n  \"log\": {\n    \"level\": \"info\"\n  }\n}\n")}})
	if err != nil {
		t.Fatal(err)
	}
	if !jsonSemanticEqual(merged, json.RawMessage(`{"log":{"level":"info"}}`)) {
		t.Fatalf("Merge() = %s", merged)
	}

	invalid := [][]ModuleFile{
		{{Content: []byte(`{}`)}},
		{{Content: []byte(`{"log":{},"dns":{}`)}},
		{{Content: []byte(`{"log":{}`)}},
		{{Content: []byte(`{"log":{}`)}, {Content: []byte(`{"log":{}`)}},
	}
	for _, files := range invalid {
		if merged, err := Merge(files); err == nil || merged != nil {
			t.Fatalf("Merge(%#v) = %s, %v", files, merged, err)
		}
	}
}

func TestDigestSortsFilesByName(t *testing.T) {
	a := []ModuleFile{{Name: "b.json", Content: []byte("b")}, {Name: "a.json", Content: []byte("a")}}
	b := []ModuleFile{{Name: "a.json", Content: []byte("a")}, {Name: "b.json", Content: []byte("b")}}
	if Digest(a) != Digest(b) {
		t.Fatal("Digest depends on input order")
	}
}

func TestDigestSortsSameNameFilesByContent(t *testing.T) {
	forward := []ModuleFile{{Name: "same.json", Content: []byte("second")}, {Name: "same.json", Content: []byte("first")}}
	reverse := []ModuleFile{{Name: "same.json", Content: []byte("first")}, {Name: "same.json", Content: []byte("second")}}
	if Digest(forward) != Digest(reverse) {
		t.Fatal("Digest depends on same-name input order")
	}
	different := []ModuleFile{{Name: "same.json", Content: []byte("first")}, {Name: "same.json", Content: []byte("third")}}
	if Digest(forward) == Digest(different) {
		t.Fatal("Digest did not include same-name file contents")
	}
}

func jsonSemanticEqual(first, second json.RawMessage) bool {
	var a, b any
	decoder := json.NewDecoder(bytes.NewReader(first))
	decoder.UseNumber()
	if decoder.Decode(&a) != nil {
		return false
	}
	decoder = json.NewDecoder(bytes.NewReader(second))
	decoder.UseNumber()
	return decoder.Decode(&b) == nil && jsonValueEqual(a, b)
}

func jsonValueEqual(a, b any) bool {
	aRaw, aErr := json.Marshal(a)
	bRaw, bErr := json.Marshal(b)
	return aErr == nil && bErr == nil && bytes.Equal(aRaw, bRaw)
}
