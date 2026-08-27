package inbounds

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"singbox-web/internal/storage"
)

type document struct {
	Version  int       `json:"version"`
	Inbounds []Inbound `json:"inbounds"`
}

type store struct {
	path string
}

var inboundIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

func newStore(path string) *store { return &store{path: path} }

func (s *store) load() ([]Inbound, error) {
	if _, err := os.Stat(s.path); os.IsNotExist(err) {
		return []Inbound{}, nil
	} else if err != nil {
		return nil, err
	}
	value, err := storage.ReadJSON[document](s.path)
	if err != nil {
		return nil, err
	}
	if value.Version != 1 {
		return nil, s.corrupt(fmt.Errorf("不支持的入站配置索引版本: %d", value.Version))
	}
	if value.Inbounds == nil {
		return nil, s.corrupt(fmt.Errorf("inbounds 不能为 null"))
	}
	if err := validateInbounds(value.Inbounds); err != nil {
		return nil, s.corrupt(err)
	}
	return cloneInbounds(value.Inbounds), nil
}

func (s *store) write(items []Inbound) error {
	if items == nil {
		items = []Inbound{}
	}
	if err := validateInbounds(items); err != nil {
		return err
	}
	return storage.WriteJSON(s.path, document{Version: 1, Inbounds: cloneInbounds(items)}, 0o600)
}

func (s *store) corrupt(err error) error {
	return &storage.CorruptFileError{Path: s.path, Err: err}
}

func validateInbounds(items []Inbound) error {
	if len(items) > maxInboundCount {
		return fmt.Errorf("入站配置最多包含 %d 项", maxInboundCount)
	}
	ids := make(map[string]struct{}, len(items))
	tags := make(map[string]struct{}, len(items))
	totalBytes := 0
	for index, item := range items {
		if !inboundIDPattern.MatchString(item.ID) {
			return fmt.Errorf("inbounds[%d].id 无效", index)
		}
		if _, exists := ids[item.ID]; exists {
			return fmt.Errorf("inbounds[%d].id 重复", index)
		}
		ids[item.ID] = struct{}{}
		mode, config, routing, header, err := normalizeInput(Input{Mode: item.Mode, Config: item.Config, Routing: item.Routing})
		if err != nil {
			return fmt.Errorf("inbounds[%d]: %w", index, err)
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, item.Config); err != nil {
			return fmt.Errorf("inbounds[%d].config 无效: %w", index, err)
		}
		totalBytes += compact.Len()
		if item.Routing != nil {
			routingJSON, _ := json.Marshal(item.Routing)
			totalBytes += len(routingJSON)
		}
		if totalBytes > maxLibraryBytes {
			return fmt.Errorf("入站配置资源库不能超过 16 MiB")
		}
		if mode != item.Mode || !bytes.Equal(config, compact.Bytes()) || !equalRouting(routing, item.Routing) {
			return fmt.Errorf("inbounds[%d] 未规范化", index)
		}
		if _, exists := tags[header.Tag]; exists {
			return fmt.Errorf("inbounds[%d].config.tag 重复: %s", index, header.Tag)
		}
		tags[header.Tag] = struct{}{}
		if item.UpdatedAt.IsZero() {
			return fmt.Errorf("inbounds[%d].updatedAt 无效", index)
		}
	}
	return nil
}

func cloneInbound(source Inbound) Inbound {
	result := source
	result.Config = append([]byte(nil), source.Config...)
	if source.Routing != nil {
		routing := *source.Routing
		result.Routing = &routing
	}
	return result
}

func equalRouting(left, right *RoutingIntent) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func cloneInbounds(source []Inbound) []Inbound {
	result := make([]Inbound, len(source))
	for index := range source {
		result[index] = cloneInbound(source[index])
	}
	return result
}
