package project

import (
	"strings"
	"testing"
)

func TestLoadRuntimeGoPolicy(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `schema_version: 1
project:
  name: catalog-service
  module: github.com/acme/catalog-service
  preset: postgres-api
runtime:
  go:
    language: 1.25.0
    toolchain: go1.25.12
`)
	config, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := config.GoLanguageVersion(); got != "1.25.0" {
		t.Fatalf("language=%q", got)
	}
	if got := config.GoToolchainVersion(); got != "go1.25.12" {
		t.Fatalf("toolchain=%q", got)
	}
}

func TestLoadLegacyProjectGoVersion(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `schema_version: 1
project:
  name: catalog-service
  module: github.com/acme/catalog-service
  go_version: 1.25.0
  preset: postgres-api
`)
	config, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := config.GoLanguageVersion(); got != "1.25.0" {
		t.Fatalf("language=%q", got)
	}
	if got := config.GoToolchainVersion(); got != "go1.25.12" {
		t.Fatalf("toolchain=%q", got)
	}
}

func TestLoadRejectsConflictingLegacyAndRuntimeGo(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `schema_version: 1
project:
  name: catalog-service
  module: github.com/acme/catalog-service
  go_version: 1.25.0
  preset: postgres-api
runtime:
  go:
    language: 1.26.0
    toolchain: go1.26.0
`)
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "conflicts with runtime.go.language") {
		t.Fatalf("Load() error=%v", err)
	}
}

func TestValidateRejectsToolchainBelowSecurityFloor(t *testing.T) {
	t.Parallel()
	config := DefaultConfigForPreset("postgres-api")
	config.Project.Name = "catalog-service"
	config.Project.Module = "github.com/acme/catalog-service"
	config.Runtime.Go.Toolchain = "go1.25.11"
	err := config.Validate()
	if err == nil || !strings.Contains(err.Error(), "runtime.go.toolchain") {
		t.Fatalf("Validate() error=%v", err)
	}
}
