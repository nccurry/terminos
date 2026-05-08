// SPDX-License-Identifier: AGPL-3.0-or-later

package capture

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nccurry/terminos/internal/apperror"
	"github.com/nccurry/terminos/internal/bundle"
	"github.com/nccurry/terminos/internal/model"
)

func TestRunCapturesStdoutStderrAndExit(t *testing.T) {
	t.Parallel()

	resolved := bundle.Resolve(t.TempDir())
	demo := testDemo([]model.Step{
		{
			ID:         "both-streams",
			Command:    "printf 'out\\n'; printf 'err\\n' >&2",
			ExpectExit: 0,
		},
	})

	result, err := Run(context.Background(), resolved, demo)
	if err != nil {
		t.Fatalf("capture run: %v", err)
	}
	if len(result.Captures) != 1 {
		t.Fatalf("expected one capture result, got %#v", result.Captures)
	}
	if result.Captures[0].Capture != "captures/both-streams.jsonl" {
		t.Fatalf("unexpected capture path %q", result.Captures[0].Capture)
	}

	events, err := ReadEvents(filepath.Join(resolved.Path, result.Captures[0].Capture))
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	if !hasEvent(events, model.EventStart, "printf 'out\\n'; printf 'err\\n' >&2") {
		t.Fatalf("expected start event in %#v", events)
	}
	if !hasEvent(events, model.EventStdout, "out\n") {
		t.Fatalf("expected stdout event in %#v", events)
	}
	if !hasEvent(events, model.EventStderr, "err\n") {
		t.Fatalf("expected stderr event in %#v", events)
	}
	if events[len(events)-1].ExitCode == nil || *events[len(events)-1].ExitCode != 0 {
		t.Fatalf("expected exit 0 event, got %#v", events[len(events)-1])
	}

	rawMetadata, err := os.ReadFile(resolved.Metadata)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	var metadata model.Metadata
	if err := json.Unmarshal(rawMetadata, &metadata); err != nil {
		t.Fatalf("decode metadata: %v\n%s", err, string(rawMetadata))
	}
	if metadata.ToolVersion != model.ToolVersion {
		t.Fatalf("unexpected metadata %#v", metadata)
	}
}

func TestRunRedactsOutput(t *testing.T) {
	t.Parallel()

	resolved := bundle.Resolve(t.TempDir())
	demo := testDemo([]model.Step{{ID: "secret", Command: "printf 'token=abc123\\n'", ExpectExit: 0}})
	demo.Redactions = []model.Redaction{{Pattern: `abc[0-9]+`, Replacement: "REDACTED"}}

	result, err := Run(context.Background(), resolved, demo)
	if err != nil {
		t.Fatalf("capture run: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(resolved.Path, result.Captures[0].Capture))
	if err != nil {
		t.Fatalf("read capture: %v", err)
	}
	if strings.Contains(string(raw), "abc123") || !strings.Contains(string(raw), "REDACTED") {
		t.Fatalf("expected redacted capture, got %s", string(raw))
	}
}

func TestRunReturnsCapturedCommandErrorForUnexpectedExit(t *testing.T) {
	t.Parallel()

	resolved := bundle.Resolve(t.TempDir())
	demo := testDemo([]model.Step{{ID: "fail", Command: "exit 7", ExpectExit: 0}})

	result, err := Run(context.Background(), resolved, demo)
	if apperror.ExitCode(err) != apperror.ExitCapturedCommand {
		t.Fatalf("expected captured command exit, got %d from %v", apperror.ExitCode(err), err)
	}
	if len(result.Captures) != 1 || result.Captures[0].ExitCode != 7 {
		t.Fatalf("expected failed capture result, got %#v", result.Captures)
	}
}

func TestRunAllowsExpectedNonZeroExit(t *testing.T) {
	t.Parallel()

	resolved := bundle.Resolve(t.TempDir())
	demo := testDemo([]model.Step{{ID: "expected-fail", Command: "exit 7", ExpectExit: 7}})

	result, err := Run(context.Background(), resolved, demo)
	if err != nil {
		t.Fatalf("capture run: %v", err)
	}
	if result.Captures[0].ExitCode != 7 {
		t.Fatalf("unexpected exit code %d", result.Captures[0].ExitCode)
	}
}

func TestReadEventsHandlesLargeJSONLines(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "large.jsonl")
	largeOutput := strings.Repeat("x", 128*1024)
	events := []model.CaptureEvent{
		{Type: model.EventStart, TimeMS: 0, StepID: "large", Command: "printf large"},
		{Type: model.EventStdout, TimeMS: 1, StepID: "large", Data: largeOutput},
		model.NewExitEvent("large", 1, 0),
	}
	if err := WriteEvents(path, events); err != nil {
		t.Fatalf("write events: %v", err)
	}

	read, err := ReadEvents(path)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	if len(read) != len(events) || read[1].Data != largeOutput {
		t.Fatalf("unexpected large event read result: %#v", read)
	}
}

func testDemo(steps []model.Step) model.Demo {
	demo := model.DefaultDemo("capture-test")
	demo.Defaults.Timeout = model.NewDuration(2 * time.Second)
	demo.Redactions = nil
	demo.Steps = steps
	return demo
}

func hasEvent(events []model.CaptureEvent, eventType string, value string) bool {
	for _, event := range events {
		if event.Type == eventType && (event.Data == value || event.Command == value) {
			return true
		}
	}
	return false
}
