package updater

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

const cachedReleaseJSON = `{"tag_name":"v1.12.3","assets":[{"name":"sing-box-1.12.3-linux-amd64.tar.gz","browser_download_url":"https://github.com/SagerNet/sing-box/releases/download/v1.12.3/sing-box-1.12.3-linux-amd64.tar.gz","digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}`

func TestLatestStableCachesVerifiedMetadataForFiveMinutes(t *testing.T) {
	now := time.Unix(1893456000, 0)
	calls := 0
	transport := roundTripperFunc(func(*http.Request) (*http.Response, error) {
		calls++
		payload := cachedReleaseJSON
		if calls > 1 {
			payload = strings.ReplaceAll(payload, "1.12.3", "1.12.4")
		}
		return response(http.StatusOK, payload), nil
	})
	client := NewReleaseClient(ReleaseClientPolicy{Transport: transport, Arch: "amd64", Clock: func() time.Time { return now }})
	first, err := client.LatestStable(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(5*time.Minute - time.Nanosecond)
	cached, err := client.LatestStable(context.Background())
	if err != nil || cached != first || cached.ArchiveSHA256 == "" || calls != 1 {
		t.Fatalf("cached=%+v calls=%d err=%v", cached, calls, err)
	}
	now = now.Add(time.Nanosecond)
	latest, err := client.LatestStable(context.Background())
	if err != nil || latest.Version != "1.12.4" || calls != 2 {
		t.Fatalf("latest=%+v calls=%d err=%v", latest, calls, err)
	}
}

func TestLatestStableDoesNotFallBackToExpiredMetadataOrCacheFailures(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		body   string
	}{
		{name: "rate limit", status: http.StatusTooManyRequests, body: `{}`},
		{name: "server error", status: http.StatusBadGateway, body: `{}`},
		{name: "invalid metadata", status: http.StatusOK, body: `{"tag_name":"v1.12.3","assets":[]}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			now := time.Unix(1893456000, 0)
			calls := 0
			client := NewReleaseClient(ReleaseClientPolicy{
				Arch: "amd64", Clock: func() time.Time { return now },
				Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
					calls++
					if calls == 2 {
						return response(test.status, test.body), nil
					}
					return response(http.StatusOK, cachedReleaseJSON), nil
				}),
			})
			if _, err := client.LatestStable(context.Background()); err != nil {
				t.Fatal(err)
			}
			now = now.Add(5 * time.Minute)
			if release, err := client.LatestStable(context.Background()); err == nil || release != (Release{}) {
				t.Fatalf("expired metadata returned: release=%+v err=%v", release, err)
			}
			if release, err := client.LatestStable(context.Background()); err != nil || release.Version != "1.12.3" || calls != 3 {
				t.Fatalf("retry: release=%+v calls=%d err=%v", release, calls, err)
			}
		})
	}
}

func TestLatestStableSharesConcurrentSuccessAndFailure(t *testing.T) {
	for _, test := range []struct {
		name           string
		status         int
		transportError error
	}{
		{name: "success", status: http.StatusOK},
		{name: "rate limit", status: http.StatusTooManyRequests},
		{name: "transport timeout", transportError: context.DeadlineExceeded},
	} {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var calls atomic.Int32
				respond := make(chan struct{})
				client := NewReleaseClient(ReleaseClientPolicy{
					Arch: "amd64",
					Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
						calls.Add(1)
						<-respond
						if test.transportError != nil {
							return nil, test.transportError
						}
						return response(test.status, cachedReleaseJSON), nil
					}),
				})
				results := make(chan error, 10)
				for range 10 {
					go func() {
						_, err := client.LatestStable(context.Background())
						results <- err
					}()
				}
				synctest.Wait()
				if got := calls.Load(); got != 1 {
					t.Fatalf("concurrent upstream requests=%d", got)
				}
				close(respond)
				for range 10 {
					err := <-results
					var limited *ReleaseRateLimitError
					if test.transportError != nil {
						if !errors.Is(err, test.transportError) {
							t.Fatalf("transport err=%v", err)
						}
					} else if (test.status == http.StatusOK && err != nil) || (test.status == http.StatusTooManyRequests && !errors.As(err, &limited)) {
						t.Fatalf("status=%d err=%v", test.status, err)
					}
				}
				if got := calls.Load(); got != 1 {
					t.Fatalf("upstream requests after completion=%d", got)
				}
			})
		})
	}
}

func TestLatestStableWaiterCanCancelWithoutCancellingSharedRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		respond := make(chan struct{})
		client := NewReleaseClient(ReleaseClientPolicy{
			Arch: "amd64",
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				<-respond
				return response(http.StatusOK, cachedReleaseJSON), nil
			}),
		})
		first := make(chan error, 1)
		go func() { _, err := client.LatestStable(context.Background()); first <- err }()
		synctest.Wait()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		waiter := make(chan error, 1)
		go func() { _, err := client.LatestStable(ctx); waiter <- err }()
		synctest.Wait()
		cancel()
		if err := <-waiter; !errors.Is(err, context.Canceled) {
			t.Fatalf("waiter err=%v", err)
		}
		close(respond)
		if err := <-first; err != nil {
			t.Fatal(err)
		}
	})
}

func TestLatestStableRetriesSharedRequestWhenFirstCallerCancels(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var calls atomic.Int32
		client := NewReleaseClient(ReleaseClientPolicy{
			Arch: "amd64",
			Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
				if calls.Add(1) == 1 {
					<-request.Context().Done()
					return nil, request.Context().Err()
				}
				return response(http.StatusOK, cachedReleaseJSON), nil
			}),
		})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		first := make(chan error, 1)
		go func() { _, err := client.LatestStable(ctx); first <- err }()
		synctest.Wait()
		waiter := make(chan error, 1)
		go func() { _, err := client.LatestStable(context.Background()); waiter <- err }()
		synctest.Wait()
		cancel()
		if err := <-first; !errors.Is(err, context.Canceled) {
			t.Fatalf("first caller err=%v", err)
		}
		if err := <-waiter; err != nil {
			t.Fatalf("waiter err=%v", err)
		}
		if got := calls.Load(); got != 2 {
			t.Fatalf("upstream requests=%d", got)
		}
	})
}
