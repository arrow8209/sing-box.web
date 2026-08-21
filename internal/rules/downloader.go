package rules

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

const (
	maxRedirects          = 5
	maxRuleBytes          = int64(64 << 20)
	connectTimeout        = 10 * time.Second
	responseHeaderTimeout = 15 * time.Second
	totalDownloadTimeout  = 60 * time.Second
)

var (
	ErrUnsupportedURL   = errors.New("仅支持 HTTP 或 HTTPS 规则地址")
	ErrTooManyRedirects = errors.New("规则下载重定向次数超过限制")
	ErrResponseTooLarge = errors.New("规则响应超过 64 MiB")
)

type DownloadedFile struct {
	Path    string
	Digest  string
	Payload []byte
}
type Downloader interface {
	Fetch(context.Context, Resource) (DownloadedFile, error)
}
type httpDownloader struct {
	client   *http.Client
	maxBytes int64
}

type DownloadPolicy struct{ Transport http.RoundTripper }

func NewDownloader(policies ...DownloadPolicy) Downloader {
	if len(policies) != 0 {
		return newDownloader(&http.Client{Transport: policies[0].Transport, Timeout: totalDownloadTimeout}, maxRuleBytes)
	}
	transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: connectTimeout, KeepAlive: 30 * time.Second}).DialContext, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: responseHeaderTimeout}
	return newDownloader(&http.Client{Transport: transport, Timeout: totalDownloadTimeout}, maxRuleBytes)
}

// DownloadPolicyForTest permits tests to supply an in-process transport while
// retaining the same redirect validation and size limits.
func DownloadPolicyForTest(transport http.RoundTripper) DownloadPolicy {
	return DownloadPolicy{Transport: transport}
}
func newDownloader(client *http.Client, maximum int64) Downloader {
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return ErrTooManyRedirects
		}
		return validateRemoteURL(req.URL)
	}
	return &httpDownloader{client: client, maxBytes: maximum}
}
func validateRemoteURL(raw *url.URL) error {
	if raw == nil || (raw.Scheme != "http" && raw.Scheme != "https") || raw.Host == "" || raw.User != nil {
		return ErrUnsupportedURL
	}
	return nil
}
func (d *httpDownloader) Fetch(ctx context.Context, resource Resource) (DownloadedFile, error) {
	parsed, err := url.Parse(resource.URL)
	if err != nil {
		return DownloadedFile{}, ErrUnsupportedURL
	}
	if err := validateRemoteURL(parsed); err != nil {
		return DownloadedFile{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return DownloadedFile{}, err
	}
	response, err := d.client.Do(req)
	if err != nil {
		if errors.Is(err, ErrUnsupportedURL) || errors.Is(err, ErrTooManyRedirects) {
			return DownloadedFile{}, err
		}
		return DownloadedFile{}, fmt.Errorf("规则下载请求失败")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return DownloadedFile{}, fmt.Errorf("规则下载响应状态异常: %d", response.StatusCode)
	}
	if response.ContentLength > d.maxBytes {
		return DownloadedFile{}, ErrResponseTooLarge
	}
	if err := os.MkdirAll(filepath.Dir(resource.LocalPath), 0o700); err != nil {
		return DownloadedFile{}, err
	}
	file, err := os.CreateTemp(filepath.Dir(resource.LocalPath), "."+filepath.Base(resource.LocalPath)+".download-")
	if err != nil {
		return DownloadedFile{}, err
	}
	path := file.Name()
	keep := false
	defer func() {
		if !keep {
			_ = file.Close()
			_ = os.Remove(path)
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		return DownloadedFile{}, err
	}
	limited := io.LimitReader(response.Body, d.maxBytes+1)
	payload, err := io.ReadAll(limited)
	if err != nil {
		return DownloadedFile{}, err
	}
	if int64(len(payload)) > d.maxBytes {
		return DownloadedFile{}, ErrResponseTooLarge
	}
	if _, err := file.Write(payload); err != nil {
		return DownloadedFile{}, err
	}
	if err := file.Sync(); err != nil {
		return DownloadedFile{}, err
	}
	if err := file.Close(); err != nil {
		return DownloadedFile{}, err
	}
	digest := sha256.Sum256(payload)
	keep = true
	return DownloadedFile{Path: path, Digest: fmt.Sprintf("%x", digest), Payload: payload}, nil
}
