package configdoc

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestRedactKeepsShapeAndMasksSensitiveValues(t *testing.T) {
	input := json.RawMessage(`{"experimental":{"clash_api":{"secret":"abc"}},"outbounds":[{"type":"vless","uuid":"u1","password":"p1","server":"example.com"}]}`)
	got, err := Redact(input)
	if err != nil {
		t.Fatal(err)
	}
	want := json.RawMessage(`{"experimental":{"clash_api":{"secret":"***REDACTED***"}},"outbounds":[{"type":"vless","uuid":"***REDACTED***","password":"***REDACTED***","server":"example.com"}]}`)
	if !jsonSemanticEqual(got, want) {
		t.Fatalf("Redact() does not match expected shape")
	}
}

func TestRedactMasksExactCaseInsensitiveKeysRecursivelyAndPreservesNumbers(t *testing.T) {
	input := json.RawMessage(`{"TOKEN":"x","not_token":"keep","nested":[{"Api_Key":123,"count":9007199254740993123456789},[true,{"private_key":null}]],"password_suffix":"keep"}`)
	got, err := Redact(input)
	if err != nil {
		t.Fatal(err)
	}
	want := json.RawMessage(`{"TOKEN":"***REDACTED***","not_token":"keep","nested":[{"Api_Key":"***REDACTED***","count":9007199254740993123456789},[true,{"private_key":"***REDACTED***"}]],"password_suffix":"keep"}`)
	if !jsonSemanticEqual(got, want) {
		t.Fatal("Redact did not preserve shape, key matching, or number precision")
	}
}

func TestRedactRejectsInvalidAndNonObjectInput(t *testing.T) {
	for _, input := range []json.RawMessage{json.RawMessage(``), json.RawMessage(`[]`), json.RawMessage(`null`), json.RawMessage(`{} {}`)} {
		if result, err := Redact(input); err == nil || result != nil {
			t.Fatalf("Redact(%q) = %s, %v", input, bytes.TrimSpace(result), err)
		}
	}
}
