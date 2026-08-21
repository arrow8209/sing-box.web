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

// FileReplacement is a short lived, recoverable installation of a downloaded
// file.  Call Commit only after the metadata pointing at the new file has been
// durably written; otherwise call Restore.
type FileReplacement struct {
	target   string
	rollback string
	hadOld   bool
	done     bool
}

// BeginFileReplacement atomically installs tempPath at target.  When target
// already exists it is retained under a private rollback name until Commit.
// Both paths must be in the same directory, so the final rename cannot cross
// filesystems.
func BeginFileReplacement(tempPath, target string, perm fs.FileMode) (*FileReplacement, error) {
	if filepath.Dir(tempPath) != filepath.Dir(target) {
		return nil, fmt.Errorf("临时规则文件必须与目标文件位于同一目录")
	}
	if err := RejectSymlinkPath(filepath.Dir(target)); err != nil {
		return nil, err
	}
	directory, err := openDirectoryNoFollow(filepath.Dir(target))
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	targetName, tempName := filepath.Base(target), filepath.Base(tempPath)
	if targetName == "." || tempName == "." {
		return nil, fmt.Errorf("无效规则文件名")
	}
	if err := os.Chmod(tempPath, perm); err != nil {
		return nil, err
	}
	r := &FileReplacement{target: target}
	if _, err := os.Lstat(target); err == nil {
		entropy := make([]byte, 16)
		if _, err := rand.Read(entropy); err != nil {
			return nil, err
		}
		r.rollback = filepath.Join(filepath.Dir(target), "."+targetName+".rollback-"+hex.EncodeToString(entropy))
		if err := syscall.Renameat(int(directory.Fd()), targetName, int(directory.Fd()), filepath.Base(r.rollback)); err != nil {
			return nil, err
		}
		r.hadOld = true
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if err := syscall.Renameat(int(directory.Fd()), tempName, int(directory.Fd()), targetName); err != nil {
		if r.hadOld {
			_ = syscall.Renameat(int(directory.Fd()), filepath.Base(r.rollback), int(directory.Fd()), targetName)
		}
		return nil, err
	}
	if err := directory.Sync(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *FileReplacement) Commit() error {
	if r == nil || r.done {
		return nil
	}
	if r.hadOld {
		if err := os.Remove(r.rollback); err != nil {
			return err
		}
		if dir, err := openDirectoryNoFollow(filepath.Dir(r.target)); err != nil {
			return err
		} else {
			defer dir.Close()
			if err := dir.Sync(); err != nil {
				return err
			}
		}
	}
	r.done = true
	return nil
}

func (r *FileReplacement) Restore() error {
	if r == nil || r.done {
		return nil
	}
	if err := os.Remove(r.target); err != nil && !os.IsNotExist(err) {
		return err
	}
	if r.hadOld {
		if err := os.Rename(r.rollback, r.target); err != nil {
			return err
		}
	}
	dir, err := openDirectoryNoFollow(filepath.Dir(r.target))
	if err != nil {
		return err
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil {
		return err
	}
	r.done = true
	return nil
}

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

// ValidateJSON verifies that path contains exactly one syntactically valid
// JSON value without imposing a schema owned by a later implementation phase.
func ValidateJSON(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	var value json.RawMessage
	if err := decoder.Decode(&value); err != nil {
		return &CorruptFileError{Path: path, Err: err}
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("包含多个 JSON 值")
		}
		return &CorruptFileError{Path: path, Err: err}
	}
	return nil
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
