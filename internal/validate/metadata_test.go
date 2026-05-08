// SPDX-License-Identifier: AGPL-3.0-or-later

package validate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nccurry/terminos/internal/bundle"
)

func TestMetadataReportsMissingAndInvalidFiles(t *testing.T) {
	t.Parallel()

	resolved := bundle.Resolve(t.TempDir())
	if diagnostics := Metadata(resolved); !hasDiagnostic(diagnostics, "metadata_missing") {
		t.Fatalf("expected missing metadata diagnostic, got %#v", diagnostics)
	}

	if err := os.MkdirAll(resolved.Path, 0o755); err != nil {
		t.Fatalf("create bundle dir: %v", err)
	}
	if err := os.WriteFile(resolved.Metadata, []byte("{"), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}
	if diagnostics := Metadata(resolved); !hasDiagnostic(diagnostics, "metadata_invalid") {
		t.Fatalf("expected invalid metadata diagnostic, got %#v", diagnostics)
	}
}

func TestMetadataValidatesRequiredFields(t *testing.T) {
	t.Parallel()

	resolved := bundle.Resolve(t.TempDir())
	if err := os.MkdirAll(resolved.Path, 0o755); err != nil {
		t.Fatalf("create bundle dir: %v", err)
	}
	invalid := []byte(`{
  "schema_version": "9.9",
  "tool_version": "",
  "updated_at": "not-a-date",
  "host": {
    "os": "",
    "arch": "",
    "go": ""
  }
}`)
	if err := os.WriteFile(filepath.Join(resolved.Path, "metadata.json"), invalid, 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	diagnostics := Metadata(resolved)
	for _, code := range []string{
		"metadata_schema_version",
		"metadata_tool_version_required",
		"metadata_updated_at",
		"metadata_host_os_required",
		"metadata_host_arch_required",
		"metadata_host_go_required",
	} {
		if !hasDiagnostic(diagnostics, code) {
			t.Fatalf("expected %s diagnostic, got %#v", code, diagnostics)
		}
	}
}

func TestMetadataAcceptsWrittenMetadata(t *testing.T) {
	t.Parallel()

	resolved := bundle.Resolve(t.TempDir())
	if err := bundle.WriteMetadata(resolved); err != nil {
		t.Fatalf("write metadata: %v", err)
	}
	if diagnostics := Metadata(resolved); len(diagnostics) != 0 {
		t.Fatalf("expected valid metadata, got %#v", diagnostics)
	}
}
