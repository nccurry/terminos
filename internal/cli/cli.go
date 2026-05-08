// SPDX-License-Identifier: AGPL-3.0-or-later

// Package cli defines the terminos command-line interface.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	urfave "github.com/urfave/cli/v3"

	"github.com/nccurry/terminos/internal/apperror"
	"github.com/nccurry/terminos/internal/bundle"
	"github.com/nccurry/terminos/internal/capture"
	"github.com/nccurry/terminos/internal/importer"
	"github.com/nccurry/terminos/internal/model"
	"github.com/nccurry/terminos/internal/render"
	"github.com/nccurry/terminos/internal/replay"
	"github.com/nccurry/terminos/internal/validate"
)

const (
	commandName = "terminos"

	flagDebug     = "debug"
	flagDryRun    = "dry-run"
	flagForce     = "force"
	flagFrom      = "from"
	flagAppend    = "append"
	flagCaptures  = "captures"
	flagFormat    = "format"
	flagNoDelay   = "no-delay"
	flagNoColor   = "no-color"
	flagOut       = "out"
	flagOutput    = "output"
	flagSpeed     = "speed"
	flagStepMode  = "step-mode"
	flagTypeDelay = "type-delay"
	flagMaxDelay  = "max-delay"
	flagVerbose   = "verbose"
)

// NewApp creates a CLI application.
func NewApp(out io.Writer, errOut io.Writer) *urfave.Command {
	return &urfave.Command{
		Name:        commandName,
		Usage:       "turn terminal transcripts into replayable demo bundles",
		Version:     model.ToolVersion,
		Reader:      os.Stdin,
		Writer:      out,
		ErrWriter:   errOut,
		Description: rootDescription(),
		Flags: []urfave.Flag{
			&urfave.StringFlag{
				Name:    flagOutput,
				Aliases: []string{"o"},
				Usage:   "Output format for data-producing commands: text, json, yaml",
				Value:   outputText,
				Sources: urfave.EnvVars("TERMINOS_OUTPUT"),
				Action: func(_ context.Context, _ *urfave.Command, value string) error {
					return validateOutput(value)
				},
			},
			&urfave.BoolFlag{
				Name:    flagVerbose,
				Usage:   "Include additional diagnostics when available",
				Sources: urfave.EnvVars("TERMINOS_VERBOSE"),
			},
			&urfave.BoolFlag{
				Name:    flagDebug,
				Usage:   "Enable debug diagnostics",
				Sources: urfave.EnvVars("TERMINOS_DEBUG"),
			},
			&urfave.BoolFlag{
				Name:  flagNoColor,
				Usage: "Disable colored output",
			},
		},
		Commands: []*urfave.Command{
			newInitCommand(),
			newImportCommand(),
			newCaptureCommand(),
			newPlayCommand(),
			newRenderCommand(),
			newValidateCommand(),
			newListStepsCommand(),
		},
	}
}

func newInitCommand() *urfave.Command {
	return &urfave.Command{
		Name:      "init",
		Usage:     "create a starter demo bundle",
		UsageText: "terminos init [command options] <bundle>",
		ArgsUsage: "<bundle>",
		Description: strings.TrimSpace(`
Create a bundle directory containing demo.yaml, metadata.json, captures/, and assets/.

AI_CONTEXT:
  Use --dry-run --output json to preview filesystem writes.
  Use --force to overwrite an existing demo.yaml.
  Exit code 0 means success; 2 means usage/config error; 6 means filesystem error.
`),
		Flags: []urfave.Flag{
			&urfave.BoolFlag{
				Name:  flagDryRun,
				Usage: "Preview the bundle files without writing them",
			},
			&urfave.BoolFlag{
				Name:  flagForce,
				Usage: "Overwrite demo.yaml if it already exists",
			},
		},
		Action: runInit,
	}
}

func newRenderCommand() *urfave.Command {
	return &urfave.Command{
		Name:      "render",
		Usage:     "export captured demos",
		UsageText: "terminos render [command options] <bundle>",
		ArgsUsage: "<bundle>",
		Description: strings.TrimSpace(`
Render captures into markdown, asciinema v2 cast, or shell script output.

AI_CONTEXT:
  Run capture or import before render so captures/*.jsonl exists.
  Use --format markdown|asciinema|script.
  Use --out path to write a file, or omit --out to write the artifact to stdout.
`),
		Flags: []urfave.Flag{
			&urfave.StringFlag{
				Name:  flagFormat,
				Usage: "Render format: markdown, asciinema, script",
				Value: render.FormatMarkdown,
			},
			&urfave.StringFlag{
				Name:  flagOut,
				Usage: "Write rendered artifact to file instead of stdout",
			},
		},
		Action: runRender,
	}
}

func newImportCommand() *urfave.Command {
	return &urfave.Command{
		Name:      "import",
		Usage:     "convert a pasted terminal transcript into captures",
		UsageText: "terminos import [command options] <bundle> --from transcript.txt",
		ArgsUsage: "<bundle>",
		Description: strings.TrimSpace(`
Parse prompt-delimited transcript text into manifest steps and capture JSONL files.

AI_CONTEXT:
  Use --from to provide the transcript file.
  By default imported steps replace the manifest steps; use --append to keep existing steps.
  Use --output json for parseable import results.
`),
		Flags: []urfave.Flag{
			&urfave.StringFlag{
				Name:     flagFrom,
				Usage:    "Read pasted terminal transcript from file",
				Required: true,
			},
			&urfave.BoolFlag{
				Name:  flagAppend,
				Usage: "Append imported steps instead of replacing manifest steps",
			},
		},
		Action: runImport,
	}
}

func newCaptureCommand() *urfave.Command {
	return &urfave.Command{
		Name:      "capture",
		Usage:     "execute demo steps and record captures",
		UsageText: "terminos capture [command options] <bundle>",
		ArgsUsage: "<bundle>",
		Description: strings.TrimSpace(`
Execute each manifest step with the configured shell and write captures/*.jsonl.

AI_CONTEXT:
  Use --output json for parseable capture results.
  Commands are non-interactive in this MVP.
  Exit code 0 means all steps matched expect_exit; 3 means manifest validation failed; 4 means a captured command had an unexpected exit; 5 means timeout.
`),
		Action: runCapture,
	}
}

func newPlayCommand() *urfave.Command {
	return &urfave.Command{
		Name:      "play",
		Usage:     "replay captured terminal steps",
		UsageText: "terminos play [command options] <bundle>",
		ArgsUsage: "<bundle>",
		Description: strings.TrimSpace(`
Replay capture JSONL files in manifest order with prompt, command, and output.

AI_CONTEXT:
  Run capture before play so captures/*.jsonl exists.
  Use --no-delay for deterministic automation, or --step-mode none --type-delay 0s.
  Use --speed, --type-delay, --max-delay, and --step-mode to override playback manifest defaults.
  Playback writes terminal text to stdout; operational errors go to stderr.
`),
		Flags: []urfave.Flag{
			&urfave.BoolFlag{
				Name:  flagNoDelay,
				Usage: "Disable typewriter delay during playback",
			},
			&urfave.FloatFlag{
				Name:  flagSpeed,
				Usage: "Playback speed multiplier; values above 1 are faster",
				Value: 1,
			},
			&urfave.StringFlag{
				Name:  flagStepMode,
				Usage: "Playback stepping mode override: enter, auto, none",
			},
			&urfave.DurationFlag{
				Name:  flagTypeDelay,
				Usage: "Delay between typed command characters, such as 18ms or 0s",
			},
			&urfave.DurationFlag{
				Name:  flagMaxDelay,
				Usage: "Maximum auto playback delay between output events",
			},
		},
		Action: runPlay,
	}
}

func newValidateCommand() *urfave.Command {
	return &urfave.Command{
		Name:      "validate",
		Usage:     "validate a demo bundle manifest",
		UsageText: "terminos validate [command options] <bundle>",
		ArgsUsage: "<bundle>",
		Description: strings.TrimSpace(`
Validate demo.yaml, metadata.json, required fields, step IDs, redaction patterns, and playback defaults.

AI_CONTEXT:
  Use --output json for parseable validation results.
  Use --captures after capture/import to verify capture files exist and match manifest steps.
  Diagnostics are written to stdout in the requested output format; operational errors go to stderr.
  Exit code 0 means valid; 3 means validation failed; 6 means files were missing or unreadable.
`),
		Flags: []urfave.Flag{
			&urfave.BoolFlag{
				Name:  flagCaptures,
				Usage: "Also validate capture files for each manifest step",
			},
		},
		Action: runValidate,
	}
}

func newListStepsCommand() *urfave.Command {
	return &urfave.Command{
		Name:      "list-steps",
		Usage:     "list the steps in a demo bundle",
		UsageText: "terminos list-steps [command options] <bundle>",
		ArgsUsage: "<bundle>",
		Description: strings.TrimSpace(`
List step IDs, titles, commands, expected exit codes, and capture file paths.

AI_CONTEXT:
  Use --output json for a stable array of step objects.
  Empty demos are validation failures, not empty successful results.
  Exit code 0 means success; 3 means validation failed; 6 means files were missing or unreadable.
`),
		Action: runListSteps,
	}
}

func runInit(_ context.Context, cmd *urfave.Command) error {
	path, err := requiredBundleArg(cmd)
	if err != nil {
		return err
	}

	result, err := bundle.Init(path, cmd.Bool(flagForce), cmd.Bool(flagDryRun))
	if err != nil {
		return err
	}

	format := cmd.String(flagOutput)
	if result.DryRun {
		return writeLine(cmd.Root().Writer, format, "would create "+result.Bundle.Manifest, result)
	}
	return writeLine(cmd.Root().Writer, format, "created "+result.Bundle.Manifest, result)
}

func runImport(_ context.Context, cmd *urfave.Command) error {
	path, err := requiredBundleArg(cmd)
	if err != nil {
		return err
	}

	result, err := importer.Import(path, importer.Options{
		SourcePath: cmd.String(flagFrom),
		Append:     cmd.Bool(flagAppend),
	})
	if err != nil {
		return err
	}
	return writeImportResult(cmd.Root().Writer, cmd.String(flagOutput), result)
}

func runCapture(ctx context.Context, cmd *urfave.Command) error {
	resolved, demo, err := loadValidBundle(cmd)
	if err != nil {
		return err
	}

	result, err := capture.Run(ctx, resolved, demo)
	if writeErr := writeCaptureResult(cmd.Root().Writer, cmd.String(flagOutput), result); writeErr != nil {
		return writeErr
	}
	return err
}

func runPlay(ctx context.Context, cmd *urfave.Command) error {
	resolved, demo, err := loadValidBundle(cmd)
	if err != nil {
		return err
	}
	if cmd.IsSet(flagSpeed) && cmd.Float64(flagSpeed) <= 0 {
		return apperror.New(apperror.CodeUsage, "--speed must be greater than zero")
	}

	return replay.Play(ctx, resolved, demo, cmd.Root().Writer, replay.Options{
		NoDelay:   cmd.Bool(flagNoDelay),
		Speed:     cmd.Float64(flagSpeed),
		StepMode:  cmd.String(flagStepMode),
		TypeDelay: durationOverride(cmd, flagTypeDelay),
		MaxDelay:  durationOverride(cmd, flagMaxDelay),
		Input:     cmd.Root().Reader,
		Prompt:    cmd.Root().ErrWriter,
	})
}

func durationOverride(cmd *urfave.Command, name string) *time.Duration {
	if !cmd.IsSet(name) {
		return nil
	}
	value := cmd.Duration(name)
	return &value
}

func runRender(ctx context.Context, cmd *urfave.Command) error {
	resolved, demo, err := loadValidBundle(cmd)
	if err != nil {
		return err
	}

	artifact, err := render.Render(ctx, resolved, demo, cmd.String(flagFormat))
	if err != nil {
		return err
	}
	outPath := strings.TrimSpace(cmd.String(flagOut))
	if outPath == "" || outPath == "-" {
		if _, err := cmd.Root().Writer.Write(artifact); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "write rendered output", err)
		}
		return nil
	}
	if err := writeFile(outPath, artifact); err != nil {
		return err
	}
	if cmd.String(flagOutput) == outputText {
		if _, err := fmt.Fprintf(cmd.Root().Writer, "rendered %s\n", outPath); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "write text output", err)
		}
		return nil
	}
	return writeStructured(cmd.Root().Writer, cmd.String(flagOutput), map[string]string{"path": outPath})
}

func runValidate(_ context.Context, cmd *urfave.Command) error {
	path, err := requiredBundleArg(cmd)
	if err != nil {
		return err
	}

	resolved, demo, err := bundle.Load(path)
	if err != nil {
		return err
	}

	result := newValidationResult(resolved, demo)
	if cmd.Bool(flagCaptures) {
		result.Errors = append(result.Errors, validate.Captures(resolved, demo)...)
		result.Valid = len(result.Errors) == 0
	}
	if err := writeValidationResult(cmd.Root().Writer, cmd.String(flagOutput), result); err != nil {
		return err
	}
	if !result.Valid {
		return apperror.New(apperror.CodeValidation, "demo bundle validation failed")
	}
	return nil
}

func runListSteps(_ context.Context, cmd *urfave.Command) error {
	path, err := requiredBundleArg(cmd)
	if err != nil {
		return err
	}

	resolved, demo, err := bundle.Load(path)
	if err != nil {
		return err
	}

	validation := newValidationResult(resolved, demo)
	if !validation.Valid {
		if err := writeValidationResult(cmd.Root().Writer, cmd.String(flagOutput), validation); err != nil {
			return err
		}
		return apperror.New(apperror.CodeValidation, "demo bundle validation failed")
	}

	steps := demo.StepSummaries()
	if cmd.String(flagOutput) != outputText {
		return writeStructured(cmd.Root().Writer, cmd.String(flagOutput), steps)
	}

	for _, step := range steps {
		if step.Title != "" {
			if _, err := fmt.Fprintf(cmd.Root().Writer, "%s\t%s\t%s\n", step.ID, step.Title, step.Capture); err != nil {
				return apperror.Wrap(apperror.CodeInternal, "write text output", err)
			}
			continue
		}
		if _, err := fmt.Fprintf(cmd.Root().Writer, "%s\t%s\n", step.ID, step.Capture); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "write text output", err)
		}
	}
	return nil
}

func requiredBundleArg(cmd *urfave.Command) (string, error) {
	if cmd.NArg() != 1 {
		return "", apperror.Newf(apperror.CodeUsage, "%s expects exactly one bundle path argument", cmd.FullName())
	}
	return cmd.Args().First(), nil
}

func loadValidBundle(cmd *urfave.Command) (bundle.Bundle, model.Demo, error) {
	path, err := requiredBundleArg(cmd)
	if err != nil {
		return bundle.Bundle{}, model.Demo{}, err
	}

	resolved, demo, err := bundle.Load(path)
	if err != nil {
		return bundle.Bundle{}, model.Demo{}, err
	}

	validation := newValidationResult(resolved, demo)
	if !validation.Valid {
		if err := writeValidationResult(cmd.Root().Writer, cmd.String(flagOutput), validation); err != nil {
			return bundle.Bundle{}, model.Demo{}, err
		}
		return bundle.Bundle{}, model.Demo{}, apperror.New(apperror.CodeValidation, "demo bundle validation failed")
	}
	return resolved, demo, nil
}

func rootDescription() string {
	return strings.TrimSpace(`
Terminos records terminal demos as plain directories that are easy to review in Git.

AI_CONTEXT:
  Prefer bundle-path commands such as "terminos validate demos/hello --output json".
  Data requested by the user is written to stdout; diagnostics and errors are written to stderr.
  Use --output json for parseable output and --dry-run where available before writing files.
  Stable exit codes: 0 success, 1 internal error, 2 usage/config/schema error, 3 validation failed, 4 captured command failed, 5 timeout, 6 filesystem error, 7 redaction policy failure, 130 interrupted.
`)
}

type validationResult struct {
	Valid         bool               `json:"valid" yaml:"valid"`
	BundlePath    string             `json:"bundle_path" yaml:"bundle_path"`
	ManifestPath  string             `json:"manifest_path" yaml:"manifest_path"`
	SchemaVersion string             `json:"schema_version" yaml:"schema_version"`
	StepCount     int                `json:"step_count" yaml:"step_count"`
	Errors        []model.Diagnostic `json:"errors" yaml:"errors"`
	Warnings      []model.Diagnostic `json:"warnings" yaml:"warnings"`
}

func newValidationResult(resolved bundle.Bundle, demo model.Demo) validationResult {
	diagnostics := demo.Validate()
	diagnostics = append(diagnostics, validate.Metadata(resolved)...)
	if diagnostics == nil {
		diagnostics = []model.Diagnostic{}
	}
	return validationResult{
		Valid:         len(diagnostics) == 0,
		BundlePath:    resolved.Path,
		ManifestPath:  resolved.Manifest,
		SchemaVersion: demo.SchemaVersion,
		StepCount:     len(demo.Steps),
		Errors:        diagnostics,
		Warnings:      []model.Diagnostic{},
	}
}

func writeValidationResult(out io.Writer, format string, result validationResult) error {
	if format != outputText {
		return writeStructured(out, format, result)
	}
	if result.Valid {
		_, err := fmt.Fprintf(out, "valid: %s (%d step(s))\n", result.ManifestPath, result.StepCount)
		if err != nil {
			return apperror.Wrap(apperror.CodeInternal, "write text output", err)
		}
		return nil
	}

	if _, err := fmt.Fprintf(out, "invalid: %s\n", result.ManifestPath); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "write text output", err)
	}
	for _, item := range result.Errors {
		if _, err := fmt.Fprintf(out, "- %s: %s\n", item.Path, item.Message); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "write text output", err)
		}
	}
	return nil
}

func writeCaptureResult(out io.Writer, format string, result capture.Result) error {
	if format != outputText {
		return writeStructured(out, format, result)
	}
	for _, item := range result.Captures {
		if _, err := fmt.Fprintf(out, "captured %s -> %s (exit %d, %dms)\n", item.StepID, item.Capture, item.ExitCode, item.DurationMS); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "write text output", err)
		}
	}
	return nil
}

func writeImportResult(out io.Writer, format string, result importer.Result) error {
	if format != outputText {
		return writeStructured(out, format, result)
	}
	for _, item := range result.Steps {
		if _, err := fmt.Fprintf(out, "imported %s -> %s\n", item.ID, item.Capture); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "write text output", err)
		}
	}
	return nil
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return apperror.Wrapf(apperror.CodeFilesystem, err, "create output directory %s", filepath.Dir(path))
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return apperror.Wrapf(apperror.CodeFilesystem, err, "write output file %s", path)
	}
	return nil
}
