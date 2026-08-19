package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"singbox-web/internal/audit"
	"singbox-web/internal/configdoc"
	"singbox-web/internal/profile"
	"singbox-web/internal/publication"
	"singbox-web/internal/singbox"
)

type deleteProfileRequest struct {
	ConfirmName string `json:"confirmName"`
}

func (service *server) validateProfile(response http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	resultCode, revision := "internal_error", uint64(0)
	started := service.deps.Now()
	defer func() {
		service.recordAudit(request.Context(), audit.Event{Operation: "validate", ProfileID: id, Revision: revision, DurationMS: elapsedMS(started, service.deps.Now()), Result: resultCode})
	}()
	if !validProfileID(id) || service.deps.Profiles == nil || service.deps.Preflight == nil {
		resultCode = "not_found"
		writeNotFound(response)
		return
	}
	if _, ok := service.profileExists(request.Context(), id); !ok {
		resultCode = "not_found"
		writeNotFound(response)
		return
	}
	draft, err := service.deps.Profiles.GetDraft(request.Context(), id)
	if err != nil {
		if errors.Is(err, profile.ErrDraftCorrupt) {
			resultCode = "draft_corrupt"
			writeAPIError(response, http.StatusConflict, resultCode, "草稿 JSON 损坏，需要管理员在服务器修复")
			return
		}
		writeInternalError(response)
		return
	}
	revision = draft.Revision
	result := service.deps.Preflight.ValidateDraft(draft)
	summary := profile.ValidationSummary{Revision: revision, Status: "valid", Stage: "preflight", Message: "校验通过", CheckedAt: result.CheckedAt}
	if !result.Valid {
		summary.Status = "invalid"
		summary.Stage = "preflight"
		if len(result.Issues) != 0 {
			summary.Stage = result.Issues[0].Stage
		}
		summary.Message = fmt.Sprintf("校验失败，共 %d 项", len(result.Issues))
	}
	if _, err := service.deps.Profiles.RecordValidation(request.Context(), id, revision, summary); err != nil {
		var conflict *profile.RevisionConflict
		if errors.As(err, &conflict) {
			resultCode = "draft_conflict"
			current := conflict.Current
			writeJSON(response, http.StatusConflict, ErrorBody{Error: APIError{Code: resultCode, Message: "草稿已被其他页面更新", CurrentRevision: &current}})
			return
		}
		writeInternalError(response)
		return
	}
	resultCode = "ok"
	writeJSON(response, http.StatusOK, result)
}

func (service *server) publishProfile(response http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	resultCode, revision, digest := "internal_error", uint64(0), ""
	started := service.deps.Now()
	defer func() {
		service.recordAudit(request.Context(), audit.Event{Operation: "publish", ProfileID: id, Revision: revision, Digest: digest, DurationMS: elapsedMS(started, service.deps.Now()), Result: resultCode})
	}()
	if !validProfileID(id) || service.deps.Profiles == nil || service.deps.Publication == nil {
		resultCode = "not_found"
		writeNotFound(response)
		return
	}
	if _, ok := service.profileExists(request.Context(), id); !ok {
		resultCode = "not_found"
		writeNotFound(response)
		return
	}
	expected, ok := parseDraftETag(request.Header.Values("If-Match"))
	if !ok {
		resultCode = "revision_required"
		writeAPIError(response, http.StatusPreconditionRequired, resultCode, "需要有效的草稿修订号")
		return
	}
	revision = expected
	input := publication.Request{ProfileID: id, ExpectedRevision: expected}
	if err := decodeRequest(request, &input); err != nil {
		resultCode = "invalid_request"
		writeAPIError(response, http.StatusBadRequest, resultCode, "请求无效")
		return
	}
	receipt, err := service.deps.Publication.Publish(request.Context(), input)
	if err == nil {
		resultCode, digest = "ok", receipt.Digest
		writeJSON(response, http.StatusOK, receipt)
		return
	}
	var conflict *profile.RevisionConflict
	switch {
	case errors.As(err, &conflict):
		resultCode = "draft_conflict"
		current := conflict.Current
		writeJSON(response, http.StatusConflict, ErrorBody{Error: APIError{Code: resultCode, Message: "草稿已被其他页面更新", CurrentRevision: &current}})
	case errors.Is(err, singbox.ErrUnavailable):
		resultCode = "singbox_unavailable"
		writeCheckerError(response, http.StatusServiceUnavailable, resultCode, "sing-box 不可用")
	case errors.Is(err, singbox.ErrTimeout):
		resultCode = "singbox_check_timeout"
		writeCheckerError(response, http.StatusGatewayTimeout, resultCode, "sing-box check 超时")
	case errors.Is(err, singbox.ErrCheckFailed) || (errors.Is(err, publication.ErrValidationFailed) && receipt.Validation.Valid):
		resultCode = "singbox_check_failed"
		writeCheckerError(response, http.StatusUnprocessableEntity, resultCode, "sing-box check 失败")
	case errors.Is(err, publication.ErrValidationFailed):
		resultCode = "validation_failed"
		writeJSON(response, http.StatusUnprocessableEntity, ErrorBody{Error: APIError{Code: resultCode, Message: "配置校验失败", FieldIssues: receipt.Validation.Issues}})
	default:
		writeInternalError(response)
	}
}

func writeCheckerError(response http.ResponseWriter, status int, code, message string) {
	writeJSON(response, status, ErrorBody{Error: APIError{Code: code, Message: message, Stderr: message}})
}

func (service *server) exportProfile(response http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	resultCode, revision := "internal_error", uint64(0)
	started := service.deps.Now()
	defer func() {
		service.recordAudit(request.Context(), audit.Event{Operation: "export", ProfileID: id, Revision: revision, DurationMS: elapsedMS(started, service.deps.Now()), Result: resultCode})
	}()
	if !validProfileID(id) || service.deps.Profiles == nil || service.deps.Publication == nil {
		resultCode = "not_found"
		writeNotFound(response)
		return
	}
	item, err := service.deps.Profiles.Get(request.Context(), id)
	if err != nil {
		resultCode = "not_found"
		writeNotFound(response)
		return
	}
	query := request.URL.Query()
	for key, values := range query {
		if (key != "mode" && key != "source" && key != "download") || len(values) != 1 {
			resultCode = "invalid_request"
			writeAPIError(response, http.StatusBadRequest, resultCode, "导出参数无效")
			return
		}
	}
	mode, source, download := query.Get("mode"), query.Get("source"), query.Get("download")
	if source == "" {
		source = "draft"
	}
	if download == "" {
		download = "0"
	}
	if (mode != "full" && mode != "redacted") || (source != "draft" && source != "published") || (download != "0" && download != "1") {
		resultCode = "invalid_request"
		writeAPIError(response, http.StatusBadRequest, resultCode, "导出参数无效")
		return
	}
	var config json.RawMessage
	if source == "draft" {
		draft, draftErr := service.deps.Profiles.GetDraft(request.Context(), id)
		if draftErr != nil {
			if errors.Is(draftErr, profile.ErrDraftCorrupt) {
				resultCode = "draft_corrupt"
				writeAPIError(response, http.StatusConflict, resultCode, "草稿 JSON 损坏，需要管理员在服务器修复")
				return
			}
			writeInternalError(response)
			return
		}
		revision, config = draft.Revision, draft.Content.Config
	} else {
		if item.PublishedRevision != nil {
			revision = *item.PublishedRevision
		}
		config, err = service.deps.Publication.ReadPublished(request.Context(), id)
		if errors.Is(err, publication.ErrNotPublished) {
			resultCode = "not_published"
			writeAPIError(response, http.StatusNotFound, resultCode, "档案尚未发布")
			return
		}
		if err != nil {
			writeInternalError(response)
			return
		}
	}
	if mode == "redacted" {
		config, err = configdoc.Redact(config)
		if err != nil {
			writeInternalError(response)
			return
		}
	}
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-store")
	if download == "1" {
		response.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s-config.json\"", item.Slug))
	}
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(config)
	resultCode = "ok"
}

func (service *server) deleteProfile(response http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	resultCode := "internal_error"
	started := service.deps.Now()
	defer func() {
		service.recordAudit(request.Context(), audit.Event{Operation: "delete", ProfileID: id, DurationMS: elapsedMS(started, service.deps.Now()), Result: resultCode})
	}()
	if service.deps.Publication == nil && service.deps.Profiles != nil && validProfileID(id) {
		if _, ok := service.profileExists(request.Context(), id); ok {
			resultCode = "method_not_allowed"
			writeMethodNotAllowed(response, "PATCH")
			return
		}
	}
	if !validProfileID(id) || service.deps.Profiles == nil || service.deps.Publication == nil {
		resultCode = "not_found"
		writeNotFound(response)
		return
	}
	item, err := service.deps.Profiles.Get(request.Context(), id)
	if err != nil {
		resultCode = "not_found"
		writeNotFound(response)
		return
	}
	var input deleteProfileRequest
	if err := decodeRequest(request, &input); err != nil {
		resultCode = "invalid_request"
		writeAPIError(response, http.StatusBadRequest, resultCode, "请求无效")
		return
	}
	if input.ConfirmName != item.Name {
		resultCode = "confirmation_mismatch"
		writeAPIError(response, http.StatusConflict, resultCode, "档案名称确认不匹配")
		return
	}
	if err := service.deps.Publication.Delete(request.Context(), id, input.ConfirmName); err != nil {
		writeInternalError(response)
		return
	}
	resultCode = "ok"
	writeJSON(response, http.StatusOK, map[string]string{"warning": "一期无法判断外部核心是否正在使用该路径"})
}

func elapsedMS(start, end time.Time) int64 {
	value := end.Sub(start).Milliseconds()
	if value < 0 {
		return 0
	}
	return value
}

func (service *server) recordAudit(ctx context.Context, event audit.Event) {
	if service.deps.Auditor == nil {
		return
	}
	event.RequestID, _ = ctx.Value(requestIDContextKey).(string)
	service.deps.Auditor.Record(event)
}
