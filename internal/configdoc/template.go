package configdoc

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
)

const credentialSentinel = "__GENERATE_AT_PROFILE_CREATION__"

type Template struct {
	raw json.RawMessage
}

func LoadTemplate(raw []byte) (*Template, error) {
	config, err := decodeObject(raw)
	if err != nil {
		return nil, fmt.Errorf("decode template: %w", err)
	}
	if err := validateCredentialSentinels(config); err != nil {
		return nil, err
	}

	return &Template{raw: bytes.Clone(raw)}, nil
}

func (template *Template) New(random io.Reader) (json.RawMessage, error) {
	if template == nil {
		return nil, fmt.Errorf("template is nil")
	}
	if random == nil {
		return nil, fmt.Errorf("random reader is nil")
	}

	config, err := decodeObject(template.raw)
	if err != nil {
		return nil, fmt.Errorf("clone template: %w", err)
	}
	if err := validateCredentialSentinels(config); err != nil {
		return nil, err
	}

	secret, err := randomHex(random, 32)
	if err != nil {
		return nil, fmt.Errorf("generate clash API secret: %w", err)
	}
	cacheID, err := randomCacheID(random)
	if err != nil {
		return nil, fmt.Errorf("generate cache ID: %w", err)
	}

	experimental := config["experimental"].(map[string]any)
	experimental["clash_api"].(map[string]any)["secret"] = secret
	experimental["cache_file"].(map[string]any)["cache_id"] = cacheID

	result, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("encode profile: %w", err)
	}
	return json.RawMessage(result), nil
}

func decodeObject(raw []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("multiple JSON values")
		}
		return nil, err
	}

	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("template must be a top-level object")
	}
	return object, nil
}

func validateCredentialSentinels(config map[string]any) error {
	experimental, err := requiredObject(config, "experimental")
	if err != nil {
		return err
	}
	clashAPI, err := requiredObject(experimental, "clash_api")
	if err != nil {
		return fmt.Errorf("experimental.%w", err)
	}
	cacheFile, err := requiredObject(experimental, "cache_file")
	if err != nil {
		return fmt.Errorf("experimental.%w", err)
	}
	if err := requiredSentinel(clashAPI, "secret"); err != nil {
		return fmt.Errorf("experimental.clash_api.%w", err)
	}
	if err := requiredSentinel(cacheFile, "cache_id"); err != nil {
		return fmt.Errorf("experimental.cache_file.%w", err)
	}
	return nil
}

func requiredObject(object map[string]any, key string) (map[string]any, error) {
	value, ok := object[key]
	if !ok {
		return nil, fmt.Errorf("missing %s", key)
	}
	nested, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an object", key)
	}
	return nested, nil
}

func requiredSentinel(object map[string]any, key string) error {
	value, ok := object[key]
	if !ok {
		return fmt.Errorf("missing %s", key)
	}
	if value != credentialSentinel {
		return fmt.Errorf("%s must equal credential sentinel", key)
	}
	return nil
}

func randomHex(random io.Reader, size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := io.ReadFull(random, bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func randomCacheID(random io.Reader) (string, error) {
	bytes := make([]byte, 9)
	if _, err := io.ReadFull(random, bytes); err != nil {
		return "", err
	}
	return "ID_" + base64.RawURLEncoding.EncodeToString(bytes), nil
}
