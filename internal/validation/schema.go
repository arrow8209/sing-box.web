package validation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"

	"singbox-web/internal/profile"
)

type SchemaValidator struct {
	schema *jsonschema.Schema
}

func NewSchemaValidator(schema []byte) (*SchemaValidator, error) {
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", bytes.NewReader(schema)); err != nil {
		return nil, fmt.Errorf("add schema resource: %w", err)
	}
	compiled, err := compiler.Compile("schema.json")
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}
	return &SchemaValidator{schema: compiled}, nil
}

func (validator *SchemaValidator) Validate(config json.RawMessage) []Issue {
	decoder := json.NewDecoder(bytes.NewReader(config))
	decoder.UseNumber()
	var instance any
	if err := decoder.Decode(&instance); err != nil {
		return []Issue{{Stage: "schema", Path: "", Code: "invalid_json", Message: "配置 JSON 无法解析"}}
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return []Issue{{Stage: "schema", Path: "", Code: "invalid_json", Message: "配置 JSON 无法解析"}}
	}
	if err := validator.schema.Validate(instance); err != nil {
		var validationError *jsonschema.ValidationError
		if !errors.As(err, &validationError) {
			return []Issue{{Stage: "schema", Path: "", Code: "schema_validation", Message: "配置不符合 Schema 约束"}}
		}
		issues := make([]Issue, 0)
		flattenValidationError(validationError, &issues)
		sort.SliceStable(issues, func(left, right int) bool {
			if issues[left].Path != issues[right].Path {
				return issues[left].Path < issues[right].Path
			}
			return issues[left].Code < issues[right].Code
		})
		return issues
	}
	return nil
}

func flattenValidationError(validationError *jsonschema.ValidationError, issues *[]Issue) {
	if len(validationError.Causes) != 0 {
		for _, cause := range validationError.Causes {
			flattenValidationError(cause, issues)
		}
		return
	}
	*issues = append(*issues, Issue{
		Stage:   "schema",
		Path:    validationError.InstanceLocation,
		Code:    schemaKeyword(validationError.KeywordLocation),
		Message: "配置不符合 Schema 约束",
	})
}

func schemaKeyword(location string) string {
	if index := strings.LastIndexByte(location, '/'); index >= 0 && index+1 < len(location) {
		return location[index+1:]
	}
	return "schema_validation"
}

type Service struct {
	schema        *SchemaValidator
	targetVersion string
	now           func() time.Time
}

func NewService(schema *SchemaValidator, targetVersion string, now func() time.Time) *Service {
	return &Service{schema: schema, targetVersion: targetVersion, now: now}
}

func (service *Service) Validate(config json.RawMessage) Result {
	issues := append([]Issue(nil), service.schema.Validate(config)...)
	issues = append(issues, ValidateSemantics(config)...)
	return service.result(issues)
}

func (service *Service) ValidateDraft(draft profile.Draft) Result {
	keys := make([]string, 0, len(draft.Content.EditorBuffers))
	for key := range draft.Content.EditorBuffers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	issues := make([]Issue, 0)
	for _, key := range keys {
		buffer := draft.Content.EditorBuffers[key]
		path := "/" + escapeJSONPointerToken(key)
		if !buffer.Valid {
			issues = append(issues, Issue{Stage: "json", Path: path, Code: "invalid_json", Message: "原始 JSON 尚未成功解析"})
		} else if !buffer.Applied {
			issues = append(issues, Issue{Stage: "json", Path: path, Code: "buffer_out_of_sync", Message: "原始 JSON 尚未应用到配置"})
		}
	}
	if len(issues) != 0 {
		return service.result(issues)
	}
	return service.Validate(draft.Content.Config)
}

func (service *Service) result(issues []Issue) Result {
	return Result{
		Valid:         len(issues) == 0,
		Issues:        issues,
		TargetVersion: service.targetVersion,
		CheckedAt:     service.now().UTC(),
	}
}

func escapeJSONPointerToken(value string) string {
	value = strings.ReplaceAll(value, "~", "~0")
	return strings.ReplaceAll(value, "/", "~1")
}
