package updater

import (
	"context"
	"crypto/sha256"
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
	archive, archiveCount := findAsset(document.Assets, archiveName)
	if archiveCount != 1 {
		return Release{}, fmt.Errorf("官方核心版本资产不完整或不唯一")
	}
	if err := validateOfficialURLString(archive.URL); err != nil {
		return Release{}, err
	}
	if digest, ok := parseGitHubAssetDigest(archive.Digest); ok {
		return Release{Version: version, Asset: archive, ArchiveSHA256: digest, Arch: arch}, nil
	}

	// GitHub started publishing the SHA-256 directly in each release asset's
	// metadata. Retain the old signed-release convention for historical
	// versions, but do not accept a release with neither verification source.
	checksumName := fmt.Sprintf("sing-box-%s-checksums.txt", version)
	checksums, checksumCount := findAsset(document.Assets, checksumName)
	if checksumCount != 1 {
		return Release{}, fmt.Errorf("官方核心版本缺少 SHA-256 校验信息")
	}
	if err := validateOfficialURLString(checksums.URL); err != nil {
		return Release{}, err
	}
	return Release{Version: version, Asset: archive, Checksums: checksums, Arch: arch}, nil
}

func parseGitHubAssetDigest(value string) (string, bool) {
	const prefix = "sha256:"
	if !strings.HasPrefix(value, prefix) {
		return "", false
	}
	digest := strings.TrimPrefix(value, prefix)
	if len(digest) != sha256.Size*2 {
		return "", false
	}
	for _, character := range digest {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return "", false
		}
	}
	return digest, true
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
