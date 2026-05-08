// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestDefaultDemoValidates(t *testing.T) {
	t.Parallel()

	demo := DefaultDemo("hello")

	if diagnostics := demo.Validate(); len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", diagnostics)
	}
	if got := demo.StepSummaries()[0].Capture; got != "captures/hello.jsonl" {
		t.Fatalf("expected default capture path, got %q", got)
	}
}

func TestValidateFindsManifestProblems(t *testing.T) {
	t.Parallel()

	demo := DefaultDemo("broken")
	demo.SchemaVersion = "9.9"
	demo.Defaults.Terminal.Cols = 0
	demo.Playback.StepMode = "manual"
	demo.Redactions = []Redaction{{Pattern: "["}}
	demo.Steps = []Step{
		{ID: "bad id", Command: "echo nope", ExpectExit: 0},
		{ID: "dup", Command: "", ExpectExit: 300},
		{ID: "dup", Command: "echo duplicate", ExpectExit: 0},
	}

	diagnostics := demo.Validate()
	for _, code := range []string{
		"schema_version",
		"terminal_cols_positive",
		"playback_step_mode",
		"redaction_pattern_invalid",
		"step_id_invalid",
		"step_command_required",
		"step_expect_exit_range",
		"step_id_duplicate",
	} {
		if !hasDiagnostic(diagnostics, code) {
			t.Fatalf("expected diagnostic %q in %#v", code, diagnostics)
		}
	}
}

func TestApplyDefaultsFillsOptionalFields(t *testing.T) {
	t.Parallel()

	demo := Demo{Name: "partial", Steps: []Step{{ID: "one", Command: "echo one"}}}
	demo.ApplyDefaults()

	if demo.SchemaVersion != ManifestSchemaVersion {
		t.Fatalf("unexpected schema version %q", demo.SchemaVersion)
	}
	if demo.Defaults.Shell != "bash" {
		t.Fatalf("unexpected shell %q", demo.Defaults.Shell)
	}
	if demo.Playback.StepMode != "enter" {
		t.Fatalf("unexpected step mode %q", demo.Playback.StepMode)
	}
	if demo.Defaults.Timeout.ToDuration() != 30*time.Second {
		t.Fatalf("unexpected timeout %s", demo.Defaults.Timeout)
	}
}

func TestApplyDefaultsPreservesExplicitInvalidYAMLValues(t *testing.T) {
	t.Parallel()

	var demo Demo
	raw := []byte(`schema_version: "1.0"
name: bad-defaults
defaults:
  shell: ""
  timeout: 0s
  terminal:
    cols: 0
    rows: 0
playback:
  step_mode: ""
  type_delay: 0s
  max_delay: -1s
steps:
  - id: hello
    command: echo hello
    expect_exit: 0
`)
	if err := yaml.Unmarshal(raw, &demo); err != nil {
		t.Fatalf("unmarshal demo: %v", err)
	}
	demo.ApplyDefaults()

	if demo.Defaults.Shell != "" {
		t.Fatalf("expected explicit empty shell to be preserved, got %q", demo.Defaults.Shell)
	}
	if demo.Defaults.Timeout.ToDuration() != 0 {
		t.Fatalf("expected explicit zero timeout to be preserved, got %s", demo.Defaults.Timeout)
	}
	if demo.Defaults.Terminal.Cols != 0 || demo.Defaults.Terminal.Rows != 0 {
		t.Fatalf("expected explicit zero terminal to be preserved, got %#v", demo.Defaults.Terminal)
	}
	if demo.Playback.StepMode != "" {
		t.Fatalf("expected explicit empty step mode to be preserved, got %q", demo.Playback.StepMode)
	}
	if diagnostics := demo.Validate(); len(diagnostics) == 0 {
		t.Fatal("expected diagnostics for explicit invalid defaults")
	}
}

func TestApplyDefaultsFillsOmittedYAMLValues(t *testing.T) {
	t.Parallel()

	var demo Demo
	raw := []byte(`schema_version: "1.0"
name: minimal
steps:
  - id: hello
    command: echo hello
    expect_exit: 0
`)
	if err := yaml.Unmarshal(raw, &demo); err != nil {
		t.Fatalf("unmarshal demo: %v", err)
	}
	demo.ApplyDefaults()

	if diagnostics := demo.Validate(); len(diagnostics) != 0 {
		t.Fatalf("expected omitted defaults to validate after defaulting, got %#v", diagnostics)
	}
	if demo.Defaults.Timeout.ToDuration() != 30*time.Second {
		t.Fatalf("unexpected timeout %s", demo.Defaults.Timeout)
	}
	if demo.Defaults.Terminal.Cols != 120 || demo.Defaults.Terminal.Rows != 32 {
		t.Fatalf("unexpected terminal defaults %#v", demo.Defaults.Terminal)
	}
}

func TestDurationYAMLRoundTrip(t *testing.T) {
	t.Parallel()

	type document struct {
		Timeout Duration `yaml:"timeout"`
	}

	var decoded document
	if err := yaml.Unmarshal([]byte("timeout: 150ms\n"), &decoded); err != nil {
		t.Fatalf("unmarshal duration: %v", err)
	}
	if decoded.Timeout.ToDuration() != 150*time.Millisecond {
		t.Fatalf("unexpected decoded duration %s", decoded.Timeout)
	}

	encoded, err := yaml.Marshal(decoded)
	if err != nil {
		t.Fatalf("marshal duration: %v", err)
	}
	if string(encoded) != "timeout: 150ms\n" {
		t.Fatalf("unexpected encoded yaml: %q", string(encoded))
	}
}

func hasDiagnostic(diagnostics []Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}
