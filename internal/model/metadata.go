// SPDX-License-Identifier: AGPL-3.0-or-later

package model

// ToolVersion is the version reported by the CLI and bundle metadata.
var ToolVersion = "0.1.0-dev"

// MetadataSchemaVersion is the current metadata.json schema version.
const MetadataSchemaVersion = "1.0"

// Metadata describes the tool and runtime that last initialized or updated a bundle.
type Metadata struct {
	SchemaVersion string       `json:"schema_version"`
	ToolVersion   string       `json:"tool_version"`
	UpdatedAt     string       `json:"updated_at"`
	Host          MetadataHost `json:"host"`
}

// MetadataHost describes non-identifying host runtime details.
type MetadataHost struct {
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Go       string `json:"go"`
	Hostname string `json:"hostname,omitempty"`
}
