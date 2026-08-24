package subscriptions

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"singbox-web/internal/storage"
)

type document struct {
	Version       int            `json:"version"`
	Subscriptions []Subscription `json:"subscriptions"`
}

type store struct {
	path string
}

var subscriptionIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

func newStore(path string) *store { return &store{path: path} }

func (s *store) load() ([]Subscription, error) {
	if _, err := os.Stat(s.path); os.IsNotExist(err) {
		return []Subscription{}, nil
	} else if err != nil {
		return nil, err
	}
	value, err := storage.ReadJSON[document](s.path)
	if err != nil {
		return nil, err
	}
	if value.Version != 1 {
		return nil, s.corrupt(fmt.Errorf("不支持的订阅索引版本: %d", value.Version))
	}
	if value.Subscriptions == nil {
		return nil, s.corrupt(fmt.Errorf("subscriptions 不能为 null"))
	}
	if err := validateSubscriptions(value.Subscriptions); err != nil {
		return nil, s.corrupt(err)
	}
	return cloneSubscriptions(value.Subscriptions), nil
}

func (s *store) write(items []Subscription) error {
	if items == nil {
		items = []Subscription{}
	}
	if err := validateSubscriptions(items); err != nil {
		return err
	}
	return storage.WriteJSON(s.path, document{Version: 1, Subscriptions: cloneSubscriptions(items)}, 0o600)
}

func (s *store) corrupt(err error) error {
	return &storage.CorruptFileError{Path: s.path, Err: err}
}

func validateSubscriptions(items []Subscription) error {
	seen := make(map[string]struct{}, len(items))
	for index, item := range items {
		if !subscriptionIDPattern.MatchString(item.ID) {
			return fmt.Errorf("subscriptions[%d].id 无效", index)
		}
		if _, exists := seen[item.ID]; exists {
			return fmt.Errorf("subscriptions[%d].id 重复", index)
		}
		seen[item.ID] = struct{}{}
		name, _, err := normalizeInput(Input{Name: item.Name, Outbounds: item.Outbounds})
		if err != nil {
			return fmt.Errorf("subscriptions[%d]: %w", index, err)
		}
		if name != item.Name {
			return fmt.Errorf("subscriptions[%d].name 未规范化", index)
		}
		if item.UpdatedAt.IsZero() {
			return fmt.Errorf("subscriptions[%d].updatedAt 无效", index)
		}
	}
	return nil
}

func cloneSubscription(source Subscription) Subscription {
	result := source
	result.Outbounds = cloneOutbounds(source.Outbounds)
	return result
}

func cloneSubscriptions(source []Subscription) []Subscription {
	result := make([]Subscription, len(source))
	for index := range source {
		result[index] = cloneSubscription(source[index])
	}
	return result
}

func cloneOutbounds(source []json.RawMessage) []json.RawMessage {
	result := make([]json.RawMessage, len(source))
	for index := range source {
		result[index] = append(json.RawMessage(nil), source[index]...)
	}
	return result
}
