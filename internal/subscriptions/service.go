package subscriptions

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"singbox-web/internal/paths"
)

const (
	maxNameRunes        = 80
	maxNodeCount        = 10000
	maxNodeBytes        = 1 << 20
	maxSubscriptionSize = 4 << 20
)

type Service struct {
	store  *store
	clock  func() time.Time
	random io.Reader
	mu     sync.Mutex
}

func Open(layout paths.Layout, clock func() time.Time, randomSource io.Reader) (*Service, error) {
	if clock == nil {
		clock = time.Now
	}
	if randomSource == nil {
		randomSource = rand.Reader
	}
	service := &Service{store: newStore(layout.SubscriptionsFile), clock: clock, random: randomSource}
	if _, err := service.store.load(); err != nil {
		return nil, err
	}
	return service, nil
}

func ValidateManagedState(layout paths.Layout) error {
	_, err := newStore(layout.SubscriptionsFile).load()
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

func (s *Service) Get(ctx context.Context, id string) (Subscription, error) {
	if err := ctx.Err(); err != nil {
		return Subscription{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.store.load()
	if err != nil {
		return Subscription{}, err
	}
	index := find(items, id)
	if index < 0 {
		return Subscription{}, ErrNotFound
	}
	return cloneSubscription(items[index]), nil
}

func (s *Service) Create(ctx context.Context, input Input) (Summary, error) {
	name, outbounds, err := normalizeInput(input)
	if err != nil {
		return Summary{}, err
	}
	if err := ctx.Err(); err != nil {
		return Summary{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.store.load()
	if err != nil {
		return Summary{}, err
	}
	id, err := s.uniqueID(items)
	if err != nil {
		return Summary{}, err
	}
	item := Subscription{ID: id, Name: name, Outbounds: outbounds, UpdatedAt: s.clock().UTC()}
	items = append(items, item)
	if err := s.store.write(items); err != nil {
		return Summary{}, err
	}
	return summarize(item), nil
}

func (s *Service) Replace(ctx context.Context, id string, input Input) (Summary, error) {
	name, outbounds, err := normalizeInput(input)
	if err != nil {
		return Summary{}, err
	}
	if err := ctx.Err(); err != nil {
		return Summary{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.store.load()
	if err != nil {
		return Summary{}, err
	}
	index := find(items, id)
	if index < 0 {
		return Summary{}, ErrNotFound
	}
	items[index] = Subscription{ID: id, Name: name, Outbounds: outbounds, UpdatedAt: s.clock().UTC()}
	if err := s.store.write(items); err != nil {
		return Summary{}, err
	}
	return summarize(items[index]), nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.store.load()
	if err != nil {
		return err
	}
	index := find(items, id)
	if index < 0 {
		return ErrNotFound
	}
	items = append(append([]Subscription(nil), items[:index]...), items[index+1:]...)
	return s.store.write(items)
}

func normalizeInput(input Input) (string, []json.RawMessage, error) {
	name, err := normalizeName(input.Name)
	if err != nil {
		return "", nil, err
	}
	outbounds, err := normalizeOutbounds(input.Outbounds)
	if err != nil {
		return "", nil, err
	}
	payload, err := json.Marshal(Input{Name: name, Outbounds: outbounds})
	if err != nil {
		return "", nil, err
	}
	if len(payload) > maxSubscriptionSize {
		return "", nil, fmt.Errorf("单个订阅不能超过 4 MiB")
	}
	return name, outbounds, nil
}

func normalizeName(value string) (string, error) {
	name := strings.TrimSpace(value)
	if name == "" {
		return "", fmt.Errorf("订阅名称不能为空")
	}
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) > maxNameRunes {
		return "", fmt.Errorf("订阅名称不能超过 %d 个字符", maxNameRunes)
	}
	return name, nil
}

func normalizeOutbounds(source []json.RawMessage) ([]json.RawMessage, error) {
	if len(source) == 0 {
		return nil, fmt.Errorf("请至少添加一个出站节点")
	}
	if len(source) > maxNodeCount {
		return nil, fmt.Errorf("单个订阅最多包含 %d 个节点", maxNodeCount)
	}
	result := make([]json.RawMessage, len(source))
	tags := make(map[string]struct{}, len(source))
	for index, raw := range source {
		if len(raw) == 0 || len(raw) > maxNodeBytes {
			return nil, fmt.Errorf("第 %d 个节点无效或超过 1 MiB", index+1)
		}
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil || object == nil {
			return nil, fmt.Errorf("第 %d 个节点必须是 JSON 对象", index+1)
		}
		typeName, err := requiredString(object, "type")
		if err != nil {
			return nil, fmt.Errorf("第 %d 个节点的 type 必须是非空字符串", index+1)
		}
		tag, err := requiredString(object, "tag")
		if err != nil {
			return nil, fmt.Errorf("第 %d 个节点的 tag 必须是非空字符串", index+1)
		}
		if _, exists := tags[tag]; exists {
			return nil, fmt.Errorf("节点标签不能重复: %s", tag)
		}
		tags[tag] = struct{}{}
		object["type"], _ = json.Marshal(typeName)
		object["tag"], _ = json.Marshal(tag)
		normalized, err := json.Marshal(object)
		if err != nil {
			return nil, err
		}
		result[index] = normalized
	}
	return result, nil
}

func requiredString(object map[string]json.RawMessage, key string) (string, error) {
	var value string
	if err := json.Unmarshal(object[key], &value); err != nil {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("字段为空")
	}
	return value, nil
}

func summarize(item Subscription) Summary {
	protocols := make([]string, 0)
	seen := map[string]struct{}{}
	for _, outbound := range item.Outbounds {
		var header struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(outbound, &header) != nil {
			continue
		}
		if _, exists := seen[header.Type]; exists {
			continue
		}
		seen[header.Type] = struct{}{}
		protocols = append(protocols, header.Type)
	}
	return Summary{ID: item.ID, Name: item.Name, NodeCount: len(item.Outbounds), Protocols: protocols, UpdatedAt: item.UpdatedAt}
}

func find(items []Subscription, id string) int {
	for index := range items {
		if items[index].ID == id {
			return index
		}
	}
	return -1
}

func (s *Service) uniqueID(items []Subscription) (string, error) {
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
	return "", fmt.Errorf("无法生成唯一订阅 ID")
}
