package configdoc

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// ModuleFile is one top-level configuration section serialized as a JSON file.
type ModuleFile struct {
	Name    string
	TopKey  string
	Content []byte
}

var moduleNames = map[string]string{
	"$schema":               "00-schema.json",
	"log":                   "10-log.json",
	"dns":                   "20-dns.json",
	"ntp":                   "30-ntp.json",
	"certificate":           "40-certificate.json",
	"certificate_providers": "41-certificate-providers.json",
	"http_clients":          "42-http-clients.json",
	"network_namespaces":    "43-network-namespaces.json",
	"endpoints":             "50-endpoints.json",
	"inbounds":              "60-inbounds.json",
	"outbounds":             "70-outbounds.json",
	"route":                 "80-route.json",
	"services":              "90-services.json",
	"experimental":          "99-experimental.json",
}

// Split separates a complete configuration object into deterministic one-key files.
func Split(config json.RawMessage) ([]ModuleFile, error) {
	object, err := decodeConfigObject(config)
	if err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	known := make([]ModuleFile, 0, len(object))
	unknownKeys := make([]string, 0)
	for key, value := range object {
		if name, ok := moduleNames[key]; ok {
			file, err := newModuleFile(name, key, value)
			if err != nil {
				return nil, err
			}
			known = append(known, file)
			continue
		}
		unknownKeys = append(unknownKeys, key)
	}
	sort.Slice(known, func(i, j int) bool { return known[i].Name < known[j].Name })
	sort.Strings(unknownKeys)

	unknown := make([]ModuleFile, 0, len(unknownKeys))
	for _, key := range unknownKeys {
		file, err := newModuleFile(unknownModuleName(key), key, object[key])
		if err != nil {
			return nil, err
		}
		unknown = append(unknown, file)
	}

	result := make([]ModuleFile, 0, len(object))
	for _, file := range known {
		if file.Name < "98-" {
			result = append(result, file)
		}
	}
	result = append(result, unknown...)
	for _, file := range known {
		if file.Name >= "98-" {
			result = append(result, file)
		}
	}
	return result, nil
}

func newModuleFile(name, key string, value any) (ModuleFile, error) {
	content, err := json.MarshalIndent(map[string]any{key: value}, "", "  ")
	if err != nil {
		return ModuleFile{}, fmt.Errorf("encode module %q: %w", key, err)
	}
	content = append(content, '\n')
	return ModuleFile{Name: name, TopKey: key, Content: content}, nil
}

func unknownModuleName(key string) string {
	sum := sha256.Sum256([]byte(key))
	return "98-" + sanitizeModuleKey(key) + "-" + hex.EncodeToString(sum[:4]) + ".json"
}

func sanitizeModuleKey(key string) string {
	var builder strings.Builder
	lastHyphen := false
	for i := 0; i < len(key); i++ {
		character := key[i]
		if character >= 'A' && character <= 'Z' {
			character += 'a' - 'A'
		}
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') {
			builder.WriteByte(character)
			lastHyphen = false
			continue
		}
		if builder.Len() > 0 && !lastHyphen {
			builder.WriteByte('-')
			lastHyphen = true
		}
	}
	result := strings.Trim(builder.String(), "-")
	if len(result) > 32 {
		result = result[:32]
	}
	if result == "" {
		return "module"
	}
	return result
}

// Merge combines one-key module files into a complete configuration object.
func Merge(files []ModuleFile) (json.RawMessage, error) {
	config := make(map[string]any, len(files))
	for _, file := range files {
		object, err := decodeConfigObject(file.Content)
		if err != nil {
			return nil, fmt.Errorf("decode module: %w", err)
		}
		if len(object) != 1 {
			return nil, fmt.Errorf("module must contain exactly one top-level key")
		}
		for key, value := range object {
			if _, exists := config[key]; exists {
				return nil, fmt.Errorf("duplicate top-level key %q", key)
			}
			config[key] = value
		}
	}

	merged, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	return json.RawMessage(merged), nil
}

// Digest returns the SHA-256 digest of module names and their contents.
func Digest(files []ModuleFile) string {
	sorted := append([]ModuleFile(nil), files...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Name != sorted[j].Name {
			return sorted[i].Name < sorted[j].Name
		}
		return bytes.Compare(sorted[i].Content, sorted[j].Content) < 0
	})
	hash := sha256.New()
	for _, file := range sorted {
		hash.Write([]byte(file.Name))
		hash.Write([]byte{0})
		hash.Write(file.Content)
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func decodeConfigObject(raw []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()

	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return nil, fmt.Errorf("config must be a top-level object")
	}

	object := make(map[string]any)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := token.(string)
		if !ok {
			return nil, fmt.Errorf("top-level object key is not a string")
		}
		if _, exists := object[key]; exists {
			return nil, fmt.Errorf("duplicate top-level key %q", key)
		}

		var value any
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		object[key] = value
	}

	token, err = decoder.Token()
	if err != nil {
		return nil, err
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '}' {
		return nil, fmt.Errorf("config must be a top-level object")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("multiple JSON values")
		}
		return nil, err
	}
	return object, nil
}
