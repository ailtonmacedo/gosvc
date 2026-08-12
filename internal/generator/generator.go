package generator

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ailtonmacedo/gosvc/internal/manifest"
	"github.com/ailtonmacedo/gosvc/internal/preset"
	"github.com/ailtonmacedo/gosvc/internal/project"
	"github.com/ailtonmacedo/gosvc/internal/resource"
)

type Request struct {
	Config           project.Config
	Destination      string
	DryRun           bool
	Force            bool
	FrameworkVersion string
}

type Result struct {
	Destination    string
	Preset         preset.Definition
	Changes        []Change
	ManifestAction Action
	Applied        bool
}

func Generate(request Request) (Result, error) {
	if request.Destination == "" {
		return Result{}, fmt.Errorf("destination is required")
	}
	if err := request.Config.Validate(); err != nil {
		return Result{}, err
	}
	definition, err := preset.ResolveVersion(request.Config.Project.Preset, request.Config.Project.PresetVersion)
	if err != nil {
		return Result{}, err
	}
	resources, err := resource.Load(request.Destination)
	if err != nil {
		return Result{}, err
	}
	if len(resources) == 0 && request.Config.Database.Enabled {
		resources = []resource.Definition{resource.DefaultOrder()}
	}
	artifacts, err := Render(request.Config, definition, resources)
	if err != nil {
		return Result{}, err
	}

	existing, err := loadExistingManifest(request.Destination)
	if err != nil {
		return Result{}, err
	}
	if existing != nil && existing.Preset != definition.Name {
		return Result{}, fmt.Errorf("project preset is %q, cannot regenerate with %q", existing.Preset, definition.Name)
	}
	if existing != nil && existing.PresetVersion != "" && existing.PresetVersion != definition.Version {
		return Result{}, fmt.Errorf("project preset version is %q, cannot regenerate with %q without an upgrade", existing.PresetVersion, definition.Version)
	}
	if existing != nil {
		artifacts, err = PreservePluginArtifacts(request.Destination, artifacts, *existing)
		if err != nil {
			return Result{}, err
		}
	}
	changes, err := Plan(request.Destination, artifacts, existing, PlanOptions{Force: request.Force})
	if err != nil {
		return Result{}, err
	}
	compatibility := manifest.Compatibility{
		MinimumGosvcVersion:       request.FrameworkVersion,
		LastValidatedGosvcVersion: request.FrameworkVersion,
	}
	if existing != nil {
		compatibility = existing.Compatibility
		if compatibility.MinimumGosvcVersion == "" {
			compatibility.MinimumGosvcVersion = request.FrameworkVersion
		}
		compatibility.LastValidatedGosvcVersion = request.FrameworkVersion
	}
	manifestAction := ActionCreate
	metadataChanged := existing == nil
	if existing != nil {
		manifestAction = ActionSkip
		metadataChanged = existing.FrameworkVersion != request.FrameworkVersion ||
			existing.Preset != definition.Name || existing.PresetVersion != definition.Version || !sameCoreFeatures(existing.Features, definition.Features) ||
			existing.Project.Name != request.Config.Project.Name ||
			existing.Project.Module != request.Config.Project.Module ||
			existing.Project.ConfigSchemaVersion != request.Config.SchemaVersion ||
			existing.Compatibility != compatibility
		if HasWrites(changes) || metadataChanged {
			manifestAction = ActionUpdate
		}
	}
	result := Result{
		Destination:    request.Destination,
		Preset:         definition,
		Changes:        changes,
		ManifestAction: manifestAction,
	}
	if request.DryRun || (!HasWrites(changes) && !metadataChanged) {
		return result, nil
	}
	plugins := []manifest.PluginReference(nil)
	history := []manifest.UpgradeRecord(nil)
	rollbacks := []manifest.RollbackRecord(nil)
	if existing != nil {
		plugins = append(plugins, existing.Plugins...)
		history = append(history, existing.UpgradeHistory...)
		rollbacks = append(rollbacks, existing.RollbackHistory...)
	}
	if err := Apply(request.Destination, changes, artifacts, ApplyOptions{
		FrameworkVersion: request.FrameworkVersion,
		Project: manifest.Project{
			Name: request.Config.Project.Name, Module: request.Config.Project.Module,
			ConfigSchemaVersion: request.Config.SchemaVersion,
		},
		Preset:          definition.Name,
		PresetVersion:   definition.Version,
		Features:        mergeCoreAndPluginFeatures(definition.Features, existingFeatures(existing)),
		Plugins:         plugins,
		UpgradeHistory:  history,
		RollbackHistory: rollbacks,
		Compatibility:   compatibility,
	}); err != nil {
		return Result{}, err
	}
	result.Applied = true
	return result, nil
}

func loadExistingManifest(destination string) (*manifest.Manifest, error) {
	value, err := manifest.Load(destination)
	if err == nil {
		return &value, nil
	}
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	manifestPath := filepath.Join(destination, filepath.FromSlash(manifest.Path))
	if _, statErr := os.Stat(manifestPath); errors.Is(statErr, fs.ErrNotExist) {
		return nil, nil
	}
	return nil, err
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	counts := make(map[string]int, len(left))
	for _, value := range left {
		counts[value]++
	}
	for _, value := range right {
		counts[value]--
		if counts[value] < 0 {
			return false
		}
	}
	for _, count := range counts {
		if count != 0 {
			return false
		}
	}
	return true
}

func existingFeatures(existing *manifest.Manifest) []string {
	if existing == nil {
		return nil
	}
	return existing.Features
}

func sameCoreFeatures(existing, core []string) bool {
	actual := make([]string, 0, len(existing))
	for _, feature := range existing {
		if !strings.HasPrefix(feature, "plugin:") {
			actual = append(actual, feature)
		}
	}
	return sameStrings(actual, core)
}

func mergeCoreAndPluginFeatures(core, existing []string) []string {
	result := append([]string(nil), core...)
	seen := make(map[string]bool, len(result))
	for _, feature := range result {
		seen[feature] = true
	}
	for _, feature := range existing {
		if strings.HasPrefix(feature, "plugin:") && !seen[feature] {
			result = append(result, feature)
			seen[feature] = true
		}
	}
	return result
}

func PreservePluginArtifacts(destination string, artifacts []Artifact, existing manifest.Manifest) ([]Artifact, error) {
	paths := make(map[string]bool, len(artifacts))
	for _, artifact := range artifacts {
		paths[artifact.Path] = true
	}
	result := append([]Artifact(nil), artifacts...)
	for _, file := range existing.Files {
		if file.Producer == "" {
			continue
		}
		if paths[file.Path] {
			return nil, fmt.Errorf("plugin %q artifact %q conflicts with a core artifact", file.Producer, file.Path)
		}
		path := filepath.Join(destination, filepath.FromSlash(file.Path))
		info, err := os.Lstat(path)
		if err != nil {
			return nil, fmt.Errorf("read plugin %q artifact %q: %w", file.Producer, file.Path, err)
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("plugin %q artifact %q must be a regular file", file.Producer, file.Path)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read plugin %q artifact %q: %w", file.Producer, file.Path, err)
		}
		if file.Ownership == string(OwnershipGenerated) && manifest.Checksum(content) != file.Checksum {
			return nil, fmt.Errorf("plugin-managed generated file %q was modified; rerun plugin %q", file.Path, file.Producer)
		}
		result = append(result, Artifact{
			Path: file.Path, Content: content, Mode: info.Mode().Perm(),
			Ownership: Ownership(file.Ownership), Producer: file.Producer,
		})
		paths[file.Path] = true
	}
	return result, nil
}
