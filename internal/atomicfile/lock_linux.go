//go:build linux

package atomicfile

import (
	"os"

	"golang.org/x/sys/unix"
)

type Lock struct {
	file *os.File
}

func Acquire(path string) (*Lock, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		file.Close()
		return nil, err
	}
	return &Lock{file: file}, nil
}

func (lock *Lock) Close() error {
	if err := unix.Flock(int(lock.file.Fd()), unix.LOCK_UN); err != nil {
		lock.file.Close()
		return err
	}
	return lock.file.Close()
}
