package preset

import (
	"fmt"
	"sort"
)

// Definition describes a built-in project preset and the features that it
// enables. Feature behavior is implemented by the generator; the preset only
// declares the desired composition.
type Definition struct {
	Name        string
	Description string
	Features    []string
}

var builtins = map[string]Definition{
	"minimal-api": {
		Name:        "minimal-api",
		Description: "Minimal Go service scaffold",
		Features:    []string{"base", "config", "clean-architecture", "chi", "health", "docker", "testing", "coverage", "lint", "ci"},
	},
	"postgres-api": {
		Name:        "postgres-api",
		Description: "Go service scaffold prepared for PostgreSQL",
		Features:    []string{"base", "config", "clean-architecture", "chi", "health", "docker", "postgres", "migrations", "sqlc", "docker-compose", "openapi", "openapi-validation", "redoc", "testing", "coverage", "lint", "ci"},
	},
	"production-api": {
		Name:        "production-api",
		Description: "Production-ready Go API with PostgreSQL, security, and observability",
		Features:    []string{"base", "config", "clean-architecture", "chi", "health", "docker", "postgres", "migrations", "sqlc", "docker-compose", "openapi", "openapi-validation", "redoc", "jwt", "refresh-token", "rbac", "rate-limit-local", "slog", "prometheus", "opentelemetry", "pprof", "testing", "coverage", "lint", "ci"},
	},
	"event-driven-api": {
		Name:        "event-driven-api",
		Description: "Distributed Go API with Redis, Kafka, transactional outbox, and Kubernetes",
		Features:    []string{"base", "config", "clean-architecture", "chi", "health", "docker", "postgres", "migrations", "sqlc", "docker-compose", "openapi", "openapi-validation", "redoc", "jwt", "refresh-token", "rbac", "rate-limit-local", "slog", "prometheus", "opentelemetry", "pprof", "redis", "kafka", "outbox", "idempotency", "dead-letter", "kubernetes", "testing", "coverage", "lint", "ci"},
	},
}

func Resolve(name string) (Definition, error) {
	definition, ok := builtins[name]
	if !ok {
		return Definition{}, fmt.Errorf("unknown preset %q; available presets: %v", name, Names())
	}
	definition.Features = append([]string(nil), definition.Features...)
	return definition, nil
}

func Names() []string {
	names := make([]string, 0, len(builtins))
	for name := range builtins {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
