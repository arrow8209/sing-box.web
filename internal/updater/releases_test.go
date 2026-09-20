package updater

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestLatestStableSelectsCurrentLinuxAssetAndChecksums(t *testing.T) {
	releaseJSON := `{"tag_name":"v1.12.3","draft":false,"prerelease":false,"assets":[{"name":"sing-box-1.12.3-linux-amd64.tar.gz","browser_download_url":"https://github.com/SagerNet/sing-box/releases/download/v1.12.3/sing-box-1.12.3-linux-amd64.tar.gz","size":14567890},{"name":"sing-box-1.12.3-checksums.txt","browser_download_url":"https://github.com/SagerNet/sing-box/releases/download/v1.12.3/sing-box-1.12.3-checksums.txt"}]}`
	release, err := parseRelease([]byte(releaseJSON), "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if release.Version != "1.12.3" || release.Asset.Name != "sing-box-1.12.3-linux-amd64.tar.gz" || release.Asset.Size != 14567890 {
		t.Fatalf("release=%+v", release)
	}
}

func TestLatestStableSelectsGitHubAssetDigestWhenChecksumFileIsAbsent(t *testing.T) {
	releaseJSON := `{"tag_name":"v1.13.19","draft":false,"prerelease":false,"assets":[{"name":"sing-box-1.13.19-linux-amd64.tar.gz","browser_download_url":"https://github.com/SagerNet/sing-box/releases/download/v1.13.19/sing-box-1.13.19-linux-amd64.tar.gz","digest":"sha256:ef88a9e577d474210867bd708933d042e9b70106529df2656182c9db90106aa1"}]}`
	release, err := parseRelease([]byte(releaseJSON), "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if release.ArchiveSHA256 != "ef88a9e577d474210867bd708933d042e9b70106529df2656182c9db90106aa1" || release.Checksums.URL != "" {
		t.Fatalf("release=%+v", release)
	}
}

func TestParseReleaseRejectsMissingOrMalformedSHA256(t *testing.T) {
	for _, digest := range []string{"", "sha256:wrong"} {
		releaseJSON := `{"tag_name":"v1.13.19","draft":false,"prerelease":false,"assets":[{"name":"sing-box-1.13.19-linux-amd64.tar.gz","browser_download_url":"https://github.com/SagerNet/sing-box/releases/download/v1.13.19/sing-box-1.13.19-linux-amd64.tar.gz","digest":"` + digest + `"}]}`
		if _, err := parseRelease([]byte(releaseJSON), "amd64"); err == nil {
			t.Fatalf("accepted digest %q", digest)
		}
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

func TestLatestStableDistinguishesRateLimitsFromOtherFailures(t *testing.T) {
	for _, test := range []struct {
		name       string
		status     int
		remaining  string
		reset      string
		retryAfter string
		limited    bool
		retryAt    time.Time
	}{
		{name: "exhausted quota", status: http.StatusForbidden, remaining: "0", reset: "1893456000", limited: true, retryAt: time.Unix(1893456000, 0)},
		{name: "malformed reset", status: http.StatusForbidden, remaining: "0", reset: "invalid", limited: true},
		{name: "secondary limit", status: http.StatusForbidden, retryAfter: "60", limited: true},
		{name: "too many requests", status: http.StatusTooManyRequests, limited: true},
		{name: "other forbidden", status: http.StatusForbidden},
		{name: "server failure", status: http.StatusBadGateway, remaining: "0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			transport := roundTripperFunc(func(*http.Request) (*http.Response, error) {
				result := response(test.status, `{"message":"private upstream details"}`)
				result.Header.Set("X-RateLimit-Remaining", test.remaining)
				result.Header.Set("X-RateLimit-Reset", test.reset)
				result.Header.Set("Retry-After", test.retryAfter)
				return result, nil
			})
			_, err := NewReleaseClient(ReleaseClientPolicyForTest(transport, "amd64")).LatestStable(context.Background())
			if err == nil {
				t.Fatal("expected release lookup failure")
			}
			var limited *ReleaseRateLimitError
			if errors.As(err, &limited) != test.limited {
				t.Fatalf("err=%v, want rate limit=%t", err, test.limited)
			}
			if limited != nil && !limited.RetryAt.Equal(test.retryAt) {
				t.Fatalf("retryAt=%v, want %v", limited.RetryAt, test.retryAt)
			}
			if strings.Contains(err.Error(), "private upstream details") {
				t.Fatal("upstream response body leaked into error")
			}
		})
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }
func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}
