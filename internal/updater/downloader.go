package updater

import (
	"bytes"
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
	"strings"
	"time"

	"singbox-web/internal/paths"
)

const (
	maxRedirects          = 5
	connectTimeout        = 10 * time.Second
	responseHeaderTimeout = 15 * time.Second
	totalDownloadTimeout  = 5 * time.Minute
	progressReportPeriod  = 250 * time.Millisecond
)

var officialHosts = map[string]struct{}{
	"api.github.com": {}, "github.com": {}, "objects.githubusercontent.com": {}, "release-assets.githubusercontent.com": {},
}

type Fetcher struct {
	layout paths.Layout
	client *http.Client
}

type FetcherPolicy struct{ Transport http.RoundTripper }

func NewFetcher(layout paths.Layout, policies ...FetcherPolicy) *Fetcher {
	var transport http.RoundTripper
	if len(policies) > 0 {
		transport = policies[0].Transport
	}
	return &Fetcher{layout: layout, client: newHTTPClient(transport, totalDownloadTimeout)}
}

func FetcherPolicyForTest(transport http.RoundTripper) FetcherPolicy {
	return FetcherPolicy{Transport: transport}
}

func newHTTPClient(transport http.RoundTripper, timeout time.Duration) *http.Client {
	if transport == nil {
		transport = &http.Transport{
			Proxy:               nil,
			DialContext:         (&net.Dialer{Timeout: connectTimeout, KeepAlive: 30 * time.Second}).DialContext,
			TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: responseHeaderTimeout,
		}
	}
	return &http.Client{Transport: transport, Timeout: timeout, CheckRedirect: func(request *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return ErrTooManyRedirects
		}
		return validateOfficialURL(request.URL)
	}}
}

func validateOfficialURLString(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ErrUntrustedURL
	}
	return validateOfficialURL(parsed)
}

func validateOfficialURL(parsed *url.URL) error {
	if parsed == nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return ErrUntrustedURL
	}
	if _, ok := officialHosts[strings.ToLower(parsed.Hostname())]; !ok {
		return ErrUntrustedURL
	}
	return nil
}

func (f *Fetcher) Prepare(ctx context.Context, release Release) (prepared PreparedBinary, cleanup func(), err error) {
	return f.prepare(ctx, release, nil)
}

// PrepareWithProgress reports only the core archive transfer. The small
// checksum document, when needed for older releases, is intentionally omitted
// so progress never jumps backwards between two different response bodies.
func (f *Fetcher) PrepareWithProgress(ctx context.Context, release Release, report func(DownloadProgress)) (prepared PreparedBinary, cleanup func(), err error) {
	return f.prepare(ctx, release, report)
}

func (f *Fetcher) prepare(ctx context.Context, release Release, report func(DownloadProgress)) (prepared PreparedBinary, cleanup func(), err error) {
	if err := validateRelease(release); err != nil {
		return PreparedBinary{}, nil, err
	}
	if err := os.MkdirAll(f.layout.BinDir, 0o700); err != nil {
		return PreparedBinary{}, nil, err
	}
	directory, err := os.MkdirTemp(f.layout.BinDir, ".prepare-")
	if err != nil {
		return PreparedBinary{}, nil, err
	}
	keep := false
	cleanupFn := func() { _ = os.RemoveAll(directory) }
	defer func() {
		if !keep {
			cleanupFn()
		}
	}()
	expected := release.ArchiveSHA256
	if expected == "" {
		checksums, err := f.download(ctx, release.Checksums.URL, maxChecksumBytes, filepath.Join(directory, "checksums.txt"))
		if err != nil {
			return PreparedBinary{}, nil, err
		}
		expected, err = FindChecksum(checksums, release.Asset.Name)
		if err != nil {
			return PreparedBinary{}, nil, err
		}
	}
	archivePath := filepath.Join(directory, release.Asset.Name)
	archive, err := f.downloadWithProgress(ctx, release.Asset.URL, maxArchiveBytes, archivePath, release.Asset.Size, report)
	if err != nil {
		return PreparedBinary{}, nil, err
	}
	archiveDigest := fmt.Sprintf("%x", sha256.Sum256(archive))
	if archiveDigest != expected {
		return PreparedBinary{}, nil, ErrChecksumMismatch
	}
	expectedBinary := fmt.Sprintf("sing-box-%s-linux-%s/sing-box", release.Version, release.Arch)
	binaryPath, err := ExtractBinary(archivePath, directory, expectedBinary)
	if err != nil {
		return PreparedBinary{}, nil, err
	}
	binary, err := os.ReadFile(binaryPath)
	if err != nil {
		return PreparedBinary{}, nil, err
	}
	keep = true
	return PreparedBinary{Path: binaryPath, Version: release.Version, ArchiveDigest: archiveDigest, BinaryDigest: fmt.Sprintf("%x", sha256.Sum256(binary))}, cleanupFn, nil
}

func validateRelease(release Release) error {
	if release.Version == "" || release.Arch == "" || release.Asset.Name == "" {
		return fmt.Errorf("核心版本信息无效")
	}
	if err := validateOfficialURLString(release.Asset.URL); err != nil {
		return err
	}
	if release.Asset.Size < 0 {
		return fmt.Errorf("核心压缩包大小无效")
	}
	if release.Asset.Size > maxArchiveBytes {
		return ErrResponseTooLarge
	}
	if release.ArchiveSHA256 != "" {
		if _, ok := parseGitHubAssetDigest("sha256:" + release.ArchiveSHA256); !ok {
			return fmt.Errorf("核心压缩包 SHA-256 无效")
		}
		return nil
	}
	if release.Checksums.Name == "" {
		return fmt.Errorf("核心版本缺少 SHA-256 校验信息")
	}
	return validateOfficialURLString(release.Checksums.URL)
}

func (f *Fetcher) download(ctx context.Context, rawURL string, maximum int64, destination string) ([]byte, error) {
	return f.downloadWithProgress(ctx, rawURL, maximum, destination, 0, nil)
}

func (f *Fetcher) downloadWithProgress(ctx context.Context, rawURL string, maximum int64, destination string, expectedSize int64, report func(DownloadProgress)) ([]byte, error) {
	if err := validateOfficialURLString(rawURL); err != nil {
		return nil, err
	}
	if expectedSize > maximum {
		return nil, ErrResponseTooLarge
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := f.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("下载官方核心资产失败: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("官方下载响应状态异常: %d", response.StatusCode)
	}
	if response.ContentLength > maximum {
		return nil, ErrResponseTooLarge
	}
	total := response.ContentLength
	if total < 0 {
		total = 0
	}
	if total == 0 && expectedSize > 0 {
		total = expectedSize
	}
	var payload []byte
	if report == nil {
		payload, err = readLimited(response.Body, maximum)
	} else {
		payload, err = readLimitedWithProgress(response.Body, maximum, total, report)
	}
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	if _, err := file.Write(payload); err != nil {
		_ = file.Close()
		_ = os.Remove(destination)
		return nil, err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(destination)
		return nil, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(destination)
		return nil, err
	}
	return payload, nil
}

func readLimitedWithProgress(reader io.Reader, maximum, total int64, report func(DownloadProgress)) ([]byte, error) {
	if report == nil {
		return readLimited(reader, maximum)
	}
	started := time.Now()
	lastReport := started
	downloaded := int64(0)
	report(DownloadProgress{TotalBytes: total})
	var payload bytes.Buffer
	buffer := make([]byte, 32<<10)
	limited := io.LimitReader(reader, maximum+1)
	for {
		count, readErr := limited.Read(buffer)
		if count > 0 {
			downloaded += int64(count)
			if downloaded > maximum {
				reportProgress(report, downloaded, total, started)
				return nil, ErrResponseTooLarge
			}
			_, _ = payload.Write(buffer[:count])
			now := time.Now()
			if now.Sub(lastReport) >= progressReportPeriod {
				reportProgressAt(report, downloaded, total, started, now)
				lastReport = now
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				if total == 0 || downloaded > total {
					total = downloaded
				}
				reportProgress(report, downloaded, total, started)
				return payload.Bytes(), nil
			}
			reportProgress(report, downloaded, total, started)
			return nil, readErr
		}
	}
}

func reportProgress(report func(DownloadProgress), downloaded, total int64, started time.Time) {
	reportProgressAt(report, downloaded, total, started, time.Now())
}

func reportProgressAt(report func(DownloadProgress), downloaded, total int64, started, now time.Time) {
	speed := int64(0)
	if elapsed := now.Sub(started); downloaded > 0 && elapsed > 0 {
		speed = int64(float64(downloaded) / elapsed.Seconds())
	}
	report(DownloadProgress{DownloadedBytes: downloaded, TotalBytes: total, BytesPerSecond: speed})
}

func readLimited(reader io.Reader, maximum int64) ([]byte, error) {
	payload, err := io.ReadAll(io.LimitReader(reader, maximum+1))
	if err != nil {
		return nil, err
	}
	if int64(len(payload)) > maximum {
		return nil, ErrResponseTooLarge
	}
	return payload, nil
}
