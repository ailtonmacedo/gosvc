package pluginapply

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ailtonmacedo/gosvc/internal/generator"
	"github.com/ailtonmacedo/gosvc/internal/manifest"
	"github.com/ailtonmacedo/gosvc/internal/plugin"
	"github.com/ailtonmacedo/gosvc/internal/projectcheck"
)

type Options struct {
	ProjectDir       string
	PluginName       string
	FrameworkVersion string
	DryRun           bool
	Force            bool
	Timeout          time.Duration
}

type Result struct {
	Plugin         plugin.Metadata
	Changes        []generator.Change
	Diagnostics    []plugin.Diagnostic
	ManifestAction generator.Action
	Applied        bool
}

func Run(ctx context.Context, options Options) (Result, error) {
	if options.ProjectDir == "" {
		options.ProjectDir = "."
	}
	if strings.TrimSpace(options.PluginName) == "" {
		return Result{}, fmt.Errorf("plugin name is required")
	}
	report, err := projectcheck.Check(options.ProjectDir)
	if err != nil {
		return Result{}, err
	}
	config := report.Config
	value, err := manifest.Load(options.ProjectDir)
	if err != nil {
		return Result{}, err
	}
	if err := validationError(report, value, options.PluginName, options.Force); err != nil {
		return Result{}, err
	}
	metadata, err := plugin.Find(options.ProjectDir, options.FrameworkVersion, options.PluginName)
	if err != nil {
		return Result{}, err
	}
	if err := metadata.ExecutionReady(); err != nil {
		return Result{}, err
	}
	if !metadata.HasCapability(plugin.CapabilityValidation) && !metadata.HasCapability(plugin.CapabilityArtifacts) {
		return Result{}, fmt.Errorf("plugin %q has no executable validation or artifacts capability", metadata.Name)
	}
	snapshot, err := plugin.AbsoluteProjectSnapshot(options.ProjectDir, config.Project.Name, config.Project.Module, config.Project.Preset, value.Features, options.DryRun)
	if err != nil {
		return Result{}, err
	}
	runner := plugin.Runner{Timeout: options.Timeout}
	var diagnostics []plugin.Diagnostic
	if metadata.HasCapability(plugin.CapabilityValidation) {
		response, runErr := runner.Run(ctx, metadata, options.ProjectDir, snapshot, plugin.ActionValidate)
		if runErr != nil {
			return Result{}, runErr
		}
		diagnostics = append(diagnostics, response.Diagnostics...)
	}
	contribution := plugin.ProtocolContribution{}
	if metadata.HasCapability(plugin.CapabilityArtifacts) {
		response, runErr := runner.Run(ctx, metadata, options.ProjectDir, snapshot, plugin.ActionContribute)
		if runErr != nil {
			return Result{}, runErr
		}
		diagnostics = append(diagnostics, response.Diagnostics...)
		contribution = response.Contribution
	}

	artifacts, preserved, err := mergeArtifacts(options.ProjectDir, value, metadata.Name, contribution.Artifacts)
	if err != nil {
		return Result{}, err
	}
	changes, err := generator.Plan(options.ProjectDir, artifacts, &value, generator.PlanOptions{Force: options.Force})
	if err != nil {
		return Result{}, err
	}
	features := mergeFeatures(value.Features, metadata.Name, contribution.Features)
	plugins := upsertPlugin(value.Plugins, options.ProjectDir, metadata)
	metadataChanged := !sameFeatures(features, value.Features) || !samePlugins(plugins, value.Plugins)
	result := Result{
		Plugin: metadata, Changes: changes, Diagnostics: diagnostics,
		ManifestAction: generator.ActionSkip,
	}
	if generator.HasWrites(changes) || metadataChanged {
		result.ManifestAction = generator.ActionUpdate
	}
	if options.DryRun || (!generator.HasWrites(changes) && !metadataChanged) {
		return result, nil
	}
	if err := generator.Apply(options.ProjectDir, changes, artifacts, generator.ApplyOptions{
		FrameworkVersion: value.FrameworkVersion,
		Project:          value.Project,
		Preset:           value.Preset,
		Features:         features,
		Plugins:          plugins,
		UpgradeHistory:   value.UpgradeHistory,
		RollbackHistory:  value.RollbackHistory,
		Compatibility:    value.Compatibility,
		PreserveFiles:    preserved,
	}); err != nil {
		return Result{}, err
	}
	result.Applied = true
	return result, nil
}

func validationError(report projectcheck.Report, value manifest.Manifest, pluginName string, force bool) error {
	if !force {
		return report.Error()
	}
	var messages []string
	for _, issue := range report.Issues {
		if issue.Severity != projectcheck.SeverityError {
			continue
		}
		record, tracked := value.File(issue.Path)
		recoverable := tracked && record.Producer == pluginName && record.Ownership == string(generator.OwnershipGenerated) &&
			strings.Contains(issue.Message, "generated file was modified")
		if !recoverable {
			messages = append(messages, issue.String())
		}
	}
	if len(messages) > 0 {
		return fmt.Errorf("project validation failed:\n%s", strings.Join(messages, "\n"))
	}
	return nil
}

func mergeArtifacts(projectDir string, value manifest.Manifest, pluginName string, contributed []plugin.ProtocolArtifact) ([]generator.Artifact, map[string]manifest.File, error) {
	artifacts := make(map[string]generator.Artifact, len(value.Files)+len(contributed))
	preserved := make(map[string]manifest.File, len(value.Files))
	records := make(map[string]manifest.File, len(value.Files))
	for _, record := range value.Files {
		path := filepath.Join(projectDir, filepath.FromSlash(record.Path))
		info, err := os.Lstat(path)
		if err != nil {
			return nil, nil, fmt.Errorf("read tracked artifact %q: %w", record.Path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, nil, fmt.Errorf("tracked artifact %q must be a regular file", record.Path)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, fmt.Errorf("read tracked artifact %q: %w", record.Path, err)
		}
		artifact := generator.Artifact{
			Path: record.Path, Content: content, Mode: info.Mode().Perm(),
			Ownership: generator.Ownership(record.Ownership), Producer: record.Producer,
		}
		if err := artifact.Validate(); err != nil {
			return nil, nil, err
		}
		artifacts[record.Path] = artifact
		preserved[record.Path] = record
		records[record.Path] = record
	}
	for _, item := range contributed {
		if reservedPath(item.Path) {
			return nil, nil, fmt.Errorf("plugin %q cannot manage reserved path %q", pluginName, item.Path)
		}
		content, err := item.Bytes()
		if err != nil {
			return nil, nil, err
		}
		artifact := generator.Artifact{
			Path: item.Path, Content: content, Mode: fs.FileMode(item.Mode),
			Ownership: generator.Ownership(item.Ownership), Producer: pluginName,
		}
		if err := artifact.Validate(); err != nil {
			return nil, nil, fmt.Errorf("plugin %q: %w", pluginName, err)
		}
		if record, tracked := records[item.Path]; tracked {
			if record.Producer != pluginName {
				owner := "gosvc core"
				if record.Producer != "" {
					owner = "plugin " + record.Producer
				}
				return nil, nil, fmt.Errorf("plugin %q artifact %q conflicts with %s", pluginName, item.Path, owner)
			}
			if record.Ownership != item.Ownership {
				return nil, nil, fmt.Errorf("plugin %q cannot change ownership of artifact %q from %q to %q", pluginName, item.Path, record.Ownership, item.Ownership)
			}
		} else {
			path := filepath.Join(projectDir, filepath.FromSlash(item.Path))
			if _, statErr := os.Lstat(path); statErr == nil {
				return nil, nil, fmt.Errorf("plugin %q artifact %q conflicts with an untracked file", pluginName, item.Path)
			} else if !errors.Is(statErr, fs.ErrNotExist) {
				return nil, nil, fmt.Errorf("inspect plugin artifact %q: %w", item.Path, statErr)
			}
		}
		artifacts[item.Path] = artifact
	}
	paths := make([]string, 0, len(artifacts))
	for path := range artifacts {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	result := make([]generator.Artifact, 0, len(paths))
	for _, path := range paths {
		result = append(result, artifacts[path])
	}
	return result, preserved, nil
}

func reservedPath(path string) bool {
	return path == ".gosvc" || strings.HasPrefix(path, ".gosvc/") ||
		path == ".git" || strings.HasPrefix(path, ".git/")
}

func mergeFeatures(existing []string, pluginName string, contributed []string) []string {
	prefix := "plugin:" + pluginName + ":"
	result := make([]string, 0, len(existing)+len(contributed))
	seen := map[string]bool{}
	for _, feature := range existing {
		if strings.HasPrefix(feature, prefix) {
			continue
		}
		if !seen[feature] {
			result = append(result, feature)
			seen[feature] = true
		}
	}
	for _, feature := range contributed {
		value := prefix + feature
		if !seen[value] {
			result = append(result, value)
			seen[value] = true
		}
	}
	sort.Strings(result)
	return result
}

func upsertPlugin(existing []manifest.PluginReference, projectDir string, metadata plugin.Metadata) []manifest.PluginReference {
	result := make([]manifest.PluginReference, 0, len(existing)+1)
	for _, item := range existing {
		if item.Name != metadata.Name {
			result = append(result, item)
		}
	}
	source := metadata.Source
	if relative, err := filepath.Rel(projectDir, filepath.FromSlash(metadata.Source)); err == nil {
		source = filepath.ToSlash(relative)
	}
	result = append(result, manifest.PluginReference{
		Name: metadata.Name, Version: metadata.Version, Source: source,
		Checksum: metadata.Checksum, ProtocolVersion: metadata.ProtocolVersion,
	})
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func sameFeatures(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	lcopy, rcopy := append([]string(nil), left...), append([]string(nil), right...)
	sort.Strings(lcopy)
	sort.Strings(rcopy)
	for index := range lcopy {
		if lcopy[index] != rcopy[index] {
			return false
		}
	}
	return true
}

func samePlugins(left, right []manifest.PluginReference) bool {
	if len(left) != len(right) {
		return false
	}
	lcopy, rcopy := append([]manifest.PluginReference(nil), left...), append([]manifest.PluginReference(nil), right...)
	sort.Slice(lcopy, func(i, j int) bool { return lcopy[i].Name < lcopy[j].Name })
	sort.Slice(rcopy, func(i, j int) bool { return rcopy[i].Name < rcopy[j].Name })
	for index := range lcopy {
		if lcopy[index] != rcopy[index] {
			return false
		}
	}
	return true
}
