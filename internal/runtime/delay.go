package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"singbox-web/internal/core"
)

const (
	proxyDelayURL           = "https://www.gstatic.com/generate_204"
	proxyDelayTimeout       = 5 * time.Second
	maximumDelayConcurrency = 5
)

var (
	ErrInvalidProxyDelay     = errors.New("invalid_proxy_delay")
	ErrProxyDelayUnsupported = errors.New("proxy_delay_unsupported")
	ErrProxyDelayBusy        = errors.New("proxy_delay_busy")
	ErrProxyRuntimeChanged   = errors.New("proxy_runtime_changed")
)

type ProxyDelayHistory struct {
	Delay    int       `json:"delay"`
	TestedAt time.Time `json:"testedAt"`
}

type ProxyDelayRequest struct {
	Tag        string `json:"tag"`
	RuntimeKey string `json:"runtimeKey"`
}

type ProxyDelayResult struct {
	Tag        string    `json:"tag"`
	Status     string    `json:"status"`
	Delay      int       `json:"delay,omitempty"`
	TestedAt   time.Time `json:"testedAt"`
	RuntimeKey string    `json:"runtimeKey"`
}

// Bind a test (including time spent queued in the browser) to one applied run.
func (s *Service) proxyRuntime(ctx context.Context) (string, error) {
	if s == nil || s.core == nil {
		return "", ErrControllerUnavailable
	}
	status := s.core.Status(ctx)
	if status.State != core.ProcessRunning {
		return "", ErrCoreNotRunning
	}
	if status.Operation.Active {
		return "", ErrProxyRuntimeChanged
	}
	var revision uint64
	if status.AppliedRevision != nil {
		revision = *status.AppliedRevision
	}
	return fmt.Sprintf("%s:%s:%d:%s", runtimeKey(status), status.CurrentProfileID, revision, status.ConfigDirectoryDigest), nil
}

func (s *Service) checkProxyRuntime(ctx context.Context, expected string) error {
	current, err := s.proxyRuntime(ctx)
	if err != nil {
		return err
	}
	if current != expected {
		return ErrProxyRuntimeChanged
	}
	return nil
}

func (s *Service) TestProxyDelay(ctx context.Context, input ProxyDelayRequest) (ProxyDelayResult, error) {
	if !validProxyTag(input.Tag) || input.RuntimeKey == "" || len(input.RuntimeKey) > 4096 {
		return ProxyDelayResult{}, ErrInvalidProxyDelay
	}
	if err := s.checkProxyRuntime(ctx, input.RuntimeKey); err != nil {
		return ProxyDelayResult{}, err
	}
	if s.delaySlots == nil || s.delayHTTP == nil {
		return ProxyDelayResult{}, ErrControllerUnavailable
	}
	// A server-wide limit also covers multiple tabs/users. Do not build an
	// unbounded second queue behind the browser's five workers.
	select {
	case s.delaySlots <- struct{}{}:
		defer func() { <-s.delaySlots }()
	default:
		return ProxyDelayResult{}, ErrProxyDelayBusy
	}
	path := "/proxies/" + url.PathEscape(input.Tag)
	var proxy clashProxy
	if err := s.readProxyJSON(ctx, path, &proxy); err != nil {
		if errors.Is(err, ErrProxyGroupNotFound) {
			err = ErrProxyNodeNotFound
		}
		return ProxyDelayResult{}, err
	}
	switch proxy.Type {
	case "", "Reject", "Block", "DNS", "Selector", "URLTest", "Fallback":
		return ProxyDelayResult{}, ErrProxyDelayUnsupported
	}
	if err := s.checkProxyRuntime(ctx, input.RuntimeKey); err != nil {
		return ProxyDelayResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return ProxyDelayResult{}, err
	}
	// sing-box 1.13 detaches delay tests from the HTTP request context. Keep
	// our slot until its bounded test finishes, even if the browser leaves.
	testCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 7*time.Second)
	defer cancel()
	query := url.Values{"url": {proxyDelayURL}, "timeout": {strconv.FormatInt(proxyDelayTimeout.Milliseconds(), 10)}}
	response, err := s.requestWithClient(testCtx, s.delayHTTP, http.MethodGet, path+"/delay?"+query.Encode(), nil)
	if err != nil {
		return ProxyDelayResult{}, err
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, (16<<10)+1))
	if err != nil || len(payload) > 16<<10 {
		return ProxyDelayResult{}, ErrControllerUnavailable
	}
	if err := s.checkProxyRuntime(testCtx, input.RuntimeKey); err != nil {
		return ProxyDelayResult{}, err
	}
	result := ProxyDelayResult{Tag: input.Tag, RuntimeKey: input.RuntimeKey, TestedAt: time.Now().UTC()}
	switch response.StatusCode {
	case http.StatusOK:
		var measured struct {
			Delay *int `json:"delay"`
		}
		if err := json.Unmarshal(payload, &measured); err != nil || measured.Delay == nil || *measured.Delay <= 0 || *measured.Delay > 65535 {
			return ProxyDelayResult{}, ErrControllerUnavailable
		}
		result.Status, result.Delay = "ok", *measured.Delay
	case http.StatusGatewayTimeout:
		result.Status = "timeout"
	case http.StatusServiceUnavailable:
		result.Status = "failed"
	case http.StatusNotFound:
		return ProxyDelayResult{}, ErrProxyNodeNotFound
	default:
		return ProxyDelayResult{}, ErrControllerUnavailable
	}
	return result, nil
}
