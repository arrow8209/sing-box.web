package rules

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var ErrEmptyRuleSet = errors.New("规则文件不能为空")

func ValidateDownloaded(format Format, payload []byte) error {
	if len(payload) == 0 {
		return ErrEmptyRuleSet
	}
	if format == FormatBinary {
		if json.Valid(bytes.TrimSpace(payload)) {
			return errors.New("binary 规则不能使用 Source JSON 内容")
		}
		return nil
	}
	if format != FormatSource {
		return errors.New("规则资源格式无效")
	}
	_, _, err := analyzeSource(payload, false)
	return err
}

// NormalizeManualSource validates a user-maintained source rule-set and
// returns stable, indented JSON plus the number of concrete rule values.
func NormalizeManualSource(payload []byte) ([]byte, int, error) {
	if len(bytes.TrimSpace(payload)) == 0 {
		return nil, 0, ErrEmptyRuleSet
	}
	_, count, err := analyzeSource(payload, true)
	if err != nil {
		return nil, 0, err
	}
	var formatted bytes.Buffer
	if err := json.Indent(&formatted, bytes.TrimSpace(payload), "", "  "); err != nil {
		return nil, 0, fmt.Errorf("source 规则 JSON 无效: %w", err)
	}
	formatted.WriteByte('\n')
	return formatted.Bytes(), count, nil
}

func SourceRuleCount(payload []byte) (int, error) {
	_, count, err := analyzeSource(payload, false)
	return count, err
}

func analyzeSource(payload []byte, requireRules bool) (int, int, error) {
	var document struct {
		Version json.RawMessage   `json:"version"`
		Rules   []json.RawMessage `json:"rules"`
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	if err := decoder.Decode(&document); err != nil {
		return 0, 0, fmt.Errorf("source 规则 JSON 无效: %w", err)
	}
	if len(document.Version) == 0 || document.Rules == nil {
		return 0, 0, errors.New("source 规则必须包含 version 和 rules")
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return 0, 0, errors.New("source 规则只能包含一个 JSON 值")
	}
	var version int
	if err := json.Unmarshal(document.Version, &version); err != nil || version < 1 {
		return 0, 0, errors.New("source 规则必须包含有效的整数 version")
	}
	if requireRules && len(document.Rules) == 0 {
		return 0, 0, errors.New("手工规则必须至少包含一个规则块")
	}
	count := 0
	for index, raw := range document.Rules {
		var rule map[string]json.RawMessage
		if err := json.Unmarshal(raw, &rule); err != nil || len(rule) == 0 {
			return 0, 0, fmt.Errorf("第 %d 个规则块必须是非空对象", index+1)
		}
		for _, value := range rule {
			trimmed := bytes.TrimSpace(value)
			if bytes.Equal(trimmed, []byte("null")) {
				continue
			}
			var values []json.RawMessage
			if len(trimmed) > 0 && trimmed[0] == '[' && json.Unmarshal(trimmed, &values) == nil {
				count += len(values)
				continue
			}
			count++
		}
	}
	if requireRules && count == 0 {
		return 0, 0, errors.New("手工规则中没有可用的匹配内容")
	}
	return len(document.Rules), count, nil
}
