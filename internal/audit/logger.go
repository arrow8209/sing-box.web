package audit

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

// Event deliberately has no free-form payload or error field.
type Event struct {
	Operation  string `json:"operation"`
	ProfileID  string `json:"profileId,omitempty"`
	Revision   uint64 `json:"revision,omitempty"`
	Digest     string `json:"digest,omitempty"`
	DurationMS int64  `json:"durationMs,omitempty"`
	Result     string `json:"result"`
	RequestID  string `json:"requestId,omitempty"`
}

type entry struct {
	Timestamp time.Time `json:"timestamp"`
	Event
}

type Logger struct {
	mu     sync.Mutex
	writer io.Writer
	now    func() time.Time
}

func New(writer io.Writer, now func() time.Time) *Logger {
	if writer == nil {
		writer = io.Discard
	}
	if now == nil {
		now = time.Now
	}
	return &Logger{writer: writer, now: now}
}

func (logger *Logger) Record(event Event) {
	if logger == nil {
		return
	}
	logger.mu.Lock()
	defer logger.mu.Unlock()
	_ = json.NewEncoder(logger.writer).Encode(entry{Timestamp: logger.now().UTC(), Event: event})
}
