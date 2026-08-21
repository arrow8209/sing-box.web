package templates

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"singbox-web/internal/rules"
)

type BindingTemplate struct {
	TemplateKey string `json:"templateKey"`
	Tag         string `json:"tag"`
	Enabled     bool   `json:"enabled"`
}

type ProfileSeed struct {
	Config      map[string]json.RawMessage
	Buffers     map[string]string
	CreatedAt   time.Time
	ClashSecret string
	CacheID     string
}

type Factory struct {
	profile   map[string]json.RawMessage
	resources []rules.ResourceTemplate
	bindings  []BindingTemplate
	random    io.Reader
}

func NewFactory(randomSource io.Reader) (*Factory, error) {
	if randomSource == nil {
		randomSource = rand.Reader
	}
	var profile map[string]json.RawMessage
	var resources []rules.ResourceTemplate
	var bindings []BindingTemplate
	if err := json.Unmarshal(defaultProfile, &profile); err != nil {
		return nil, fmt.Errorf("默认档案模板无效: %w", err)
	}
	if err := json.Unmarshal(defaultResources, &resources); err != nil {
		return nil, fmt.Errorf("默认规则资源模板无效: %w", err)
	}
	if err := json.Unmarshal(defaultBindings, &bindings); err != nil {
		return nil, fmt.Errorf("默认规则绑定模板无效: %w", err)
	}
	return &Factory{profile: profile, resources: resources, bindings: bindings, random: randomSource}, nil
}

func (f *Factory) NewProfileSeed(now time.Time) (ProfileSeed, error) {
	config := cloneRawMap(f.profile)
	secret, err := randomHex(f.random, 32)
	if err != nil {
		return ProfileSeed{}, err
	}
	suffix, err := randomHex(f.random, 16)
	if err != nil {
		return ProfileSeed{}, err
	}
	cacheID := "ID_" + suffix
	if err := setNestedString(config, []string{"experimental", "clash_api", "secret"}, secret); err != nil {
		return ProfileSeed{}, err
	}
	if err := setNestedString(config, []string{"experimental", "cache_file", "cache_id"}, cacheID); err != nil {
		return ProfileSeed{}, err
	}
	return ProfileSeed{Config: config, Buffers: moduleBuffers(config), CreatedAt: now.UTC(), ClashSecret: secret, CacheID: cacheID}, nil
}

func (f *Factory) ResourceTemplates() []rules.ResourceTemplate {
	return append([]rules.ResourceTemplate(nil), f.resources...)
}
func (f *Factory) BindingTemplates() []BindingTemplate {
	return append([]BindingTemplate(nil), f.bindings...)
}

func randomHex(source io.Reader, size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := io.ReadFull(source, bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
func cloneRawMap(source map[string]json.RawMessage) map[string]json.RawMessage {
	target := make(map[string]json.RawMessage, len(source))
	for key, value := range source {
		target[key] = append(json.RawMessage(nil), value...)
	}
	return target
}
func setNestedString(current map[string]json.RawMessage, path []string, value string) error {
	if len(path) == 1 {
		raw, _ := json.Marshal(value)
		current[path[0]] = raw
		return nil
	}
	var child map[string]json.RawMessage
	if err := json.Unmarshal(current[path[0]], &child); err != nil || child == nil {
		return fmt.Errorf("默认档案缺少 %s", path[0])
	}
	if err := setNestedString(child, path[1:], value); err != nil {
		return err
	}
	raw, _ := json.Marshal(child)
	current[path[0]] = raw
	return nil
}
func moduleBuffers(config map[string]json.RawMessage) map[string]string {
	buffers := make(map[string]string, len(config))
	for key, value := range config {
		buffers[key] = string(value)
	}
	return buffers
}
