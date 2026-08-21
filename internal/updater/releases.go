package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
)

type releaseDocument struct {
	TagName    string  `json:"tag_name"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}

// ReleaseClient always requests the official latest-release endpoint.  The
// optional policy exists only to make the fixed endpoint testable in-process.
type ReleaseClient struct {
	client *http.Client
	arch   string
}

type ReleaseClientPolicy struct {
	Transport http.RoundTripper
	Arch      string
}

func NewReleaseClient(policies ...ReleaseClientPolicy) *ReleaseClient {
	policy := ReleaseClientPolicy{}
	if len(policies) > 0 {
		policy = policies[0]
	}
	arch := policy.Arch
	if arch == "" {
		arch = runtime.GOARCH
	}
	return &ReleaseClient{client: newHTTPClient(policy.Transport, totalDownloadTimeout), arch: arch}
}

func ReleaseClientPolicyForTest(transport http.RoundTripper, arch string) ReleaseClientPolicy {
	return ReleaseClientPolicy{Transport: transport, Arch: arch}
}

func (c *ReleaseClient) LatestStable(ctx context.Context) (Release, error) {
	if err := validateOfficialURLString(latestReleaseURL); err != nil {
		return Release{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		return Release{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := c.client.Do(request)
	if err != nil {
		return Release{}, fmt.Errorf("获取官方核心版本失败: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return Release{}, fmt.Errorf("官方核心版本响应状态异常: %d", response.StatusCode)
	}
	payload, err := readLimited(response.Body, maxChecksumBytes)
	if err != nil {
		return Release{}, err
	}
	return parseRelease(payload, c.arch)
}

func parseRelease(payload []byte, goarch string) (Release, error) {
	var document releaseDocument
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	if err := decoder.Decode(&document); err != nil {
		return Release{}, fmt.Errorf("官方核心版本元数据无效: %w", err)
	}
	if document.Draft || document.Prerelease || !strings.HasPrefix(document.TagName, "v") {
		return Release{}, fmt.Errorf("官方最新版本不是稳定版")
	}
	version := strings.TrimPrefix(document.TagName, "v")
	if version == "" || strings.ContainsAny(version, "/\\\x00") {
		return Release{}, fmt.Errorf("官方核心版本号无效")
	}
	arch, ok := linuxArchitecture(goarch)
	if !ok {
		return Release{}, ErrUnsupportedArchitecture
	}
	archiveName := fmt.Sprintf("sing-box-%s-linux-%s.tar.gz", version, arch)
	checksumName := fmt.Sprintf("sing-box-%s-checksums.txt", version)
	archive, archiveCount := findAsset(document.Assets, archiveName)
	checksums, checksumCount := findAsset(document.Assets, checksumName)
	if archiveCount != 1 || checksumCount != 1 {
		return Release{}, fmt.Errorf("官方核心版本资产不完整或不唯一")
	}
	if err := validateOfficialURLString(archive.URL); err != nil {
		return Release{}, err
	}
	if err := validateOfficialURLString(checksums.URL); err != nil {
		return Release{}, err
	}
	return Release{Version: version, Asset: archive, Checksums: checksums, Arch: arch}, nil
}

func linuxArchitecture(goarch string) (string, bool) {
	switch goarch {
	case "amd64", "arm64", "386":
		return goarch, true
	case "arm":
		return "armv7", true
	default:
		return "", false
	}
}

func findAsset(assets []Asset, name string) (Asset, int) {
	var result Asset
	count := 0
	for _, asset := range assets {
		if asset.Name == name {
			result = asset
			count++
		}
	}
	return result, count
}
