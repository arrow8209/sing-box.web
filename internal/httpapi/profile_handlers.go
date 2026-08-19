package httpapi

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"singbox-web/internal/profile"
)

const maxModuleBytes = 1_000_000

var (
	httpProfileIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
	httpSlugPattern      = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
)

type ProfileResponse struct {
	profile.Profile
	DirectoryHint string `json:"directoryHint"`
	CheckCommand  string `json:"checkCommand"`
	DraftReadOnly bool   `json:"draftReadOnly"`
	DraftError    string `json:"draftError"`
}

type renameProfileRequest struct {
	Name string `json:"name"`
}

type draftRequest struct {
	Config        json.RawMessage                 `json:"config"`
	EditorBuffers map[string]profile.EditorBuffer `json:"editorBuffers"`
}

func (service *server) createProfile(response http.ResponseWriter, request *http.Request) {
	if service.deps.Profiles == nil || service.deps.ConfigRoot == "" {
		writeInternalError(response)
		return
	}
	var input profile.CreateInput
	if err := decodeRequest(request, &input); err != nil {
		writeAPIError(response, http.StatusBadRequest, "invalid_request", "请求无效")
		return
	}
	if !validProfileName(input.Name) || !httpSlugPattern.MatchString(input.Slug) || filepath.IsAbs(input.Slug) {
		writeAPIError(response, http.StatusUnprocessableEntity, "validation_failed", "档案字段无效")
		return
	}
	items, err := service.deps.Profiles.List(request.Context())
	if err != nil {
		writeInternalError(response)
		return
	}
	for _, item := range items {
		if item.Slug == input.Slug {
			writeAPIError(response, http.StatusConflict, "duplicate_slug", "档案标识已存在")
			return
		}
	}
	path := filepath.Join(service.deps.ConfigRoot, input.Slug)
	if _, err := os.Lstat(path); err == nil {
		writeAPIError(response, http.StatusConflict, "slug_path_occupied", "档案目录已被占用")
		return
	} else if !errors.Is(err, os.ErrNotExist) {
		writeInternalError(response)
		return
	}
	item, _, err := service.deps.Profiles.Create(request.Context(), input)
	if err != nil {
		if strings.Contains(err.Error(), "profile slug") && strings.Contains(err.Error(), "already exists") {
			writeAPIError(response, http.StatusConflict, "duplicate_slug", "档案标识已存在")
			return
		}
		if isValidationError(err) {
			writeAPIError(response, http.StatusUnprocessableEntity, "validation_failed", "档案字段无效")
			return
		}
		writeInternalError(response)
		return
	}
	result, err := service.profileResponse(request.Context(), item)
	if err != nil {
		writeInternalError(response)
		return
	}
	writeJSON(response, http.StatusCreated, result)
}

func (service *server) profileRouteNotFound(response http.ResponseWriter, request *http.Request) {
	relative := strings.TrimPrefix(request.URL.Path, "/api/v1/profiles/")
	segments := strings.Split(relative, "/")
	allow := ""
	switch {
	case len(segments) == 1:
		allow = "PATCH"
	case len(segments) == 2 && segments[1] == "draft":
		allow = "GET, PUT"
	default:
		writeNotFound(response)
		return
	}
	if !validProfileID(segments[0]) || service.deps.Profiles == nil {
		writeNotFound(response)
		return
	}
	if _, ok := service.profileExists(request.Context(), segments[0]); !ok {
		writeNotFound(response)
		return
	}
	writeMethodNotAllowed(response, allow)
}

func (service *server) listProfiles(response http.ResponseWriter, request *http.Request) {
	if service.deps.Profiles == nil || service.deps.ConfigRoot == "" {
		writeInternalError(response)
		return
	}
	items, err := service.deps.Profiles.List(request.Context())
	if err != nil {
		writeInternalError(response)
		return
	}
	result := make([]ProfileResponse, 0, len(items))
	for _, item := range items {
		value, err := service.profileResponse(request.Context(), item)
		if err != nil {
			writeInternalError(response)
			return
		}
		result = append(result, value)
	}
	writeJSON(response, http.StatusOK, result)
}

func (service *server) renameProfile(response http.ResponseWriter, request *http.Request) {
	if service.deps.Profiles == nil || service.deps.ConfigRoot == "" {
		writeInternalError(response)
		return
	}
	id := request.PathValue("id")
	if !validProfileID(id) {
		writeNotFound(response)
		return
	}
	var input renameProfileRequest
	if err := decodeRequest(request, &input); err != nil {
		writeAPIError(response, http.StatusBadRequest, "invalid_request", "请求无效")
		return
	}
	if !validProfileName(input.Name) {
		writeAPIError(response, http.StatusUnprocessableEntity, "validation_failed", "档案字段无效")
		return
	}
	if _, ok := service.profileExists(request.Context(), id); !ok {
		writeNotFound(response)
		return
	}
	item, err := service.deps.Profiles.Rename(request.Context(), id, input.Name)
	if err != nil {
		if isValidationError(err) {
			writeAPIError(response, http.StatusUnprocessableEntity, "validation_failed", "档案字段无效")
			return
		}
		writeInternalError(response)
		return
	}
	result, err := service.profileResponse(request.Context(), item)
	if err != nil {
		writeInternalError(response)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (service *server) getDraft(response http.ResponseWriter, request *http.Request) {
	if service.deps.Profiles == nil {
		writeInternalError(response)
		return
	}
	id := request.PathValue("id")
	if !validProfileID(id) {
		writeNotFound(response)
		return
	}
	if _, ok := service.profileExists(request.Context(), id); !ok {
		writeNotFound(response)
		return
	}
	if request.Method != http.MethodGet {
		writeMethodNotAllowed(response, "GET, PUT")
		return
	}
	draft, err := service.deps.Profiles.GetDraft(request.Context(), id)
	if err != nil {
		if errors.Is(err, profile.ErrDraftCorrupt) {
			writeAPIError(response, http.StatusConflict, "draft_corrupt", "草稿 JSON 损坏，需要管理员在服务器修复")
			return
		}
		writeInternalError(response)
		return
	}
	response.Header().Set("ETag", draftETag(draft.Revision))
	writeJSON(response, http.StatusOK, draft)
}

func (service *server) saveDraft(response http.ResponseWriter, request *http.Request) {
	if service.deps.Profiles == nil {
		writeInternalError(response)
		return
	}
	id := request.PathValue("id")
	if !validProfileID(id) {
		writeNotFound(response)
		return
	}
	if _, ok := service.profileExists(request.Context(), id); !ok {
		writeNotFound(response)
		return
	}
	expected, ok := parseDraftETag(request.Header.Values("If-Match"))
	if !ok {
		writeAPIError(response, http.StatusPreconditionRequired, "revision_required", "需要有效的草稿修订号")
		return
	}
	var input draftRequest
	if err := decodeRequest(request, &input); err != nil || len(input.Config) == 0 {
		writeAPIError(response, http.StatusBadRequest, "invalid_request", "请求无效")
		return
	}
	if key, count, oversized, err := oversizedDraftPart(input); err != nil {
		writeAPIError(response, http.StatusUnprocessableEntity, "validation_failed", "草稿字段无效")
		return
	} else if oversized {
		writeModuleTooLarge(response, key, count)
		return
	}
	draft, err := service.deps.Profiles.SaveDraft(request.Context(), id, expected, profile.DraftContent{Config: input.Config, EditorBuffers: input.EditorBuffers})
	if err != nil {
		if errors.Is(err, profile.ErrDraftCorrupt) {
			writeAPIError(response, http.StatusConflict, "draft_corrupt", "草稿 JSON 损坏，需要管理员在服务器修复")
			return
		}
		var conflict *profile.RevisionConflict
		if errors.As(err, &conflict) {
			current := conflict.Current
			writeJSON(response, http.StatusConflict, ErrorBody{Error: APIError{Code: "draft_conflict", Message: "草稿已被其他页面更新", CurrentRevision: &current}})
			return
		}
		if isValidationError(err) {
			writeAPIError(response, http.StatusUnprocessableEntity, "validation_failed", "草稿字段无效")
			return
		}
		writeInternalError(response)
		return
	}
	response.Header().Set("ETag", draftETag(draft.Revision))
	writeJSON(response, http.StatusOK, draft)
}

func (service *server) profileResponse(ctx context.Context, item profile.Profile) (ProfileResponse, error) {
	if !httpSlugPattern.MatchString(item.Slug) || filepath.IsAbs(item.Slug) {
		return ProfileResponse{}, fmt.Errorf("invalid profile slug")
	}
	directory := filepath.Join(service.deps.ConfigRoot, item.Slug)
	status := service.deps.Profiles.DraftStatus(ctx, item.ID)
	return ProfileResponse{Profile: item, DirectoryHint: directory, CheckCommand: "sing-box check -C " + shellQuote(directory), DraftReadOnly: status.ReadOnly, DraftError: status.Message}, nil
}

func (service *server) profileExists(ctx context.Context, id string) (profile.Profile, bool) {
	item, err := service.deps.Profiles.Get(ctx, id)
	return item, err == nil
}

func validProfileID(id string) bool {
	if !httpProfileIDPattern.MatchString(id) {
		return false
	}
	decoded, err := hex.DecodeString(id)
	return err == nil && len(decoded) == 16
}

func validProfileName(name string) bool {
	trimmed := strings.TrimSpace(name)
	return utf8.ValidString(trimmed) && utf8.RuneCountInString(trimmed) >= 1 && utf8.RuneCountInString(trimmed) <= 80
}

func parseDraftETag(values []string) (uint64, bool) {
	if len(values) != 1 || len(values[0]) < len(`"draft-1"`) || !strings.HasPrefix(values[0], `"draft-`) || !strings.HasSuffix(values[0], `"`) {
		return 0, false
	}
	raw := values[0][len(`"draft-`) : len(values[0])-1]
	if raw == "" || raw[0] == '0' {
		return 0, false
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	return value, err == nil && value > 0
}

func draftETag(revision uint64) string { return fmt.Sprintf(`"draft-%d"`, revision) }

func oversizedDraftPart(input draftRequest) (string, int, bool, error) {
	decoder := json.NewDecoder(bytes.NewReader(input.Config))
	decoder.UseNumber()
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return "", 0, false, errors.New("config must be an object")
	}
	seen := make(map[string]struct{})
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return "", 0, false, errors.New("invalid config member")
		}
		key, ok := token.(string)
		if !ok {
			return "", 0, false, errors.New("invalid config key")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return "", 0, false, errors.New("invalid config value")
		}
		if len(value) > maxModuleBytes {
			return safeErrorKey(key), len(value), true, nil
		}
		if _, exists := seen[key]; exists {
			return "", 0, false, errors.New("duplicate config key")
		}
		seen[key] = struct{}{}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return "", 0, false, errors.New("invalid config object")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return "", 0, false, errors.New("config contains trailing value")
	}
	for key, value := range input.EditorBuffers {
		if len([]byte(value.Text)) > maxModuleBytes {
			return safeErrorKey(key), len([]byte(value.Text)), true, nil
		}
	}
	return "", 0, false, nil
}

func safeErrorKey(key string) string {
	if key == "" || len(key) > 128 || !utf8.ValidString(key) || strings.IndexFunc(key, unicode.IsControl) >= 0 {
		return "(invalid)"
	}
	return key
}

func shellQuote(value string) string {
	return `'` + strings.ReplaceAll(value, `'`, `'"'"'`) + `'`
}

func isValidationError(err error) bool {
	text := err.Error()
	return strings.Contains(text, "invalid") || strings.Contains(text, "must") || strings.Contains(text, "unsafe") || strings.Contains(text, "contain")
}

func writeNotFound(response http.ResponseWriter) {
	writeAPIError(response, http.StatusNotFound, "not_found", "资源不存在")
}

func writeMethodNotAllowed(response http.ResponseWriter, allow string) {
	response.Header().Set("Allow", allow)
	writeAPIError(response, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不受支持")
}

func writeInternalError(response http.ResponseWriter) {
	writeAPIError(response, http.StatusInternalServerError, "internal_error", "服务器内部错误")
}

func writeModuleTooLarge(response http.ResponseWriter, key string, count int) {
	writeJSON(response, http.StatusRequestEntityTooLarge, ErrorBody{Error: APIError{Code: "module_too_large", Message: "模块或编辑缓冲过大", Key: key, Bytes: &count}})
}
