package configdoc

import (
	"encoding/json"
	"fmt"
	"strings"
)

const redactedValue = "***REDACTED***"

var sensitiveKeys = map[string]struct{}{
	"password":       {},
	"passwd":         {},
	"secret":         {},
	"token":          {},
	"access_token":   {},
	"refresh_token":  {},
	"authorization":  {},
	"api_key":        {},
	"client_secret":  {},
	"uuid":           {},
	"private_key":    {},
	"pre_shared_key": {},
}

// Redact recursively masks sensitive configuration fields while preserving JSON values otherwise.
func Redact(config json.RawMessage) (json.RawMessage, error) {
	object, err := decodeConfigObject(config)
	if err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	redactValue(object)
	result, err := json.Marshal(object)
	if err != nil {
		return nil, fmt.Errorf("encode redacted config: %w", err)
	}
	return json.RawMessage(result), nil
}

func redactValue(value any) {
	switch value := value.(type) {
	case map[string]any:
		for key, nested := range value {
			if _, sensitive := sensitiveKeys[strings.ToLower(key)]; sensitive {
				value[key] = redactedValue
				continue
			}
			redactValue(nested)
		}
	case []any:
		for _, nested := range value {
			redactValue(nested)
		}
	}
}
