package rules

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fixedCatalogFetcher struct {
	document catalogRemoteDocument
	digest   string
	err      error
}

func (f fixedCatalogFetcher) Fetch(context.Context) (catalogRemoteDocument, string, error) {
	return f.document, f.digest, f.err
}

type catalogRuleDownloader struct{}

func (catalogRuleDownloader) Fetch(_ context.Context, resource Resource) (DownloadedFile, error) {
	payload := []byte("binary")
	if resource.Format == FormatSource {
		payload = []byte(`{"version":3,"rules":[{"domain_suffix":["example.com","example.org"]}]}`)
	}
	path, err := stageRulePayload(resource.LocalPath, payload)
	if err != nil {
		return DownloadedFile{}, err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(payload))
	return DownloadedFile{Path: path, Digest: digest, Payload: payload}, nil
}

func catalogFixture(count int) catalogRemoteDocument {
	items := make([]catalogRemoteItem, 0, count)
	for index := range count {
		itemType := "geosite"
		if index%9 == 0 {
			itemType = "geoip"
		}
		items = append(items, catalogRemoteItem{Name: fmt.Sprintf("item-%04d", index), Type: itemType, Count: index})
	}
	return catalogRemoteDocument{GeoSite: geositeBaseURL, GeoIP: geoipBaseURL, List: items}
}

type catalogRoundTripFunc func(*http.Request) (*http.Response, error)

func (f catalogRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestHTTPCatalogFetcherUsesChecksummedDirectory(t *testing.T) {
	document := catalogRemoteDocument{GeoSite: geositeBaseURL, GeoIP: geoipBaseURL, List: []catalogRemoteItem{{Name: "cn", Type: "geosite", Count: 10}}}
	payload, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(payload))
	transport := catalogRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := payload
		if request.URL.String() == catalogVersionURL {
			body = []byte("SHA2-256= " + digest + "\n")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body)), Request: request}, nil
	})
	fetched, fetchedDigest, err := NewCatalogFetcher(DownloadPolicyForTest(transport)).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if fetchedDigest != digest || len(fetched.List) != 1 || fetched.List[0].Name != "cn" {
		t.Fatalf("catalog=%+v digest=%s", fetched, fetchedDigest)
	}
}

func TestCatalogRefreshCachesFullDirectoryAndPreservesItOnFailure(t *testing.T) {
	resourceService, layout := newRuleService(t, nil)
	digest := fmt.Sprintf("%064x", 1)
	service := NewCatalogService(layout.RuleCatalogFile, layout, resourceService, fixedCatalogFetcher{document: catalogFixture(2139), digest: digest}, fixedClock(time.Unix(10, 0)))
	refreshed, err := service.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Total != 2139 || refreshed.GeoSiteCount != 1901 || refreshed.GeoIPCount != 238 || refreshed.Digest != digest {
		t.Fatalf("catalog=%+v", refreshed)
	}
	service.fetcher = fixedCatalogFetcher{err: errors.New("offline")}
	if _, err := service.Refresh(context.Background()); err == nil {
		t.Fatal("offline refresh succeeded")
	}
	cached, err := service.List(context.Background())
	if err != nil || cached.Total != 2139 || cached.Digest != digest {
		t.Fatalf("cached=%+v err=%v", cached, err)
	}
}

func TestCatalogImportDownloadsRealResourceAndRejectsDuplicateFormat(t *testing.T) {
	resourceService, layout := newRuleService(t, nil)
	resourceService.random = bytes.NewReader(append(bytes.Repeat([]byte{0x11}, 16), bytes.Repeat([]byte{0x22}, 16)...))
	resourceService.SetDownloader(catalogRuleDownloader{})
	document := catalogRemoteDocument{GeoSite: geositeBaseURL, GeoIP: geoipBaseURL, List: []catalogRemoteItem{{Name: "cn", Type: "geosite", Count: 10}}}
	service := NewCatalogService(layout.RuleCatalogFile, layout, resourceService, fixedCatalogFetcher{document: document, digest: fmt.Sprintf("%064x", 2)}, fixedClock(time.Unix(10, 0)))
	if _, err := service.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	result, err := service.Import(context.Background(), CatalogImportInput{Key: "geosite:cn", Format: FormatSource})
	if err != nil {
		t.Fatal(err)
	}
	if result.Warning != "" || result.Update == nil || result.Resource.CatalogKey != "geosite:cn" || result.Resource.RuleCount == nil || *result.Resource.RuleCount != 2 || result.Resource.UpdateInterval != "24h" {
		t.Fatalf("result=%+v", result)
	}
	if _, err := service.Import(context.Background(), CatalogImportInput{Key: "geosite:cn", Format: FormatSource}); err == nil {
		t.Fatal("duplicate import succeeded")
	} else {
		var duplicate *CatalogAlreadyImportedError
		if !errors.As(err, &duplicate) {
			t.Fatalf("err=%v", err)
		}
	}
	if _, err := service.Import(context.Background(), CatalogImportInput{Key: "geosite:cn", Format: FormatBinary}); err != nil {
		t.Fatalf("binary companion import failed: %v", err)
	}
}

func TestCatalogPreviewReturnsBoundedRealSource(t *testing.T) {
	resourceService, layout := newRuleService(t, nil)
	resourceService.SetDownloader(catalogRuleDownloader{})
	document := catalogRemoteDocument{GeoSite: geositeBaseURL, GeoIP: geoipBaseURL, List: []catalogRemoteItem{{Name: "private", Type: "geosite", Count: 2}}}
	service := NewCatalogService(layout.RuleCatalogFile, layout, resourceService, fixedCatalogFetcher{document: document, digest: fmt.Sprintf("%064x", 3)}, fixedClock(time.Unix(10, 0)))
	if _, err := service.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	preview, err := service.Preview(context.Background(), "geosite:private")
	if err != nil {
		t.Fatal(err)
	}
	if preview.RuleCount != 2 || preview.RuleBlocks != 1 || preview.Truncated || len(preview.Content) == 0 {
		t.Fatalf("preview=%+v", preview)
	}
	if matches, _ := filepath.Glob(filepath.Join(layout.RuleSetsDir, "*.catalog-preview*")); len(matches) != 0 {
		t.Fatalf("preview files remain: %v", matches)
	}
}

func TestDecodeCatalogRejectsUntrustedBaseAndDuplicateItems(t *testing.T) {
	for _, body := range []string{
		`{"geosite":"https://example.test/","geoip":"` + geoipBaseURL + `","list":[{"name":"cn","type":"geosite","count":1}]}`,
		`{"geosite":"` + geositeBaseURL + `","geoip":"` + geoipBaseURL + `","list":[{"name":"cn","type":"geosite","count":1},{"name":"cn","type":"geosite","count":1}]}`,
	} {
		if _, err := decodeCatalog([]byte(body)); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
}

func TestCatalogListIsUnavailableBeforeFirstRefresh(t *testing.T) {
	resourceService, layout := newRuleService(t, nil)
	service := NewCatalogService(layout.RuleCatalogFile, layout, resourceService, fixedCatalogFetcher{}, time.Now)
	result, err := service.List(context.Background())
	if err != nil || result.Available || len(result.Items) != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if _, err := os.Stat(layout.RuleCatalogFile); !os.IsNotExist(err) {
		t.Fatalf("catalog cache unexpectedly created: %v", err)
	}
}
