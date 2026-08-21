package rules

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"singbox-web/internal/paths"
)

// RecoverRuleTransactions resolves an interrupted replacement before any
// scheduler can touch a rule file.  A rollback is trusted only when its digest
// agrees with the persisted resource state; ambiguous files stop startup.
func RecoverRuleTransactions(store *Store, layout paths.Layout) error {
	if store == nil {
		return fmt.Errorf("规则资源存储不能为空")
	}
	resources, err := store.list()
	if err != nil {
		return err
	}
	for _, resource := range resources {
		fixed, _, err := layout.RuleSetPath(resource.ID, extension(resource.Format))
		if err != nil {
			return err
		}
		base := filepath.Base(fixed)
		for _, candidate := range []string{filepath.Join(filepath.Dir(fixed), "."+base+".download-*")} {
			matches, globErr := filepath.Glob(candidate)
			if globErr != nil {
				return globErr
			}
			for _, match := range matches {
				if err := removeRegularFile(match); err != nil {
					return err
				}
			}
		}
		pattern := regexp.MustCompile(`^\.` + regexp.QuoteMeta(base) + `\.rollback-[0-9a-f]{32}$`)
		entries, err := os.ReadDir(filepath.Dir(fixed))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		var rollbacks []string
		for _, entry := range entries {
			if pattern.MatchString(entry.Name()) {
				rollbacks = append(rollbacks, filepath.Join(filepath.Dir(fixed), entry.Name()))
			}
		}
		if len(rollbacks) > 1 {
			return fmt.Errorf("规则资源 %s 存在多个未完成替换", resource.ID)
		}
		if len(rollbacks) == 0 {
			if resource.LastDigest == "" {
				if _, err := os.Lstat(fixed); err == nil {
					return fmt.Errorf("规则资源 %s 存在未提交规则文件", resource.ID)
				} else if !os.IsNotExist(err) {
					return err
				}
			} else if exists, digest, err := fileDigest(fixed); err != nil {
				return err
			} else if !exists || digest != resource.LastDigest {
				return fmt.Errorf("规则资源 %s 的固定文件与状态摘要不一致", resource.ID)
			}
			continue
		}
		rollback := rollbacks[0]
		if err := rejectSymlink(rollback); err != nil {
			return err
		}
		_, fixedDigest, fixedErr := fileDigest(fixed)
		if fixedErr != nil {
			return fixedErr
		}
		_, rollbackDigest, err := fileDigest(rollback)
		if err != nil {
			return err
		}
		switch {
		case resource.LastDigest != "" && fixedDigest == resource.LastDigest:
			if err := os.Remove(rollback); err != nil {
				return err
			}
		case resource.LastDigest != "" && rollbackDigest == resource.LastDigest:
			if err := os.Remove(fixed); err != nil && !os.IsNotExist(err) {
				return err
			}
			if err := os.Rename(rollback, fixed); err != nil {
				return err
			}
		default:
			return fmt.Errorf("规则资源 %s 的替换恢复摘要不一致", resource.ID)
		}
	}
	return nil
}

func removeRegularFile(path string) error {
	if err := rejectSymlink(path); err != nil {
		return err
	}
	return os.Remove(path)
}
func rejectSymlink(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("规则恢复文件不能是符号链接: %s", path)
	}
	return nil
}
func fileDigest(path string) (bool, string, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, "", fmt.Errorf("规则文件不能是符号链接: %s", path)
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return false, "", err
	}
	sum := sha256.Sum256(payload)
	return true, fmt.Sprintf("%x", sum), nil
}
