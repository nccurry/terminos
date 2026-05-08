// SPDX-License-Identifier: AGPL-3.0-or-later

package bundle

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nccurry/terminos/internal/apperror"
	"github.com/nccurry/terminos/internal/model"
)

func TestInitCreatesLoadableBundle(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "hello")
	result, err := Init(dir, false, false)
	if err != nil {
		t.Fatalf("init bundle: %v", err)
	}

	for _, path := range []string{result.Bundle.Manifest, result.Bundle.Metadata, result.Bundle.CapturesDir, result.Bundle.AssetsDir} {
		if _, statErr := os.Stat(path); statErr != nil {
			t.Fatalf("expected %s to exist: %v", path, statErr)
		}
	}

	resolved, demo, err := Load(dir)
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	if resolved.Manifest != result.Bundle.Manifest {
		t.Fatalf("expected manifest %q, got %q", result.Bundle.Manifest, resolved.Manifest)
	}
	if diagnostics := demo.Validate(); len(diagnostics) != 0 {
		t.Fatalf("expected valid starter manifest, got %#v", diagnostics)
	}

	raw, err := os.ReadFile(result.Bundle.Metadata)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	var metadata model.Metadata
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatalf("decode metadata: %v\n%s", err, string(raw))
	}
	if metadata.SchemaVersion != model.MetadataSchemaVersion || metadata.ToolVersion != model.ToolVersion {
		t.Fatalf("unexpected metadata %#v", metadata)
	}
	if metadata.Host.Hostname != "" {
		t.Fatalf("metadata should not include local hostname, got %#v", metadata.Host)
	}
}

func TestInitDryRunDoesNotWrite(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "dry")
	result, err := Init(dir, false, true)
	if err != nil {
		t.Fatalf("dry-run init bundle: %v", err)
	}
	if !result.DryRun {
		t.Fatal("expected dry-run result")
	}
	if _, err := os.Stat(result.Bundle.Manifest); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no manifest to be written, got %v", err)
	}
}

func TestInitRefusesExistingManifestWithoutForce(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "existing")
	if _, err := Init(dir, false, false); err != nil {
		t.Fatalf("first init bundle: %v", err)
	}

	_, err := Init(dir, false, false)
	if apperror.ExitCode(err) != apperror.ExitUsage {
		t.Fatalf("expected usage exit, got %d from %v", apperror.ExitCode(err), err)
	}
}

func TestLoadReportsMissingManifestAsFilesystemError(t *testing.T) {
	t.Parallel()

	_, _, err := Load(filepath.Join(t.TempDir(), "missing"))
	if apperror.ExitCode(err) != apperror.ExitFilesystem {
		t.Fatalf("expected filesystem exit, got %d from %v", apperror.ExitCode(err), err)
	}
}

func TestLoadReportsInvalidYAMLAsUsageError(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "invalid")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create bundle dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ManifestFile), []byte("schema_version: ["), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	_, _, err := Load(dir)
	if apperror.ExitCode(err) != apperror.ExitUsage {
		t.Fatalf("expected usage exit, got %d from %v", apperror.ExitCode(err), err)
	}
}

func TestResolveAcceptsManifestPath(t *testing.T) {
	t.Parallel()

	resolved := Resolve(filepath.Join("demos", "hello", ManifestFile))
	if resolved.Path != filepath.Join("demos", "hello") {
		t.Fatalf("unexpected bundle path %q", resolved.Path)
	}
	if resolved.Manifest != filepath.Join("demos", "hello", ManifestFile) {
		t.Fatalf("unexpected manifest path %q", resolved.Manifest)
	}
	if resolved.Metadata != filepath.Join("demos", "hello", MetadataFile) {
		t.Fatalf("unexpected metadata path %q", resolved.Metadata)
	}
}
