package audit

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

type Event struct {
	Timestamp  time.Time `json:"timestamp"`
	Operation  string    `json:"operation"`
	InternalID string    `json:"internalId"`
	Revision   int       `json:"revision"`
	Digest     string    `json:"digest"`
	Stage      string    `json:"stage"`
	DurationMS int64     `json:"durationMs"`
	Result     string    `json:"result"`
}

type Logger struct {
	writer io.Writer
	mu     sync.Mutex
}

func NewLogger(writer io.Writer) *Logger {
	return &Logger{writer: writer}
}

func (logger *Logger) Record(event Event) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	logger.mu.Lock()
	defer logger.mu.Unlock()
	_, _ = logger.writer.Write(append(payload, '\n'))
}
