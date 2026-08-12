package generator

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ailtonmacedo/gosvc/internal/manifest"
)

type ApplyOptions struct {
	FrameworkVersion string
	Project          manifest.Project
	Preset           string
	PresetVersion    string
	Features         []string
	Plugins          []manifest.PluginReference
	UpgradeHistory   []manifest.UpgradeRecord
	RollbackHistory  []manifest.RollbackRecord
	Compatibility    manifest.Compatibility
	PreserveFiles    map[string]manifest.File
}

func Apply(destination string, changes []Change, artifacts []Artifact, options ApplyOptions) error {
	parent := filepath.Dir(destination)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create destination parent: %w", err)
	}
	stage, err := os.MkdirTemp(parent, ".gosvc-stage-")
	if err != nil {
		return fmt.Errorf("create staging directory: %w", err)
	}
	defer os.RemoveAll(stage)

	if info, statErr := os.Stat(destination); statErr == nil && info.IsDir() {
		if err := copyTree(destination, stage); err != nil {
			return fmt.Errorf("copy existing project into staging: %w", err)
		}
	}
	for _, change := range changes {
		if change.Action != ActionCreate && change.Action != ActionUpdate {
			continue
		}
		path := filepath.Join(stage, filepath.FromSlash(change.Artifact.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create parent for %q: %w", change.Artifact.Path, err)
		}
		if err := os.WriteFile(path, change.Artifact.Content, change.Artifact.Mode); err != nil {
			return fmt.Errorf("write staged file %q: %w", change.Artifact.Path, err)
		}
	}

	value, err := buildManifest(stage, changes, artifacts, options)
	if err != nil {
		return err
	}
	data, err := manifest.Encode(value)
	if err != nil {
		return err
	}
	manifestPath := filepath.Join(stage, filepath.FromSlash(manifest.Path))
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		return fmt.Errorf("create manifest directory: %w", err)
	}
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return swapDirectory(destination, stage)
}

func buildManifest(stage string, changes []Change, artifacts []Artifact, options ApplyOptions) (manifest.Manifest, error) {
	modified := make(map[string]bool, len(changes))
	for _, change := range changes {
		if change.Action == ActionCreate || change.Action == ActionUpdate {
			modified[change.Artifact.Path] = true
		}
	}
	files := make([]manifest.File, 0, len(artifacts))
	for _, artifact := range artifacts {
		path := filepath.Join(stage, filepath.FromSlash(artifact.Path))
		content, err := os.ReadFile(path)
		if err != nil {
			return manifest.Manifest{}, fmt.Errorf("read staged artifact %q for manifest: %w", artifact.Path, err)
		}
		if preserved, ok := options.PreserveFiles[artifact.Path]; ok && !modified[artifact.Path] {
			files = append(files, preserved)
			continue
		}
		files = append(files, manifest.File{
			Path:      artifact.Path,
			Ownership: string(artifact.Ownership),
			Checksum:  manifest.Checksum(content),
			Producer:  artifact.Producer,
		})
	}
	return manifest.Manifest{
		FrameworkVersion: options.FrameworkVersion,
		SchemaVersion:    manifest.CurrentSchemaVersion,
		Project:          options.Project,
		Preset:           options.Preset,
		PresetVersion:    options.PresetVersion,
		Features:         append([]string(nil), options.Features...),
		Compatibility:    options.Compatibility,
		Plugins:          append([]manifest.PluginReference(nil), options.Plugins...),
		UpgradeHistory:   append([]manifest.UpgradeRecord(nil), options.UpgradeHistory...),
		RollbackHistory:  append([]manifest.RollbackRecord(nil), options.RollbackHistory...),
		Files:            files,
	}, nil
}

func swapDirectory(destination, stage string) error {
	_, err := os.Stat(destination)
	if os.IsNotExist(err) {
		if err := os.Rename(stage, destination); err != nil {
			return fmt.Errorf("activate generated project: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect destination before swap: %w", err)
	}

	backup := destination + ".gosvc-backup"
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("remove stale backup: %w", err)
	}
	if err := os.Rename(destination, backup); err != nil {
		return fmt.Errorf("backup existing project: %w", err)
	}
	if err := os.Rename(stage, destination); err != nil {
		_ = os.Rename(backup, destination)
		return fmt.Errorf("activate generated project: %w", err)
	}
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("remove project backup: %w", err)
	}
	return nil
}

func copyTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		target := filepath.Join(destination, relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic link %q is not supported", relative)
		}
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported file type at %q", relative)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			_ = input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		inputCloseErr := input.Close()
		outputCloseErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if inputCloseErr != nil {
			return inputCloseErr
		}
		return outputCloseErr
	})
}
