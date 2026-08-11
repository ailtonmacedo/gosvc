package generator

import (
	"fmt"
	"path/filepath"

	"github.com/ailtonmacedo/gosvc/internal/manifest"
	"github.com/ailtonmacedo/gosvc/internal/preset"
	"github.com/ailtonmacedo/gosvc/internal/project"
	"github.com/ailtonmacedo/gosvc/internal/resource"
)

type AddResourceRequest struct {
	ProjectDir       string
	Definition       resource.Definition
	DryRun           bool
	Force            bool
	FrameworkVersion string
}

func AddResource(request AddResourceRequest) (Result, bool, error) {
	if request.ProjectDir == "" {
		request.ProjectDir = "."
	}
	config, err := project.Load(filepath.Join(request.ProjectDir, "project.yaml"))
	if err != nil {
		return Result{}, false, fmt.Errorf("load project configuration: %w", err)
	}
	if (config.Project.Preset != "postgres-api" && config.Project.Preset != "production-api" && config.Project.Preset != "event-driven-api") || !config.Database.Enabled {
		return Result{}, false, fmt.Errorf("add resource currently requires the postgres-api, production-api, or event-driven-api preset")
	}
	definition, err := preset.Resolve(config.Project.Preset)
	if err != nil {
		return Result{}, false, err
	}
	resources, err := resource.Load(request.ProjectDir)
	if err != nil {
		return Result{}, false, err
	}
	if len(resources) == 0 {
		resources = []resource.Definition{resource.DefaultOrder()}
	}
	resources, added, err := resource.Add(resources, request.Definition)
	if err != nil {
		return Result{}, false, err
	}
	artifacts, err := Render(config, definition, resources)
	if err != nil {
		return Result{}, false, err
	}
	existing, err := manifest.Load(request.ProjectDir)
	if err != nil {
		return Result{}, false, err
	}
	changes, err := Plan(request.ProjectDir, artifacts, &existing, PlanOptions{Force: request.Force})
	if err != nil {
		return Result{}, false, err
	}
	result := Result{Destination: request.ProjectDir, Preset: definition, Changes: changes, ManifestAction: ActionSkip}
	if HasWrites(changes) {
		result.ManifestAction = ActionUpdate
	}
	if request.DryRun || !HasWrites(changes) {
		return result, added, nil
	}
	if err := Apply(request.ProjectDir, changes, artifacts, ApplyOptions{
		FrameworkVersion: request.FrameworkVersion,
		Project: manifest.Project{
			Name: config.Project.Name, Module: config.Project.Module,
			ConfigSchemaVersion: config.SchemaVersion,
		},
		Preset: definition.Name, Features: definition.Features,
		Plugins: existing.Plugins, UpgradeHistory: existing.UpgradeHistory,
		RollbackHistory: existing.RollbackHistory, Compatibility: existing.Compatibility,
	}); err != nil {
		return Result{}, false, err
	}
	result.Applied = true
	return result, added, nil
}
