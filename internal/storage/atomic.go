package storage

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
)

// ReadJSON reads a managed JSON document into a concrete, schema-bearing type.
// Map and interface type arguments are rejected because they cannot enforce an
// unknown-field boundary with json.Decoder.DisallowUnknownFields.
func ReadJSON[T any](path string) (T, error) {
	var value T
	if kind := reflect.TypeFor[T]().Kind(); kind == reflect.Map || kind == reflect.Interface {
		return value, fmt.Errorf("受管 JSON 必须读取到具名结构类型，不支持 %s", kind)
	}
	file, err := os.Open(path)
	if err != nil {
		return value, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, &CorruptFileError{Path: path, Err: err}
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("包含多个 JSON 值")
		}
		return value, &CorruptFileError{Path: path, Err: err}
	}
	return value, nil
}

func WriteJSON(path string, value any, perm fs.FileMode) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return WriteFile(path, append(payload, '\n'), perm)
}

func WriteFile(path string, payload []byte, perm fs.FileMode) error {
	directory := filepath.Dir(path)
	directoryFile, err := openDirectoryNoFollow(directory)
	if err != nil {
		return err
	}
	defer directoryFile.Close()
	name := filepath.Base(path)
	if name == "." || name == string(filepath.Separator) {
		return fmt.Errorf("无效文件名: %s", path)
	}
	tempName, file, err := createTempAt(int(directoryFile.Fd()), name, perm)
	if err != nil {
		return err
	}
	keep := false
	defer func() {
		_ = file.Close()
		if !keep {
			_ = syscall.Unlinkat(int(directoryFile.Fd()), tempName)
		}
	}()
	if _, err := file.Write(payload); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := syscall.Renameat(int(directoryFile.Fd()), tempName, int(directoryFile.Fd()), name); err != nil {
		return err
	}
	keep = true
	return directoryFile.Sync()
}

func openDirectoryNoFollow(path string) (*os.File, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	fd, err := syscall.Open(string(filepath.Separator), syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	for _, component := range strings.Split(strings.TrimPrefix(absolute, string(filepath.Separator)), string(filepath.Separator)) {
		if component == "" || component == "." || component == ".." {
			continue
		}
		next, openErr := syscall.Openat(fd, component, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		if errorsIsNotExist(openErr) {
			if mkdirErr := syscall.Mkdirat(fd, component, 0o700); mkdirErr != nil && !errorsIsExist(mkdirErr) {
				_ = syscall.Close(fd)
				return nil, mkdirErr
			}
			next, openErr = syscall.Openat(fd, component, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		}
		_ = syscall.Close(fd)
		if openErr != nil {
			return nil, fmt.Errorf("无法安全打开目录 %s: %w", path, openErr)
		}
		fd = next
	}
	return os.NewFile(uintptr(fd), absolute), nil
}

func createTempAt(directoryFD int, targetName string, perm fs.FileMode) (string, *os.File, error) {
	for range 100 {
		entropy := make([]byte, 16)
		if _, err := rand.Read(entropy); err != nil {
			return "", nil, err
		}
		name := "." + targetName + ".tmp-" + hex.EncodeToString(entropy)
		fd, err := syscall.Openat(directoryFD, name, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, uint32(perm.Perm()))
		if errorsIsExist(err) {
			continue
		}
		if err != nil {
			return "", nil, err
		}
		return name, os.NewFile(uintptr(fd), name), nil
	}
	return "", nil, fmt.Errorf("无法创建唯一临时文件")
}

func errorsIsNotExist(err error) bool {
	return err == syscall.ENOENT
}

func errorsIsExist(err error) bool {
	return err == syscall.EEXIST
}

func RejectSymlinkPath(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	volume := filepath.VolumeName(absolute)
	current := volume + string(filepath.Separator)
	for _, component := range strings.Split(strings.TrimPrefix(absolute, current), string(filepath.Separator)) {
		if component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("路径包含符号链接: %s", current)
		}
	}
	return nil
}
