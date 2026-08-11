package upgrade

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ailtonmacedo/gosvc/internal/generator"
	"github.com/ailtonmacedo/gosvc/internal/manifest"
	"github.com/ailtonmacedo/gosvc/internal/project"
)

func TestRunMigratesLegacyManifestAndPreservesUserFiles(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "service")
	config := project.DefaultConfigForPreset("minimal-api")
	config.Project.Name = "service"
	config.Project.Module = "github.com/acme/service"
	if _, err := generator.Generate(generator.Request{Config: config, Destination: dir, FrameworkVersion: "0.7.0"}); err != nil {
		t.Fatal(err)
	}
	current, err := manifest.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	legacy := map[string]any{
		"framework_version": current.FrameworkVersion,
		"schema_version":    1,
		"preset":            current.Preset,
		"features":          current.Features,
		"files":             current.Files,
	}
	data, _ := json.MarshalIndent(legacy, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, ".gosvc", "manifest.json"), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	custom := []byte("# custom documentation\n")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), custom, 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Run(Options{ProjectDir: dir, TargetVersion: "1.0.0", RuntimeVersion: "dev", Now: func() time.Time { return time.Date(2026, 8, 5, 21, 0, 0, 0, time.UTC) }})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.FromSchemaVersion != 1 || result.ToSchemaVersion != 3 || result.BackupPath == "" {
		t.Fatalf("result = %+v", result)
	}
	content, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != string(custom) {
		t.Fatalf("README was overwritten: %s", content)
	}
	upgraded, err := manifest.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if upgraded.FrameworkVersion != "1.0.0" || upgraded.Project.Name != "service" {
		t.Fatalf("upgraded manifest = %+v", upgraded)
	}
	if len(upgraded.UpgradeHistory) != 1 || upgraded.UpgradeHistory[0].AppliedAt != "2026-08-05T21:00:00Z" || upgraded.UpgradeHistory[0].Backup == "" {
		t.Fatalf("history = %+v", upgraded.UpgradeHistory)
	}
}

func TestRunDryRunDoesNotModifyManifest(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "service")
	config := project.DefaultConfigForPreset("minimal-api")
	config.Project.Name = "service"
	config.Project.Module = "github.com/acme/service"
	if _, err := generator.Generate(generator.Request{Config: config, Destination: dir, FrameworkVersion: "0.9.0"}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(dir, ".gosvc", "manifest.json"))
	result, err := Run(Options{ProjectDir: dir, TargetVersion: "1.0.0", RuntimeVersion: "dev", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !result.UpgradeRequired || result.Applied {
		t.Fatalf("result = %+v", result)
	}
	after, _ := os.ReadFile(filepath.Join(dir, ".gosvc", "manifest.json"))
	if string(before) != string(after) {
		t.Fatal("dry-run modified manifest")
	}
}

func TestRunIsIdempotentAfterUpgrade(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "service")
	config := project.DefaultConfigForPreset("minimal-api")
	config.Project.Name = "service"
	config.Project.Module = "github.com/acme/service"
	if _, err := generator.Generate(generator.Request{Config: config, Destination: dir, FrameworkVersion: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	result, err := Run(Options{ProjectDir: dir, TargetVersion: "1.0.0", RuntimeVersion: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if result.UpgradeRequired || result.Applied {
		t.Fatalf("result = %+v", result)
	}
}

func TestRunRejectsDowngrade(t *testing.T) {
	t.Parallel()
	if err := rejectDowngrade("2.0.0", "1.0.0"); err == nil || !strings.Contains(err.Error(), "downgrade") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunCanSkipBackupExplicitly(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "service")
	config := project.DefaultConfigForPreset("minimal-api")
	config.Project.Name = "service"
	config.Project.Module = "github.com/acme/service"
	if _, err := generator.Generate(generator.Request{Config: config, Destination: dir, FrameworkVersion: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	result, err := Run(Options{ProjectDir: dir, TargetVersion: "1.1.0", RuntimeVersion: "dev", NoBackup: true})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.BackupPath != "" {
		t.Fatalf("result=%+v", result)
	}
}
