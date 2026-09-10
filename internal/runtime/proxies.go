package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	ErrInvalidProxySelection = errors.New("invalid_proxy_selection")
	ErrProxyGroupNotFound    = errors.New("proxy_group_not_found")
	ErrProxyGroupReadOnly    = errors.New("proxy_group_read_only")
	ErrProxyNodeNotFound     = errors.New("proxy_node_not_found")
	ErrProxySelectionFailed  = errors.New("proxy_selection_failed")
)

type ProxyMember struct {
	Tag  string `json:"tag"`
	Type string `json:"type"`
}

type ProxyGroup struct {
	Tag      string        `json:"tag"`
	Type     string        `json:"type"`
	Selected string        `json:"selected"`
	Members  []ProxyMember `json:"members"`
}

type ProxySelection struct {
	Tag      string `json:"tag"`
	Selected string `json:"selected"`
}

type clashProxy struct {
	Type string   `json:"type"`
	Now  string   `json:"now"`
	All  []string `json:"all"`
}

// ProxyGroups exposes only runtime group membership and selection. The
// controller's synthetic GLOBAL/Fallback entry is not a selectable outbound.
func (s *Service) ProxyGroups(ctx context.Context) ([]ProxyGroup, error) {
	var snapshot struct {
		Proxies map[string]clashProxy `json:"proxies"`
	}
	if err := s.readProxyJSON(ctx, "/proxies", &snapshot); err != nil {
		return nil, err
	}
	if snapshot.Proxies == nil {
		return nil, ErrControllerUnavailable
	}
	// sing-box's synthetic GLOBAL lists outbounds in configuration order.
	order := append([]string(nil), snapshot.Proxies["GLOBAL"].All...)
	remaining := make([]string, 0, len(snapshot.Proxies))
	for tag := range snapshot.Proxies {
		remaining = append(remaining, tag)
	}
	sort.Strings(remaining)
	order = append(order, remaining...)
	seen := make(map[string]bool)
	groups := make([]ProxyGroup, 0)
	for _, tag := range order {
		proxy, exists := snapshot.Proxies[tag]
		if !exists || seen[tag] || (proxy.Type != "Selector" && proxy.Type != "URLTest") {
			continue
		}
		seen[tag] = true
		group := ProxyGroup{Tag: tag, Type: proxy.Type, Selected: proxy.Now, Members: make([]ProxyMember, 0, len(proxy.All))}
		for _, member := range proxy.All {
			group.Members = append(group.Members, ProxyMember{Tag: member, Type: snapshot.Proxies[member].Type})
		}
		groups = append(groups, group)
	}
	return groups, nil
}

func (s *Service) SelectProxy(ctx context.Context, input ProxySelection) (ProxySelection, error) {
	if !validProxyTag(input.Tag) || !validProxyTag(input.Selected) {
		return ProxySelection{}, ErrInvalidProxySelection
	}
	path := "/proxies/" + url.PathEscape(input.Tag)
	var proxy clashProxy
	if err := s.readProxyJSON(ctx, path, &proxy); err != nil {
		return ProxySelection{}, err
	}
	if proxy.Type != "Selector" {
		return ProxySelection{}, ErrProxyGroupReadOnly
	}
	if !slices.Contains(proxy.All, input.Selected) {
		return ProxySelection{}, ErrProxyNodeNotFound
	}
	payload, err := json.Marshal(struct {
		Name string `json:"name"`
	}{Name: input.Selected})
	if err != nil {
		return ProxySelection{}, ErrInvalidProxySelection
	}
	response, err := s.request(ctx, http.MethodPut, path, bytes.NewReader(payload))
	if err != nil {
		return ProxySelection{}, err
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	response.Body.Close()
	switch response.StatusCode {
	case http.StatusOK, http.StatusNoContent:
	case http.StatusNotFound:
		return ProxySelection{}, ErrProxyGroupNotFound
	case http.StatusBadRequest:
		return ProxySelection{}, ErrProxySelectionFailed
	default:
		return ProxySelection{}, ErrControllerUnavailable
	}
	// Read back the core's selection before reporting success to the browser.
	if err := s.readProxyJSON(ctx, path, &proxy); err != nil {
		return ProxySelection{}, err
	}
	if proxy.Now != input.Selected {
		return ProxySelection{}, ErrProxySelectionFailed
	}
	return ProxySelection{Tag: input.Tag, Selected: proxy.Now}, nil
}

func validProxyTag(tag string) bool {
	return strings.TrimSpace(tag) != "" && len(tag) <= 4096 && utf8.ValidString(tag) && strings.IndexFunc(tag, unicode.IsControl) < 0
}

func (s *Service) readProxyJSON(ctx context.Context, path string, value any) error {
	response, err := s.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return ErrProxyGroupNotFound
	}
	if response.StatusCode != http.StatusOK {
		return ErrControllerUnavailable
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, maximumConnectionsBody+1))
	if err != nil || len(payload) > maximumConnectionsBody {
		return fmt.Errorf("%w: unreadable proxy response", ErrControllerUnavailable)
	}
	if err := json.Unmarshal(payload, value); err != nil {
		return fmt.Errorf("%w: invalid proxy response", ErrControllerUnavailable)
	}
	return nil
}
