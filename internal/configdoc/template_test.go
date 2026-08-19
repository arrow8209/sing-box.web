package configdoc

import (
	"bytes"
	"encoding/json"
	"reflect"
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

func TestLoadTemplateRejectsNonObjectAndMultipleValues(t *testing.T) {
	loadMustFail(t, []byte(`null`))
	loadMustFail(t, []byte(`[]`))
	loadMustFail(t, append(bytes.Clone(assets.DefaultConfig), []byte(`{}`)...))
}

func TestLoadTemplateRejectsMissingOrInvalidContainers(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{
			name: "missing experimental",
			mutate: func(config map[string]any) {
				delete(config, "experimental")
			},
		},
		{
			name: "experimental is not object",
			mutate: func(config map[string]any) {
				config["experimental"] = "invalid"
			},
		},
		{
			name: "missing clash api",
			mutate: func(config map[string]any) {
				delete(experimental(config), "clash_api")
			},
		},
		{
			name: "clash api is not object",
			mutate: func(config map[string]any) {
				experimental(config)["clash_api"] = "invalid"
			},
		},
		{
			name: "missing cache file",
			mutate: func(config map[string]any) {
				delete(experimental(config), "cache_file")
			},
		},
		{
			name: "cache file is not object",
			mutate: func(config map[string]any) {
				experimental(config)["cache_file"] = "invalid"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			loadMustFail(t, mutatedTemplate(t, test.mutate))
		})
	}
}

func TestLoadTemplateRejectsMissingOrInvalidSentinels(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{
			name: "missing secret",
			mutate: func(config map[string]any) {
				delete(clashAPI(config), "secret")
			},
		},
		{
			name: "invalid secret",
			mutate: func(config map[string]any) {
				clashAPI(config)["secret"] = "invalid"
			},
		},
		{
			name: "missing cache id",
			mutate: func(config map[string]any) {
				delete(cacheFile(config), "cache_id")
			},
		},
		{
			name: "invalid cache id",
			mutate: func(config map[string]any) {
				cacheFile(config)["cache_id"] = "invalid"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			loadMustFail(t, mutatedTemplate(t, test.mutate))
		})
	}
}

func TestTemplateNewRejectsShortRandomReaderWithNilConfig(t *testing.T) {
	template, err := LoadTemplate(assets.DefaultConfig)
	if err != nil {
		t.Fatal(err)
	}

	for _, size := range []int{31, 40} {
		t.Run("short reader", func(t *testing.T) {
			config, err := template.New(bytes.NewReader(bytes.Repeat([]byte{1}, size)))
			if err == nil {
				t.Fatal("Template.New succeeded with a short random reader")
			}
			if config != nil {
				t.Fatal("Template.New returned a config after random reader failure")
			}
		})
	}
}

func TestTemplateNewPreservesLargeNumbers(t *testing.T) {
	const largeInteger = "9007199254740993123456789"
	raw := mutatedTemplate(t, func(config map[string]any) {
		config["large_integer"] = json.Number(largeInteger)
	})
	template, err := LoadTemplate(raw)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := template.New(bytes.NewReader(bytes.Repeat([]byte{1}, 64)))
	if err != nil {
		t.Fatal(err)
	}
	value := decodedObject(t, profile)["large_integer"]
	number, ok := value.(json.Number)
	if !ok || number.String() != largeInteger {
		t.Fatal("large integer was not preserved exactly")
	}
}

func TestTemplateNewOnlyChangesGeneratedCredentials(t *testing.T) {
	template, err := LoadTemplate(assets.DefaultConfig)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := template.New(bytes.NewReader(bytes.Repeat([]byte{1}, 64)))
	if err != nil {
		t.Fatal(err)
	}

	source := decodedObject(t, assets.DefaultConfig)
	generated := decodedObject(t, profile)
	deleteGeneratedCredentials(source)
	deleteGeneratedCredentials(generated)
	if !reflect.DeepEqual(source, generated) {
		t.Fatal("generated profile changed template content beyond generated credentials")
	}
}

func loadMustFail(t *testing.T, raw []byte) {
	t.Helper()
	template, err := LoadTemplate(raw)
	if err == nil {
		t.Fatal("LoadTemplate succeeded for an invalid template")
	}
	if template != nil {
		t.Fatal("LoadTemplate returned a template after failure")
	}
}

func mutatedTemplate(t *testing.T, mutate func(map[string]any)) []byte {
	t.Helper()
	config := decodedObject(t, assets.DefaultConfig)
	mutate(config)
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func decodedObject(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatal("test fixture is not an object")
	}
	return object
}

func experimental(config map[string]any) map[string]any {
	return config["experimental"].(map[string]any)
}

func clashAPI(config map[string]any) map[string]any {
	return experimental(config)["clash_api"].(map[string]any)
}

func cacheFile(config map[string]any) map[string]any {
	return experimental(config)["cache_file"].(map[string]any)
}

func deleteGeneratedCredentials(config map[string]any) {
	delete(clashAPI(config), "secret")
	delete(cacheFile(config), "cache_id")
}
