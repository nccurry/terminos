// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"

	"github.com/nccurry/terminos/internal/apperror"
)

const (
	outputJSON = "json"
	outputText = "text"
	outputYAML = "yaml"
)

func validateOutput(format string) error {
	switch format {
	case outputText, outputJSON, outputYAML:
		return nil
	default:
		return apperror.Newf(apperror.CodeUsage, "invalid --output value %q; expected text, json, or yaml", format)
	}
}

func writeStructured(out io.Writer, format string, value any) error {
	switch format {
	case outputJSON:
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(value); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "write JSON output", err)
		}
		return nil
	case outputYAML:
		encoder := yaml.NewEncoder(out)
		encoder.SetIndent(2)
		if err := encoder.Encode(value); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "write YAML output", err)
		}
		if err := encoder.Close(); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "write YAML output", err)
		}
		return nil
	case outputText:
		return apperror.New(apperror.CodeInternal, "text output needs a command-specific writer")
	default:
		return validateOutput(format)
	}
}

func writeLine(out io.Writer, format string, text string, value any) error {
	if format != outputText {
		return writeStructured(out, format, value)
	}
	if _, err := fmt.Fprintln(out, text); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "write text output", err)
	}
	return nil
}
