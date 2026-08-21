package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var transactionDirectoryName = regexp.MustCompile(`^config\.(?:staging|rollback)-[0-9a-f]{32}$`)

func ListTransactionDirs(workDir string) ([]string, error) {
	if err := RejectSymlinkPath(workDir); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0)
	for _, entry := range entries {
		if !transactionDirectoryName.MatchString(entry.Name()) {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("事务目录不能是符号链接: %s", filepath.Join(workDir, entry.Name()))
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			paths = append(paths, filepath.Join(workDir, entry.Name()))
		}
	}
	return paths, nil
}
