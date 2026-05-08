// SPDX-License-Identifier: AGPL-3.0-or-later

// Package render exports captured demos into shareable artifact formats.
package render

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nccurry/terminos/internal/apperror"
	"github.com/nccurry/terminos/internal/bundle"
	"github.com/nccurry/terminos/internal/capture"
	"github.com/nccurry/terminos/internal/model"
	"github.com/nccurry/terminos/internal/redact"
)

// Supported render formats.
const (
	FormatAsciinema = "asciinema"
	FormatMarkdown  = "markdown"
	FormatScript    = "script"
)

// Render returns a rendered demo artifact.
func Render(ctx context.Context, resolved bundle.Bundle, demo model.Demo, format string) ([]byte, error) {
	switch format {
	case FormatMarkdown, FormatAsciinema, FormatScript:
	default:
		return nil, apperror.Newf(apperror.CodeUsage, "invalid --format value %q; expected asciinema, markdown, or script", format)
	}

	redactor, err := redact.New(demo.Redactions)
	if err != nil {
		return nil, err
	}

	switch format {
	case FormatMarkdown:
		return renderMarkdown(ctx, resolved, demo, redactor)
	case FormatAsciinema:
		return renderAsciinema(ctx, resolved, demo, redactor)
	case FormatScript:
		return renderScript(demo, redactor), nil
	default:
		return nil, apperror.Newf(apperror.CodeInternal, "unhandled render format %q", format)
	}
}

func renderMarkdown(ctx context.Context, resolved bundle.Bundle, demo model.Demo, redactor redact.RuleSet) ([]byte, error) {
	var out bytes.Buffer
	fmt.Fprintf(&out, "# %s\n\n", redactor.Apply(demo.Name))
	if strings.TrimSpace(demo.Summary) != "" {
		fmt.Fprintf(&out, "%s\n\n", redactor.Apply(demo.Summary))
	}

	for _, step := range demo.Steps {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		events, err := capture.ReadEvents(capture.CapturePath(resolved, step))
		if err != nil {
			return nil, err
		}

		title := step.Title
		if title == "" {
			title = step.ID
		}
		fmt.Fprintf(&out, "## %s\n\n", redactor.Apply(title))
		out.WriteString("```console\n")
		fmt.Fprintf(&out, "%s%s\n", redactor.Apply(demo.Defaults.Prompt), displayCommand(step, events, redactor))

		writeOutputEvents(&out, events, redactor)
		out.WriteString("```\n\n")
	}
	return out.Bytes(), nil
}

func renderAsciinema(ctx context.Context, resolved bundle.Bundle, demo model.Demo, redactor redact.RuleSet) ([]byte, error) {
	var out bytes.Buffer
	header := map[string]any{
		"version":   2,
		"width":     demo.Defaults.Terminal.Cols,
		"height":    demo.Defaults.Terminal.Rows,
		"timestamp": time.Now().Unix(),
		"env": map[string]string{
			"SHELL": redactor.Apply(demo.Defaults.Shell),
		},
	}
	if err := writeJSONLine(&out, header); err != nil {
		return nil, err
	}

	var elapsed float64
	for _, step := range demo.Steps {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		events, err := capture.ReadEvents(capture.CapturePath(resolved, step))
		if err != nil {
			return nil, err
		}
		if err := writeJSONLine(&out, []any{elapsed, "o", redactor.Apply(demo.Defaults.Prompt) + displayCommand(step, events, redactor) + "\n"}); err != nil {
			return nil, err
		}

		stepStart := elapsed
		lastElapsed := stepStart
		for _, event := range events {
			switch event.Type {
			case model.EventStdout, model.EventStderr:
				eventElapsed := stepStart + seconds(event.TimeMS)
				if eventElapsed < lastElapsed {
					eventElapsed = lastElapsed
				}
				lastElapsed = eventElapsed
				if err := writeJSONLine(&out, []any{eventElapsed, "o", redactor.Apply(event.Data)}); err != nil {
					return nil, err
				}
			}
		}
		elapsed = lastElapsed + 0.25
	}
	return out.Bytes(), nil
}

func renderScript(demo model.Demo, redactor redact.RuleSet) []byte {
	var out bytes.Buffer
	out.WriteString("#!/usr/bin/env bash\nset -euo pipefail\n\n")
	for _, step := range demo.Steps {
		if step.Title != "" {
			fmt.Fprintf(&out, "echo %s\n", shellQuote("# "+redactor.Apply(step.Title)))
		}
		fmt.Fprintln(&out, redactor.Apply(step.Command))
		fmt.Fprintln(&out)
	}
	return out.Bytes()
}

func displayCommand(step model.Step, events []model.CaptureEvent, redactor redact.RuleSet) string {
	command := step.Command
	for _, event := range events {
		if event.Type == model.EventStart && event.Command != "" {
			command = event.Command
			break
		}
	}
	return redactor.Apply(command)
}

func writeOutputEvents(out *bytes.Buffer, events []model.CaptureEvent, redactor redact.RuleSet) {
	for _, event := range events {
		switch event.Type {
		case model.EventStdout, model.EventStderr:
			out.WriteString(redactor.Apply(event.Data))
		}
	}
}

func writeJSONLine(out *bytes.Buffer, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "encode asciinema output", err)
	}
	out.Write(encoded)
	out.WriteByte('\n')
	return nil
}

func seconds(ms int64) float64 {
	if ms <= 0 {
		return 0.001
	}
	return float64(ms) / 1000
}

func shellQuote(value string) string {
	return strconv.Quote(value)
}
