package upgrade

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/ailtonmacedo/gosvc/internal/backup"
	"github.com/ailtonmacedo/gosvc/internal/generator"
	"github.com/ailtonmacedo/gosvc/internal/manifest"
	"github.com/ailtonmacedo/gosvc/internal/preset"
	"github.com/ailtonmacedo/gosvc/internal/project"
	"github.com/ailtonmacedo/gosvc/internal/resource"
	versionutil "github.com/ailtonmacedo/gosvc/internal/version"
)

type Options struct {
	ProjectDir          string
	TargetVersion       string
	TargetPresetVersion string
	RuntimeVersion      string
	DryRun              bool
	Force               bool
	NoBackup            bool
	Now                 func() time.Time
}

type Result struct {
	ProjectDir        string
	FromVersion       string
	ToVersion         string
	FromPresetVersion string
	ToPresetVersion   string
	FromSchemaVersion int
	ToSchemaVersion   int
	Changes           []generator.Change
	ManifestAction    generator.Action
	BackupPath        string
	Applied           bool
	UpgradeRequired   bool
}

func Run(options Options) (Result, error) {
	if options.ProjectDir == "" {
		options.ProjectDir = "."
	}
	if options.RuntimeVersion == "" {
		options.RuntimeVersion = "dev"
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	target, err := resolveTarget(options.TargetVersion, options.RuntimeVersion)
	if err != nil {
		return Result{}, err
	}
	config, err := project.Load(filepath.Join(options.ProjectDir, "project.yaml"))
	if err != nil {
		return Result{}, fmt.Errorf("load project configuration: %w", err)
	}
	document, err := manifest.LoadDocument(options.ProjectDir)
	if err != nil {
		return Result{}, err
	}
	if err := rejectDowngrade(document.Manifest.FrameworkVersion, target); err != nil {
		return Result{}, err
	}
	fromPresetVersion := config.Project.PresetVersion
	targetPresetVersion := strings.TrimSpace(options.TargetPresetVersion)
	if targetPresetVersion == "" {
		targetPresetVersion = fromPresetVersion
	}
	definition, err := preset.ResolveVersion(config.Project.Preset, targetPresetVersion)
	if err != nil {
		return Result{}, fmt.Errorf("resolve target preset version: %w", err)
	}
	config.Project.PresetVersion = definition.Version
	if err := config.Validate(); err != nil {
		return Result{}, fmt.Errorf("target preset configuration is incompatible: %w", err)
	}
	resources, err := resource.Load(options.ProjectDir)
	if err != nil {
		return Result{}, err
	}
	if len(resources) == 0 && config.Database.Enabled {
		resources = []resource.Definition{resource.DefaultOrder()}
	}
	artifacts, err := generator.Render(config, definition, resources)
	if err != nil {
		return Result{}, err
	}
	artifacts, err = generator.PreservePluginArtifacts(options.ProjectDir, artifacts, document.Manifest)
	if err != nil {
		return Result{}, err
	}
	migrated := document.Manifest
	migrated.Project = manifest.Project{
		Name: config.Project.Name, Module: config.Project.Module,
		ConfigSchemaVersion: config.SchemaVersion,
	}
	migrated.Compatibility = manifest.Compatibility{
		MinimumGosvcVersion:       target,
		LastValidatedGosvcVersion: target,
	}
	changes, err := generator.Plan(options.ProjectDir, artifacts, &migrated, generator.PlanOptions{Force: options.Force})
	if err != nil {
		return Result{}, err
	}
	if fromPresetVersion != definition.Version {
		for index := range changes {
			if changes[index].Artifact.Path == "project.yaml" {
				changes[index].Action = generator.ActionUpdate
				changes[index].Reason = "explicit preset version migration"
				break
			}
		}
	}
	metadataChanged := document.SourceSchemaVersion != manifest.CurrentSchemaVersion ||
		document.Manifest.FrameworkVersion != target ||
		document.Manifest.Project != migrated.Project ||
		document.Manifest.Compatibility != migrated.Compatibility ||
		document.Manifest.Preset != definition.Name ||
		document.Manifest.PresetVersion != definition.Version
	result := Result{
		ProjectDir:  options.ProjectDir,
		FromVersion: document.Manifest.FrameworkVersion, ToVersion: target,
		FromPresetVersion: fromPresetVersion, ToPresetVersion: definition.Version,
		FromSchemaVersion: document.SourceSchemaVersion, ToSchemaVersion: manifest.CurrentSchemaVersion,
		Changes: changes, ManifestAction: generator.ActionSkip,
		UpgradeRequired: metadataChanged || generator.HasWrites(changes),
	}
	if result.UpgradeRequired {
		result.ManifestAction = generator.ActionUpdate
	}
	if options.DryRun || !result.UpgradeRequired {
		return result, nil
	}

	var backupPath string
	if !options.NoBackup {
		backupValue := document.Manifest
		backupValue.SchemaVersion = document.SourceSchemaVersion
		entry, backupErr := backup.Create(options.ProjectDir, backupValue, options.Now())
		if backupErr != nil {
			return Result{}, fmt.Errorf("create pre-upgrade backup: %w", backupErr)
		}
		backupPath = entry.Path
		result.BackupPath = entry.Path
	}
	history := append([]manifest.UpgradeRecord(nil), document.Manifest.UpgradeHistory...)
	history = append(history, manifest.UpgradeRecord{
		FromFrameworkVersion: document.Manifest.FrameworkVersion,
		ToFrameworkVersion:   target,
		FromSchemaVersion:    document.SourceSchemaVersion,
		ToSchemaVersion:      manifest.CurrentSchemaVersion,
		AppliedAt:            options.Now().UTC().Format(time.RFC3339),
		Backup:               backupPath,
	})
	if err := generator.Apply(options.ProjectDir, changes, artifacts, generator.ApplyOptions{
		FrameworkVersion: target,
		Project:          migrated.Project,
		Preset:           definition.Name,
		PresetVersion:    definition.Version,
		Features:         mergeFeatures(definition.Features, document.Manifest.Features),
		Plugins:          document.Manifest.Plugins,
		UpgradeHistory:   history,
		RollbackHistory:  document.Manifest.RollbackHistory,
		Compatibility:    migrated.Compatibility,
	}); err != nil {
		return Result{}, err
	}
	result.Applied = true
	return result, nil
}

func resolveTarget(requested, runtime string) (string, error) {
	if requested == "" || requested == "current" {
		return runtime, nil
	}
	if _, err := versionutil.Parse(requested); err != nil {
		return "", err
	}
	if runtime != "dev" {
		requestedValue, _ := versionutil.Parse(requested)
		runtimeValue, err := versionutil.Parse(runtime)
		if err != nil {
			return "", fmt.Errorf("invalid runtime version %q: %w", runtime, err)
		}
		if versionutil.Compare(requestedValue, runtimeValue) != 0 {
			return "", fmt.Errorf("this gosvc binary can only upgrade to %s, not %s", runtimeValue.String(), requestedValue.String())
		}
	}
	return strings.TrimPrefix(requested, "v"), nil
}

func rejectDowngrade(from, to string) error {
	if from == "" || from == "dev" || to == "dev" {
		return nil
	}
	fromValue, err := versionutil.Parse(from)
	if err != nil {
		return nil
	}
	toValue, err := versionutil.Parse(to)
	if err != nil {
		return err
	}
	if versionutil.Compare(toValue, fromValue) < 0 {
		return fmt.Errorf("downgrade from %s to %s is not supported", from, to)
	}
	return nil
}

func mergeFeatures(core, existing []string) []string {
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
