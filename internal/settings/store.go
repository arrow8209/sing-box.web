// Package settings owns durable service settings that are not part of a
// sing-box configuration profile.
package settings

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	"singbox-web/internal/storage"
)

const (
	settingsVersion  = 1
	maxProxyURLBytes = 2048
	defaultAutoStart = true
)

var ErrInvalidProxyURL = errors.New("代理地址无效")

type document struct {
	Version    int    `json:"version"`
	ProxyURL   string `json:"proxyUrl"`
	AutoStart  bool   `json:"autoStart"`
	AutoReload bool   `json:"autoReload"`
}

// diskDocument keeps the two behavior fields optional while reading so
// settings.json files written before they existed retain the old startup
// behavior. Every subsequent write stores the complete canonical document.
type diskDocument struct {
	Version    int    `json:"version"`
	ProxyURL   string `json:"proxyUrl"`
	AutoStart  *bool  `json:"autoStart,omitempty"`
	AutoReload *bool  `json:"autoReload,omitempty"`
}

// ProxySettings is the public settings representation returned by the API.
type ProxySettings struct {
	ProxyURL string `json:"proxyUrl"`
}

// CoreSettings controls service-owned core lifecycle behavior. AutoStart is
// evaluated when the Web service starts; AutoReload is evaluated after each
// successful draft save.
type CoreSettings struct {
	AutoStart  bool `json:"autoStart"`
	AutoReload bool `json:"autoReload"`
}

// Store keeps the last durably committed service settings in memory. Download
// clients can consult the proxy for each request without rereading the file.
type Store struct {
	file  string
	mu    sync.RWMutex
	state document
	proxy *url.URL
}

func OpenStore(file string) (*Store, error) {
	if file == "" {
		return nil, fmt.Errorf("设置文件不能为空")
	}
	state := document{Version: settingsVersion, AutoStart: defaultAutoStart}
	if loaded, err := storage.ReadJSON[diskDocument](file); err == nil {
		state.Version = loaded.Version
		state.ProxyURL = loaded.ProxyURL
		if loaded.AutoStart != nil {
			state.AutoStart = *loaded.AutoStart
		}
		if loaded.AutoReload != nil {
			state.AutoReload = *loaded.AutoReload
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	normalized, proxy, err := normalizeProxyURL(state.ProxyURL)
	if err != nil || state.Version != settingsVersion || normalized != state.ProxyURL {
		if err == nil && state.Version != settingsVersion {
			err = fmt.Errorf("不支持的设置版本: %d", state.Version)
		}
		if err == nil {
			err = fmt.Errorf("代理地址未使用规范格式")
		}
		return nil, &storage.CorruptFileError{Path: file, Err: err}
	}
	return &Store{file: file, state: state, proxy: proxy}, nil
}

func (s *Store) Get() (ProxySettings, error) {
	if s == nil {
		return ProxySettings{}, fmt.Errorf("设置服务不可用")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return ProxySettings{ProxyURL: s.state.ProxyURL}, nil
}

func (s *Store) GetCore() (CoreSettings, error) {
	if s == nil {
		return CoreSettings{}, fmt.Errorf("设置服务不可用")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return CoreSettings{AutoStart: s.state.AutoStart, AutoReload: s.state.AutoReload}, nil
}

// UpdateProxy writes the new value before making it effective for downloads.
// A failed durable write therefore leaves both memory and disk on the old URL.
func (s *Store) UpdateProxy(rawURL string) (ProxySettings, error) {
	if s == nil || s.file == "" {
		return ProxySettings{}, fmt.Errorf("设置服务不可用")
	}
	normalized, proxy, err := normalizeProxyURL(rawURL)
	if err != nil {
		return ProxySettings{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.state
	next.ProxyURL = normalized
	if err := storage.WriteJSON(s.file, next, 0o600); err != nil {
		return ProxySettings{}, fmt.Errorf("写入代理设置失败: %w", err)
	}
	s.state = next
	s.proxy = proxy
	return ProxySettings{ProxyURL: normalized}, nil
}

// UpdateCore persists both behavior switches as one setting so a failed write
// cannot leave the UI showing a partially committed combination.
func (s *Store) UpdateCore(input CoreSettings) (CoreSettings, error) {
	if s == nil || s.file == "" {
		return CoreSettings{}, fmt.Errorf("设置服务不可用")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.state
	next.AutoStart = input.AutoStart
	next.AutoReload = input.AutoReload
	if err := storage.WriteJSON(s.file, next, 0o600); err != nil {
		return CoreSettings{}, fmt.Errorf("写入核心设置失败: %w", err)
	}
	s.state = next
	return CoreSettings{AutoStart: next.AutoStart, AutoReload: next.AutoReload}, nil
}

// ProxyURL returns the currently configured proxy for both HTTP and HTTPS
// requests. A nil result is an explicit direct connection and does not fall
// back to HTTP_PROXY or HTTPS_PROXY from the process environment.
func (s *Store) ProxyURL(_ *http.Request) (*url.URL, error) {
	if s == nil {
		return nil, fmt.Errorf("设置服务不可用")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.proxy == nil {
		return nil, nil
	}
	copy := *s.proxy
	return &copy, nil
}

func normalizeProxyURL(value string) (string, *url.URL, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil, nil
	}
	if len(value) > maxProxyURLBytes {
		return "", nil, ErrInvalidProxyURL
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Opaque != "" || parsed.Host == "" || parsed.Hostname() == "" || parsed.User != nil {
		return "", nil, ErrInvalidProxyURL
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", nil, ErrInvalidProxyURL
	}
	if strings.HasSuffix(parsed.Host, ":") {
		return "", nil, ErrInvalidProxyURL
	}
	if port := parsed.Port(); port != "" {
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 {
			return "", nil, ErrInvalidProxyURL
		}
	}
	if (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return "", nil, ErrInvalidProxyURL
	}
	parsed.Path = ""
	return parsed.String(), parsed, nil
}
