package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	fixtureCatalogURL     = "https://github.com/GUI-for-Cores/Ruleset-Hub/releases/download/latest/sing-full.json"
	fixtureVersionURL     = "https://github.com/GUI-for-Cores/Ruleset-Hub/releases/download/latest/sing-full-version.txt"
	fixtureGeoSiteBaseURL = "https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/geo/geosite/"
	fixtureGeoIPBaseURL   = "https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/geo/geoip/"
)

type fixtureCatalogItem struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Count int    `json:"count"`
}

type fixtureCatalog struct {
	GeoSite string               `json:"geosite"`
	GeoIP   string               `json:"geoip"`
	List    []fixtureCatalogItem `json:"list"`
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

// rulesetFixtureTransport keeps browser tests deterministic and offline while
// exercising the same catalog checksum, preview, import and download paths as
// production. The item totals mirror the upstream Ruleset-Hub directory.
func rulesetFixtureTransport() http.RoundTripper {
	catalogPayload, err := json.Marshal(fixtureRulesetCatalog())
	if err != nil {
		panic(err)
	}
	digest := sha256.Sum256(catalogPayload)
	versionPayload := []byte(fmt.Sprintf("SHA2-256= %x\n", digest))
	return roundTripFunc(func(request *http.Request) (*http.Response, error) {
		endpoint := request.URL.String()
		switch endpoint {
		case fixtureCatalogURL:
			return fixtureHTTPResponse(request, http.StatusOK, "application/json", catalogPayload), nil
		case fixtureVersionURL:
			return fixtureHTTPResponse(request, http.StatusOK, "text/plain", versionPayload), nil
		}
		if strings.HasPrefix(endpoint, fixtureGeoSiteBaseURL) || strings.HasPrefix(endpoint, fixtureGeoIPBaseURL) {
			if strings.HasSuffix(endpoint, ".json") {
				payload := []byte(`{"version":3,"rules":[{"domain_suffix":["example.com","example.org"]}]}`)
				if strings.HasPrefix(endpoint, fixtureGeoIPBaseURL) {
					payload = []byte(`{"version":3,"rules":[{"ip_cidr":["192.0.2.0/24","2001:db8::/32"]}]}`)
				}
				return fixtureHTTPResponse(request, http.StatusOK, "application/json", payload), nil
			}
			if strings.HasSuffix(endpoint, ".srs") {
				return fixtureHTTPResponse(request, http.StatusOK, "application/octet-stream", []byte("e2e-binary-rule-set")), nil
			}
		}
		return fixtureHTTPResponse(request, http.StatusNotFound, "text/plain", []byte("not found")), nil
	})
}

func fixtureRulesetCatalog() fixtureCatalog {
	geosite := []fixtureCatalogItem{
		{Name: "cn", Type: "geosite", Count: 111868},
		{Name: "private", Type: "geosite", Count: 131},
		{Name: "category-ads-all", Type: "geosite", Count: 905},
		{Name: "geolocation-!cn", Type: "geosite", Count: 27001},
		{Name: "google", Type: "geosite", Count: 1070},
		{Name: "github", Type: "geosite", Count: 63},
	}
	for index := len(geosite); index < 1879; index++ {
		geosite = append(geosite, fixtureCatalogItem{Name: fmt.Sprintf("site-%04d", index), Type: "geosite", Count: index + 1})
	}
	geoip := []fixtureCatalogItem{
		{Name: "cn", Type: "geoip", Count: 5817},
		{Name: "private", Type: "geoip", Count: 18},
		{Name: "cloudflare", Type: "geoip", Count: 711},
	}
	for index := len(geoip); index < 260; index++ {
		geoip = append(geoip, fixtureCatalogItem{Name: fmt.Sprintf("ip-%04d", index), Type: "geoip", Count: index + 1})
	}
	return fixtureCatalog{GeoSite: fixtureGeoSiteBaseURL, GeoIP: fixtureGeoIPBaseURL, List: append(geosite, geoip...)}
}

func fixtureHTTPResponse(request *http.Request, status int, contentType string, payload []byte) *http.Response {
	return &http.Response{
		StatusCode:    status,
		Status:        fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:        http.Header{"Content-Type": []string{contentType}},
		Body:          io.NopCloser(bytes.NewReader(payload)),
		ContentLength: int64(len(payload)),
		Request:       request,
	}
}
