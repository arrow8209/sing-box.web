package application

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"singbox-web/internal/configgen"
	"singbox-web/internal/paths"
)

func TestPreserveFailedCheckDirectoryReplacesPreviousReviewCopy(t *testing.T) {
	layout, err := paths.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	review := filepath.Join(layout.WorkDir, failedCheckDirectoryName)
	if err := os.Mkdir(review, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(review, "old.json"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	staging, err := layout.StagingConfigDir("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "60-inbounds.json"), []byte("candidate"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := preserveFailedCheckDirectory(layout, staging)
	if err != nil {
		t.Fatal(err)
	}
	if got != review {
		t.Fatalf("review=%q want=%q", got, review)
	}
	if payload, err := os.ReadFile(filepath.Join(review, "60-inbounds.json")); err != nil || string(payload) != "candidate" {
		t.Fatalf("candidate=%q err=%v", payload, err)
	}
	if _, err := os.Stat(filepath.Join(review, "old.json")); !os.IsNotExist(err) {
		t.Fatalf("old review copy remains: %v", err)
	}
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatalf("staging remains after preservation: %v", err)
	}
}

func TestWriteApplyErrorIncludesCoreOutputAndReviewLocation(t *testing.T) {
	failure := &CheckFailure{
		Cause:                  fmt.Errorf("%w: exit status 1", configgen.ErrCheckFailed),
		Result:                 configgen.CheckResult{Stderr: "FATAL decode 60-inbounds.json", ExitCode: 1, OutputTruncated: true},
		BinaryPath:             "/service/data/bin/sing-box",
		CheckedConfigDirectory: "/service/config.staging-0123456789abcdef0123456789abcdef",
		ReviewConfigDirectory:  "/service/config.check-failed",
		ConfigFiles:            []string{"10-log.json", "60-inbounds.json"},
		ProfileID:              "0123456789abcdef0123456789abcdef",
		Revision:               21,
		ConfigDigest:           "digest-a",
	}
	recorder := httptest.NewRecorder()
	(&Handler{}).writeApplyError(recorder, failure)

	if recorder.Code != 422 {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Error struct {
			Code    string              `json:"code"`
			Message string              `json:"message"`
			Stage   string              `json:"stage"`
			Details checkFailureDetails `json:"details"`
		} `json:"error"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "config_check_failed" || response.Error.Stage != "check_config" || response.Error.Message != "sing-box 配置检查失败（退出码 1）" {
		t.Fatalf("error=%+v", response.Error)
	}
	wantReviewCommand := []string{"/service/data/bin/sing-box", "check", "-C", "/service/config.check-failed"}
	if !reflect.DeepEqual(response.Error.Details.ReviewCommand, wantReviewCommand) {
		t.Fatalf("review command=%v", response.Error.Details.ReviewCommand)
	}
	if response.Error.Details.Stderr != "FATAL decode 60-inbounds.json" || response.Error.Details.ExitCode != 1 || !response.Error.Details.OutputTruncated || response.Error.Details.Revision != 21 || response.Error.Details.ConfigDigest != "digest-a" {
		t.Fatalf("details=%+v", response.Error.Details)
	}
}
