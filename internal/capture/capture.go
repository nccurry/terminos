// SPDX-License-Identifier: AGPL-3.0-or-later

// Package capture executes demo steps and stores JSONL capture streams.
package capture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/nccurry/terminos/internal/apperror"
	"github.com/nccurry/terminos/internal/bundle"
	"github.com/nccurry/terminos/internal/model"
	"github.com/nccurry/terminos/internal/redact"
)

// StepResult describes one captured step.
type StepResult struct {
	StepID     string `json:"step_id" yaml:"step_id"`
	Capture    string `json:"capture" yaml:"capture"`
	ExitCode   int    `json:"exit_code" yaml:"exit_code"`
	ExpectExit int    `json:"expect_exit" yaml:"expect_exit"`
	DurationMS int64  `json:"duration_ms" yaml:"duration_ms"`
}

// Result describes a capture run.
type Result struct {
	BundlePath string       `json:"bundle_path" yaml:"bundle_path"`
	Captures   []StepResult `json:"captures" yaml:"captures"`
}

// Run executes all steps and writes their capture files.
func Run(ctx context.Context, resolved bundle.Bundle, demo model.Demo) (Result, error) {
	redactor, err := redact.New(demo.Redactions)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		BundlePath: resolved.Path,
		Captures:   []StepResult{},
	}
	for _, step := range demo.Steps {
		stepResult, err := runStep(ctx, resolved, demo, step, redactor)
		result.Captures = append(result.Captures, stepResult)
		if err != nil {
			if metadataErr := bundle.WriteMetadata(resolved); metadataErr != nil {
				return result, metadataErr
			}
			return result, err
		}
	}
	if err := bundle.WriteMetadata(resolved); err != nil {
		return result, err
	}
	return result, nil
}

// ReadEvents reads a capture JSONL file.
func ReadEvents(path string) ([]model.CaptureEvent, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, apperror.Wrapf(apperror.CodeFilesystem, err, "read capture %s", path)
	}
	defer func() {
		_ = file.Close()
	}()

	decoder := json.NewDecoder(file)
	events := []model.CaptureEvent{}
	for {
		var event model.CaptureEvent
		if err := decoder.Decode(&event); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, apperror.Wrapf(apperror.CodeUsage, err, "parse capture %s", path)
		}
		events = append(events, event)
	}
	return events, nil
}

// CapturePath resolves a step capture file path inside a bundle.
func CapturePath(resolved bundle.Bundle, step model.Step) string {
	capturePath := model.CapturePath(step)
	if filepath.IsAbs(capturePath) {
		return capturePath
	}
	return filepath.Join(resolved.Path, capturePath)
}

func runStep(ctx context.Context, resolved bundle.Bundle, demo model.Demo, step model.Step, redactor redact.RuleSet) (StepResult, error) {
	capturePath := CapturePath(resolved, step)
	if err := os.MkdirAll(filepath.Dir(capturePath), 0o755); err != nil {
		return StepResult{}, apperror.Wrapf(apperror.CodeFilesystem, err, "create capture directory %s", filepath.Dir(capturePath))
	}

	start := time.Now()
	runCtx, cancel := context.WithTimeout(ctx, demo.Defaults.Timeout.ToDuration())
	defer cancel()

	command := exec.CommandContext(runCtx, demo.Defaults.Shell, "-c", step.Command)
	command.Dir = resolveCWD(resolved.Path, demo.Defaults.CWD)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	exitCode := 0
	runErr := command.Run()
	durationMS := time.Since(start).Milliseconds()
	if runCtx.Err() != nil {
		exitCode = 124
	} else if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	events := []model.CaptureEvent{
		{
			Type:    model.EventStart,
			TimeMS:  0,
			StepID:  step.ID,
			Command: redactor.Apply(step.Command),
			CWD:     redactor.Apply(command.Dir),
			Shell:   demo.Defaults.Shell,
		},
	}
	if stdout.Len() > 0 {
		events = append(events, model.CaptureEvent{
			Type:   model.EventStdout,
			TimeMS: durationMS,
			StepID: step.ID,
			Data:   redactor.Apply(stdout.String()),
		})
	}
	if stderr.Len() > 0 {
		events = append(events, model.CaptureEvent{
			Type:   model.EventStderr,
			TimeMS: durationMS,
			StepID: step.ID,
			Data:   redactor.Apply(stderr.String()),
		})
	}
	events = append(events, model.NewExitEvent(step.ID, durationMS, exitCode))

	stepResult := StepResult{
		StepID:     step.ID,
		Capture:    model.CapturePath(step),
		ExitCode:   exitCode,
		ExpectExit: step.ExpectExit,
		DurationMS: durationMS,
	}
	if err := WriteEvents(capturePath, events); err != nil {
		return stepResult, err
	}
	if runCtx.Err() != nil {
		return stepResult, apperror.Wrapf(apperror.CodeTimeout, runCtx.Err(), "step %q timed out", step.ID)
	}
	if runErr != nil && exitCode != step.ExpectExit {
		return stepResult, apperror.Newf(apperror.CodeCapturedCommand, "step %q exited %d; expected %d", step.ID, exitCode, step.ExpectExit)
	}
	if exitCode != step.ExpectExit {
		return stepResult, apperror.Newf(apperror.CodeCapturedCommand, "step %q exited %d; expected %d", step.ID, exitCode, step.ExpectExit)
	}
	return stepResult, nil
}

// WriteEvents writes capture events as JSONL.
func WriteEvents(path string, events []model.CaptureEvent) error {
	file, err := os.Create(path)
	if err != nil {
		return apperror.Wrapf(apperror.CodeFilesystem, err, "write capture %s", path)
	}

	encoder := json.NewEncoder(file)
	for _, event := range events {
		if err := encoder.Encode(event); err != nil {
			_ = file.Close()
			return apperror.Wrapf(apperror.CodeFilesystem, err, "write capture %s", path)
		}
	}
	if err := file.Close(); err != nil {
		return apperror.Wrapf(apperror.CodeFilesystem, err, "close capture %s", path)
	}
	return nil
}

func resolveCWD(bundlePath string, cwd string) string {
	if cwd == "" {
		cwd = "."
	}
	if filepath.IsAbs(cwd) {
		return cwd
	}
	return filepath.Join(bundlePath, cwd)
}
