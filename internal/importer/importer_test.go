// SPDX-License-Identifier: AGPL-3.0-or-later

package importer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/nccurry/terminos/internal/bundle"
	"github.com/nccurry/terminos/internal/capture"
	"github.com/nccurry/terminos/internal/model"
)

func TestParseTranscriptSplitsPromptBlocks(t *testing.T) {
	t.Parallel()

	blocks, err := ParseTranscript("$ one\nfirst\n$ two\nsecond\n", "$ ")
	if err != nil {
		t.Fatalf("parse transcript: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("expected two blocks, got %#v", blocks)
	}
	if blocks[0].Command != "one" || blocks[0].Output != "first\n" {
		t.Fatalf("unexpected first block %#v", blocks[0])
	}
	if blocks[1].Command != "two" || blocks[1].Output != "second\n" {
		t.Fatalf("unexpected second block %#v", blocks[1])
	}
}

func TestImportCreatesBundleManifestAndCaptures(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	transcript := filepath.Join(root, "transcript.txt")
	if err := os.WriteFile(transcript, []byte("$ echo hello\nhello\n$ printf 'bye\\n'\nbye\n"), 0o644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	bundlePath := filepath.Join(root, "demo")
	result, err := Import(bundlePath, Options{SourcePath: transcript})
	if err != nil {
		t.Fatalf("import transcript: %v", err)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("expected two imported steps, got %#v", result.Steps)
	}

	_, demo, err := bundle.Load(bundlePath)
	if err != nil {
		t.Fatalf("load imported bundle: %v", err)
	}
	if len(demo.Steps) != 2 || demo.Steps[0].ID != "echo-hello" {
		t.Fatalf("unexpected imported manifest %#v", demo.Steps)
	}

	rawMetadata, err := os.ReadFile(filepath.Join(bundlePath, "metadata.json"))
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	var metadata model.Metadata
	if err := json.Unmarshal(rawMetadata, &metadata); err != nil {
		t.Fatalf("decode metadata: %v\n%s", err, string(rawMetadata))
	}
	if metadata.SchemaVersion != model.MetadataSchemaVersion || metadata.ToolVersion != model.ToolVersion {
		t.Fatalf("unexpected metadata %#v", metadata)
	}

	events, err := capture.ReadEvents(filepath.Join(bundlePath, "captures", "echo-hello.jsonl"))
	if err != nil {
		t.Fatalf("read imported capture: %v", err)
	}
	if len(events) != 3 || events[1].Data != "hello\n" {
		t.Fatalf("unexpected imported events %#v", events)
	}
}

func TestImportUsesPromptAndRedactionsFromExistingManifest(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	bundlePath := filepath.Join(root, "demo")
	if err := os.MkdirAll(bundlePath, 0o755); err != nil {
		t.Fatalf("create bundle dir: %v", err)
	}
	demo := model.DefaultDemo("demo")
	demo.Defaults.Prompt = "> "
	demo.Redactions = []model.Redaction{{Pattern: `secret-[0-9]+`, Replacement: "SECRET"}}
	encoded, err := yaml.Marshal(demo)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundlePath, "demo.yaml"), encoded, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	transcript := filepath.Join(root, "transcript.txt")
	if err := os.WriteFile(transcript, []byte("> echo secret-123\nsecret-123\n"), 0o644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	result, err := Import(bundlePath, Options{SourcePath: transcript})
	if err != nil {
		t.Fatalf("import transcript: %v", err)
	}
	if result.Steps[0].Command != "echo SECRET" {
		t.Fatalf("expected redacted command, got %#v", result.Steps[0])
	}

	raw, err := os.ReadFile(filepath.Join(bundlePath, result.Steps[0].Capture))
	if err != nil {
		t.Fatalf("read capture: %v", err)
	}
	if strings.Contains(string(raw), "secret-123") || !strings.Contains(string(raw), "SECRET") {
		t.Fatalf("expected redacted capture, got %s", string(raw))
	}
}

func TestImportAppendAvoidsDuplicateStepIDs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	bundlePath := filepath.Join(root, "demo")
	if err := os.MkdirAll(bundlePath, 0o755); err != nil {
		t.Fatalf("create bundle dir: %v", err)
	}
	demo := model.DefaultDemo("demo")
	demo.Steps = []model.Step{{ID: "echo-hello", Command: "echo existing", ExpectExit: 0}}
	if err := bundle.WriteManifest(filepath.Join(bundlePath, "demo.yaml"), demo); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	transcript := filepath.Join(root, "transcript.txt")
	if err := os.WriteFile(transcript, []byte("$ echo hello\nhello\n"), 0o644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	result, err := Import(bundlePath, Options{SourcePath: transcript, Append: true})
	if err != nil {
		t.Fatalf("import transcript: %v", err)
	}
	if len(result.Steps) != 2 || result.Steps[1].ID != "echo-hello-2" {
		t.Fatalf("expected unique appended id, got %#v", result.Steps)
	}
}

func TestImportAppendAvoidsGeneratedSuffixCollisions(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	bundlePath := filepath.Join(root, "demo")
	if err := os.MkdirAll(bundlePath, 0o755); err != nil {
		t.Fatalf("create bundle dir: %v", err)
	}
	demo := model.DefaultDemo("demo")
	demo.Steps = []model.Step{
		{ID: "echo-hello", Command: "echo existing", ExpectExit: 0},
		{ID: "echo-hello-2", Command: "echo existing suffix", ExpectExit: 0},
	}
	if err := bundle.WriteManifest(filepath.Join(bundlePath, "demo.yaml"), demo); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	transcript := filepath.Join(root, "transcript.txt")
	if err := os.WriteFile(transcript, []byte("$ echo hello\nhello\n"), 0o644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	result, err := Import(bundlePath, Options{SourcePath: transcript, Append: true})
	if err != nil {
		t.Fatalf("import transcript: %v", err)
	}
	if len(result.Steps) != 3 || result.Steps[2].ID != "echo-hello-3" {
		t.Fatalf("expected generated suffix to skip existing IDs, got %#v", result.Steps)
	}
}

func TestImportCanReplaceInvalidExistingSteps(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	bundlePath := filepath.Join(root, "demo")
	if err := os.MkdirAll(bundlePath, 0o755); err != nil {
		t.Fatalf("create bundle dir: %v", err)
	}
	manifest := []byte(`schema_version: "1.0"
name: demo
steps: []
`)
	if err := os.WriteFile(filepath.Join(bundlePath, "demo.yaml"), manifest, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	transcript := filepath.Join(root, "transcript.txt")
	if err := os.WriteFile(transcript, []byte("$ echo hello\nhello\n"), 0o644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	result, err := Import(bundlePath, Options{SourcePath: transcript})
	if err != nil {
		t.Fatalf("import transcript: %v", err)
	}
	if len(result.Steps) != 1 || result.Steps[0].ID != "echo-hello" {
		t.Fatalf("expected replacement import to create a valid step, got %#v", result.Steps)
	}
}
