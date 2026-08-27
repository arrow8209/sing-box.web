package configgen

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var persistentStagingName = regexp.MustCompile(`^config\.staging-[0-9a-f]{32}$`)
var stagingReplacementName = regexp.MustCompile(`^(config\.staging-[0-9a-f]{32})\.(?:preparing|previous)$`)

// RecoverStagingDirectories resolves interrupted candidate refreshes without
// deleting any canonical config.staging-<profileID> directory.
func RecoverStagingDirectories(workDir string) error {
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return err
	}
	targets := make(map[string]struct{})
	for _, entry := range entries {
		name := entry.Name()
		switch {
		case persistentStagingName.MatchString(name):
			targets[name] = struct{}{}
		case stagingReplacementName.MatchString(name):
			targets[stagingReplacementName.FindStringSubmatch(name)[1]] = struct{}{}
		}
	}
	for name := range targets {
		target := filepath.Join(workDir, name)
		if err := recoverStagingReplacement(target, target+".preparing", target+".previous"); err != nil {
			return err
		}
	}
	return nil
}

// ReplaceStagingDirectory atomically refreshes a profile's persistent check
// directory. The canonical directory remains after both successful and failed
// core checks; .preparing and .previous only protect an in-progress refresh.
func (g *Generator) ReplaceStagingDirectory(ctx context.Context, output Output, target string) error {
	if !persistentStagingName.MatchString(filepath.Base(target)) || filepath.Dir(target) == "." {
		return fmt.Errorf("档案检查目录无效")
	}
	preparing, previous := target+".preparing", target+".previous"
	if err := recoverStagingReplacement(target, preparing, previous); err != nil {
		return err
	}
	if err := g.WriteDirectory(ctx, output, preparing); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		_ = removeStagingDirectory(preparing)
		return err
	}

	hadPrevious := false
	if _, err := requireStagingDirectory(target); err == nil {
		if err := renameStagingDirectory(target, previous); err != nil {
			_ = removeStagingDirectory(preparing)
			return err
		}
		hadPrevious = true
	} else if !errors.Is(err, os.ErrNotExist) {
		_ = removeStagingDirectory(preparing)
		return err
	}
	if err := renameStagingDirectory(preparing, target); err != nil {
		var restore error
		if hadPrevious {
			restore = renameStagingDirectory(previous, target)
		}
		return errors.Join(err, restore)
	}
	if hadPrevious {
		if err := removeStagingDirectory(previous); err != nil {
			return err
		}
	}
	return syncStagingParent(target)
}

func recoverStagingReplacement(target, preparing, previous string) error {
	_, targetErr := requireStagingDirectory(target)
	_, previousErr := requireStagingDirectory(previous)
	switch {
	case targetErr == nil && previousErr == nil:
		if err := removeStagingDirectory(previous); err != nil {
			return err
		}
	case errors.Is(targetErr, os.ErrNotExist) && previousErr == nil:
		if err := renameStagingDirectory(previous, target); err != nil {
			return err
		}
	case targetErr != nil && !errors.Is(targetErr, os.ErrNotExist):
		return targetErr
	case previousErr != nil && !errors.Is(previousErr, os.ErrNotExist):
		return previousErr
	}
	if err := removeStagingDirectory(preparing); err != nil {
		return err
	}
	return syncStagingParent(target)
}

func requireStagingDirectory(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("档案检查路径不是普通目录: %s", filepath.Base(path))
	}
	return info, nil
}

func removeStagingDirectory(path string) error {
	if _, err := requireStagingDirectory(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return os.RemoveAll(path)
}

func renameStagingDirectory(oldPath, newPath string) error {
	if filepath.Dir(oldPath) != filepath.Dir(newPath) || filepath.Base(oldPath) == "." || filepath.Base(newPath) == "." {
		return fmt.Errorf("档案检查目录切换路径无效")
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return err
	}
	return syncStagingParent(oldPath)
}

func syncStagingParent(path string) error {
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
