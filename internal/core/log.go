package core

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"singbox-web/internal/storage"
)

const truncatedLineSuffix = " [truncated]"

// BoundedLog is a shared stdout/stderr writer that retains only a bounded
// tail. It is deliberately not a general log-reading API.
type BoundedLog struct {
	path       string
	maxBytes   int
	maxLine    int
	mu         sync.Mutex
	file       *os.File
	output     io.Writer
	pending    []byte
	pendingCut bool
	closed     bool
}

func NewBoundedLog(path string, maxBytes, maxLine int) (*BoundedLog, error) {
	if path == "" || maxBytes <= 0 || maxLine <= len(truncatedLineSuffix)+1 || maxLine > maxBytes {
		return nil, fmt.Errorf("无效核心日志上限")
	}
	if err := storage.RejectSymlinkPath(filepath.Dir(path)); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if err := storage.RejectSymlinkPath(path); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &BoundedLog{path: path, maxBytes: maxBytes, maxLine: maxLine, file: file}, nil
}

// SetOutput mirrors newly completed, size-limited lines to output. Output
// failures do not interrupt the core's pipes or the durable log.
func (l *BoundedLog) SetOutput(output io.Writer) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output = output
}

func (l *BoundedLog) Write(payload []byte) (int, error) {
	if l == nil {
		return 0, io.ErrClosedPipe
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.file == nil {
		return 0, io.ErrClosedPipe
	}
	for _, character := range payload {
		if character == '\n' {
			if err := l.writePendingLocked(); err != nil {
				return 0, err
			}
			continue
		}
		if !l.pendingCut {
			// Reserve a newline and the truncation marker for an overlong line.
			if len(l.pending) < l.maxLine-len(truncatedLineSuffix)-1 {
				l.pending = append(l.pending, character)
			} else {
				l.pendingCut = true
			}
		}
	}
	return len(payload), nil
}

// Flush completes a pending partial line after a core exits. The log stays
// open so a later core restart can use the same writer.
func (l *BoundedLog) Flush() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.file == nil {
		return io.ErrClosedPipe
	}
	if len(l.pending) != 0 || l.pendingCut {
		return l.writePendingLocked()
	}
	return nil
}

func (l *BoundedLog) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	if len(l.pending) != 0 || l.pendingCut {
		if err := l.writePendingLocked(); err != nil {
			return err
		}
	}
	l.closed = true
	if err := l.file.Sync(); err != nil {
		_ = l.file.Close()
		return err
	}
	return l.file.Close()
}

// Recent returns at most maxBytes of the retained output. It is intended only
// for internal failure summaries, never for an HTTP log download endpoint.
func (l *BoundedLog) Recent(maxBytes int) string {
	if l == nil || maxBytes <= 0 {
		return ""
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		_ = l.file.Sync()
	}
	payload, err := os.ReadFile(l.path)
	if err != nil {
		return ""
	}
	if len(payload) > maxBytes {
		payload = payload[len(payload)-maxBytes:]
	}
	return string(payload)
}

func (l *BoundedLog) writePendingLocked() error {
	line := append([]byte(nil), l.pending...)
	if l.pendingCut {
		line = append(line, truncatedLineSuffix...)
	}
	line = append(line, '\n')
	l.pending = l.pending[:0]
	l.pendingCut = false
	if err := l.ensureCapacityLocked(len(line)); err != nil {
		return err
	}
	if _, err := l.file.Write(line); err != nil {
		return err
	}
	if l.output != nil {
		_, _ = l.output.Write(line)
	}
	return nil
}

func (l *BoundedLog) ensureCapacityLocked(next int) error {
	info, err := l.file.Stat()
	if err != nil {
		return err
	}
	if info.Size()+int64(next) <= int64(l.maxBytes) {
		return nil
	}
	if err := l.file.Sync(); err != nil {
		return err
	}
	payload, err := os.ReadFile(l.path)
	if err != nil {
		return err
	}
	keep := newestCompleteLines(payload, l.maxBytes/2)
	if len(keep)+next > l.maxBytes {
		// next is always no greater than maxLine, which NewBoundedLog requires
		// to be no greater than maxBytes. Keep less history if necessary.
		keep = newestCompleteLines(payload, l.maxBytes-next)
	}
	if err := l.file.Close(); err != nil {
		return err
	}
	if err := storage.WriteFile(l.path, keep, 0o600); err != nil {
		return err
	}
	file, err := os.OpenFile(l.path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	l.file = file
	return l.file.Sync()
}

func newestCompleteLines(payload []byte, limit int) []byte {
	if limit <= 0 || len(payload) == 0 {
		return nil
	}
	if len(payload) <= limit {
		return append([]byte(nil), payload...)
	}
	start := len(payload) - limit
	if index := bytes.IndexByte(payload[start:], '\n'); index >= 0 {
		start += index + 1
	} else {
		return nil
	}
	return append([]byte(nil), payload[start:]...)
}
