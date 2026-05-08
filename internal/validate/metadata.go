// SPDX-License-Identifier: AGPL-3.0-or-later

package validate

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/nccurry/terminos/internal/bundle"
	"github.com/nccurry/terminos/internal/model"
)

// Metadata validates metadata.json for a bundle.
func Metadata(resolved bundle.Bundle) []model.Diagnostic {
	raw, err := os.ReadFile(resolved.Metadata)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []model.Diagnostic{diagnostic("metadata_missing", "metadata", "metadata.json is missing")}
		}
		return []model.Diagnostic{diagnostic("metadata_unreadable", "metadata", "metadata.json is not readable")}
	}

	var metadata model.Metadata
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return []model.Diagnostic{diagnostic("metadata_invalid", "metadata", "metadata.json must be valid JSON")}
	}

	var diagnostics []model.Diagnostic
	if metadata.SchemaVersion != model.MetadataSchemaVersion {
		diagnostics = append(diagnostics, diagnostic("metadata_schema_version", "metadata.schema_version", "metadata schema_version must be \"1.0\""))
	}
	if strings.TrimSpace(metadata.ToolVersion) == "" {
		diagnostics = append(diagnostics, diagnostic("metadata_tool_version_required", "metadata.tool_version", "metadata tool_version is required"))
	}
	if _, err := time.Parse(time.RFC3339, metadata.UpdatedAt); err != nil {
		diagnostics = append(diagnostics, diagnostic("metadata_updated_at", "metadata.updated_at", "metadata updated_at must be RFC3339"))
	}
	if strings.TrimSpace(metadata.Host.OS) == "" {
		diagnostics = append(diagnostics, diagnostic("metadata_host_os_required", "metadata.host.os", "metadata host os is required"))
	}
	if strings.TrimSpace(metadata.Host.Arch) == "" {
		diagnostics = append(diagnostics, diagnostic("metadata_host_arch_required", "metadata.host.arch", "metadata host arch is required"))
	}
	if strings.TrimSpace(metadata.Host.Go) == "" {
		diagnostics = append(diagnostics, diagnostic("metadata_host_go_required", "metadata.host.go", "metadata host go version is required"))
	}
	return diagnostics
}
