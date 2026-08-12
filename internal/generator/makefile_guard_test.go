package generator

import (
	"strings"
	"testing"

	"github.com/ailtonmacedo/gosvc/internal/preset"
	"github.com/ailtonmacedo/gosvc/internal/project"
)

func TestGeneratedMakefileScopesGitDriftChecksToProjectRoot(t *testing.T) {
	t.Parallel()

	config := project.DefaultConfigForPreset("postgres-api")
	config.Project.Name = "catalog-service"
	config.Project.Module = "github.com/acme/catalog-service"
	definition, err := preset.Resolve("postgres-api")
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := Render(config, definition, nil)
	if err != nil {
		t.Fatal(err)
	}

	var makefile string
	for _, artifact := range artifacts {
		if artifact.Path == "Makefile" {
			makefile = string(artifact.Content)
			break
		}
	}
	if makefile == "" {
		t.Fatal("generated Makefile not found")
	}
	for _, want := range []string{
		`git rev-parse --show-toplevel`,
		`$(pwd -P)`,
		`STRICT_GIT_DRIFT ?= 0`,
		`UPDATE go.mod/go.sum normalized locally`,
		`ERROR module metadata drift detected`,
		`UPDATE generated artifacts refreshed locally`,
		`ERROR generated-code drift detected`,
		`SKIP git tidy drift check (project is not its own Git worktree)`,
		`SKIP git generated drift check (project is not its own Git worktree)`,
		`verify-strict`,
	} {
		if !strings.Contains(makefile, want) {
			t.Fatalf("generated Makefile missing %q", want)
		}
	}
}

func TestGeneratedMakefileUsesProjectLocalGolangciCache(t *testing.T) {
	t.Parallel()

	config := project.DefaultConfigForPreset("postgres-api")
	config.Project.Name = "cache-isolation"
	config.Project.Module = "github.com/acme/cache-isolation"
	definition, err := preset.Resolve("postgres-api")
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := Render(config, definition, nil)
	if err != nil {
		t.Fatal(err)
	}

	var makefile, gitignore string
	for _, artifact := range artifacts {
		switch artifact.Path {
		case "Makefile":
			makefile = string(artifact.Content)
		case ".gitignore":
			gitignore = string(artifact.Content)
		}
	}
	if makefile == "" || gitignore == "" {
		t.Fatal("generated Makefile or .gitignore not found")
	}
	for _, expected := range []string{
		"GOLANGCI_LINT_CACHE := $(CURDIR)/.cache/golangci-lint",
		"GOLANGCI_LINT_CACHE_ROOT := $(GOLANGCI_LINT_CACHE)/.project-root",
		"RESET golangci-lint cache (project path changed)",
		"GOLANGCI_LINT_CACHE=$(GOLANGCI_LINT_CACHE) $(GOLANGCI_LINT) run",
		"rm -rf bin .cache coverage.out coverage.filtered.out",
	} {
		if !strings.Contains(makefile, expected) {
			t.Fatalf("generated Makefile missing %q", expected)
		}
	}
	if !strings.Contains(gitignore, "/.cache/") {
		t.Fatal("generated .gitignore must ignore project-local linter cache")
	}
}
