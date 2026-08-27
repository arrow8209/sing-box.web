package updater

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUpdateStatusHandlerExposesDownloadProgress(t *testing.T) {
	service := &Service{status: Status{
		OperationID:     "update-a",
		Stage:           "downloading",
		DownloadedBytes: 8 << 20,
		TotalBytes:      32 << 20,
		BytesPerSecond:  4 << 20,
	}}
	response := httptest.NewRecorder()
	NewHandler(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/core/update", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		DownloadedBytes int64 `json:"downloadedBytes"`
		TotalBytes      int64 `json:"totalBytes"`
		BytesPerSecond  int64 `json:"bytesPerSecond"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.DownloadedBytes != 8<<20 || body.TotalBytes != 32<<20 || body.BytesPerSecond != 4<<20 {
		t.Fatalf("body=%+v", body)
	}
}
