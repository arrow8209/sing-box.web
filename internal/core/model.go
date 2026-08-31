// Package core owns the persistent and in-memory state of the managed
// sing-box process.
package core

import "time"

const persistentStateVersion = 1

// ProcessState is the observed process state. Persisted state never contains
// a PID, because a PID is not meaningful after the web process restarts.
type ProcessState string

const (
	ProcessNotInstalled ProcessState = "not_installed"
	ProcessStopped      ProcessState = "stopped"
	ProcessStarting     ProcessState = "starting"
	ProcessRunning      ProcessState = "running"
	ProcessStopping     ProcessState = "stopping"
	ProcessStartFailed  ProcessState = "start_failed"
	ProcessExited       ProcessState = "exited"
)

// PersistentState is the schema stored in data/core.json.
type PersistentState struct {
	Version               int        `json:"version"`
	InstalledVersion      string     `json:"installedVersion"`
	InstalledDigest       string     `json:"installedDigest"`
	CurrentProfileID      string     `json:"currentProfileId"`
	AppliedRevision       *uint64    `json:"appliedRevision"`
	ExpectedRunning       bool       `json:"expectedRunning"`
	ConfigDirectoryDigest string     `json:"configDirectoryDigest"`
	LastStartError        string     `json:"lastStartError"`
	LastStartedAt         *time.Time `json:"lastStartedAt"`
}

// OperationKind identifies a globally serialized lifecycle operation.
type OperationKind string

const (
	OperationValidate    OperationKind = "validate"
	OperationApply       OperationKind = "apply"
	OperationCoreStart   OperationKind = "core_start"
	OperationCoreRestart OperationKind = "core_restart"
	OperationCoreUpdate  OperationKind = "core_update"
	OperationCoreStop    OperationKind = "core_stop"
)

// OperationStatus is safe to expose in the core status API.
type OperationStatus struct {
	Active    bool       `json:"active"`
	Kind      string     `json:"kind,omitempty"`
	Stage     string     `json:"stage,omitempty"`
	StartedAt *time.Time `json:"startedAt,omitempty"`
	Error     string     `json:"error,omitempty"`
}

// Status combines durable metadata with process facts maintained in memory.
type Status struct {
	InstalledVersion      string          `json:"installedVersion"`
	InstalledDigest       string          `json:"installedDigest"`
	State                 ProcessState    `json:"state"`
	PID                   int             `json:"pid"`
	CurrentProfileID      string          `json:"currentProfileId"`
	AppliedRevision       *uint64         `json:"appliedRevision"`
	ExpectedRunning       bool            `json:"expectedRunning"`
	ConfigDirectoryDigest string          `json:"configDirectoryDigest"`
	StartedAt             *time.Time      `json:"startedAt"`
	LastError             string          `json:"lastError"`
	PendingProfileID      string          `json:"pendingProfileId,omitempty"`
	PendingRevision       *uint64         `json:"pendingRevision,omitempty"`
	Operation             OperationStatus `json:"operation"`
}

func (s Status) Running() bool {
	return s.State == ProcessRunning || s.State == ProcessStarting
}
