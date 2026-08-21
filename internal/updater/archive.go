package updater

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// FindChecksum only accepts a single SHA-256 record whose file name is an
// exact match. Prefixes and similarly named assets are deliberately ignored.
func FindChecksum(payload []byte, assetName string) (string, error) {
	var result string
	for _, line := range strings.Split(string(payload), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		if name != assetName {
			continue
		}
		if len(fields[0]) != sha256.Size*2 || !isLowerHex(fields[0]) {
			return "", ErrChecksumNotFound
		}
		if result != "" {
			return "", ErrChecksumNotFound
		}
		result = fields[0]
	}
	if result == "" {
		return "", ErrChecksumNotFound
	}
	return result, nil
}

func isLowerHex(value string) bool {
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

// ExtractBinary extracts exactly the expected ordinary program from a tar.gz
// archive. Every archive entry must be a canonical path under its root; links
// and special files are never accepted.
func ExtractBinary(archivePath, destination, expected string) (string, error) {
	if !canonicalArchivePath(expected) || path.Base(expected) != "sing-box" {
		return "", ErrUnsafeArchive
	}
	root := path.Dir(expected)
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return "", fmt.Errorf("%w: gzip", ErrUnsafeArchive)
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	var total int64
	var binaryPath string
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return cleanupExtractFailure(binaryPath, err)
		}
		entryName := header.Name
		if header.Typeflag == tar.TypeDir {
			entryName = strings.TrimSuffix(entryName, "/")
		}
		if !canonicalArchivePath(entryName) || (entryName != root && !strings.HasPrefix(entryName, root+"/")) {
			return cleanupExtractFailure(binaryPath, ErrUnsafeArchive)
		}
		if header.Size < 0 || header.Size > maxArchiveBytes-total {
			return cleanupExtractFailure(binaryPath, ErrUnsafeArchive)
		}
		total += header.Size
		switch header.Typeflag {
		case tar.TypeDir:
			if entryName == expected {
				return cleanupExtractFailure(binaryPath, ErrUnsafeArchive)
			}
		case tar.TypeReg, tar.TypeRegA:
			if entryName != expected {
				if _, err := discardExact(reader, header.Size); err != nil {
					return cleanupExtractFailure(binaryPath, err)
				}
				continue
			}
			if binaryPath != "" {
				return cleanupExtractFailure(binaryPath, ErrUnsafeArchive)
			}
			binaryPath = filepath.Join(destination, "sing-box")
			if err := copyProgram(reader, binaryPath, header.Size); err != nil {
				return cleanupExtractFailure(binaryPath, err)
			}
		default:
			return cleanupExtractFailure(binaryPath, ErrUnsafeArchive)
		}
	}
	if binaryPath == "" {
		return "", ErrUnsafeArchive
	}
	if err := os.Chmod(binaryPath, 0o700); err != nil {
		return cleanupExtractFailure(binaryPath, err)
	}
	return binaryPath, nil
}

func canonicalArchivePath(name string) bool {
	return name != "" && !strings.HasPrefix(name, "/") && path.Clean(name) == name && name != "." && !strings.HasPrefix(name, "../") && name != ".."
}

func discardExact(reader io.Reader, size int64) (int64, error) {
	return io.CopyN(io.Discard, reader, size)
}

func copyProgram(reader io.Reader, destination string, size int64) error {
	program, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.CopyN(program, reader, size)
	syncErr := program.Sync()
	closeErr := program.Close()
	if copyErr != nil {
		return copyErr
	}
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

func cleanupExtractFailure(binaryPath string, err error) (string, error) {
	if binaryPath != "" {
		_ = os.Remove(binaryPath)
	}
	return "", err
}
