package storage

import "fmt"

var ErrAlreadyLocked = fmt.Errorf("应用实例已被锁定")

type CorruptFileError struct {
	Path string
	Err  error
}

func (err *CorruptFileError) Error() string {
	return fmt.Sprintf("受管 JSON 文件损坏 %s: %v", err.Path, err.Err)
}

func (err *CorruptFileError) Unwrap() error {
	return err.Err
}
