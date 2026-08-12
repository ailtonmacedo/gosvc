package preset

import (
	"fmt"
	"sort"
	"strings"
)

// Definition describes a built-in project preset and the features that it
// enables. Preset versions are independent from the gosvc CLI version so the
// generator can evolve without forcing every template family to move in lockstep.
type ComponentOptions struct {
	DefaultRouter      string
	Routers            []string
	DefaultPersistence string
	Persistence        []string
}

type Definition struct {
	Name        string
	Version     string
	Description string
	Kind        string
	Features    []string
	Components  ComponentOptions
}

var registry = map[string]map[string]Definition{
	"bare": {
		"1.0.0": {
			Name: "bare", Version: "1.0.0", Kind: "app",
			Description: "Clean Architecture application scaffold without HTTP or database adapters",
			Features:    []string{"base", "config", "clean-architecture", "docker", "testing", "coverage", "lint", "ci"},
		},
	},
	"worker": {
		"1.0.0": {
			Name: "worker", Version: "1.0.0", Kind: "worker",
			Description: "Long-running background worker scaffold without HTTP or database adapters",
			Features:    []string{"base", "config", "clean-architecture", "worker", "docker", "testing", "coverage", "lint", "ci"},
		},
	},
	"minimal-api": {
		"1.0.0": {
			Name: "minimal-api", Version: "1.0.0", Kind: "api",
			Description: "Minimal Go HTTP service scaffold (legacy golden path)",
			Features:    []string{"base", "config", "clean-architecture", "http", "router", "health", "docker", "testing", "coverage", "lint", "ci"},
			Components:  ComponentOptions{DefaultRouter: "chi", Routers: []string{"chi"}},
		},
		"1.1.0": {
			Name: "minimal-api", Version: "1.1.0", Kind: "api",
			Description: "Minimal Go HTTP service scaffold",
			Features:    []string{"base", "config", "clean-architecture", "http", "router", "health", "docker", "testing", "coverage", "lint", "ci"},
			Components:  ComponentOptions{DefaultRouter: "chi", Routers: []string{"chi", "echo"}},
		},
	},
	"postgres-api": {
		"1.0.0": {
			Name: "postgres-api", Version: "1.0.0", Kind: "api",
			Description: "Go HTTP service scaffold prepared for PostgreSQL (Chi + sqlc golden path)",
			Features:    []string{"base", "config", "clean-architecture", "http", "router", "health", "docker", "postgres", "migrations", "persistence", "docker-compose", "openapi", "openapi-validation", "redoc", "testing", "coverage", "lint", "ci"},
			Components:  ComponentOptions{DefaultRouter: "chi", Routers: []string{"chi"}, DefaultPersistence: "sqlc", Persistence: []string{"sqlc"}},
		},
		"1.1.0": {
			Name: "postgres-api", Version: "1.1.0", Kind: "api",
			Description: "Go HTTP service scaffold prepared for PostgreSQL",
			Features:    []string{"base", "config", "clean-architecture", "http", "router", "health", "docker", "postgres", "migrations", "persistence", "docker-compose", "openapi", "openapi-validation", "redoc", "testing", "coverage", "lint", "ci"},
			Components:  ComponentOptions{DefaultRouter: "chi", Routers: []string{"chi", "echo"}, DefaultPersistence: "sqlc", Persistence: []string{"sqlc", "gorm"}},
		},
	},
	"production-api": {
		"1.0.0": {
			Name: "production-api", Version: "1.0.0", Kind: "api",
			Description: "Production-ready Go API with PostgreSQL, security, and observability",
			Features:    []string{"base", "config", "clean-architecture", "http", "router", "health", "docker", "postgres", "migrations", "sqlc", "docker-compose", "openapi", "openapi-validation", "redoc", "jwt", "refresh-token", "rbac", "rate-limit-local", "slog", "prometheus", "opentelemetry", "pprof", "testing", "coverage", "lint", "ci"},
			Components:  ComponentOptions{DefaultRouter: "chi", Routers: []string{"chi"}, DefaultPersistence: "sqlc", Persistence: []string{"sqlc"}},
		},
	},
	"event-driven-api": {
		"1.0.0": {
			Name: "event-driven-api", Version: "1.0.0", Kind: "api",
			Description: "Distributed Go API with Redis, Kafka, transactional outbox, and Kubernetes",
			Features:    []string{"base", "config", "clean-architecture", "http", "router", "health", "docker", "postgres", "migrations", "sqlc", "docker-compose", "openapi", "openapi-validation", "redoc", "jwt", "refresh-token", "rbac", "rate-limit-local", "slog", "prometheus", "opentelemetry", "pprof", "redis", "kafka", "outbox", "idempotency", "dead-letter", "kubernetes", "testing", "coverage", "lint", "ci"},
			Components:  ComponentOptions{DefaultRouter: "chi", Routers: []string{"chi"}, DefaultPersistence: "sqlc", Persistence: []string{"sqlc"}},
		},
	},
}

var current = map[string]string{
	"bare":             "1.0.0",
	"worker":           "1.0.0",
	"minimal-api":      "1.1.0",
	"postgres-api":     "1.1.0",
	"production-api":   "1.0.0",
	"event-driven-api": "1.0.0",
}

func Resolve(name string) (Definition, error) {
	version, ok := current[name]
	if !ok {
		return Definition{}, fmt.Errorf("unknown preset %q; available presets: %v", name, Names())
	}
	return ResolveVersion(name, version)
}

func ResolveVersion(name, version string) (Definition, error) {
	versions, ok := registry[name]
	if !ok {
		return Definition{}, fmt.Errorf("unknown preset %q; available presets: %v", name, Names())
	}
	if strings.TrimSpace(version) == "" || version == "current" {
		version = current[name]
	}
	definition, ok := versions[version]
	if !ok {
		return Definition{}, fmt.Errorf("unsupported version %q for preset %q; available versions: %v", version, name, Versions(name))
	}
	definition.Features = append([]string(nil), definition.Features...)
	definition.Components.Routers = append([]string(nil), definition.Components.Routers...)
	definition.Components.Persistence = append([]string(nil), definition.Components.Persistence...)
	return definition, nil
}

func CurrentVersion(name string) (string, error) {
	version, ok := current[name]
	if !ok {
		return "", fmt.Errorf("unknown preset %q; available presets: %v", name, Names())
	}
	return version, nil
}

func Names() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func Versions(name string) []string {
	versions := registry[name]
	result := make([]string, 0, len(versions))
	for version := range versions {
		result = append(result, version)
	}
	sort.Strings(result)
	return result
}

func Definitions() []Definition {
	result := make([]Definition, 0, len(current))
	for _, name := range Names() {
		definition, _ := Resolve(name)
		result = append(result, definition)
	}
	return result
}
