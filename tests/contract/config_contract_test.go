package contract_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"singbox-web/internal/assets"
	"singbox-web/internal/configdoc"
	"singbox-web/internal/singbox"
)

type schemaMetadata struct {
	TargetVersion string `json:"targetVersion"`
}

func readSchemaMetadata(t *testing.T) schemaMetadata {
	t.Helper()
	var metadata schemaMetadata
	if err := json.Unmarshal(assets.SchemaVersion, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.TargetVersion == "" {
		t.Fatal("schema metadata has empty targetVersion")
	}
	return metadata
}

func newTemplateConfig(t *testing.T) json.RawMessage {
	t.Helper()
	template, err := configdoc.LoadTemplate(assets.DefaultConfig)
	if err != nil {
		t.Fatal(err)
	}
	config, err := template.New(bytes.NewReader(bytes.Repeat([]byte{5}, 128)))
	if err != nil {
		t.Fatal(err)
	}
	return config
}

func writeModuleFixture(t *testing.T, files []configdoc.ModuleFile) string {
	t.Helper()
	dir := t.TempDir()
	for _, file := range files {
		if filepath.Base(file.Name) != file.Name {
			t.Fatalf("unsafe module name %q", file.Name)
		}
		if err := os.WriteFile(filepath.Join(dir, file.Name), file.Content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func decodeJSON(t *testing.T, raw []byte) any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestDefaultTemplatePassesPinnedLatestSingBox(t *testing.T) {
	binary := os.Getenv("SING_BOX_BIN")
	if binary == "" {
		t.Skip("set SING_BOX_BIN for release contract verification")
	}
	metadata := readSchemaMetadata(t)
	runner := singbox.NewRunner(binary, 30*time.Second, 64*1024)
	version, err := runner.Version(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(version)
	if len(fields) == 0 || fields[len(fields)-1] != metadata.TargetVersion {
		t.Fatalf("installed=%q target=%q", version, metadata.TargetVersion)
	}
	config := newTemplateConfig(t)
	files, err := configdoc.Split(config)
	if err != nil {
		t.Fatal(err)
	}
	dir := writeModuleFixture(t, files)
	result, err := runner.Check(context.Background(), dir)
	if err != nil || !result.Valid {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestDefaultTemplateModuleContract(t *testing.T) {
	expectedRaw, err := os.ReadFile("testdata/expected-modules.json")
	if err != nil {
		t.Fatal(err)
	}
	var expected []string
	if err := json.Unmarshal(expectedRaw, &expected); err != nil {
		t.Fatal(err)
	}
	config := newTemplateConfig(t)
	files, err := configdoc.Split(config)
	if err != nil {
		t.Fatal(err)
	}
	actual := make([]string, len(files))
	for index, file := range files {
		actual[index] = file.Name
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("modules=%q expected=%q", actual, expected)
	}
	merged, err := configdoc.Merge(files)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decodeJSON(t, merged), decodeJSON(t, config)) {
		t.Fatalf("merged config differs from template\nmerged=%s\ntemplate=%s", merged, config)
	}
}
