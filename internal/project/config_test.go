package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadAppliesDefaults(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `schema_version: 1
project:
  name: order-service
  module: github.com/acme/order-service
  preset: minimal-api
api:
  enabled: true
`)

	config, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if config.API.Port != 8080 {
		t.Fatalf("API.Port = %d, want 8080", config.API.Port)
	}
	if config.API.ReadTimeout != 10*time.Second {
		t.Fatalf("API.ReadTimeout = %s, want 10s", config.API.ReadTimeout)
	}
	if config.Quality.Coverage.Minimum != 80 {
		t.Fatalf("coverage = %d, want 80", config.Quality.Coverage.Minimum)
	}
}

func TestLoadParsesDurations(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `schema_version: 1
project:
  name: order-service
  module: github.com/acme/order-service
  preset: minimal-api
api:
  enabled: true
  read_timeout: 3s
  write_timeout: 4s
  idle_timeout: 30s
  shutdown_timeout: 8s
`)

	config, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if config.API.ReadTimeout != 3*time.Second {
		t.Fatalf("API.ReadTimeout = %s, want 3s", config.API.ReadTimeout)
	}
}

func TestValidateReturnsMultipleErrors(t *testing.T) {
	t.Parallel()

	config := Config{
		SchemaVersion: 99,
		Project: ProjectSection{
			Name:   "Order Service",
			Module: "invalid",
			Preset: "unknown",
		},
		Architecture: ArchitectureConfig{Type: "hexagonal", Layout: "vertical"},
		API:          APIConfig{Enabled: true, Router: "mux", Port: 70000},
		Quality:      QualityConfig{Coverage: CoverageConfig{Minimum: 120}},
	}

	err := config.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want validation errors")
	}
	message := err.Error()
	for _, expected := range []string{
		"schema_version",
		"project.name",
		"project.module",
		"project.preset",
		"architecture.type",
		"api.port",
		"quality.coverage.minimum",
	} {
		if !strings.Contains(message, expected) {
			t.Errorf("error %q does not contain %q", message, expected)
		}
	}
}

func TestLoadRejectsUnknownField(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `schema_version: 1
project:
  name: order-service
  module: github.com/acme/order-service
  preset: minimal-api
mystery: true
`)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "field mystery not found") {
		t.Fatalf("Load() error = %v, want unknown field error", err)
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "project.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadAppliesPostgresPresetDefaults(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `schema_version: 1
project:
  name: order-service
  module: github.com/acme/order-service
  preset: postgres-api
`)

	config, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !config.Database.Enabled {
		t.Fatal("Database.Enabled = false, want true")
	}
	if config.Database.Driver != "pgx" || config.Database.Pool != "pgxpool" {
		t.Fatalf("database defaults = driver:%s pool:%s", config.Database.Driver, config.Database.Pool)
	}
	if !config.OpenAPI.Enabled || !config.OpenAPI.RequestValidation || config.OpenAPI.Documentation != "redoc" {
		t.Fatalf("openapi defaults = enabled:%t validation:%t documentation:%s", config.OpenAPI.Enabled, config.OpenAPI.RequestValidation, config.OpenAPI.Documentation)
	}
	if !config.Deployment.Compose {
		t.Fatal("Deployment.Compose = false, want true")
	}
}

func TestValidateRejectsDisabledDatabaseForPostgresPreset(t *testing.T) {
	t.Parallel()

	config := DefaultConfigForPreset("postgres-api")
	config.Project.Name = "order-service"
	config.Project.Module = "github.com/acme/order-service"
	config.Database.Enabled = false

	err := config.Validate()
	if err == nil || !strings.Contains(err.Error(), "database.enabled") {
		t.Fatalf("Validate() error = %v, want database.enabled error", err)
	}
}

func TestLoadAppliesEventDrivenPresetDefaults(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `schema_version: 1
project:
  name: event-service
  module: github.com/acme/event-service
  preset: event-driven-api
`)
	config, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !config.Cache.Enabled || !config.Messaging.Enabled || !config.Outbox.Enabled {
		t.Fatalf("distributed defaults disabled: %+v", config)
	}
	if !config.Deployment.Kubernetes {
		t.Fatal("Kubernetes default disabled")
	}
	if config.Messaging.Provider != "kafka" || config.Cache.Provider != "redis" {
		t.Fatalf("providers: cache=%s messaging=%s", config.Cache.Provider, config.Messaging.Provider)
	}
}

func TestValidateRejectsIncompleteEventDrivenPreset(t *testing.T) {
	t.Parallel()
	config := DefaultConfigForPreset("event-driven-api")
	config.Project.Name = "event-service"
	config.Project.Module = "github.com/acme/event-service"
	config.Messaging.Enabled = false
	config.Outbox.Enabled = false
	config.Deployment.Kubernetes = false
	err := config.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	for _, field := range []string{"messaging.enabled", "outbox.enabled", "deployment.kubernetes"} {
		if !strings.Contains(err.Error(), field) {
			t.Fatalf("error %q missing %s", err, field)
		}
	}
}
