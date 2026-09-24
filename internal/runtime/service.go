package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"singbox-web/internal/core"
	"singbox-web/internal/paths"
	"singbox-web/internal/storage"
)

const (
	maximumConfigBytes     = 2 << 20
	maximumConnectionsBody = 16 << 20
)

var (
	ErrCoreNotRunning          = errors.New("core_not_running")
	ErrControllerNotConfigured = errors.New("clash_api_not_configured")
	ErrControllerUnavailable   = errors.New("clash_api_unavailable")
	ErrConnectionNotFound      = errors.New("connection_not_found")
	ErrInvalidConnectionID     = errors.New("invalid_connection_id")
	connectionIDPattern        = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
)

type statusSource interface {
	Status(context.Context) core.Status
}

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Service reads only the currently applied config directory and talks to the
// loopback Clash API. Draft controller settings never affect runtime access.
type Service struct {
	layout paths.Layout
	core   statusSource
	http   httpDoer
}

type controllerSettings struct {
	address string
	secret  string
}

func NewService(layout paths.Layout, status statusSource) *Service {
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           (&net.Dialer{Timeout: 2 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     false,
		MaxIdleConns:          4,
		MaxIdleConnsPerHost:   2,
		IdleConnTimeout:       30 * time.Second,
		ResponseHeaderTimeout: 3 * time.Second,
	}
	return NewServiceWithClient(layout, status, &http.Client{Transport: transport, Timeout: 5 * time.Second})
}

// NewServiceWithClient keeps controller transport deterministic in tests.
func NewServiceWithClient(layout paths.Layout, status statusSource, client httpDoer) *Service {
	return &Service{layout: layout, core: status, http: client}
}

func (s *Service) Connections(ctx context.Context) (ConnectionsSnapshot, error) {
	var snapshot ConnectionsSnapshot
	payload, err := s.readConnectionsPayload(ctx)
	if err != nil {
		return snapshot, err
	}
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		return snapshot, fmt.Errorf("%w: invalid response", ErrControllerUnavailable)
	}
	if snapshot.Connections == nil {
		snapshot.Connections = []Connection{}
	}
	for index := range snapshot.Connections {
		if snapshot.Connections[index].Metadata == nil {
			snapshot.Connections[index].Metadata = map[string]any{}
		}
		if snapshot.Connections[index].Chains == nil {
			snapshot.Connections[index].Chains = []string{}
		}
	}
	return snapshot, nil
}

func (s *Service) readConnectionsPayload(ctx context.Context) ([]byte, error) {
	response, err := s.request(ctx, http.MethodGet, "/connections", nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", ErrControllerUnavailable, response.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, maximumConnectionsBody+1))
	if err != nil {
		return nil, fmt.Errorf("%w: read response", ErrControllerUnavailable)
	}
	if len(payload) > maximumConnectionsBody {
		return nil, fmt.Errorf("%w: response too large", ErrControllerUnavailable)
	}
	return payload, nil
}

func (s *Service) CloseConnection(ctx context.Context, id string) error {
	if !connectionIDPattern.MatchString(id) {
		return ErrInvalidConnectionID
	}
	return s.close(ctx, "/connections/"+id)
}

func (s *Service) CloseAllConnections(ctx context.Context) error {
	return s.close(ctx, "/connections")
}

func (s *Service) close(ctx context.Context, path string) error {
	response, err := s.request(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	switch response.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return ErrConnectionNotFound
	default:
		return fmt.Errorf("%w: status %d", ErrControllerUnavailable, response.StatusCode)
	}
}

func (s *Service) request(ctx context.Context, method, escapedPath string, body io.Reader) (*http.Response, error) {
	if s == nil || s.core == nil || s.http == nil {
		return nil, ErrControllerUnavailable
	}
	if s.core.Status(ctx).State != core.ProcessRunning {
		return nil, ErrCoreNotRunning
	}
	settings, err := s.loadControllerSettings()
	if err != nil {
		return nil, err
	}
	endpoint := url.URL{Scheme: "http", Host: settings.address}
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String()+escapedPath, body)
	if err != nil {
		return nil, fmt.Errorf("%w: create request", ErrControllerUnavailable)
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if settings.secret != "" {
		request.Header.Set("Authorization", "Bearer "+settings.secret)
	}
	response, err := s.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: request failed", ErrControllerUnavailable)
	}
	return response, nil
}

func (s *Service) loadControllerSettings() (controllerSettings, error) {
	path := filepath.Join(s.layout.ConfigDir, "99-experimental.json")
	if err := storage.RejectSymlinkPath(path); err != nil {
		return controllerSettings{}, fmt.Errorf("%w: unsafe config path", ErrControllerNotConfigured)
	}
	file, err := os.Open(path)
	if err != nil {
		return controllerSettings{}, fmt.Errorf("%w: applied config unavailable", ErrControllerNotConfigured)
	}
	defer file.Close()
	payload, err := io.ReadAll(io.LimitReader(file, maximumConfigBytes+1))
	if err != nil || len(payload) > maximumConfigBytes {
		return controllerSettings{}, fmt.Errorf("%w: applied config unreadable", ErrControllerNotConfigured)
	}
	var module struct {
		Experimental json.RawMessage `json:"experimental"`
	}
	if err := json.Unmarshal(payload, &module); err != nil || len(module.Experimental) == 0 {
		return controllerSettings{}, fmt.Errorf("%w: experimental config unavailable", ErrControllerNotConfigured)
	}
	var experimental struct {
		ClashAPI json.RawMessage `json:"clash_api"`
	}
	if err := json.Unmarshal(module.Experimental, &experimental); err != nil || len(experimental.ClashAPI) == 0 {
		return controllerSettings{}, fmt.Errorf("%w: clash api unavailable", ErrControllerNotConfigured)
	}
	var clashAPI struct {
		ExternalController string `json:"external_controller"`
		Secret             string `json:"secret"`
	}
	if err := json.Unmarshal(experimental.ClashAPI, &clashAPI); err != nil {
		return controllerSettings{}, fmt.Errorf("%w: clash api invalid", ErrControllerNotConfigured)
	}
	address, err := localControllerAddress(clashAPI.ExternalController)
	if err != nil {
		return controllerSettings{}, fmt.Errorf("%w: controller address invalid", ErrControllerNotConfigured)
	}
	return controllerSettings{address: address, secret: clashAPI.Secret}, nil
}

func localControllerAddress(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, "://") {
		return "", errors.New("controller address missing")
	}
	host, portValue, err := net.SplitHostPort(value)
	if err != nil {
		return "", err
	}
	port, err := strconv.Atoi(portValue)
	if err != nil || port < 1 || port > 65535 {
		return "", errors.New("controller port invalid")
	}
	switch strings.ToLower(host) {
	case "localhost":
		host = "127.0.0.1"
	default:
		ip := net.ParseIP(host)
		if ip == nil || (!ip.IsLoopback() && !ip.IsUnspecified()) {
			return "", errors.New("controller must be local")
		}
		if ip.IsUnspecified() {
			if ip.To4() == nil {
				host = "::1"
			} else {
				host = "127.0.0.1"
			}
		}
	}
	return net.JoinHostPort(host, strconv.Itoa(port)), nil
}
