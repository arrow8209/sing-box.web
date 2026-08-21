package rules

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDownloaderRevalidatesEveryRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "file:///etc/passwd", http.StatusFound)
	}))
	defer server.Close()
	downloader := NewDownloader(DownloadPolicyForTest(server.Client().Transport))
	_, err := downloader.Fetch(context.Background(), Resource{URL: server.URL, Format: FormatBinary})
	if !errors.Is(err, ErrUnsupportedURL) {
		t.Fatalf("err=%v", err)
	}
}

func TestDownloaderRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "67108865")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	_, err := NewDownloader(DownloadPolicyForTest(server.Client().Transport)).Fetch(context.Background(), Resource{URL: server.URL, Format: FormatBinary})
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("err=%v", err)
	}
}
