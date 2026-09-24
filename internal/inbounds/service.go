package inbounds

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"singbox-web/internal/configref"
	"singbox-web/internal/paths"
)

const (
	maxInboundCount = 1000
	maxInboundBytes = 1 << 20
	maxLibraryBytes = 16 << 20
	maxTagRunes     = 128
	maxTypeRunes    = 64
)

var structuredTypes = map[string]struct{}{
	"mixed": {}, "http": {}, "socks": {}, "tun": {}, "direct": {},
}

type configHeader struct {
	Type string
	Tag  string
}

type Service struct {
	store    *store
	clock    func() time.Time
	random   io.Reader
	mu       sync.Mutex
	onChange func(context.Context, string)
}

func (s *Service) SetOnChange(hook func(context.Context, string)) { s.onChange = hook }

func (s *Service) CurrentDigests(ctx context.Context) (map[string]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.store.load()
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(items))
	for _, item := range items {
		result["inbound:"+item.ID] = configref.InboundDigest(string(item.Mode), item.Config)
	}
	return result, nil
}

func Open(layout paths.Layout, clock func() time.Time, randomSource io.Reader) (*Service, error) {
	if clock == nil {
		clock = time.Now
	}
	if randomSource == nil {
		randomSource = rand.Reader
	}
	service := &Service{store: newStore(layout.InboundResourcesFile), clock: clock, random: randomSource}
	if _, err := service.store.load(); err != nil {
		return nil, err
	}
	return service, nil
}

func ValidateManagedState(layout paths.Layout) error {
	_, err := newStore(layout.InboundResourcesFile).load()
	return err
}

func (s *Service) List(ctx context.Context) ([]Summary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.store.load()
	if err != nil {
		return nil, err
	}
	result := make([]Summary, len(items))
	for index := range items {
		result[index] = summarize(items[index])
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context, id string) (Inbound, error) {
	if err := ctx.Err(); err != nil {
		return Inbound{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.store.load()
	if err != nil {
		return Inbound{}, err
	}
	index := find(items, id)
	if index < 0 {
		return Inbound{}, ErrNotFound
	}
	return cloneInbound(items[index]), nil
}

func (s *Service) Create(ctx context.Context, input Input) (Summary, error) {
	mode, config, header, err := normalizeInput(input)
	if err != nil {
		return Summary{}, err
	}
	if err := ctx.Err(); err != nil {
		return Summary{}, err
	}
	s.mu.Lock()
	stored := false
	changedID := ""
	defer func() {
		s.mu.Unlock()
		if stored && s.onChange != nil {
			s.onChange(context.Background(), changedID)
		}
	}()
	items, err := s.store.load()
	if err != nil {
		return Summary{}, err
	}
	if len(items) >= maxInboundCount {
		return Summary{}, fmt.Errorf("入站配置最多包含 %d 项", maxInboundCount)
	}
	if tagExists(items, header.Tag, "") {
		return Summary{}, fmt.Errorf("%w: %s", ErrDuplicateTag, header.Tag)
	}
	id, err := s.uniqueID(items)
	if err != nil {
		return Summary{}, err
	}
	item := Inbound{ID: id, Mode: mode, Config: config, UpdatedAt: s.clock().UTC()}
	items = append([]Inbound{item}, items...)
	if err := s.store.write(items); err != nil {
		return Summary{}, err
	}
	stored, changedID = true, id
	return summarize(item), nil
}

func (s *Service) Replace(ctx context.Context, id string, input Input) (Summary, error) {
	mode, config, header, err := normalizeInput(input)
	if err != nil {
		return Summary{}, err
	}
	if err := ctx.Err(); err != nil {
		return Summary{}, err
	}
	s.mu.Lock()
	stored := false
	defer func() {
		s.mu.Unlock()
		if stored && s.onChange != nil {
			s.onChange(context.Background(), id)
		}
	}()
	items, err := s.store.load()
	if err != nil {
		return Summary{}, err
	}
	index := find(items, id)
	if index < 0 {
		return Summary{}, ErrNotFound
	}
	if tagExists(items, header.Tag, id) {
		return Summary{}, fmt.Errorf("%w: %s", ErrDuplicateTag, header.Tag)
	}
	items[index] = Inbound{ID: id, Mode: mode, Config: config, UpdatedAt: s.clock().UTC()}
	if err := s.store.write(items); err != nil {
		return Summary{}, err
	}
	stored = true
	return summarize(items[index]), nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	stored := false
	defer func() {
		s.mu.Unlock()
		if stored && s.onChange != nil {
			s.onChange(context.Background(), id)
		}
	}()
	items, err := s.store.load()
	if err != nil {
		return err
	}
	index := find(items, id)
	if index < 0 {
		return ErrNotFound
	}
	items = append(append([]Inbound(nil), items[:index]...), items[index+1:]...)
	if err := s.store.write(items); err != nil {
		return err
	}
	stored = true
	return nil
}

func normalizeInput(input Input) (Mode, json.RawMessage, configHeader, error) {
	if input.Mode != ModeStructured && input.Mode != ModeCustom {
		return "", nil, configHeader{}, fmt.Errorf("mode 必须是 structured 或 custom")
	}
	if len(input.Config) == 0 || len(input.Config) > maxInboundBytes {
		return "", nil, configHeader{}, fmt.Errorf("单个入站配置无效或超过 1 MiB")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(input.Config, &object); err != nil || object == nil {
		return "", nil, configHeader{}, fmt.Errorf("config 必须是 JSON 对象")
	}
	typeName, err := requiredString(object, "type", maxTypeRunes)
	if err != nil {
		return "", nil, configHeader{}, fmt.Errorf("config.type 必须是非空字符串且不超过 %d 个字符", maxTypeRunes)
	}
	tag, err := requiredString(object, "tag", maxTagRunes)
	if err != nil {
		return "", nil, configHeader{}, fmt.Errorf("config.tag 必须是非空字符串且不超过 %d 个字符", maxTagRunes)
	}
	if input.Mode == ModeStructured {
		if _, supported := structuredTypes[typeName]; !supported {
			return "", nil, configHeader{}, fmt.Errorf("结构化编辑不支持入站类型: %s", typeName)
		}
	}
	object["type"], _ = json.Marshal(typeName)
	object["tag"], _ = json.Marshal(tag)
	normalized, err := json.Marshal(object)
	if err != nil {
		return "", nil, configHeader{}, err
	}
	return input.Mode, normalized, configHeader{Type: typeName, Tag: tag}, nil
}

func requiredString(object map[string]json.RawMessage, key string, limit int) (string, error) {
	var value string
	if err := json.Unmarshal(object[key], &value); err != nil {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > limit {
		return "", fmt.Errorf("字段无效")
	}
	return value, nil
}

func summarize(item Inbound) Summary {
	var object map[string]json.RawMessage
	_ = json.Unmarshal(item.Config, &object)
	typeName, _ := requiredString(object, "type", maxTypeRunes)
	tag, _ := requiredString(object, "tag", maxTagRunes)
	return Summary{
		ID: item.ID, Mode: item.Mode, Type: typeName, Tag: tag,
		Endpoint: endpoint(typeName, object), FieldCount: len(object), UpdatedAt: item.UpdatedAt,
	}
}

func endpoint(typeName string, object map[string]json.RawMessage) string {
	if typeName == "tun" {
		name := optionalString(object, "interface_name")
		if name == "" {
			name = "自动命名"
		}
		var addresses []string
		_ = json.Unmarshal(object["address"], &addresses)
		if len(addresses) > 0 && strings.TrimSpace(addresses[0]) != "" {
			return name + " · " + strings.TrimSpace(addresses[0])
		}
		return name
	}
	listen := optionalString(object, "listen")
	if listen == "" {
		listen = "由完整 JSON 定义"
	}
	if raw := object["listen_port"]; len(raw) > 0 {
		var number json.Number
		if json.Unmarshal(raw, &number) == nil {
			if _, err := strconv.ParseUint(number.String(), 10, 16); err == nil {
				return listen + ":" + number.String()
			}
		}
	}
	return listen
}

func optionalString(object map[string]json.RawMessage, key string) string {
	var value string
	if json.Unmarshal(object[key], &value) != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func tagExists(items []Inbound, tag, excludeID string) bool {
	for _, item := range items {
		if item.ID == excludeID {
			continue
		}
		var object map[string]json.RawMessage
		if json.Unmarshal(item.Config, &object) != nil {
			continue
		}
		candidate, _ := requiredString(object, "tag", maxTagRunes)
		if candidate == tag {
			return true
		}
	}
	return false
}

func find(items []Inbound, id string) int {
	for index := range items {
		if items[index].ID == id {
			return index
		}
	}
	return -1
}

func (s *Service) uniqueID(items []Inbound) (string, error) {
	for range 8 {
		buffer := make([]byte, 16)
		if _, err := io.ReadFull(s.random, buffer); err != nil {
			return "", err
		}
		id := hex.EncodeToString(buffer)
		if find(items, id) < 0 {
			return id, nil
		}
	}
	return "", fmt.Errorf("无法生成唯一入站配置 ID")
}
