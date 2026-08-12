package projectcheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ailtonmacedo/gosvc/internal/generator"
	"github.com/ailtonmacedo/gosvc/internal/manifest"
	"github.com/ailtonmacedo/gosvc/internal/project"
)

func TestCheckAcceptsGeneratedMinimalProject(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "service")
	config := project.DefaultConfigForPreset("minimal-api")
	config.Project.Name = "test-service"
	config.Project.Module = "github.com/acme/test-service"
	if _, err := generator.Generate(generator.Request{Config: config, Destination: root, FrameworkVersion: "test"}); err != nil {
		t.Fatal(err)
	}
	report, err := Check(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.HasErrors() {
		t.Fatalf("issues=%+v", report.Issues)
	}
}

func TestCheckRejectsModifiedGeneratedFileAndWarnsForUserFile(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "service")
	config := project.DefaultConfigForPreset("minimal-api")
	config.Project.Name = "test-service"
	config.Project.Module = "github.com/acme/test-service"
	if _, err := generator.Generate(generator.Request{Config: config, Destination: root, FrameworkVersion: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("custom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := Check(root)
	if err != nil {
		t.Fatal(err)
	}
	var errors, warnings int
	for _, issue := range report.Issues {
		switch issue.Severity {
		case SeverityError:
			errors++
		case SeverityWarning:
			warnings++
		}
	}
	if errors == 0 || warnings == 0 {
		t.Fatalf("errors=%d warnings=%d issues=%+v", errors, warnings, report.Issues)
	}
}

func TestCheckDetectsManifestProjectMismatch(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "service")
	config := project.DefaultConfigForPreset("minimal-api")
	config.Project.Name = "service"
	config.Project.Module = "github.com/acme/service"
	if _, err := generator.Generate(generator.Request{Config: config, Destination: dir, FrameworkVersion: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	value, err := manifest.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	value.Project.Module = "github.com/other/service"
	data, err := manifest.Encode(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gosvc", "manifest.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := Check(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report.Error().Error(), "manifest project module") {
		t.Fatalf("report = %+v", report.Issues)
	}
}
