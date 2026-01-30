package process

import "time"

type Status string

const (
	StatusStopped  Status = "stopped"
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusFailed   Status = "failed"
	StatusRetrying Status = "retrying"
	StatusStopping Status = "stopping"
)

type ProcessState struct {
	Name        string    `json:"name"`
	Status      Status    `json:"status"`
	PID         int       `json:"pid,omitempty"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	StoppedAt   time.Time `json:"stopped_at,omitempty"`
	RetryCount  int       `json:"retry_count"`
	NextRetryAt time.Time `json:"next_retry_at,omitempty"`
	LastError   string    `json:"last_error,omitempty"`
	ExitCode    int       `json:"exit_code,omitempty"`
}

func (s ProcessState) Uptime() time.Duration {
	if s.StartedAt.IsZero() {
		return 0
	}
	if s.Status == StatusRunning || s.Status == StatusStopping {
		return time.Since(s.StartedAt)
	}
	if !s.StoppedAt.IsZero() {
		return s.StoppedAt.Sub(s.StartedAt)
	}
	return 0
}
