package updater

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"singbox-web/internal/paths"
)

func TestOfficialURLRequiresHTTPSAllowedHost(t *testing.T) {
	for _, raw := range []string{"http://github.com/a", "https://example.com/a", "https://user@github.com/a"} {
		if err := validateOfficialURLString(raw); !errors.Is(err, ErrUntrustedURL) {
			t.Fatalf("url=%s err=%v", raw, err)
		}
	}
}

func TestFetcherRejectsRedirectToUntrustedHost(t *testing.T) {
	client := newHTTPClient(roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{"https://example.com/evil"}}, Body: http.NoBody}, nil
	}), 0)
	request, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://github.com/a", nil)
	_, err := client.Do(request)
	if !errors.Is(err, ErrUntrustedURL) {
		t.Fatalf("err=%v", err)
	}
}

func TestReadLimitedRejectsOversize(t *testing.T) {
	if _, err := readLimited(&repeatReader{left: 10}, 4); !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("err=%v", err)
	}
}

func TestPrepareRejectsChecksumMismatchWithoutExecutableResidue(t *testing.T) {
	root := t.TempDir()
	layout, err := paths.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	archive := preparedArchive(t, "sing-box-1.12.3-linux-amd64/sing-box")
	transport := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/checksums":
			return response(http.StatusOK, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  sing-box-1.12.3-linux-amd64.tar.gz\n"), nil
		case "/archive":
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(archive))}, nil
		default:
			return response(http.StatusNotFound, ""), nil
		}
	})
	release := Release{Version: "1.12.3", Arch: "amd64", Asset: Asset{Name: "sing-box-1.12.3-linux-amd64.tar.gz", URL: "https://github.com/archive"}, Checksums: Asset{Name: "sing-box-1.12.3-checksums.txt", URL: "https://github.com/checksums"}}
	if _, cleanup, err := NewFetcher(layout, FetcherPolicyForTest(transport)).Prepare(context.Background(), release); !errors.Is(err, ErrChecksumMismatch) || cleanup != nil {
		t.Fatalf("prepared cleanupPresent=%t err=%v", cleanup != nil, err)
	}
	entries, err := os.ReadDir(layout.BinDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary assets remained: %v", entries)
	}
}

func TestPrepareVerifiesAndExtractsBinary(t *testing.T) {
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	archive := preparedArchive(t, "sing-box-1.12.3-linux-amd64/sing-box")
	digest := sha256.Sum256(archive)
	transport := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/checksums" {
			return response(http.StatusOK, string(fmt.Sprintf("%x  sing-box-1.12.3-linux-amd64.tar.gz\n", digest))), nil
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(archive))}, nil
	})
	release := Release{Version: "1.12.3", Arch: "amd64", Asset: Asset{Name: "sing-box-1.12.3-linux-amd64.tar.gz", URL: "https://github.com/archive"}, Checksums: Asset{Name: "sing-box-1.12.3-checksums.txt", URL: "https://github.com/checksums"}}
	prepared, cleanup, err := NewFetcher(layout, FetcherPolicyForTest(transport)).Prepare(context.Background(), release)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Path == "" || prepared.ArchiveDigest != fmt.Sprintf("%x", digest) {
		t.Fatalf("prepared=%+v", prepared)
	}
	cleanup()
	if _, err := os.Stat(filepath.Dir(prepared.Path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cleanup err=%v", err)
	}
}

func preparedArchive(t *testing.T, program string) []byte {
	t.Helper()
	var payload bytes.Buffer
	gzipWriter := gzip.NewWriter(&payload)
	writer := tar.NewWriter(gzipWriter)
	if err := writer.WriteHeader(&tar.Header{Name: filepath.ToSlash(filepath.Dir(program)) + "/", Typeflag: tar.TypeDir, Mode: 0o755}); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteHeader(&tar.Header{Name: program, Typeflag: tar.TypeReg, Mode: 0o755, Size: int64(len("core"))}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("core")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return payload.Bytes()
}

type repeatReader struct{ left int }

func (r *repeatReader) Read(target []byte) (int, error) {
	if r.left == 0 {
		return 0, errors.New("done")
	}
	count := len(target)
	if count > r.left {
		count = r.left
	}
	for index := 0; index < count; index++ {
		target[index] = 'x'
	}
	r.left -= count
	return count, nil
}
