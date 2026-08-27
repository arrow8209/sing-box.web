package application

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"singbox-web/internal/paths"
	"singbox-web/internal/storage"
)

var transactionStagingName = regexp.MustCompile(`^config\.staging-[0-9a-f]{32}$`)

const failedCheckDirectoryName = "config.check-failed"

// preserveFailedCheckDirectory moves the last configuration rejected by
// sing-box to a stable administrator-only location. A newer failed check
// replaces the older copy, so runtime failures cannot accumulate unbounded
// configuration directories.
func preserveFailedCheckDirectory(layout paths.Layout, staging string) (string, error) {
	if err := validateSwapPaths(layout, staging); err != nil {
		return "", err
	}
	info, err := os.Lstat(staging)
	if err != nil {
		return "", fmt.Errorf("读取失败配置审核副本: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", fmt.Errorf("失败配置审核副本不是普通目录: %s", staging)
	}
	review := filepath.Join(layout.WorkDir, failedCheckDirectoryName)
	if filepath.Dir(review) != layout.WorkDir || filepath.Base(review) != failedCheckDirectoryName {
		return "", fmt.Errorf("失败配置审核路径无效")
	}
	if err := removeOwnedDirectory(review); err != nil {
		return "", fmt.Errorf("清理上一次失败配置审核副本: %w", err)
	}
	if err := renameAndSync(layout.WorkDir, filepath.Base(staging), filepath.Base(review)); err != nil {
		return "", fmt.Errorf("保留失败配置审核副本: %w", err)
	}
	return review, nil
}

// DirectoryTransaction retains the previous config until the caller has
// successfully started the core. Restore is safe to call repeatedly.
type DirectoryTransaction struct {
	layout        paths.Layout
	staging       string
	rollback      string
	hadOld        bool
	installedHash string
	active        bool
	keepRollback  bool
}

// BeginDirectorySwap atomically moves staging into the fixed config location.
// It re-digests the old directory after moving it to a private rollback path,
// which closes the time-of-check/time-of-replace window for host edits.
func BeginDirectorySwap(layout paths.Layout, staging, expectedCurrentDigest string) (*DirectoryTransaction, error) {
	if err := validateSwapPaths(layout, staging); err != nil {
		return nil, err
	}
	if err := storage.RejectSymlinkPath(layout.WorkDir); err != nil {
		return nil, err
	}
	stagingInfo, err := os.Lstat(staging)
	if err != nil {
		return nil, err
	}
	if stagingInfo.Mode()&os.ModeSymlink != 0 || !stagingInfo.IsDir() {
		return nil, fmt.Errorf("临时配置必须是普通目录: %s", staging)
	}
	installedHash, err := DigestDirectory(staging)
	if err != nil {
		return nil, err
	}

	transaction := &DirectoryTransaction{layout: layout, staging: staging, installedHash: installedHash, keepRollback: true}
	configInfo, err := os.Lstat(layout.ConfigDir)
	if errors.Is(err, os.ErrNotExist) {
		if expectedCurrentDigest != "" {
			return nil, fmt.Errorf("%w: 正式配置已不存在", ErrExternalDrift)
		}
		if err := renameAndSync(layout.WorkDir, filepath.Base(staging), filepath.Base(layout.ConfigDir)); err != nil {
			return nil, err
		}
		transaction.active = true
		return transaction, nil
	}
	if err != nil {
		return nil, err
	}
	if configInfo.Mode()&os.ModeSymlink != 0 && expectedCurrentDigest == "" {
		return nil, fmt.Errorf("%w: 正式配置为符号链接", ErrExternalDrift)
	}

	rollback, err := rollbackPath(layout)
	if err != nil {
		return nil, err
	}
	if err := renameAndSync(layout.WorkDir, filepath.Base(layout.ConfigDir), filepath.Base(rollback)); err != nil {
		return nil, err
	}
	transaction.rollback, transaction.hadOld = rollback, true
	currentDigest, digestErr := DigestDirectory(rollback)
	if digestErr != nil || currentDigest != expectedCurrentDigest {
		restoreErr := renameAndSync(layout.WorkDir, filepath.Base(rollback), filepath.Base(layout.ConfigDir))
		if digestErr != nil {
			if restoreErr != nil {
				return nil, fmt.Errorf("读取正式配置摘要失败: %w；恢复原配置失败: %v", digestErr, restoreErr)
			}
			return nil, digestErr
		}
		if restoreErr != nil {
			return nil, fmt.Errorf("%w: 摘要变更且恢复原配置失败: %v", ErrExternalDrift, restoreErr)
		}
		return nil, fmt.Errorf("%w: 正式配置在切换前发生变更", ErrExternalDrift)
	}
	if err := renameAndSync(layout.WorkDir, filepath.Base(staging), filepath.Base(layout.ConfigDir)); err != nil {
		restoreErr := renameAndSync(layout.WorkDir, filepath.Base(rollback), filepath.Base(layout.ConfigDir))
		if restoreErr != nil {
			return nil, fmt.Errorf("安装新配置失败: %w；恢复原配置失败: %v", err, restoreErr)
		}
		return nil, err
	}
	transaction.active = true
	return transaction, nil
}

// Restore removes the transaction-installed config and puts the retained old
// config back. If another actor changed the installed config, it is retained
// together with the rollback directory rather than overwritten.
func (transaction *DirectoryTransaction) Restore() error {
	if transaction == nil || !transaction.active {
		return nil
	}
	current, err := DigestDirectory(transaction.layout.ConfigDir)
	if err != nil {
		return err
	}
	if current != transaction.installedHash {
		transaction.keepRollback = true
		return fmt.Errorf("%w: 新配置已在主机侧变更，拒绝恢复", ErrExternalDrift)
	}
	if err := removeOwnedDirectory(transaction.layout.ConfigDir); err != nil {
		return err
	}
	if transaction.hadOld {
		if err := renameAndSync(transaction.layout.WorkDir, filepath.Base(transaction.rollback), filepath.Base(transaction.layout.ConfigDir)); err != nil {
			return err
		}
	}
	transaction.active = false
	transaction.keepRollback = false
	return syncDirectory(transaction.layout.WorkDir)
}

// Cleanup removes only transaction-private staging and rollback paths. It
// never touches the live config path and leaves rollback intact after a
// failed restore caused by newly observed external drift.
func (transaction *DirectoryTransaction) Cleanup() {
	if transaction == nil {
		return
	}
	if transaction.staging != "" {
		_ = removeOwnedDirectory(transaction.staging)
	}
	if transaction.rollback != "" && !transaction.keepRollback {
		_ = removeOwnedDirectory(transaction.rollback)
	}
	_ = syncDirectory(transaction.layout.WorkDir)
}

func validateSwapPaths(layout paths.Layout, staging string) error {
	workDir, err := filepath.Abs(layout.WorkDir)
	if err != nil {
		return err
	}
	configDir, err := filepath.Abs(layout.ConfigDir)
	if err != nil {
		return err
	}
	staging, err = filepath.Abs(staging)
	if err != nil {
		return err
	}
	if configDir != filepath.Join(workDir, "config") || filepath.Dir(staging) != workDir || !transactionStagingName.MatchString(filepath.Base(staging)) {
		return fmt.Errorf("临时配置路径无效")
	}
	return nil
}

func rollbackPath(layout paths.Layout) (string, error) {
	entropy := make([]byte, 16)
	if _, err := rand.Read(entropy); err != nil {
		return "", err
	}
	return filepath.Join(layout.WorkDir, "config.rollback-"+hex.EncodeToString(entropy)), nil
}

func renameAndSync(parent, oldName, newName string) error {
	if filepath.Base(oldName) != oldName || filepath.Base(newName) != newName {
		return fmt.Errorf("目录切换名称无效")
	}
	if err := os.Rename(filepath.Join(parent, oldName), filepath.Join(parent, newName)); err != nil {
		return err
	}
	return syncDirectory(parent)
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func removeOwnedDirectory(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return os.Remove(path)
	}
	if !info.IsDir() {
		return fmt.Errorf("事务路径不是目录: %s", path)
	}
	return os.RemoveAll(path)
}
