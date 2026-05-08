// SPDX-License-Identifier: AGPL-3.0-or-later

package replay

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nccurry/terminos/internal/apperror"
	"github.com/nccurry/terminos/internal/bundle"
	"github.com/nccurry/terminos/internal/capture"
	"github.com/nccurry/terminos/internal/model"
)

func TestPlayReplaysCapturedSteps(t *testing.T) {
	t.Parallel()

	resolved := bundle.Resolve(t.TempDir())
	demo := model.DefaultDemo("play-test")
	demo.Redactions = nil
	demo.Steps = []model.Step{
		{
			ID:         "one",
			Title:      "First step",
			Command:    "printf 'one\\n'",
			ExpectExit: 0,
		},
		{
			ID:         "two",
			Title:      "Second step",
			Command:    "printf 'two\\n'",
			ExpectExit: 0,
		},
	}

	if _, err := capture.Run(context.Background(), resolved, demo); err != nil {
		t.Fatalf("capture run: %v", err)
	}

	var out bytes.Buffer
	err := Play(context.Background(), resolved, demo, &out, Options{NoDelay: true})
	if err != nil {
		t.Fatalf("play: %v", err)
	}

	text := out.String()
	for _, want := range []string{
		"# First step\n",
		"$ printf 'one\\n'\n",
		"one\n",
		"# Second step\n",
		"$ printf 'two\\n'\n",
		"two\n",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected playback to contain %q, got:\n%s", want, text)
		}
	}
}

func TestPlayEnterModePromptsBeforeOutput(t *testing.T) {
	t.Parallel()

	resolved := bundle.Resolve(t.TempDir())
	demo := model.DefaultDemo("enter-test")
	demo.Redactions = nil
	demo.Steps = []model.Step{
		{ID: "one", Title: "First step", Command: "printf 'one\\n'", ExpectExit: 0},
		{ID: "two", Title: "Second step", Command: "printf 'two\\n'", ExpectExit: 0},
	}
	if _, err := capture.Run(context.Background(), resolved, demo); err != nil {
		t.Fatalf("capture run: %v", err)
	}

	var out bytes.Buffer
	var prompts bytes.Buffer
	zero := time.Duration(0)
	err := Play(context.Background(), resolved, demo, &out, Options{
		StepMode:  "enter",
		TypeDelay: &zero,
		Input:     strings.NewReader("\n\n"),
		Prompt:    &prompts,
	})
	if err != nil {
		t.Fatalf("play: %v", err)
	}

	if !strings.Contains(prompts.String(), "Press Enter to show First step") || !strings.Contains(prompts.String(), "Press Enter to show Second step") {
		t.Fatalf("expected enter prompts, got %q", prompts.String())
	}
	if !strings.Contains(out.String(), "$ printf 'one\\n'\none\n") || !strings.Contains(out.String(), "$ printf 'two\\n'\ntwo\n") {
		t.Fatalf("unexpected playback output:\n%s", out.String())
	}
}

func TestPlayRejectsInvalidOptions(t *testing.T) {
	t.Parallel()

	demo := model.DefaultDemo("invalid-options")
	if _, err := newConfig(demo, Options{Speed: -1}); apperror.ExitCode(err) != apperror.ExitUsage {
		t.Fatalf("expected usage exit for speed, got %d from %v", apperror.ExitCode(err), err)
	}
	if _, err := newConfig(demo, Options{StepMode: "pause"}); apperror.ExitCode(err) != apperror.ExitUsage {
		t.Fatalf("expected usage exit for step mode, got %d from %v", apperror.ExitCode(err), err)
	}
	negative := -1 * time.Millisecond
	if _, err := newConfig(demo, Options{TypeDelay: &negative}); apperror.ExitCode(err) != apperror.ExitUsage {
		t.Fatalf("expected usage exit for type delay, got %d from %v", apperror.ExitCode(err), err)
	}
}

func TestPlayAutoModeAllowsZeroMaxDelayOverride(t *testing.T) {
	t.Parallel()

	resolved := bundle.Resolve(t.TempDir())
	demo := model.DefaultDemo("auto-delay-test")
	demo.Redactions = nil
	demo.Steps = []model.Step{{ID: "slow", Command: "echo slow", ExpectExit: 0}}
	if err := os.MkdirAll(resolved.CapturesDir, 0o755); err != nil {
		t.Fatalf("create captures dir: %v", err)
	}
	if err := capture.WriteEvents(filepath.Join(resolved.CapturesDir, "slow.jsonl"), []model.CaptureEvent{
		{Type: model.EventStart, TimeMS: 0, StepID: "slow", Command: "echo slow"},
		{Type: model.EventStdout, TimeMS: 60_000, StepID: "slow", Data: "slow\n"},
		model.NewExitEvent("slow", 60_000, 0),
	}); err != nil {
		t.Fatalf("write capture: %v", err)
	}

	var out bytes.Buffer
	zero := time.Duration(0)
	var delays []time.Duration
	err := Play(context.Background(), resolved, demo, &out, Options{
		StepMode:  "auto",
		TypeDelay: &zero,
		MaxDelay:  &zero,
		Sleep: func(ctx context.Context, delay time.Duration) error {
			delays = append(delays, delay)
			return ctx.Err()
		},
	})
	if err != nil {
		t.Fatalf("play: %v", err)
	}
	if len(delays) != 1 || delays[0] != 0 {
		t.Fatalf("expected one zero output delay, got %#v", delays)
	}
	if !strings.Contains(out.String(), "slow\n") {
		t.Fatalf("unexpected playback output: %s", out.String())
	}
}

func TestPlayAutoModeCapsOutputDelay(t *testing.T) {
	t.Parallel()

	resolved := bundle.Resolve(t.TempDir())
	demo := model.DefaultDemo("auto-delay-cap-test")
	demo.Redactions = nil
	demo.Playback.MaxDelay = model.NewDuration(250 * time.Millisecond)
	demo.Steps = []model.Step{{ID: "slow", Command: "echo slow", ExpectExit: 0}}
	if err := os.MkdirAll(resolved.CapturesDir, 0o755); err != nil {
		t.Fatalf("create captures dir: %v", err)
	}
	if err := capture.WriteEvents(filepath.Join(resolved.CapturesDir, "slow.jsonl"), []model.CaptureEvent{
		{Type: model.EventStart, TimeMS: 0, StepID: "slow", Command: "echo slow"},
		{Type: model.EventStdout, TimeMS: 60_000, StepID: "slow", Data: "slow\n"},
		model.NewExitEvent("slow", 60_000, 0),
	}); err != nil {
		t.Fatalf("write capture: %v", err)
	}

	var out bytes.Buffer
	zeroTypeDelay := time.Duration(0)
	var delays []time.Duration
	err := Play(context.Background(), resolved, demo, &out, Options{
		StepMode:  "auto",
		TypeDelay: &zeroTypeDelay,
		Sleep: func(ctx context.Context, delay time.Duration) error {
			delays = append(delays, delay)
			return ctx.Err()
		},
	})
	if err != nil {
		t.Fatalf("play: %v", err)
	}
	if len(delays) != 1 || delays[0] != 250*time.Millisecond {
		t.Fatalf("expected output delay capped at 250ms, got %#v", delays)
	}
	if !strings.Contains(out.String(), "slow\n") {
		t.Fatalf("unexpected playback output: %s", out.String())
	}
}
