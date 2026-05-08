// SPDX-License-Identifier: AGPL-3.0-or-later

// Package bundle resolves demo bundle paths and reads/writes bundle metadata.
package bundle

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/nccurry/terminos/internal/apperror"
	"github.com/nccurry/terminos/internal/model"
)

// ManifestFile is the filename of the demo manifest inside a bundle.
const ManifestFile = "demo.yaml"

// MetadataFile is the filename of the bundle metadata document.
const MetadataFile = "metadata.json"

// Bundle describes the resolved locations of a demo bundle.
type Bundle struct {
	Path        string `json:"path" yaml:"path"`
	Manifest    string `json:"manifest" yaml:"manifest"`
	Metadata    string `json:"metadata" yaml:"metadata"`
	CapturesDir string `json:"captures_dir" yaml:"captures_dir"`
	AssetsDir   string `json:"assets_dir" yaml:"assets_dir"`
}

// InitResult is the stable result shape returned by terminos init.
type InitResult struct {
	Bundle      Bundle   `json:"bundle" yaml:"bundle"`
	Created     []string `json:"created" yaml:"created"`
	DryRun      bool     `json:"dry_run" yaml:"dry_run"`
	Overwritten bool     `json:"overwritten" yaml:"overwritten"`
}

// Resolve returns canonical bundle paths for a user-supplied bundle path.
func Resolve(input string) Bundle {
	path := strings.TrimSpace(input)
	if path == "" {
		path = "."
	}
	path = filepath.Clean(path)
	if filepath.Base(path) == ManifestFile {
		path = filepath.Dir(path)
	}

	return Bundle{
		Path:        path,
		Manifest:    filepath.Join(path, ManifestFile),
		Metadata:    filepath.Join(path, MetadataFile),
		CapturesDir: filepath.Join(path, "captures"),
		AssetsDir:   filepath.Join(path, "assets"),
	}
}

// Init creates a starter bundle.
func Init(path string, force bool, dryRun bool) (InitResult, error) {
	resolved := Resolve(path)
	created := []string{resolved.Path, resolved.CapturesDir, resolved.AssetsDir, resolved.Manifest, resolved.Metadata}

	manifestExists, err := exists(resolved.Manifest)
	if err != nil {
		return InitResult{}, err
	}
	if manifestExists && !force {
		return InitResult{}, apperror.Newf(apperror.CodeUsage, "%s already exists; pass --force to overwrite it", resolved.Manifest)
	}

	result := InitResult{
		Bundle:      resolved,
		Created:     created,
		DryRun:      dryRun,
		Overwritten: manifestExists && force,
	}
	if dryRun {
		return result, nil
	}

	if err := os.MkdirAll(resolved.CapturesDir, 0o755); err != nil {
		return InitResult{}, apperror.Wrapf(apperror.CodeFilesystem, err, "create captures directory %s", resolved.CapturesDir)
	}
	if err := os.MkdirAll(resolved.AssetsDir, 0o755); err != nil {
		return InitResult{}, apperror.Wrapf(apperror.CodeFilesystem, err, "create assets directory %s", resolved.AssetsDir)
	}

	demo := model.DefaultDemo(defaultName(resolved.Path))
	if err := WriteManifest(resolved.Manifest, demo); err != nil {
		return InitResult{}, err
	}
	if err := WriteMetadata(resolved); err != nil {
		return InitResult{}, err
	}
	return result, nil
}

// Load reads demo.yaml from a bundle.
func Load(path string) (Bundle, model.Demo, error) {
	resolved := Resolve(path)
	raw, err := os.ReadFile(resolved.Manifest)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Bundle{}, model.Demo{}, apperror.Wrapf(apperror.CodeFilesystem, err, "read manifest %s", resolved.Manifest)
		}
		return Bundle{}, model.Demo{}, apperror.Wrapf(apperror.CodeFilesystem, err, "read manifest %s", resolved.Manifest)
	}

	var demo model.Demo
	if err := yaml.Unmarshal(raw, &demo); err != nil {
		return Bundle{}, model.Demo{}, apperror.Wrapf(apperror.CodeUsage, err, "parse manifest %s", resolved.Manifest)
	}
	demo.ApplyDefaults()
	return resolved, demo, nil
}

// WriteManifest writes a manifest as YAML.
func WriteManifest(path string, demo model.Demo) error {
	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(demo); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "encode manifest", err)
	}
	if err := encoder.Close(); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "encode manifest", err)
	}

	if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
		return apperror.Wrapf(apperror.CodeFilesystem, err, "write manifest %s", path)
	}
	return nil
}

// WriteMetadata writes metadata.json for a bundle.
func WriteMetadata(resolved Bundle) error {
	metadata := model.Metadata{
		SchemaVersion: model.MetadataSchemaVersion,
		ToolVersion:   model.ToolVersion,
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
		Host: model.MetadataHost{
			OS:   runtime.GOOS,
			Arch: runtime.GOARCH,
			Go:   runtime.Version(),
		},
	}

	encoded, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return apperror.Wrap(apperror.CodeInternal, "encode metadata", err)
	}
	encoded = append(encoded, '\n')
	if err := os.MkdirAll(resolved.Path, 0o755); err != nil {
		return apperror.Wrapf(apperror.CodeFilesystem, err, "create bundle directory %s", resolved.Path)
	}
	if err := os.WriteFile(resolved.Metadata, encoded, 0o644); err != nil {
		return apperror.Wrapf(apperror.CodeFilesystem, err, "write metadata %s", resolved.Metadata)
	}
	return nil
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, apperror.Wrapf(apperror.CodeFilesystem, err, "stat %s", path)
}

func defaultName(path string) string {
	name := filepath.Base(filepath.Clean(path))
	if name == "." || name == string(filepath.Separator) {
		return "terminos-demo"
	}
	return name
}
