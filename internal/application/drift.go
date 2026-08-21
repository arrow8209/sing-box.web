// Package application contains the durable configuration-application
// primitives. It deliberately keeps the live config directory separate from
// configuration generation so an administrator must explicitly acknowledge
// host-side changes before they can be replaced.
package application

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	maxTreeEntries = 1024
	maxFileBytes   = 16 << 20
	maxTotalBytes  = 64 << 20
)

// ErrExternalDrift means that config differs from the digest last generated
// by this application and that the caller did not confirm this exact state.
var ErrExternalDrift = errors.New("config_external_drift")

// DriftResult gives callers the digest that must be sent back unchanged to
// acknowledge the currently observed host-side configuration.
type DriftResult struct {
	ExpectedDigest     string `json:"expectedDigest"`
	CurrentDigest      string `json:"currentDigest"`
	ConfirmationDigest string `json:"confirmationDigest"`
}

type treeEntry struct {
	relativePath string
	absolutePath string
	typeName     string
	mode         fs.FileMode
}

// DigestDirectory returns a stable digest of every node in dir. Directories,
// unusual files and symlinks are represented rather than rejected so an
// administrator can explicitly confirm external host-side content. Symlinks
// are never followed. A missing directory has the empty digest, which is the
// first-application state.
func DigestDirectory(dir string) (string, error) {
	entries, err := collectTreeEntries(dir)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	slices.SortFunc(entries, func(left, right treeEntry) int {
		return strings.Compare(left.relativePath, right.relativePath)
	})

	hash := sha256.New()
	var totalBytes int64
	for _, entry := range entries {
		writeDigestField(hash, entry.relativePath)
		writeDigestField(hash, entry.typeName)
		writeDigestField(hash, entry.mode.String())
		switch entry.typeName {
		case "file":
			payload, err := readLimited(entry.absolutePath, maxFileBytes)
			if err != nil {
				return "", err
			}
			totalBytes += int64(len(payload))
			if totalBytes > maxTotalBytes {
				return "", fmt.Errorf("配置目录普通文件总大小超过 %d 字节，需先在主机侧清理", maxTotalBytes)
			}
			if _, err := hash.Write(payload); err != nil {
				return "", err
			}
		case "symlink":
			target, err := os.Readlink(entry.absolutePath)
			if err != nil {
				return "", err
			}
			writeDigestField(hash, target)
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// CheckDrift allows an apply to proceed only when the currently observed
// digest is still the expected one or the caller echoed this exact digest.
func CheckDrift(expected, confirmed, current string) (DriftResult, error) {
	result := DriftResult{ExpectedDigest: expected, CurrentDigest: current}
	if expected == current || confirmed == current {
		return result, nil
	}
	result.ConfirmationDigest = current
	return result, fmt.Errorf("%w: 当前配置已在主机侧变更", ErrExternalDrift)
}

func collectTreeEntries(dir string) ([]treeEntry, error) {
	entries := make([]treeEntry, 0)
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == dir {
			return nil
		}
		if len(entries) >= maxTreeEntries {
			return fmt.Errorf("配置目录项目超过 %d 项，需先在主机侧清理", maxTreeEntries)
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		entryType := "other"
		switch {
		case entry.Type()&os.ModeSymlink != 0:
			entryType = "symlink"
		case info.IsDir():
			entryType = "directory"
		case info.Mode().IsRegular():
			entryType = "file"
		}
		entries = append(entries, treeEntry{relativePath: filepath.ToSlash(relative), absolutePath: path, typeName: entryType, mode: info.Mode()})
		return nil
	})
	return entries, err
}

func readLimited(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	payload, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(payload)) > limit {
		return nil, fmt.Errorf("配置文件超过 %d 字节，需先在主机侧清理: %s", limit, path)
	}
	return payload, nil
}

func writeDigestField(writer io.Writer, value string) {
	_, _ = io.WriteString(writer, value)
	_, _ = writer.Write([]byte{0})
}
