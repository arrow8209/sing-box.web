package application

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"singbox-web/internal/paths"
)

var transactionRollbackName = regexp.MustCompile(`^config\.rollback-[0-9a-f]{32}$`)

// RecoverConfigTransactions resolves rollback directories left by the former
// application transaction. config.staging-<profileID> directories are durable
// check outputs and must never be treated as transaction debris.
func RecoverConfigTransactions(layout paths.Layout, persistedDigest string) error {
	entries, err := os.ReadDir(layout.WorkDir)
	if err != nil {
		return err
	}
	var rollback string
	for _, entry := range entries {
		name := entry.Name()
		if transactionRollbackName.MatchString(name) {
			if rollback != "" {
				return fmt.Errorf("配置恢复歧义：存在多个 rollback 目录")
			}
			rollback = filepath.Join(layout.WorkDir, name)
		}
	}
	if rollback == "" {
		return nil
	}
	if err := requireRecoveryDirectory(rollback); err != nil {
		return err
	}
	rollbackDigest, err := DigestDirectory(rollback)
	if err != nil {
		return err
	}
	currentDigest, err := DigestDirectory(layout.ConfigDir)
	if err != nil {
		return err
	}
	switch {
	case rollbackDigest == persistedDigest:
		if currentDigest != "" {
			if err := removeOwnedDirectory(layout.ConfigDir); err != nil {
				return err
			}
		}
		if err := renameAndSync(layout.WorkDir, filepath.Base(rollback), filepath.Base(layout.ConfigDir)); err != nil {
			return err
		}
	case currentDigest == persistedDigest:
		if err := removeOwnedDirectory(rollback); err != nil {
			return err
		}
	default:
		return fmt.Errorf("配置恢复歧义：正式目录和 rollback 均不匹配持久摘要")
	}
	return syncDirectory(layout.WorkDir)
}

func requireRecoveryDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("配置恢复路径不是受管普通目录: %s", path)
	}
	return nil
}
