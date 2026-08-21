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
		return nil
	}
	if format != FormatSource {
		return errors.New("规则资源格式无效")
	}
	var document struct {
		Version json.RawMessage   `json:"version"`
		Rules   []json.RawMessage `json:"rules"`
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	if err := decoder.Decode(&document); err != nil {
		return fmt.Errorf("source 规则 JSON 无效: %w", err)
	}
	if len(document.Version) == 0 || document.Rules == nil {
		return errors.New("source 规则必须包含 version 和 rules")
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("source 规则只能包含一个 JSON 值")
	}
	return nil
}
