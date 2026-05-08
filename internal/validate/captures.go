// SPDX-License-Identifier: AGPL-3.0-or-later

// Package validate provides cross-file bundle validation.
package validate

import (
	"errors"
	"os"
	"strconv"

	"github.com/nccurry/terminos/internal/bundle"
	"github.com/nccurry/terminos/internal/capture"
	"github.com/nccurry/terminos/internal/model"
	"github.com/nccurry/terminos/internal/redact"
)

// Captures validates that capture files exist and match their manifest steps.
func Captures(resolved bundle.Bundle, demo model.Demo) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	redactor, err := redact.New(demo.Redactions)
	if err != nil {
		return []model.Diagnostic{diagnostic("redaction_pattern_invalid", "redactions", err.Error())}
	}
	for i, step := range demo.Steps {
		path := capture.CapturePath(resolved, step)
		if _, err := os.Stat(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				diagnostics = append(diagnostics, diagnostic("capture_missing", stepPath(i, "capture"), "capture file is missing"))
				continue
			}
			diagnostics = append(diagnostics, diagnostic("capture_unreadable", stepPath(i, "capture"), "capture file is not readable"))
			continue
		}

		events, err := capture.ReadEvents(path)
		if err != nil {
			diagnostics = append(diagnostics, diagnostic("capture_invalid", stepPath(i, "capture"), err.Error()))
			continue
		}
		diagnostics = append(diagnostics, validateEvents(i, step, events, redactor)...)
	}
	return diagnostics
}

func validateEvents(index int, step model.Step, events []model.CaptureEvent, redactor redact.RuleSet) []model.Diagnostic {
	var diagnostics []model.Diagnostic
	expectedCommand := redactor.Apply(step.Command)
	var sawStart bool
	var sawExit bool
	var sawStepIDMismatch bool
	for _, event := range events {
		if event.StepID != "" && event.StepID != step.ID && !sawStepIDMismatch {
			sawStepIDMismatch = true
			diagnostics = append(diagnostics, diagnostic("capture_step_id_mismatch", stepPath(index, "capture"), "capture step_id does not match manifest step id"))
		}
		switch event.Type {
		case model.EventStart:
			sawStart = true
			if event.Command != "" && event.Command != expectedCommand {
				diagnostics = append(diagnostics, diagnostic("capture_command_mismatch", stepPath(index, "command"), "capture command does not match manifest command"))
			}
		case model.EventStdout, model.EventStderr:
		case model.EventExit:
			sawExit = true
			if event.ExitCode == nil {
				diagnostics = append(diagnostics, diagnostic("capture_exit_missing", stepPath(index, "expect_exit"), "capture exit event is missing exit_code"))
			} else if *event.ExitCode != step.ExpectExit {
				diagnostics = append(diagnostics, diagnostic("capture_exit_mismatch", stepPath(index, "expect_exit"), "capture exit_code does not match expect_exit"))
			}
		default:
			diagnostics = append(diagnostics, diagnostic("capture_event_type_invalid", stepPath(index, "capture"), "capture event type must be start, stdout, stderr, or exit"))
		}
	}
	if !sawStart {
		diagnostics = append(diagnostics, diagnostic("capture_start_missing", stepPath(index, "capture"), "capture is missing a start event"))
	}
	if !sawExit {
		diagnostics = append(diagnostics, diagnostic("capture_exit_missing", stepPath(index, "expect_exit"), "capture is missing an exit event"))
	}
	return diagnostics
}

func diagnostic(code string, path string, message string) model.Diagnostic {
	return model.Diagnostic{
		Severity: "error",
		Code:     code,
		Path:     path,
		Message:  message,
	}
}

func stepPath(index int, field string) string {
	return "steps[" + strconv.Itoa(index) + "]." + field
}
