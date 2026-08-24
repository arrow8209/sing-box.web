package rules

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"singbox-web/internal/paths"
	"singbox-web/internal/storage"
)

const (
	catalogSourceURL  = "https://github.com/GUI-for-Cores/Ruleset-Hub/releases/download/latest/sing-full.json"
	catalogVersionURL = "https://github.com/GUI-for-Cores/Ruleset-Hub/releases/download/latest/sing-full-version.txt"
	geositeBaseURL    = "https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/geo/geosite/"
	geoipBaseURL      = "https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/geo/geoip/"
	maxCatalogBytes   = int64(2 << 20)
)

var (
	ErrCatalogUnavailable = errors.New("规则集目录尚未下载")
	catalogNamePattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9!@._+:-]{0,79}$`)
	catalogDigestPattern  = regexp.MustCompile(`(?i)SHA2-256=\s*([a-f0-9]{64})`)
)

type catalogRemoteItem struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Count int    `json:"count"`
}

type catalogRemoteDocument struct {
	GeoSite string              `json:"geosite"`
	GeoIP   string              `json:"geoip"`
	List    []catalogRemoteItem `json:"list"`
}

type catalogCache struct {
	Version   int                   `json:"version"`
	Digest    string                `json:"digest"`
	FetchedAt time.Time             `json:"fetchedAt"`
	Catalog   catalogRemoteDocument `json:"catalog"`
}

type CatalogItem struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Count int    `json:"count"`
}

type CatalogResponse struct {
	Available    bool          `json:"available"`
	Source       string        `json:"source"`
	GeoSite      string        `json:"geosite"`
	GeoIP        string        `json:"geoip"`
	Digest       string        `json:"digest,omitempty"`
	FetchedAt    *time.Time    `json:"fetchedAt,omitempty"`
	Total        int           `json:"total"`
	GeoSiteCount int           `json:"geositeCount"`
	GeoIPCount   int           `json:"geoipCount"`
	Items        []CatalogItem `json:"items"`
}

type CatalogPreview struct {
	Key        string          `json:"key"`
	RuleBlocks int             `json:"ruleBlocks"`
	RuleCount  int             `json:"ruleCount"`
	Truncated  bool            `json:"truncated"`
	Content    json.RawMessage `json:"content"`
}

type CatalogImportInput struct {
	Key            string `json:"key"`
	Format         Format `json:"format"`
	UpdateInterval string `json:"updateInterval"`
}

type CatalogImportResult struct {
	Resource ResourceResponse `json:"resource"`
	Update   *UpdateResult    `json:"update,omitempty"`
	Warning  string           `json:"warning,omitempty"`
}

type CatalogAlreadyImportedError struct {
	Key    string
	Format Format
}

func (e *CatalogAlreadyImportedError) Error() string {
	return fmt.Sprintf("规则集 %s 的 %s 格式已经导入", e.Key, e.Format)
}

type CatalogFetcher interface {
	Fetch(context.Context) (catalogRemoteDocument, string, error)
}

type httpCatalogFetcher struct{ client *http.Client }

func NewCatalogFetcher(policies ...DownloadPolicy) CatalogFetcher {
	var transport http.RoundTripper
	if len(policies) != 0 && policies[0].Transport != nil {
		transport = policies[0].Transport
	} else {
		transport = &http.Transport{
			Proxy:                 nil,
			DialContext:           (&net.Dialer{Timeout: connectTimeout, KeepAlive: 30 * time.Second}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: responseHeaderTimeout,
		}
	}
	client := &http.Client{Transport: transport, Timeout: totalDownloadTimeout}
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return ErrTooManyRedirects
		}
		return validateCatalogDownloadURL(request.URL)
	}
	return &httpCatalogFetcher{client: client}
}

func validateCatalogDownloadURL(value *url.URL) error {
	if value == nil || value.Scheme != "https" || value.User != nil {
		return ErrUnsupportedURL
	}
	switch strings.ToLower(value.Hostname()) {
	case "github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com":
		return nil
	default:
		return ErrUnsupportedURL
	}
}

func (f *httpCatalogFetcher) Fetch(ctx context.Context) (catalogRemoteDocument, string, error) {
	version, err := f.fetchBytes(ctx, catalogVersionURL, 1024)
	if err != nil {
		return catalogRemoteDocument{}, "", err
	}
	digestMatch := catalogDigestPattern.FindSubmatch(version)
	if len(digestMatch) != 2 {
		return catalogRemoteDocument{}, "", errors.New("规则集目录校验文件无效")
	}
	payload, err := f.fetchBytes(ctx, catalogSourceURL, maxCatalogBytes)
	if err != nil {
		return catalogRemoteDocument{}, "", err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(payload))
	if !strings.EqualFold(digest, string(digestMatch[1])) {
		return catalogRemoteDocument{}, "", errors.New("规则集目录摘要不匹配")
	}
	document, err := decodeCatalog(payload)
	if err != nil {
		return catalogRemoteDocument{}, "", err
	}
	return document, digest, nil
}

func (f *httpCatalogFetcher) fetchBytes(ctx context.Context, endpoint string, maximum int64) ([]byte, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || validateCatalogDownloadURL(parsed) != nil {
		return nil, ErrUnsupportedURL
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	response, err := f.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("下载规则集目录失败: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("规则集目录响应状态异常: %d", response.StatusCode)
	}
	if response.ContentLength > maximum {
		return nil, ErrResponseTooLarge
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, maximum+1))
	if err != nil {
		return nil, err
	}
	if int64(len(payload)) > maximum {
		return nil, ErrResponseTooLarge
	}
	return payload, nil
}

func decodeCatalog(payload []byte) (catalogRemoteDocument, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var document catalogRemoteDocument
	if err := decoder.Decode(&document); err != nil {
		return catalogRemoteDocument{}, fmt.Errorf("规则集目录 JSON 无效: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return catalogRemoteDocument{}, errors.New("规则集目录只能包含一个 JSON 值")
	}
	if err := validateCatalog(document); err != nil {
		return catalogRemoteDocument{}, err
	}
	return document, nil
}

func validateCatalog(document catalogRemoteDocument) error {
	if document.GeoSite != geositeBaseURL || document.GeoIP != geoipBaseURL {
		return errors.New("规则集目录包含不受信任的下载基址")
	}
	if len(document.List) == 0 || len(document.List) > 10000 {
		return errors.New("规则集目录条目数量无效")
	}
	seen := make(map[string]struct{}, len(document.List))
	for index, item := range document.List {
		if (item.Type != "geosite" && item.Type != "geoip") || !catalogNamePattern.MatchString(item.Name) || item.Count < 0 {
			return fmt.Errorf("规则集目录第 %d 项无效", index+1)
		}
		key := catalogKey(item.Type, item.Name)
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("规则集目录包含重复项: %s", key)
		}
		seen[key] = struct{}{}
	}
	return nil
}

type CatalogService struct {
	file      string
	layout    paths.Layout
	resources *Service
	fetcher   CatalogFetcher
	clock     func() time.Time
	mu        sync.Mutex
}

func NewCatalogService(file string, layout paths.Layout, resources *Service, fetcher CatalogFetcher, clock func() time.Time) *CatalogService {
	if fetcher == nil {
		fetcher = NewCatalogFetcher()
	}
	if clock == nil {
		clock = time.Now
	}
	return &CatalogService{file: file, layout: layout, resources: resources, fetcher: fetcher, clock: clock}
}

func (s *CatalogService) List(ctx context.Context) (CatalogResponse, error) {
	if err := ctx.Err(); err != nil {
		return CatalogResponse{}, err
	}
	cache, err := s.readCache()
	if os.IsNotExist(err) {
		return CatalogResponse{Available: false, Source: catalogSourceURL, GeoSite: geositeBaseURL, GeoIP: geoipBaseURL, Items: []CatalogItem{}}, nil
	}
	if err != nil {
		return CatalogResponse{}, err
	}
	return catalogResponse(cache), nil
}

func (s *CatalogService) Refresh(ctx context.Context) (CatalogResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	document, digest, err := s.fetcher.Fetch(ctx)
	if err != nil {
		return CatalogResponse{}, err
	}
	cache := catalogCache{Version: 1, Digest: digest, FetchedAt: s.clock().UTC(), Catalog: document}
	if err := storage.WriteJSON(s.file, cache, 0o600); err != nil {
		return CatalogResponse{}, err
	}
	return catalogResponse(cache), nil
}

func (s *CatalogService) Preview(ctx context.Context, key string) (CatalogPreview, error) {
	cache, item, err := s.lookup(key)
	if err != nil {
		return CatalogPreview{}, err
	}
	endpoint := catalogItemURL(cache.Catalog, item, FormatSource)
	target := filepath.Join(s.layout.RuleSetsDir, ".catalog-preview.json")
	downloaded, err := s.resources.downloader.Fetch(ctx, Resource{SourceType: SourceRemote, URL: endpoint, Format: FormatSource, LocalPath: target})
	if err != nil {
		return CatalogPreview{}, err
	}
	defer os.Remove(downloaded.Path)
	if err := ValidateDownloaded(FormatSource, downloaded.Payload); err != nil {
		return CatalogPreview{}, err
	}
	content, blocks, count, truncated, err := buildCatalogPreview(downloaded.Payload)
	if err != nil {
		return CatalogPreview{}, err
	}
	return CatalogPreview{Key: key, RuleBlocks: blocks, RuleCount: count, Truncated: truncated, Content: content}, nil
}

func (s *CatalogService) Import(ctx context.Context, input CatalogImportInput) (CatalogImportResult, error) {
	cache, item, err := s.lookup(input.Key)
	if err != nil {
		return CatalogImportResult{}, err
	}
	if input.Format == "" {
		input.Format = FormatBinary
	}
	if input.Format != FormatBinary {
		return CatalogImportResult{}, errors.New("网络规则只能使用 binary 格式")
	}
	if input.UpdateInterval == "" {
		input.UpdateInterval = "24h"
	}
	endpoint := catalogItemURL(cache.Catalog, item, input.Format)
	resources, err := s.resources.List(ctx)
	if err != nil {
		return CatalogImportResult{}, err
	}
	for _, resource := range resources {
		if resource.Format == input.Format && (resource.CatalogKey == input.Key || resource.URL == endpoint) {
			return CatalogImportResult{}, &CatalogAlreadyImportedError{Key: input.Key, Format: input.Format}
		}
	}
	resource, err := s.resources.create(ctx, CreateInput{SourceType: SourceRemote, Name: item.Name + "-" + item.Type, URL: endpoint, Format: input.Format, UpdateInterval: input.UpdateInterval}, input.Key)
	if err != nil {
		return CatalogImportResult{}, err
	}
	result := CatalogImportResult{Resource: resource}
	updated, updateErr := s.resources.UpdateNow(ctx, resource.ID)
	if updateErr != nil {
		result.Warning = "规则集已添加，但首次下载失败；可在资源列表中重试。"
		if detail, getErr := s.resources.Get(context.Background(), resource.ID); getErr == nil {
			result.Resource = detail.ResourceResponse
		}
		return result, nil
	}
	result.Update = &updated
	if detail, getErr := s.resources.Get(ctx, resource.ID); getErr == nil {
		result.Resource = detail.ResourceResponse
	}
	return result, nil
}

func (s *CatalogService) lookup(key string) (catalogCache, catalogRemoteItem, error) {
	cache, err := s.readCache()
	if os.IsNotExist(err) {
		return catalogCache{}, catalogRemoteItem{}, ErrCatalogUnavailable
	}
	if err != nil {
		return catalogCache{}, catalogRemoteItem{}, err
	}
	for _, item := range cache.Catalog.List {
		if catalogKey(item.Type, item.Name) == key {
			return cache, item, nil
		}
	}
	return catalogCache{}, catalogRemoteItem{}, errors.New("规则集目录中不存在该项目")
}

func (s *CatalogService) readCache() (catalogCache, error) {
	cache, err := storage.ReadJSON[catalogCache](s.file)
	if err != nil {
		return catalogCache{}, err
	}
	if cache.Version != 1 || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(cache.Digest) || cache.FetchedAt.IsZero() {
		return catalogCache{}, errors.New("规则集目录缓存元数据无效")
	}
	if err := validateCatalog(cache.Catalog); err != nil {
		return catalogCache{}, err
	}
	return cache, nil
}

func catalogResponse(cache catalogCache) CatalogResponse {
	response := CatalogResponse{Available: true, Source: catalogSourceURL, GeoSite: cache.Catalog.GeoSite, GeoIP: cache.Catalog.GeoIP, Digest: cache.Digest, FetchedAt: ptrTime(cache.FetchedAt), Total: len(cache.Catalog.List), Items: make([]CatalogItem, 0, len(cache.Catalog.List))}
	for _, item := range cache.Catalog.List {
		response.Items = append(response.Items, CatalogItem{Key: catalogKey(item.Type, item.Name), Name: item.Name, Type: item.Type, Count: item.Count})
		if item.Type == "geosite" {
			response.GeoSiteCount++
		} else {
			response.GeoIPCount++
		}
	}
	return response
}

func catalogKey(itemType, name string) string { return itemType + ":" + name }

func validCatalogKey(key string) bool {
	parts := strings.SplitN(key, ":", 2)
	return len(parts) == 2 && (parts[0] == "geosite" || parts[0] == "geoip") && catalogNamePattern.MatchString(parts[1])
}

func catalogItemURL(document catalogRemoteDocument, item catalogRemoteItem, format Format) string {
	base := document.GeoSite
	if item.Type == "geoip" {
		base = document.GeoIP
	}
	suffix := ".json"
	if format == FormatBinary {
		suffix = ".srs"
	}
	return base + item.Name + suffix
}

func buildCatalogPreview(payload []byte) (json.RawMessage, int, int, bool, error) {
	var document struct {
		Version json.RawMessage              `json:"version"`
		Rules   []map[string]json.RawMessage `json:"rules"`
	}
	if err := json.Unmarshal(payload, &document); err != nil {
		return nil, 0, 0, false, err
	}
	count, err := SourceRuleCount(payload)
	if err != nil {
		return nil, 0, 0, false, err
	}
	blocks := len(document.Rules)
	truncated := blocks > 3
	if len(document.Rules) > 3 {
		document.Rules = document.Rules[:3]
	}
	for _, rule := range document.Rules {
		for key, raw := range rule {
			var values []json.RawMessage
			if json.Unmarshal(raw, &values) == nil && len(values) > 8 {
				encoded, _ := json.Marshal(values[:8])
				rule[key] = encoded
				truncated = true
			}
		}
	}
	content, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, 0, 0, false, err
	}
	return content, blocks, count, truncated, nil
}
