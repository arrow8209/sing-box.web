// Package apierror defines the stable JSON error contract used by HTTP APIs.
package apierror

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Error is safe to return to an API client. It must never contain secrets.
type Error struct {
	Code           string `json:"code"`
	Message        string `json:"message"`
	Stage          string `json:"stage,omitempty"`
	FieldPath      string `json:"fieldPath,omitempty"`
	LatestRevision uint64 `json:"latestRevision,omitempty"`
	Details        any    `json:"details,omitempty"`
}

// HTTPError associates a client-safe error with its HTTP status.
type HTTPError struct {
	Status int
	Public Error
	Cause  error
}

func (e *HTTPError) Unwrap() error { return e.Cause }

func (e *HTTPError) Error() string { return e.Public.Code }

func New(status int, code, message string, cause error) *HTTPError {
	return &HTTPError{Status: status, Public: Error{Code: code, Message: message}, Cause: cause}
}

func BadRequest(code, message string, cause error) *HTTPError {
	return New(http.StatusBadRequest, code, message, cause)
}

func PayloadTooLarge(code, message string, cause error) *HTTPError {
	return New(http.StatusRequestEntityTooLarge, code, message, cause)
}

// Write writes the common {"error": ...} response shape.
func Write(w http.ResponseWriter, status int, value Error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Error Error `json:"error"`
	}{Error: value})
}

// WriteError converts known errors to their public form. Unknown errors remain
// deliberately generic so internal state is not exposed.
func WriteError(w http.ResponseWriter, err error) {
	var httpError *HTTPError
	if errors.As(err, &httpError) {
		Write(w, httpError.Status, httpError.Public)
		return
	}
	Write(w, http.StatusInternalServerError, Error{Code: "internal_error", Message: "服务器内部错误"})
}

// Field builds a bad-request error associated with a request field.
func Field(code, message, stage, fieldPath string, cause error) *HTTPError {
	err := BadRequest(code, message, cause)
	err.Public.Stage = stage
	err.Public.FieldPath = fieldPath
	return err
}
