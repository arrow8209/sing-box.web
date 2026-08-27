package updater

import "testing"

func TestServicePublishesDownloadProgressAndStopsSpeedAfterTransfer(t *testing.T) {
	service := &Service{active: true, status: Status{Stage: "downloading"}}
	service.setDownloadProgress(DownloadProgress{
		DownloadedBytes: 8 << 20,
		TotalBytes:      32 << 20,
		BytesPerSecond:  4 << 20,
	})
	status := service.Status()
	if status.DownloadedBytes != 8<<20 || status.TotalBytes != 32<<20 || status.BytesPerSecond != 4<<20 {
		t.Fatalf("status=%+v", status)
	}
	service.setStage("verifying_checksum")
	status = service.Status()
	if status.DownloadedBytes != 8<<20 || status.TotalBytes != 32<<20 || status.BytesPerSecond != 0 {
		t.Fatalf("status after transfer=%+v", status)
	}
}
