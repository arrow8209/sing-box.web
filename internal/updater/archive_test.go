package updater_test

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"singbox-web/internal/updater"
)

type tarEntry struct {
	Name     string
	Mode     int64
	Typeflag byte
	Linkname string
	Payload  []byte
}

func TestExtractRejectsTraversalAndLinks(t *testing.T) {
	expected := "sing-box-1.12.3-linux-amd64/sing-box"
	for _, entry := range []tarEntry{
		{Name: "../../sing-box", Mode: 0o755}, {Name: "/tmp/sing-box", Mode: 0o755},
		{Name: expected, Mode: 0o755, Typeflag: tar.TypeSymlink, Linkname: "/bin/sh"},
	} {
		archive := makeTarGz(t, entry)
		if _, err := updater.ExtractBinary(archive, t.TempDir(), expected); err == nil {
			t.Fatalf("accepted entry=%+v", entry)
		}
	}
}

func TestExtractWritesOnlyOneExecutableTarget(t *testing.T) {
	expected := "sing-box-1.12.3-linux-amd64/sing-box"
	archive := makeTarGz(t, tarEntry{Name: "sing-box-1.12.3-linux-amd64/", Typeflag: tar.TypeDir}, tarEntry{Name: expected, Mode: 0o755, Payload: []byte("program")})
	destination := t.TempDir()
	program, err := updater.ExtractBinary(archive, destination, expected)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(program)
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("info=%v err=%v", info, err)
	}
}

func TestChecksumMustMatchExactAssetName(t *testing.T) {
	checksums := []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  other.tar.gz\n")
	if _, err := updater.FindChecksum(checksums, "sing-box-1.12.3-linux-amd64.tar.gz"); err == nil {
		t.Fatal("wrong checksum accepted")
	}
}

func makeTarGz(t *testing.T, entries ...tarEntry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "core.tar.gz")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(file)
	writer := tar.NewWriter(gzipWriter)
	for _, entry := range entries {
		typeflag := entry.Typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}
		header := &tar.Header{Name: entry.Name, Mode: entry.Mode, Typeflag: typeflag, Linkname: entry.Linkname, Size: int64(len(entry.Payload))}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(entry.Payload); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
