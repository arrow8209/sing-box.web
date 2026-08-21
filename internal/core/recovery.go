package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"singbox-web/internal/paths"
)

var versionLine = regexp.MustCompile(`(?m)^sing-box version ([^\s]+)\s*$`)

// RecoverInstallation reconciles the atomically replaced program with its
// durable record. It deliberately refuses the only unsafe state: two files
// whose digests cannot be tied to the committed record.
func RecoverInstallation(layout paths.Layout, store *Store) (PersistentState, error) {
	state, err := store.Load()
	if err != nil {
		return PersistentState{}, err
	}
	primary, primaryErr := fileDigest(layout.CoreBinary)
	backup, backupErr := fileDigest(layout.CoreBackup)
	primaryOK, backupOK := primaryErr == nil, backupErr == nil
	if primaryErr != nil && !errors.Is(primaryErr, os.ErrNotExist) {
		return state, primaryErr
	}
	if backupErr != nil && !errors.Is(backupErr, os.ErrNotExist) {
		return state, backupErr
	}
	switch {
	case !primaryOK && backupOK:
		if err := os.Rename(layout.CoreBackup, layout.CoreBinary); err != nil {
			return state, err
		}
		if err := syncBinDirectory(layout.BinDir); err != nil {
			return state, err
		}
		primary, primaryOK = backup, true
	case primaryOK && backupOK && backup == state.InstalledDigest && primary != state.InstalledDigest:
		if err := os.Remove(layout.CoreBinary); err != nil {
			return state, err
		}
		if err := os.Rename(layout.CoreBackup, layout.CoreBinary); err != nil {
			return state, err
		}
		if err := syncBinDirectory(layout.BinDir); err != nil {
			return state, err
		}
		primary = backup
	case primaryOK && backupOK && primary != state.InstalledDigest && backup != state.InstalledDigest:
		return state, fmt.Errorf("核心程序恢复歧义：正式程序和备份均不匹配持久摘要")
	}
	if !primaryOK {
		return state, nil
	}
	version, err := calibratedVersion(layout)
	if err != nil {
		return state, fmt.Errorf("已安装核心不可用: %w", err)
	}
	if err := store.RecordInstalled(version, primary); err != nil {
		return state, err
	}
	return store.Load()
}

func syncBinDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func fileDigest(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("核心程序不是普通文件: %s", path)
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(payload)), nil
}

func calibratedVersion(layout paths.Layout) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.FromSlash("data/bin/sing-box"), "version")
	cmd.Dir = layout.WorkDir
	cmd.Env = allowedEnvironment()
	var output limitedBuffer
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if output.exceeded {
		return "", fmt.Errorf("核心 version 输出超过 64 KiB")
	}
	if err != nil {
		return "", fmt.Errorf("核心 version 执行失败: %w", err)
	}
	match := versionLine.FindSubmatch(output.Bytes())
	if len(match) != 2 {
		return "", fmt.Errorf("核心 version 输出无效")
	}
	return strings.TrimSpace(string(match[1])), nil
}

type limitedBuffer struct {
	bytes.Buffer
	exceeded bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	const limit = 64 << 10
	if b.Len()+len(p) > limit {
		remaining := limit - b.Len()
		if remaining > 0 {
			_, _ = b.Buffer.Write(p[:remaining])
		}
		b.exceeded = true
		return len(p), nil
	}
	return b.Buffer.Write(p)
}
