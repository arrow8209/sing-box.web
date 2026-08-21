package updater

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestLatestStableSelectsCurrentLinuxAssetAndChecksums(t *testing.T) {
	releaseJSON := `{"tag_name":"v1.12.3","draft":false,"prerelease":false,"assets":[{"name":"sing-box-1.12.3-linux-amd64.tar.gz","browser_download_url":"https://github.com/SagerNet/sing-box/releases/download/v1.12.3/sing-box-1.12.3-linux-amd64.tar.gz"},{"name":"sing-box-1.12.3-checksums.txt","browser_download_url":"https://github.com/SagerNet/sing-box/releases/download/v1.12.3/sing-box-1.12.3-checksums.txt"}]}`
	release, err := parseRelease([]byte(releaseJSON), "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if release.Version != "1.12.3" || release.Asset.Name != "sing-box-1.12.3-linux-amd64.tar.gz" {
		t.Fatalf("release=%+v", release)
	}
}

func TestParseReleaseRejectsUnstableAndUnsupportedArch(t *testing.T) {
	base := `{"tag_name":"v1.12.3","draft":false,"prerelease":false,"assets":[]}`
	if _, err := parseRelease([]byte(strings.Replace(base, `"prerelease":false`, `"prerelease":true`, 1)), "amd64"); err == nil {
		t.Fatal("accepted prerelease")
	}
	if _, err := parseRelease([]byte(base), "mips64"); !errors.Is(err, ErrUnsupportedArchitecture) {
		t.Fatalf("err=%v", err)
	}
}

func TestLatestStableUsesFixedOfficialEndpoint(t *testing.T) {
	called := ""
	transport := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		called = request.URL.String()
		return response(http.StatusOK, `{"tag_name":"v1.12.3","assets":[]}`), nil
	})
	_, err := NewReleaseClient(ReleaseClientPolicyForTest(transport, "mips64")).LatestStable(context.Background())
	if !errors.Is(err, ErrUnsupportedArchitecture) || called != latestReleaseURL {
		t.Fatalf("url=%q err=%v", called, err)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }
func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}
