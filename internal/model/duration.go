// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"encoding/json"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration is a YAML/JSON friendly wrapper around time.Duration.
type Duration time.Duration

// NewDuration wraps a time.Duration.
func NewDuration(value time.Duration) Duration {
	return Duration(value)
}

// ToDuration returns the underlying time.Duration.
func (d Duration) ToDuration() time.Duration {
	return time.Duration(d)
}

// String returns the Go duration string.
func (d Duration) String() string {
	return time.Duration(d).String()
}

// MarshalYAML emits durations as readable strings, such as "30s".
func (d Duration) MarshalYAML() (any, error) {
	return d.String(), nil
}

// UnmarshalYAML parses Go duration strings from YAML.
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var raw string
	if err := value.Decode(&raw); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", raw, err)
	}
	*d = Duration(parsed)
	return nil
}

// MarshalJSON emits durations as readable strings, such as "30s".
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// UnmarshalJSON parses Go duration strings from JSON.
func (d *Duration) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", raw, err)
	}
	*d = Duration(parsed)
	return nil
}
