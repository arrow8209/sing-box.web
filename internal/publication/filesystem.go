package publication

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"singbox-web/internal/configdoc"
)

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func ensureDirectory(path string, mode fs.FileMode, groupID *int) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(path, mode); err != nil {
			return err
		}
		if err := os.Chmod(path, mode); err != nil {
			return err
		}
		if groupID != nil {
			if err := os.Chown(path, -1, *groupID); err != nil {
				return err
			}
		}
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is not a real directory", path)
	}
	return nil
}

func writeModules(path string, files []configdoc.ModuleFile, mode fs.FileMode, groupID *int) error {
	for _, module := range files {
		if filepath.Base(module.Name) != module.Name || !strings.HasSuffix(module.Name, ".json") {
			return fmt.Errorf("unsafe module name %q", module.Name)
		}
		name := filepath.Join(path, module.Name)
		file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err != nil {
			return err
		}
		_, writeErr := file.Write(module.Content)
		if writeErr == nil {
			writeErr = file.Chmod(mode)
		}
		if writeErr == nil && groupID != nil {
			writeErr = file.Chown(-1, *groupID)
		}
		if writeErr == nil {
			writeErr = file.Sync()
		}
		closeErr := file.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return syncDirectory(path)
}

func readModules(path string) ([]configdoc.ModuleFile, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("generation is not a real directory")
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	files := make([]configdoc.ModuleFile, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		entryInfo, err := os.Lstat(filepath.Join(path, name))
		if err != nil {
			return nil, err
		}
		if !entryInfo.Mode().IsRegular() || !strings.HasSuffix(name, ".json") {
			return nil, fmt.Errorf("unexpected generation entry %q", name)
		}
		content, err := os.ReadFile(filepath.Join(path, name))
		if err != nil {
			return nil, err
		}
		files = append(files, configdoc.ModuleFile{Name: name, Content: content})
	}
	return files, nil
}

func parsePublicationID(name string) (uint64, string, error) {
	parts := strings.Split(name, "-")
	if len(parts) != 2 || len(parts[1]) != 12 || strings.ToLower(parts[1]) != parts[1] {
		return 0, "", fmt.Errorf("invalid publication ID %q", name)
	}
	revision, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil || revision == 0 {
		return 0, "", fmt.Errorf("invalid publication ID %q", name)
	}
	for _, char := range parts[1] {
		if !strings.ContainsRune("0123456789abcdef", char) {
			return 0, "", fmt.Errorf("invalid publication ID %q", name)
		}
	}
	return revision, parts[1], nil
}
