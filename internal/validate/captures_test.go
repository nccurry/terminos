// SPDX-License-Identifier: AGPL-3.0-or-later

package validate

import (
	"context"
	"os"
	"testing"

	"github.com/nccurry/terminos/internal/bundle"
	"github.com/nccurry/terminos/internal/capture"
	"github.com/nccurry/terminos/internal/model"
)

func TestCapturesReportsMissingAndMismatchedFiles(t *testing.T) {
	t.Parallel()

	resolved := bundle.Resolve(t.TempDir())
	demo := model.DefaultDemo("validate-captures")
	demo.Steps = []model.Step{
		{ID: "missing", Command: "echo missing", ExpectExit: 0},
	}

	diagnostics := Captures(resolved, demo)
	if !hasDiagnostic(diagnostics, "capture_missing") {
		t.Fatalf("expected missing capture diagnostic, got %#v", diagnostics)
	}

	demo.Steps = []model.Step{{ID: "bad-exit", Command: "exit 7", ExpectExit: 7}}
	if _, err := capture.Run(context.Background(), resolved, demo); err != nil {
		t.Fatalf("capture expected failure: %v", err)
	}
	demo.Steps[0].ExpectExit = 0
	diagnostics = Captures(resolved, demo)
	if !hasDiagnostic(diagnostics, "capture_exit_mismatch") {
		t.Fatalf("expected exit mismatch diagnostic, got %#v", diagnostics)
	}
}

func TestCapturesAcceptsMatchingCapture(t *testing.T) {
	t.Parallel()

	resolved := bundle.Resolve(t.TempDir())
	demo := model.DefaultDemo("validate-captures")
	demo.Redactions = nil
	demo.Steps = []model.Step{{ID: "hello", Command: "echo hello", ExpectExit: 0}}

	if _, err := capture.Run(context.Background(), resolved, demo); err != nil {
		t.Fatalf("capture run: %v", err)
	}
	if diagnostics := Captures(resolved, demo); len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", diagnostics)
	}
}

func TestCapturesComparesRedactedCommand(t *testing.T) {
	t.Parallel()

	resolved := bundle.Resolve(t.TempDir())
	demo := model.DefaultDemo("validate-redacted-captures")
	demo.Redactions = []model.Redaction{{Pattern: `/tmp/[a-z]+`, Replacement: "/tmp/REDACTED"}}
	demo.Steps = []model.Step{{ID: "path", Command: "printf '/tmp/secret\\n'", ExpectExit: 0}}

	if _, err := capture.Run(context.Background(), resolved, demo); err != nil {
		t.Fatalf("capture run: %v", err)
	}
	if diagnostics := Captures(resolved, demo); len(diagnostics) != 0 {
		t.Fatalf("expected redacted command to validate, got %#v", diagnostics)
	}
}

func TestCapturesReportsInvalidEventSchema(t *testing.T) {
	t.Parallel()

	resolved := bundle.Resolve(t.TempDir())
	demo := model.DefaultDemo("validate-event-schema")
	demo.Redactions = nil
	demo.Steps = []model.Step{{ID: "hello", Command: "echo hello", ExpectExit: 0}}
	if err := os.MkdirAll(resolved.CapturesDir, 0o755); err != nil {
		t.Fatalf("create captures dir: %v", err)
	}
	if err := capture.WriteEvents(capture.CapturePath(resolved, demo.Steps[0]), []model.CaptureEvent{
		{Type: model.EventStart, TimeMS: 0, StepID: "wrong", Command: "echo hello"},
		{Type: "mystery", TimeMS: 1, StepID: "hello"},
		model.NewExitEvent("hello", 1, 0),
	}); err != nil {
		t.Fatalf("write capture: %v", err)
	}

	diagnostics := Captures(resolved, demo)
	for _, code := range []string{"capture_step_id_mismatch", "capture_event_type_invalid"} {
		if !hasDiagnostic(diagnostics, code) {
			t.Fatalf("expected %s diagnostic, got %#v", code, diagnostics)
		}
	}
}

func hasDiagnostic(diagnostics []model.Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}
