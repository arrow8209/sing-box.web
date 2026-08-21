package configgen

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteDirectory creates dir exactly once and writes the generated module
// files into it. It never updates an existing directory, including the live
// config directory. Any partial new directory is removed on failure.
func (g *Generator) WriteDirectory(ctx context.Context, output Output, dir string) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if dir == "" || filepath.Base(dir) == "." {
		return fmt.Errorf("配置目录无效")
	}
	if err := validateOutputFiles(output.Files); err != nil {
		return err
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("配置目录已经存在")
		}
		return err
	}
	created := true
	defer func() {
		if err != nil && created {
			_ = os.RemoveAll(dir)
		}
	}()
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	for _, file := range output.Files {
		if err := ctx.Err(); err != nil {
			return err
		}
		path := filepath.Join(dir, file.Name)
		handle, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return err
		}
		if _, err := handle.Write(file.Content); err != nil {
			_ = handle.Close()
			return err
		}
		if err := handle.Sync(); err != nil {
			_ = handle.Close()
			return err
		}
		if err := handle.Close(); err != nil {
			return err
		}
		if err := os.Chmod(path, 0o600); err != nil {
			return err
		}
	}
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return err
	}
	created = false
	return nil
}

func validateOutputFiles(files []File) error {
	seen := make(map[string]struct{}, len(files))
	previous := ""
	for _, file := range files {
		if !knownFileName(file.Name) || filepath.Base(file.Name) != file.Name || strings.Contains(file.Name, string(filepath.Separator)) {
			return fmt.Errorf("配置文件名不受支持: %q", file.Name)
		}
		if _, exists := seen[file.Name]; exists {
			return fmt.Errorf("配置文件名重复: %q", file.Name)
		}
		if previous != "" && file.Name <= previous {
			return fmt.Errorf("配置文件顺序不稳定")
		}
		seen[file.Name] = struct{}{}
		previous = file.Name
	}
	return nil
}
