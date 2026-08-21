// Package httpapi contains shared HTTP request and response helpers.
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"singbox-web/internal/apierror"
)

// DecodeJSON accepts exactly one JSON value, rejects unknown fields, and
// applies a hard request-body limit.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any, maxBytes int64) error {
	if maxBytes <= 0 {
		return apierror.PayloadTooLarge("request_too_large", "请求体过大", nil)
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return apierror.PayloadTooLarge("request_too_large", "请求体过大", err)
		}
		return apierror.BadRequest("invalid_json", "请求 JSON 无效", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return apierror.BadRequest("multiple_json_values", "请求只能包含一个 JSON 值", nil)
		}
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return apierror.PayloadTooLarge("request_too_large", "请求体过大", err)
		}
		return apierror.BadRequest("invalid_json", "请求 JSON 无效", err)
	}
	return nil
}

// WriteJSON writes a JSON response unless the status forbids a response body.
func WriteJSON(w http.ResponseWriter, status int, value any) {
	if status == http.StatusNoContent || status == http.StatusNotModified {
		w.WriteHeader(status)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// MethodNotAllowed writes a consistent method error and Allow header.
func MethodNotAllowed(w http.ResponseWriter, methods ...string) {
	w.Header().Set("Allow", strings.Join(methods, ", "))
	apierror.Write(w, http.StatusMethodNotAllowed, apierror.Error{Code: "method_not_allowed", Message: "请求方法不被允许"})
}
