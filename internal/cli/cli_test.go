// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nccurry/terminos/internal/apperror"
	"github.com/nccurry/terminos/internal/bundle"
	capturepkg "github.com/nccurry/terminos/internal/capture"
	"github.com/nccurry/terminos/internal/importer"
	"github.com/nccurry/terminos/internal/model"
)

func TestInitValidateAndListStepsJSON(t *testing.T) {
	t.Parallel()

	bundlePath := filepath.Join(t.TempDir(), "hello")

	stdout, stderr, err := runApp("init", bundlePath, "--output", "json")
	if err != nil {
		t.Fatalf("init failed: %v\nstderr=%s", err, stderr)
	}
	var initResult bundle.InitResult
	if err := json.Unmarshal([]byte(stdout), &initResult); err != nil {
		t.Fatalf("decode init json: %v\n%s", err, stdout)
	}
	if initResult.Bundle.Manifest != filepath.Join(bundlePath, "demo.yaml") {
		t.Fatalf("unexpected init result %#v", initResult)
	}

	stdout, stderr, err = runApp("validate", bundlePath, "--output", "json")
	if err != nil {
		t.Fatalf("validate failed: %v\nstderr=%s", err, stderr)
	}
	var validation validationResult
	if err := json.Unmarshal([]byte(stdout), &validation); err != nil {
		t.Fatalf("decode validation json: %v\n%s", err, stdout)
	}
	if !validation.Valid || validation.StepCount != 1 || len(validation.Errors) != 0 {
		t.Fatalf("unexpected validation result: %#v", validation)
	}

	stdout, stderr, err = runApp("list-steps", bundlePath, "--output", "json")
	if err != nil {
		t.Fatalf("list-steps failed: %v\nstderr=%s", err, stderr)
	}
	var steps []model.StepSummary
	if err := json.Unmarshal([]byte(stdout), &steps); err != nil {
		t.Fatalf("decode list-steps json: %v\n%s", err, stdout)
	}
	if len(steps) != 1 {
		t.Fatalf("expected one step, got %#v", steps)
	}
	want := model.StepSummary{
		ID:         "hello",
		Title:      "Print a friendly message",
		Command:    "printf 'hello from terminos\\n'",
		Capture:    "captures/hello.jsonl",
		ExpectExit: 0,
	}
	if steps[0] != want {
		t.Fatalf("unexpected step summary:\n got %#v\nwant %#v", steps[0], want)
	}
}

func TestValidateYAMLOutput(t *testing.T) {
	t.Parallel()

	bundlePath := filepath.Join(t.TempDir(), "hello")
	if _, _, err := runApp("init", bundlePath); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	stdout, stderr, err := runApp("validate", bundlePath, "--output", "yaml")
	if err != nil {
		t.Fatalf("validate failed: %v\nstderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, "valid: true") || !strings.Contains(stdout, "errors: []") {
		t.Fatalf("unexpected YAML output: %s", stdout)
	}
}

func TestValidateInvalidManifestReturnsValidationExit(t *testing.T) {
	t.Parallel()

	bundlePath := filepath.Join(t.TempDir(), "broken")
	if err := os.MkdirAll(bundlePath, 0o755); err != nil {
		t.Fatalf("create bundle dir: %v", err)
	}
	manifest := []byte("schema_version: \"1.0\"\nname: broken\nsteps: []\n")
	if err := os.WriteFile(filepath.Join(bundlePath, "demo.yaml"), manifest, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := bundle.WriteMetadata(bundle.Resolve(bundlePath)); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	stdout, _, err := runApp("validate", bundlePath, "--output", "json")
	if apperror.ExitCode(err) != apperror.ExitValidation {
		t.Fatalf("expected validation exit, got %d from %v", apperror.ExitCode(err), err)
	}
	assertGoldenFile(t, "validate-empty-steps.json.golden", normalizeValidationPaths(stdout, bundlePath))
	var validation validationResult
	if err := json.Unmarshal([]byte(stdout), &validation); err != nil {
		t.Fatalf("decode validation json: %v\n%s", err, stdout)
	}
	if validation.Valid || !hasDiagnostic(validation.Errors, "steps_required") {
		t.Fatalf("expected validation diagnostics, got %#v", validation)
	}
}

func TestValidateRejectsExplicitInvalidDefaults(t *testing.T) {
	t.Parallel()

	bundlePath := filepath.Join(t.TempDir(), "bad-defaults")
	if err := os.MkdirAll(bundlePath, 0o755); err != nil {
		t.Fatalf("create bundle dir: %v", err)
	}
	manifest := []byte(`schema_version: "1.0"
name: bad-defaults
defaults:
  timeout: 0s
  terminal:
    cols: 0
    rows: 0
steps:
  - id: hello
    command: echo hello
    expect_exit: 0
`)
	if err := os.WriteFile(filepath.Join(bundlePath, "demo.yaml"), manifest, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	stdout, _, err := runApp("validate", bundlePath, "--output", "json")
	if apperror.ExitCode(err) != apperror.ExitValidation {
		t.Fatalf("expected validation exit, got %d from %v\n%s", apperror.ExitCode(err), err, stdout)
	}
	var validation validationResult
	if err := json.Unmarshal([]byte(stdout), &validation); err != nil {
		t.Fatalf("decode validation json: %v\n%s", err, stdout)
	}
	for _, code := range []string{
		"default_timeout_positive",
		"terminal_cols_positive",
		"terminal_rows_positive",
	} {
		if !hasDiagnostic(validation.Errors, code) {
			t.Fatalf("expected diagnostic %q in %#v", code, validation.Errors)
		}
	}
}

func TestValidateAllowsOmittedDefaults(t *testing.T) {
	t.Parallel()

	bundlePath := filepath.Join(t.TempDir(), "minimal")
	if err := os.MkdirAll(bundlePath, 0o755); err != nil {
		t.Fatalf("create bundle dir: %v", err)
	}
	manifest := []byte(`schema_version: "1.0"
name: minimal
steps:
  - id: hello
    command: echo hello
    expect_exit: 0
`)
	if err := os.WriteFile(filepath.Join(bundlePath, "demo.yaml"), manifest, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := bundle.WriteMetadata(bundle.Resolve(bundlePath)); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	stdout, stderr, err := runApp("validate", bundlePath, "--output", "json")
	if err != nil {
		t.Fatalf("validate failed: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}
	var validation validationResult
	if err := json.Unmarshal([]byte(stdout), &validation); err != nil {
		t.Fatalf("decode validation json: %v\n%s", err, stdout)
	}
	if !validation.Valid {
		t.Fatalf("expected valid result, got %#v", validation)
	}
}

func TestCaptureAndPlayCommands(t *testing.T) {
	t.Parallel()

	bundlePath := filepath.Join(t.TempDir(), "demo")
	if _, _, err := runApp("init", bundlePath); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	stdout, stderr, err := runApp("capture", bundlePath, "--output", "json")
	if err != nil {
		t.Fatalf("capture failed: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}
	var captureResult capturepkg.Result
	if err := json.Unmarshal([]byte(stdout), &captureResult); err != nil {
		t.Fatalf("decode capture json: %v\n%s", err, stdout)
	}
	if len(captureResult.Captures) != 1 || captureResult.Captures[0].StepID != "hello" || captureResult.Captures[0].Capture != "captures/hello.jsonl" {
		t.Fatalf("unexpected capture output: %#v", captureResult)
	}

	stdout, stderr, err = runApp("play", bundlePath, "--no-delay")
	if err != nil {
		t.Fatalf("play failed: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, "$ printf 'hello from terminos\\n'\n") || !strings.Contains(stdout, "hello from terminos\n") {
		t.Fatalf("unexpected playback output: %s", stdout)
	}

	stdout, stderr, err = runApp("play", bundlePath, "--step-mode", "none", "--type-delay", "0s", "--speed", "2", "--max-delay", "1ms")
	if err != nil {
		t.Fatalf("play with controls failed: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, "hello from terminos\n") {
		t.Fatalf("unexpected controlled playback output: %s", stdout)
	}

	stdout, stderr, err = runApp("validate", bundlePath, "--captures", "--output", "json")
	if err != nil {
		t.Fatalf("validate captures failed: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}
	var validation validationResult
	if err := json.Unmarshal([]byte(stdout), &validation); err != nil {
		t.Fatalf("decode capture validation json: %v\n%s", err, stdout)
	}
	if !validation.Valid {
		t.Fatalf("unexpected capture validation output: %#v", validation)
	}
}

func TestPlayUsesManifestStepMode(t *testing.T) {
	t.Parallel()

	bundlePath := filepath.Join(t.TempDir(), "demo")
	if _, _, err := runApp("init", bundlePath); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if stdout, stderr, err := runApp("capture", bundlePath); err != nil {
		t.Fatalf("capture failed: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}

	stdout, stderr, err := runAppWithInput("\n", "play", bundlePath, "--type-delay", "0s")
	if err != nil {
		t.Fatalf("play with manifest enter mode failed: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}
	if !strings.Contains(stderr, "Press Enter to show Print a friendly message") {
		t.Fatalf("expected manifest enter mode prompt, got stderr=%q stdout=%q", stderr, stdout)
	}
	if !strings.Contains(stdout, "hello from terminos\n") {
		t.Fatalf("unexpected enter-mode playback output: %s", stdout)
	}

	resolved, demo, err := bundle.Load(bundlePath)
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	demo.Playback.StepMode = "none"
	demo.Playback.TypeDelay = model.NewDuration(0)
	if err := bundle.WriteManifest(resolved.Manifest, demo); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	stdout, stderr, err = runApp("play", bundlePath)
	if err != nil {
		t.Fatalf("play with manifest none mode failed: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}
	if strings.Contains(stderr, "Press Enter") {
		t.Fatalf("expected manifest none mode to skip prompts, got stderr=%q", stderr)
	}
	if !strings.Contains(stdout, "hello from terminos\n") {
		t.Fatalf("unexpected none-mode playback output: %s", stdout)
	}
}

func TestImportCommandCreatesPlayableBundle(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	transcript := filepath.Join(root, "transcript.txt")
	data := []byte("$ echo one\none\n$ printf 'two\\n'\ntwo\n")
	if err := os.WriteFile(transcript, data, 0o644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	bundlePath := filepath.Join(root, "demo")
	stdout, stderr, err := runApp("import", bundlePath, "--from", transcript, "--output", "json")
	if err != nil {
		t.Fatalf("import failed: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}
	var importResult importer.Result
	if err := json.Unmarshal([]byte(stdout), &importResult); err != nil {
		t.Fatalf("decode import json: %v\n%s", err, stdout)
	}
	if len(importResult.Steps) != 2 || importResult.Steps[0].ID != "echo-one" || importResult.Steps[1].ID != "printf-two-n" {
		t.Fatalf("unexpected import output: %#v", importResult)
	}

	stdout, stderr, err = runApp("play", bundlePath, "--no-delay")
	if err != nil {
		t.Fatalf("play failed: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, "$ echo one\none\n") || !strings.Contains(stdout, "$ printf 'two\\n'\ntwo\n") {
		t.Fatalf("unexpected playback output: %s", stdout)
	}
}

func TestRenderCommandWritesArtifacts(t *testing.T) {
	t.Parallel()

	bundlePath := filepath.Join(t.TempDir(), "demo")
	if _, _, err := runApp("init", bundlePath); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if stdout, stderr, err := runApp("capture", bundlePath); err != nil {
		t.Fatalf("capture failed: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}

	stdout, stderr, err := runApp("render", bundlePath, "--format", "markdown")
	if err != nil {
		t.Fatalf("render markdown failed: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, "# demo") || !strings.Contains(stdout, "hello from terminos") {
		t.Fatalf("unexpected markdown: %s", stdout)
	}

	outPath := filepath.Join(t.TempDir(), "demo.cast")
	stdout, stderr, err = runApp("render", bundlePath, "--format", "asciinema", "--out", outPath, "--output", "json")
	if err != nil {
		t.Fatalf("render asciinema failed: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
	}
	var rendered struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(stdout), &rendered); err != nil {
		t.Fatalf("decode render json: %v\n%s", err, stdout)
	}
	if rendered.Path != outPath {
		t.Fatalf("unexpected render output: %#v", rendered)
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read rendered file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	var header struct {
		Version int `json:"version"`
		Width   int `json:"width"`
		Height  int `json:"height"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &header); err != nil {
		t.Fatalf("decode asciinema header: %v\n%s", err, string(raw))
	}
	if header.Version != 2 || header.Width == 0 || header.Height == 0 {
		t.Fatalf("unexpected asciinema header: %#v", header)
	}
}

func TestPlayAndRenderRequireCaptures(t *testing.T) {
	t.Parallel()

	bundlePath := filepath.Join(t.TempDir(), "demo")
	if _, _, err := runApp("init", bundlePath); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	_, _, err := runApp("play", bundlePath, "--no-delay")
	if apperror.ExitCode(err) != apperror.ExitFilesystem {
		t.Fatalf("expected play filesystem exit for missing captures, got %d from %v", apperror.ExitCode(err), err)
	}

	_, _, err = runApp("render", bundlePath, "--format", "markdown")
	if apperror.ExitCode(err) != apperror.ExitFilesystem {
		t.Fatalf("expected render filesystem exit for missing captures, got %d from %v", apperror.ExitCode(err), err)
	}
}

func TestInvalidOutputFlagReturnsUsageExit(t *testing.T) {
	t.Parallel()

	_, _, err := runApp("validate", "examples/hello", "--output", "xml")
	if apperror.ExitCode(err) != apperror.ExitUsage {
		t.Fatalf("expected usage exit, got %d from %v", apperror.ExitCode(err), err)
	}
}

func TestInvalidPlaybackFlagsReturnUsageExit(t *testing.T) {
	t.Parallel()

	bundlePath := filepath.Join(t.TempDir(), "demo")
	if _, _, err := runApp("init", bundlePath); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if _, _, err := runApp("capture", bundlePath); err != nil {
		t.Fatalf("capture failed: %v", err)
	}

	_, _, err := runApp("play", bundlePath, "--speed", "0")
	if apperror.ExitCode(err) != apperror.ExitUsage {
		t.Fatalf("expected usage exit for speed, got %d from %v", apperror.ExitCode(err), err)
	}
	_, _, err = runApp("play", bundlePath, "--step-mode", "pause")
	if apperror.ExitCode(err) != apperror.ExitUsage {
		t.Fatalf("expected usage exit for step mode, got %d from %v", apperror.ExitCode(err), err)
	}
}

func TestHelpIncludesAIContext(t *testing.T) {
	t.Parallel()

	stdout, stderr, err := runApp("validate", "--help")
	if err != nil {
		t.Fatalf("help failed: %v\nstderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, "AI_CONTEXT:") {
		t.Fatalf("expected AI_CONTEXT in help, got %s", stdout)
	}
}

func TestInitRequiresOneBundleArgument(t *testing.T) {
	t.Parallel()

	_, _, err := runApp("init")
	if apperror.ExitCode(err) != apperror.ExitUsage {
		t.Fatalf("expected usage exit, got %d from %v", apperror.ExitCode(err), err)
	}
}

func runApp(args ...string) (string, string, error) {
	return runAppWithInput("", args...)
}

func runAppWithInput(input string, args ...string) (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := NewApp(&stdout, &stderr)
	if input != "" {
		app.Reader = strings.NewReader(input)
	}
	err := app.Run(context.Background(), append([]string{"terminos"}, args...))
	return stdout.String(), stderr.String(), err
}

func hasDiagnostic(diagnostics []model.Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

func normalizeValidationPaths(output string, bundlePath string) string {
	return strings.ReplaceAll(
		strings.ReplaceAll(output, filepath.Join(bundlePath, "demo.yaml"), "<manifest>"),
		bundlePath,
		"<bundle>",
	)
}

func assertGoldenFile(t *testing.T, name string, got string) {
	t.Helper()

	want, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	if got != string(want) {
		t.Fatalf("%s mismatch\n--- got ---\n%s\n--- want ---\n%s", name, got, string(want))
	}
}
