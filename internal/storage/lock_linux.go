//go:build linux

package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type InstanceLock struct {
	file *os.File
}

func AcquireInstanceLock(path string) (*InstanceLock, error) {
	directory := filepath.Dir(path)
	directoryFile, err := openDirectoryNoFollow(directory)
	if err != nil {
		return nil, err
	}
	defer directoryFile.Close()
	name := filepath.Base(path)
	fd, err := syscall.Openat(int(directoryFile.Fd()), name, syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("无法安全打开实例锁 %s: %w", path, err)
	}
	file := os.NewFile(uintptr(fd), name)
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, ErrAlreadyLocked
		}
		return nil, err
	}
	return &InstanceLock{file: file}, nil
}

func (lock *InstanceLock) Close() error {
	if lock == nil || lock.file == nil {
		return nil
	}
	file := lock.file
	lock.file = nil
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_UN); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
