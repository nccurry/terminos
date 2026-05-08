// SPDX-License-Identifier: AGPL-3.0-or-later

// Package importer converts pasted terminal transcripts into demo bundles.
package importer

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/nccurry/terminos/internal/apperror"
	"github.com/nccurry/terminos/internal/bundle"
	"github.com/nccurry/terminos/internal/capture"
	"github.com/nccurry/terminos/internal/model"
	"github.com/nccurry/terminos/internal/redact"
)

// Options controls transcript import behavior.
type Options struct {
	SourcePath string
	Append     bool
}

// Result describes imported transcript steps.
type Result struct {
	BundlePath string              `json:"bundle_path" yaml:"bundle_path"`
	SourcePath string              `json:"source_path" yaml:"source_path"`
	Steps      []model.StepSummary `json:"steps" yaml:"steps"`
}

// Import parses a transcript, updates demo.yaml, and writes capture JSONL files.
func Import(path string, opts Options) (Result, error) {
	if strings.TrimSpace(opts.SourcePath) == "" {
		return Result{}, apperror.New(apperror.CodeUsage, "--from is required")
	}

	resolved, demo, err := loadOrDefault(path)
	if err != nil {
		return Result{}, err
	}
	if diagnostics := importBlockingDiagnostics(demo, opts.Append); len(diagnostics) != 0 {
		return Result{}, apperror.New(apperror.CodeValidation, "demo bundle validation failed")
	}

	raw, err := os.ReadFile(opts.SourcePath)
	if err != nil {
		return Result{}, apperror.Wrapf(apperror.CodeFilesystem, err, "read transcript %s", opts.SourcePath)
	}
	blocks, err := ParseTranscript(string(raw), demo.Defaults.Prompt)
	if err != nil {
		return Result{}, err
	}
	redactor, err := redact.New(demo.Redactions)
	if err != nil {
		return Result{}, err
	}
	blocks = redactBlocks(blocks, redactor)

	steps := stepsFromBlocks(blocks, existingStepIDs(demo.Steps, opts.Append))
	if opts.Append {
		steps = append(demo.Steps, steps...)
	}
	demo.Steps = steps
	if diagnostics := demo.Validate(); len(diagnostics) != 0 {
		return Result{}, apperror.New(apperror.CodeValidation, "imported demo bundle validation failed")
	}

	if err := os.MkdirAll(resolved.CapturesDir, 0o755); err != nil {
		return Result{}, apperror.Wrapf(apperror.CodeFilesystem, err, "create captures directory %s", resolved.CapturesDir)
	}
	for _, block := range blocksFromSteps(demo.Steps, blocks, opts.Append) {
		if err := writeImportedCapture(resolved, block.step, block.output); err != nil {
			return Result{}, err
		}
	}
	if err := bundle.WriteManifest(resolved.Manifest, demo); err != nil {
		return Result{}, err
	}
	if err := bundle.WriteMetadata(resolved); err != nil {
		return Result{}, err
	}

	return Result{
		BundlePath: resolved.Path,
		SourcePath: opts.SourcePath,
		Steps:      demo.StepSummaries(),
	}, nil
}

// TranscriptBlock is one prompt command and its captured stdout.
type TranscriptBlock struct {
	Command string
	Output  string
}

// ParseTranscript splits pasted terminal text into prompt-delimited blocks.
func ParseTranscript(text string, prompt string) ([]TranscriptBlock, error) {
	if prompt == "" {
		prompt = "$ "
	}

	var blocks []TranscriptBlock
	var current *TranscriptBlock
	lines := strings.SplitAfter(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSuffix(line, "\n")
		trimmed = strings.TrimSuffix(trimmed, "\r")
		if strings.HasPrefix(trimmed, prompt) {
			if current != nil {
				blocks = append(blocks, *current)
			}
			current = &TranscriptBlock{Command: strings.TrimSpace(strings.TrimPrefix(trimmed, prompt))}
			continue
		}
		if current != nil {
			current.Output += line
		}
	}
	if current != nil {
		blocks = append(blocks, *current)
	}

	if len(blocks) == 0 {
		return nil, apperror.Newf(apperror.CodeUsage, "transcript has no command lines starting with prompt %q", prompt)
	}
	for i, block := range blocks {
		if strings.TrimSpace(block.Command) == "" {
			return nil, apperror.Newf(apperror.CodeUsage, "transcript command %d is empty", i+1)
		}
	}
	return blocks, nil
}

func loadOrDefault(path string) (bundle.Bundle, model.Demo, error) {
	resolved := bundle.Resolve(path)
	if _, err := os.Stat(resolved.Manifest); err == nil {
		return bundle.Load(path)
	} else if !os.IsNotExist(err) {
		return bundle.Bundle{}, model.Demo{}, apperror.Wrapf(apperror.CodeFilesystem, err, "stat %s", resolved.Manifest)
	}

	demo := model.DefaultDemo(defaultName(resolved.Path))
	if err := os.MkdirAll(resolved.AssetsDir, 0o755); err != nil {
		return bundle.Bundle{}, model.Demo{}, apperror.Wrapf(apperror.CodeFilesystem, err, "create assets directory %s", resolved.AssetsDir)
	}
	if err := os.MkdirAll(resolved.CapturesDir, 0o755); err != nil {
		return bundle.Bundle{}, model.Demo{}, apperror.Wrapf(apperror.CodeFilesystem, err, "create captures directory %s", resolved.CapturesDir)
	}
	return resolved, demo, nil
}

func stepsFromBlocks(blocks []TranscriptBlock, seen map[string]int) []model.Step {
	steps := make([]model.Step, 0, len(blocks))
	for _, block := range blocks {
		id := uniqueID(slugify(block.Command), seen)
		steps = append(steps, model.Step{
			ID:         id,
			Title:      "Run " + block.Command,
			Command:    block.Command,
			ExpectExit: 0,
		})
	}
	return steps
}

func existingStepIDs(steps []model.Step, appendMode bool) map[string]int {
	seen := map[string]int{}
	if !appendMode {
		return seen
	}
	for _, step := range steps {
		seen[step.ID]++
	}
	return seen
}

func importBlockingDiagnostics(demo model.Demo, appendMode bool) []model.Diagnostic {
	diagnostics := demo.Validate()
	if appendMode {
		return diagnostics
	}

	blocking := make([]model.Diagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		if strings.HasPrefix(diagnostic.Path, "steps") {
			continue
		}
		blocking = append(blocking, diagnostic)
	}
	return blocking
}

type importBlock struct {
	step   model.Step
	output string
}

func blocksFromSteps(steps []model.Step, blocks []TranscriptBlock, appendMode bool) []importBlock {
	imported := make([]importBlock, 0, len(blocks))
	offset := 0
	if appendMode {
		offset = len(steps) - len(blocks)
	}
	for i, block := range blocks {
		imported = append(imported, importBlock{step: steps[offset+i], output: block.Output})
	}
	return imported
}

func writeImportedCapture(resolved bundle.Bundle, step model.Step, output string) error {
	capturePath := capture.CapturePath(resolved, step)
	if err := os.MkdirAll(filepath.Dir(capturePath), 0o755); err != nil {
		return apperror.Wrapf(apperror.CodeFilesystem, err, "create capture directory %s", filepath.Dir(capturePath))
	}

	exitCode := 0
	events := []model.CaptureEvent{
		{
			Type:    model.EventStart,
			TimeMS:  0,
			StepID:  step.ID,
			Command: step.Command,
		},
	}
	if output != "" {
		events = append(events, model.CaptureEvent{
			Type:   model.EventStdout,
			TimeMS: 1,
			StepID: step.ID,
			Data:   output,
		})
	}
	events = append(events, model.NewExitEvent(step.ID, 1, exitCode))
	return capture.WriteEvents(capturePath, events)
}

func redactBlocks(blocks []TranscriptBlock, redactor redact.RuleSet) []TranscriptBlock {
	redacted := make([]TranscriptBlock, 0, len(blocks))
	for _, block := range blocks {
		redacted = append(redacted, TranscriptBlock{
			Command: redactor.Apply(block.Command),
			Output:  redactor.Apply(block.Output),
		})
	}
	return redacted
}

var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(command string) string {
	slug := strings.ToLower(command)
	slug = slugPattern.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "step"
	}
	parts := strings.Split(slug, "-")
	if len(parts) > 4 {
		parts = parts[:4]
	}
	slug = strings.Join(parts, "-")
	if len(slug) > 40 {
		slug = strings.Trim(slug[:40], "-")
	}
	if slug == "" {
		return "step"
	}
	if slug[0] < 'a' || slug[0] > 'z' {
		slug = "step-" + slug
	}
	return slug
}

func uniqueID(base string, seen map[string]int) string {
	if seen[base] == 0 {
		seen[base] = 1
		return base
	}
	for suffix := 2; ; suffix++ {
		candidate := base + "-" + strconv.Itoa(suffix)
		if seen[candidate] == 0 {
			seen[candidate] = 1
			return candidate
		}
	}
}

func defaultName(path string) string {
	name := filepath.Base(filepath.Clean(path))
	if name == "." || name == string(filepath.Separator) {
		return "terminos-demo"
	}
	return name
}
