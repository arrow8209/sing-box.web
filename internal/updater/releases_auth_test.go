package updater

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestLatestStableSupportsOptionalGitHubToken(t *testing.T) {
	for _, token := range []string{"", "  synthetic-token\n"} {
		transport := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
			want := ""
			if strings.TrimSpace(token) != "" {
				want = "Bearer synthetic-token"
			}
			if request.URL.String() != latestReleaseURL || request.Header.Get("Authorization") != want {
				t.Fatal("unexpected release endpoint or authentication header")
			}
			return response(http.StatusOK, cachedReleaseJSON), nil
		})
		client := NewReleaseClient(ReleaseClientPolicy{Transport: transport, Arch: "amd64", GitHubToken: token})
		if _, err := client.LatestStable(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLatestStableDoesNotSendTokenToRedirectedAssetHosts(t *testing.T) {
	for _, target := range []string{
		"https://api.github.com/repos/SagerNet/sing-box/releases/123",
		"https://github.com/metadata",
		"https://objects.githubusercontent.com/metadata",
		"https://release-assets.githubusercontent.com/metadata",
		"https://api.github.com:8443/metadata",
	} {
		t.Run(target, func(t *testing.T) {
			calls := 0
			transport := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					result := response(http.StatusFound, "")
					result.Header.Set("Location", target)
					return result, nil
				}
				want := ""
				if request.URL.Host == "api.github.com" {
					want = "Bearer synthetic-token"
				}
				if request.Header.Get("Authorization") != want {
					t.Fatal("unexpected authentication header after redirect")
				}
				return response(http.StatusOK, cachedReleaseJSON), nil
			})
			client := NewReleaseClient(ReleaseClientPolicy{Transport: transport, Arch: "amd64", GitHubToken: "synthetic-token"})
			if _, err := client.LatestStable(context.Background()); err != nil || calls != 2 {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
		})
	}
}

func TestLatestStableAuthenticationFailureDoesNotExposeTokenOrRetryAnonymously(t *testing.T) {
	calls := 0
	transport := roundTripperFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return response(http.StatusUnauthorized, `{"message":"synthetic-token"}`), nil
	})
	client := NewReleaseClient(ReleaseClientPolicy{Transport: transport, Arch: "amd64", GitHubToken: "synthetic-token"})
	_, err := client.LatestStable(context.Background())
	if err == nil || strings.Contains(err.Error(), "synthetic-token") || calls != 1 {
		t.Fatalf("calls=%d, expected one sanitized authentication failure", calls)
	}
}
