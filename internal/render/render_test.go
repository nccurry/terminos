// SPDX-License-Identifier: AGPL-3.0-or-later

package render

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nccurry/terminos/internal/bundle"
	"github.com/nccurry/terminos/internal/capture"
	"github.com/nccurry/terminos/internal/model"
)

func TestRenderMarkdownAsciinemaAndScript(t *testing.T) {
	t.Parallel()

	resolved := bundle.Resolve(t.TempDir())
	demo := renderTestDemo()
	if err := writeRenderTestCapture(resolved, demo); err != nil {
		t.Fatalf("write capture: %v", err)
	}

	markdown, err := Render(context.Background(), resolved, demo, FormatMarkdown)
	if err != nil {
		t.Fatalf("render markdown: %v", err)
	}
	assertGolden(t, "markdown.golden", markdown)

	script, err := Render(context.Background(), resolved, demo, FormatScript)
	if err != nil {
		t.Fatalf("render script: %v", err)
	}
	assertGolden(t, "script.golden", script)

	cast, err := Render(context.Background(), resolved, demo, FormatAsciinema)
	if err != nil {
		t.Fatalf("render asciinema: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(cast)), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected asciinema header and events, got %s", string(cast))
	}
	var header map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &header); err != nil {
		t.Fatalf("decode asciinema header: %v", err)
	}
	if header["version"].(float64) != 2 {
		t.Fatalf("unexpected asciinema header %#v", header)
	}
	assertGolden(t, "asciinema.golden", normalizeAsciinemaTimestamp(t, cast))
}

func TestRenderAsciinemaUsesStepRelativeEventTimes(t *testing.T) {
	t.Parallel()

	resolved := bundle.Resolve(t.TempDir())
	demo := model.DefaultDemo("timing-test")
	demo.Redactions = nil
	demo.Steps = []model.Step{{ID: "mixed", Command: "echo mixed", ExpectExit: 0}}
	if err := os.MkdirAll(resolved.CapturesDir, 0o755); err != nil {
		t.Fatalf("create captures dir: %v", err)
	}
	if err := capture.WriteEvents(capture.CapturePath(resolved, demo.Steps[0]), []model.CaptureEvent{
		{Type: model.EventStart, TimeMS: 0, StepID: "mixed", Command: "echo mixed"},
		{Type: model.EventStdout, TimeMS: 100, StepID: "mixed", Data: "out\n"},
		{Type: model.EventStderr, TimeMS: 100, StepID: "mixed", Data: "err\n"},
		model.NewExitEvent("mixed", 100, 0),
	}); err != nil {
		t.Fatalf("write capture: %v", err)
	}

	cast, err := Render(context.Background(), resolved, demo, FormatAsciinema)
	if err != nil {
		t.Fatalf("render asciinema: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(cast)), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected header, prompt, and two output events, got %d lines:\n%s", len(lines), string(cast))
	}

	firstOutputAt := asciinemaEventTime(t, lines[2])
	secondOutputAt := asciinemaEventTime(t, lines[3])
	if math.Abs(firstOutputAt-0.1) > 0.000001 || math.Abs(secondOutputAt-0.1) > 0.000001 {
		t.Fatalf("expected both output events at 0.1s, got %.6f and %.6f\n%s", firstOutputAt, secondOutputAt, string(cast))
	}
}

func TestRenderAppliesRedactionsToCommandsAndOutput(t *testing.T) {
	t.Parallel()

	resolved := bundle.Resolve(t.TempDir())
	demo := model.DefaultDemo("redact-secret-123")
	demo.Summary = "Summary mentions secret-123."
	demo.Redactions = []model.Redaction{{
		Pattern:     `secret-[0-9]+`,
		Replacement: "[REDACTED]",
	}}
	demo.Steps = []model.Step{{
		ID:         "redacted",
		Title:      "Use secret-123",
		Command:    "curl -H 'token=secret-123' https://example.test",
		ExpectExit: 0,
	}}
	if err := os.MkdirAll(resolved.CapturesDir, 0o755); err != nil {
		t.Fatalf("create captures dir: %v", err)
	}
	if err := capture.WriteEvents(capture.CapturePath(resolved, demo.Steps[0]), []model.CaptureEvent{
		{Type: model.EventStart, TimeMS: 0, StepID: "redacted", Command: "curl -H 'token=secret-123' https://example.test"},
		{Type: model.EventStdout, TimeMS: 10, StepID: "redacted", Data: "token=secret-123\n"},
		{Type: model.EventStderr, TimeMS: 10, StepID: "redacted", Data: "warning secret-123\n"},
		model.NewExitEvent("redacted", 10, 0),
	}); err != nil {
		t.Fatalf("write capture: %v", err)
	}

	for _, format := range []string{FormatMarkdown, FormatAsciinema, FormatScript} {
		format := format
		t.Run(format, func(t *testing.T) {
			t.Parallel()

			rendered, err := Render(context.Background(), resolved, demo, format)
			if err != nil {
				t.Fatalf("render %s: %v", format, err)
			}
			text := string(rendered)
			if strings.Contains(text, "secret-123") {
				t.Fatalf("rendered %s leaked secret:\n%s", format, text)
			}
			if !strings.Contains(text, "[REDACTED]") {
				t.Fatalf("rendered %s did not include replacement:\n%s", format, text)
			}
		})
	}
}

func asciinemaEventTime(t *testing.T, line string) float64 {
	t.Helper()

	var event []any
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		t.Fatalf("decode asciinema event: %v\n%s", err, line)
	}
	if len(event) != 3 {
		t.Fatalf("unexpected asciinema event %#v", event)
	}
	timestamp, ok := event[0].(float64)
	if !ok {
		t.Fatalf("unexpected asciinema timestamp %#v", event[0])
	}
	return timestamp
}

func renderTestDemo() model.Demo {
	demo := model.DefaultDemo("render-test")
	demo.Summary = "A render test."
	demo.Redactions = nil
	demo.Steps = []model.Step{{
		ID:         "hello",
		Title:      "Say hello",
		Command:    "printf 'hello\\n'",
		ExpectExit: 0,
	}}
	return demo
}

func writeRenderTestCapture(resolved bundle.Bundle, demo model.Demo) error {
	if err := os.MkdirAll(resolved.CapturesDir, 0o755); err != nil {
		return err
	}
	return capture.WriteEvents(capture.CapturePath(resolved, demo.Steps[0]), []model.CaptureEvent{
		{Type: model.EventStart, TimeMS: 0, StepID: "hello", Command: "printf 'hello\\n'"},
		{Type: model.EventStdout, TimeMS: 1, StepID: "hello", Data: "hello\n"},
		model.NewExitEvent("hello", 1, 0),
	})
}

func normalizeAsciinemaTimestamp(t *testing.T, cast []byte) []byte {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(string(cast)), "\n")
	var header map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &header); err != nil {
		t.Fatalf("decode asciinema header: %v", err)
	}
	header["timestamp"] = float64(0)
	normalized, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("encode normalized header: %v", err)
	}
	lines[0] = string(normalized)
	return []byte(strings.Join(lines, "\n") + "\n")
}

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()

	want, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	if string(got) != string(want) {
		t.Fatalf("%s mismatch\n--- got ---\n%s\n--- want ---\n%s", name, string(got), string(want))
	}
}
