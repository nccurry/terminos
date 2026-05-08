// SPDX-License-Identifier: AGPL-3.0-or-later

// Package model defines manifest and capture metadata structures.
package model

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ManifestSchemaVersion is the current demo.yaml schema version.
const ManifestSchemaVersion = "1.0"

const (
	defaultShell     = "bash"
	defaultCWD       = "."
	defaultPrompt    = "$ "
	defaultCols      = 120
	defaultRows      = 32
	defaultStepMode  = "enter"
	defaultTypeDelay = 18 * time.Millisecond
	defaultMaxDelay  = 2 * time.Second
	defaultTimeout   = 30 * time.Second
)

var stepIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// Demo is the root demo.yaml manifest.
type Demo struct {
	SchemaVersion string      `json:"schema_version" yaml:"schema_version"`
	Name          string      `json:"name" yaml:"name"`
	Summary       string      `json:"summary,omitempty" yaml:"summary,omitempty"`
	Defaults      Defaults    `json:"defaults" yaml:"defaults"`
	Playback      Playback    `json:"playback" yaml:"playback"`
	Redactions    []Redaction `json:"redactions,omitempty" yaml:"redactions,omitempty"`
	Steps         []Step      `json:"steps" yaml:"steps"`
}

// Defaults contains command and terminal defaults for the demo.
type Defaults struct {
	Shell    string   `json:"shell" yaml:"shell"`
	CWD      string   `json:"cwd" yaml:"cwd"`
	Prompt   string   `json:"prompt" yaml:"prompt"`
	Timeout  Duration `json:"timeout" yaml:"timeout"`
	Terminal Terminal `json:"terminal" yaml:"terminal"`

	shellSet    bool
	cwdSet      bool
	promptSet   bool
	timeoutSet  bool
	terminalSet bool
}

// Terminal contains the terminal dimensions used for playback and rendering.
type Terminal struct {
	Cols int `json:"cols" yaml:"cols"`
	Rows int `json:"rows" yaml:"rows"`

	colsSet bool
	rowsSet bool
}

// Playback contains default playback behavior.
type Playback struct {
	StepMode  string   `json:"step_mode" yaml:"step_mode"`
	TypeDelay Duration `json:"type_delay" yaml:"type_delay"`
	MaxDelay  Duration `json:"max_delay" yaml:"max_delay"`

	stepModeSet  bool
	typeDelaySet bool
	maxDelaySet  bool
}

// Redaction replaces matching text before output is stored or rendered.
type Redaction struct {
	Pattern     string `json:"pattern" yaml:"pattern"`
	Replacement string `json:"replacement" yaml:"replacement"`
}

// Step describes one recorded command in a demo.
type Step struct {
	ID         string `json:"id" yaml:"id"`
	Title      string `json:"title,omitempty" yaml:"title,omitempty"`
	Command    string `json:"command" yaml:"command"`
	Capture    string `json:"capture,omitempty" yaml:"capture,omitempty"`
	ExpectExit int    `json:"expect_exit" yaml:"expect_exit"`
}

// Diagnostic describes one validation issue.
type Diagnostic struct {
	Severity string `json:"severity" yaml:"severity"`
	Code     string `json:"code" yaml:"code"`
	Path     string `json:"path" yaml:"path"`
	Message  string `json:"message" yaml:"message"`
}

// StepSummary is the stable machine-readable shape returned by list-steps.
type StepSummary struct {
	ID         string `json:"id" yaml:"id"`
	Title      string `json:"title,omitempty" yaml:"title,omitempty"`
	Command    string `json:"command" yaml:"command"`
	Capture    string `json:"capture" yaml:"capture"`
	ExpectExit int    `json:"expect_exit" yaml:"expect_exit"`
}

// DefaultDemo returns a starter manifest.
func DefaultDemo(name string) Demo {
	name = strings.TrimSpace(name)
	if name == "" || name == "." {
		name = "terminos-demo"
	}

	return Demo{
		SchemaVersion: ManifestSchemaVersion,
		Name:          name,
		Summary:       "A starter terminos demo.",
		Defaults: Defaults{
			Shell:   defaultShell,
			CWD:     defaultCWD,
			Prompt:  defaultPrompt,
			Timeout: NewDuration(defaultTimeout),
			Terminal: Terminal{
				Cols: defaultCols,
				Rows: defaultRows,
			},
		},
		Playback: Playback{
			StepMode:  defaultStepMode,
			TypeDelay: NewDuration(defaultTypeDelay),
			MaxDelay:  NewDuration(defaultMaxDelay),
		},
		Redactions: []Redaction{
			{
				Pattern:     `/home/[^ ]+`,
				Replacement: "~",
			},
		},
		Steps: []Step{
			{
				ID:         "hello",
				Title:      "Print a friendly message",
				Command:    `printf 'hello from terminos\n'`,
				ExpectExit: 0,
			},
		},
	}
}

// UnmarshalYAML tracks whether default fields were omitted or explicitly set.
func (d *Defaults) UnmarshalYAML(value *yaml.Node) error {
	type defaults Defaults
	var raw struct {
		Shell    *string   `yaml:"shell"`
		CWD      *string   `yaml:"cwd"`
		Prompt   *string   `yaml:"prompt"`
		Timeout  *Duration `yaml:"timeout"`
		Terminal *Terminal `yaml:"terminal"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}

	*d = Defaults(defaults{})
	if raw.Shell != nil {
		d.Shell = *raw.Shell
		d.shellSet = true
	}
	if raw.CWD != nil {
		d.CWD = *raw.CWD
		d.cwdSet = true
	}
	if raw.Prompt != nil {
		d.Prompt = *raw.Prompt
		d.promptSet = true
	}
	if raw.Timeout != nil {
		d.Timeout = *raw.Timeout
		d.timeoutSet = true
	}
	if raw.Terminal != nil {
		d.Terminal = *raw.Terminal
		d.terminalSet = true
	}
	return nil
}

// UnmarshalYAML tracks whether terminal fields were omitted or explicitly set.
func (t *Terminal) UnmarshalYAML(value *yaml.Node) error {
	type terminal Terminal
	var raw struct {
		Cols *int `yaml:"cols"`
		Rows *int `yaml:"rows"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}

	*t = Terminal(terminal{})
	if raw.Cols != nil {
		t.Cols = *raw.Cols
		t.colsSet = true
	}
	if raw.Rows != nil {
		t.Rows = *raw.Rows
		t.rowsSet = true
	}
	return nil
}

// UnmarshalYAML tracks whether playback fields were omitted or explicitly set.
func (p *Playback) UnmarshalYAML(value *yaml.Node) error {
	type playback Playback
	var raw struct {
		StepMode  *string   `yaml:"step_mode"`
		TypeDelay *Duration `yaml:"type_delay"`
		MaxDelay  *Duration `yaml:"max_delay"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}

	*p = Playback(playback{})
	if raw.StepMode != nil {
		p.StepMode = *raw.StepMode
		p.stepModeSet = true
	}
	if raw.TypeDelay != nil {
		p.TypeDelay = *raw.TypeDelay
		p.typeDelaySet = true
	}
	if raw.MaxDelay != nil {
		p.MaxDelay = *raw.MaxDelay
		p.maxDelaySet = true
	}
	return nil
}

// ApplyDefaults fills omitted manifest fields with stable defaults.
func (d *Demo) ApplyDefaults() {
	if d.SchemaVersion == "" {
		d.SchemaVersion = ManifestSchemaVersion
	}
	if !d.Defaults.shellSet && d.Defaults.Shell == "" {
		d.Defaults.Shell = defaultShell
	}
	if !d.Defaults.cwdSet && d.Defaults.CWD == "" {
		d.Defaults.CWD = defaultCWD
	}
	if !d.Defaults.promptSet && d.Defaults.Prompt == "" {
		d.Defaults.Prompt = defaultPrompt
	}
	if !d.Defaults.timeoutSet && d.Defaults.Timeout == 0 {
		d.Defaults.Timeout = NewDuration(defaultTimeout)
	}
	if (!d.Defaults.terminalSet || !d.Defaults.Terminal.colsSet) && d.Defaults.Terminal.Cols == 0 {
		d.Defaults.Terminal.Cols = defaultCols
	}
	if (!d.Defaults.terminalSet || !d.Defaults.Terminal.rowsSet) && d.Defaults.Terminal.Rows == 0 {
		d.Defaults.Terminal.Rows = defaultRows
	}
	if !d.Playback.stepModeSet && d.Playback.StepMode == "" {
		d.Playback.StepMode = defaultStepMode
	}
	if !d.Playback.typeDelaySet && d.Playback.TypeDelay == 0 {
		d.Playback.TypeDelay = NewDuration(defaultTypeDelay)
	}
	if !d.Playback.maxDelaySet && d.Playback.MaxDelay == 0 {
		d.Playback.MaxDelay = NewDuration(defaultMaxDelay)
	}
}

// Validate returns all validation diagnostics for a manifest.
func (d Demo) Validate() []Diagnostic {
	var diagnostics []Diagnostic

	if d.SchemaVersion != ManifestSchemaVersion {
		diagnostics = append(diagnostics, errorDiagnostic(
			"schema_version",
			"schema_version",
			"schema_version must be \"1.0\"",
		))
	}
	if strings.TrimSpace(d.Name) == "" {
		diagnostics = append(diagnostics, errorDiagnostic("name_required", "name", "name is required"))
	}
	if strings.TrimSpace(d.Defaults.Shell) == "" {
		diagnostics = append(diagnostics, errorDiagnostic("default_shell_required", "defaults.shell", "defaults.shell is required"))
	}
	if d.Defaults.Timeout.ToDuration() <= 0 {
		diagnostics = append(diagnostics, errorDiagnostic("default_timeout_positive", "defaults.timeout", "defaults.timeout must be greater than zero"))
	}
	if d.Defaults.Terminal.Cols <= 0 {
		diagnostics = append(diagnostics, errorDiagnostic("terminal_cols_positive", "defaults.terminal.cols", "terminal cols must be greater than zero"))
	}
	if d.Defaults.Terminal.Rows <= 0 {
		diagnostics = append(diagnostics, errorDiagnostic("terminal_rows_positive", "defaults.terminal.rows", "terminal rows must be greater than zero"))
	}
	if !isStepMode(d.Playback.StepMode) {
		diagnostics = append(diagnostics, errorDiagnostic("playback_step_mode", "playback.step_mode", "playback.step_mode must be one of: auto, enter, none"))
	}
	if d.Playback.TypeDelay.ToDuration() < 0 {
		diagnostics = append(diagnostics, errorDiagnostic("playback_type_delay", "playback.type_delay", "playback.type_delay cannot be negative"))
	}
	if d.Playback.MaxDelay.ToDuration() < 0 {
		diagnostics = append(diagnostics, errorDiagnostic("playback_max_delay", "playback.max_delay", "playback.max_delay cannot be negative"))
	}

	for i, redaction := range d.Redactions {
		if strings.TrimSpace(redaction.Pattern) == "" {
			diagnostics = append(diagnostics, errorDiagnostic("redaction_pattern_required", redactionPath(i, "pattern"), "redaction pattern is required"))
			continue
		}
		if _, err := regexp.Compile(redaction.Pattern); err != nil {
			diagnostics = append(diagnostics, errorDiagnostic("redaction_pattern_invalid", redactionPath(i, "pattern"), "redaction pattern must be a valid regular expression"))
		}
	}

	if len(d.Steps) == 0 {
		diagnostics = append(diagnostics, errorDiagnostic("steps_required", "steps", "at least one step is required"))
	}

	seen := map[string]int{}
	for i, step := range d.Steps {
		diagnostics = append(diagnostics, validateStep(i, step, seen)...)
	}

	return diagnostics
}

// StepSummaries returns the stable list-steps representation.
func (d Demo) StepSummaries() []StepSummary {
	summaries := make([]StepSummary, 0, len(d.Steps))
	for _, step := range d.Steps {
		summaries = append(summaries, StepSummary{
			ID:         step.ID,
			Title:      step.Title,
			Command:    step.Command,
			Capture:    CapturePath(step),
			ExpectExit: step.ExpectExit,
		})
	}
	return summaries
}

// CapturePath returns the capture path for a step.
func CapturePath(step Step) string {
	if strings.TrimSpace(step.Capture) != "" {
		return step.Capture
	}
	return "captures/" + step.ID + ".jsonl"
}

func validateStep(index int, step Step, seen map[string]int) []Diagnostic {
	var diagnostics []Diagnostic

	stepPath := func(field string) string {
		return stepPath(index, field)
	}
	if strings.TrimSpace(step.ID) == "" {
		diagnostics = append(diagnostics, errorDiagnostic("step_id_required", stepPath("id"), "step id is required"))
	} else if !stepIDPattern.MatchString(step.ID) {
		diagnostics = append(diagnostics, errorDiagnostic("step_id_invalid", stepPath("id"), "step id must match ^[a-z][a-z0-9_-]*$"))
	} else if previous, ok := seen[step.ID]; ok {
		diagnostics = append(diagnostics, errorDiagnostic("step_id_duplicate", stepPath("id"), "step id duplicates steps["+strconv.Itoa(previous)+"].id"))
	} else {
		seen[step.ID] = index
	}

	if strings.TrimSpace(step.Command) == "" {
		diagnostics = append(diagnostics, errorDiagnostic("step_command_required", stepPath("command"), "step command is required"))
	}
	if step.ExpectExit < 0 || step.ExpectExit > 255 {
		diagnostics = append(diagnostics, errorDiagnostic("step_expect_exit_range", stepPath("expect_exit"), "step expect_exit must be between 0 and 255"))
	}

	return diagnostics
}

func isStepMode(value string) bool {
	switch value {
	case "auto", "enter", "none":
		return true
	default:
		return false
	}
}

func errorDiagnostic(code, path, message string) Diagnostic {
	return Diagnostic{
		Severity: "error",
		Code:     code,
		Path:     path,
		Message:  message,
	}
}

func redactionPath(index int, field string) string {
	return "redactions[" + strconv.Itoa(index) + "]." + field
}

func stepPath(index int, field string) string {
	return "steps[" + strconv.Itoa(index) + "]." + field
}
