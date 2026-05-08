// SPDX-License-Identifier: AGPL-3.0-or-later

package model

// Capture event types stored in captures/*.jsonl.
const (
	EventStart  = "start"
	EventStdout = "stdout"
	EventStderr = "stderr"
	EventExit   = "exit"
)

// CaptureEvent is one line in a capture JSONL stream.
type CaptureEvent struct {
	Type     string `json:"type"`
	TimeMS   int64  `json:"time_ms"`
	StepID   string `json:"step_id,omitempty"`
	Command  string `json:"command,omitempty"`
	CWD      string `json:"cwd,omitempty"`
	Shell    string `json:"shell,omitempty"`
	Data     string `json:"data,omitempty"`
	ExitCode *int   `json:"exit_code,omitempty"`
}

// NewExitEvent returns an exit event with a present exit code, including zero.
func NewExitEvent(stepID string, timeMS int64, exitCode int) CaptureEvent {
	return CaptureEvent{
		Type:     EventExit,
		TimeMS:   timeMS,
		StepID:   stepID,
		ExitCode: &exitCode,
	}
}
