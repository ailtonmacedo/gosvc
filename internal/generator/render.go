package generator

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"runtime"
	"strings"
	"text/template"

	"github.com/ailtonmacedo/gosvc/internal/preset"
	"github.com/ailtonmacedo/gosvc/internal/project"
	"github.com/ailtonmacedo/gosvc/internal/resource"
)

//go:embed all:templates
var templateFS embed.FS

type TemplateData struct {
	Config          project.Config
	Preset          preset.Definition
	GoVersion       string
	GoToolchain     string
	DockerGoVersion string
}

type templateSpec struct {
	Source      string
	Path        string
	Mode        uint32
	Ownership   Ownership
	Presets     map[string]bool
	Routers     map[string]bool
	Persistence map[string]bool
}

var templateSpecs = []templateSpec{
	{Source: "templates/base/go.mod.tmpl", Path: "go.mod", Mode: 0o644, Ownership: OwnershipUser},
	{Source: "templates/base/project.yaml.tmpl", Path: "project.yaml", Mode: 0o644, Ownership: OwnershipUser},
	{Source: "templates/base/README.md.tmpl", Path: "README.md", Mode: 0o644, Ownership: OwnershipUser},
	{Source: "templates/base/gitignore.tmpl", Path: ".gitignore", Mode: 0o644, Ownership: OwnershipUser},
	{Source: "templates/base/env.example.tmpl", Path: ".env.example", Mode: 0o644, Ownership: OwnershipUser},
	{Source: "templates/base/dockerignore.tmpl", Path: ".dockerignore", Mode: 0o644, Ownership: OwnershipGenerated},
	{Source: "templates/base/Dockerfile.tmpl", Path: "Dockerfile", Mode: 0o644, Ownership: OwnershipGenerated},
	{Source: "templates/base/Makefile.tmpl", Path: "Makefile", Mode: 0o644, Ownership: OwnershipGenerated},
	{Source: "templates/base/ci.yaml.tmpl", Path: ".github/workflows/ci.yaml", Mode: 0o644, Ownership: OwnershipGenerated},
	{Source: "templates/base/golangci.yaml.tmpl", Path: ".golangci.yml", Mode: 0o644, Ownership: OwnershipGenerated},
	{Source: "templates/base/check-coverage.sh.tmpl", Path: "scripts/check-coverage.sh", Mode: 0o755, Ownership: OwnershipGenerated},
	{Source: "templates/base/main.go.tmpl", Path: "cmd/api/main.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"minimal-api": true, "postgres-api": true, "production-api": true, "event-driven-api": true}},
	{Source: "templates/base/domain.go.tmpl", Path: "internal/domain/doc.go", Mode: 0o644, Ownership: OwnershipUser},
	{Source: "templates/base/application.go.tmpl", Path: "internal/application/doc.go", Mode: 0o644, Ownership: OwnershipUser},
	{Source: "templates/base/ports.go.tmpl", Path: "internal/ports/doc.go", Mode: 0o644, Ownership: OwnershipUser},
	{Source: "templates/base/infrastructure.go.tmpl", Path: "internal/infrastructure/doc.go", Mode: 0o644, Ownership: OwnershipUser},
	{Source: "templates/base/config.go.tmpl", Path: "internal/config/config.go", Mode: 0o644, Ownership: OwnershipUser},
	{Source: "templates/nonhttp/app_main.go.tmpl", Path: "cmd/app/main.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"bare": true}},
	{Source: "templates/nonhttp/app_bootstrap.go.tmpl", Path: "internal/bootstrap/app.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"bare": true}},
	{Source: "templates/nonhttp/worker_main.go.tmpl", Path: "cmd/worker/main.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"worker": true}},
	{Source: "templates/nonhttp/worker.go.tmpl", Path: "internal/application/worker.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"worker": true}},
	{Source: "templates/nonhttp/worker_test.go.tmpl", Path: "internal/application/worker_test.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"worker": true}},
	{Source: "templates/nonhttp/worker_bootstrap.go.tmpl", Path: "internal/bootstrap/worker.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"worker": true}},
	{Source: "templates/base/bootstrap.go.tmpl", Path: "internal/bootstrap/app.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"minimal-api": true, "postgres-api": true, "production-api": true, "event-driven-api": true}},
	{Source: "templates/base/router.go.tmpl", Path: "internal/infrastructure/http/router.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"minimal-api": true, "postgres-api": true, "production-api": true, "event-driven-api": true}, Routers: map[string]bool{"chi": true}},
	{Source: "templates/base/router_echo.go.tmpl", Path: "internal/infrastructure/http/router.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"minimal-api": true, "postgres-api": true}, Routers: map[string]bool{"echo": true}},
	{Source: "templates/base/router_test.go.tmpl", Path: "internal/infrastructure/http/router_test.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"minimal-api": true, "postgres-api": true, "production-api": true, "event-driven-api": true}, Routers: map[string]bool{"chi": true}},
	{Source: "templates/base/router_echo_test.go.tmpl", Path: "internal/infrastructure/http/router_test.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"minimal-api": true, "postgres-api": true}, Routers: map[string]bool{"echo": true}},

	{Source: "templates/postgres/order.go.tmpl", Path: "internal/domain/order.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"postgres-api": true, "production-api": true, "event-driven-api": true}},
	{Source: "templates/postgres/errors.go.tmpl", Path: "internal/domain/errors.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"postgres-api": true, "production-api": true, "event-driven-api": true}},
	{Source: "templates/postgres/order_repository_port.go.tmpl", Path: "internal/ports/order_repository.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"postgres-api": true, "production-api": true, "event-driven-api": true}},
	{Source: "templates/postgres/order_service.go.tmpl", Path: "internal/application/order_service.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"postgres-api": true, "production-api": true, "event-driven-api": true}},
	{Source: "templates/postgres/order_service_test.go.tmpl", Path: "internal/application/order_service_test.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"postgres-api": true, "production-api": true, "event-driven-api": true}},
	{Source: "templates/postgres/pool.go.tmpl", Path: "internal/infrastructure/persistence/postgres/pool.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"postgres-api": true, "production-api": true, "event-driven-api": true}, Persistence: map[string]bool{"sqlc": true}},
	{Source: "templates/postgres/transaction.go.tmpl", Path: "internal/infrastructure/persistence/postgres/transaction.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"postgres-api": true, "production-api": true, "event-driven-api": true}, Persistence: map[string]bool{"sqlc": true}},
	{Source: "templates/postgres/timestamp.go.tmpl", Path: "internal/infrastructure/persistence/postgres/timestamp.go", Mode: 0o644, Ownership: OwnershipGenerated, Presets: map[string]bool{"postgres-api": true, "production-api": true, "event-driven-api": true}, Persistence: map[string]bool{"sqlc": true}},
	{Source: "templates/postgres/order_repository.go.tmpl", Path: "internal/infrastructure/persistence/postgres/order_repository.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"postgres-api": true, "production-api": true, "event-driven-api": true}, Persistence: map[string]bool{"sqlc": true}},
	{Source: "templates/postgres/sqlc_db.go.tmpl", Path: "internal/generated/sqlc/db.go", Mode: 0o644, Ownership: OwnershipGenerated, Presets: map[string]bool{"postgres-api": true, "production-api": true, "event-driven-api": true}, Persistence: map[string]bool{"sqlc": true}},
	{Source: "templates/postgres/sqlc_models.go.tmpl", Path: "internal/generated/sqlc/models.go", Mode: 0o644, Ownership: OwnershipGenerated, Presets: map[string]bool{"postgres-api": true, "production-api": true, "event-driven-api": true}, Persistence: map[string]bool{"sqlc": true}},
	{Source: "templates/postgres/sqlc_orders.go.tmpl", Path: "internal/generated/sqlc/orders.sql.go", Mode: 0o644, Ownership: OwnershipGenerated, Presets: map[string]bool{"postgres-api": true, "production-api": true, "event-driven-api": true}, Persistence: map[string]bool{"sqlc": true}},
	{Source: "templates/postgres/migration_up.sql.tmpl", Path: "db/migrations/000001_create_orders.up.sql", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"postgres-api": true, "production-api": true, "event-driven-api": true}},
	{Source: "templates/postgres/migration_down.sql.tmpl", Path: "db/migrations/000001_create_orders.down.sql", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"postgres-api": true, "production-api": true, "event-driven-api": true}},
	{Source: "templates/postgres/orders.sql.tmpl", Path: "db/queries/orders.sql", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"postgres-api": true, "production-api": true, "event-driven-api": true}, Persistence: map[string]bool{"sqlc": true}},
	{Source: "templates/postgres/sqlc.yaml.tmpl", Path: "sqlc.yaml", Mode: 0o644, Ownership: OwnershipGenerated, Presets: map[string]bool{"postgres-api": true, "production-api": true, "event-driven-api": true}, Persistence: map[string]bool{"sqlc": true}},
	{Source: "templates/postgres/compose.yaml.tmpl", Path: "compose.yaml", Mode: 0o644, Ownership: OwnershipGenerated, Presets: map[string]bool{"postgres-api": true, "production-api": true, "event-driven-api": true}},
	{Source: "templates/postgres/integration_test.go.tmpl", Path: "tests/integration/order_repository_test.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"postgres-api": true, "production-api": true, "event-driven-api": true}, Persistence: map[string]bool{"sqlc": true}},
	{Source: "templates/gorm/pool.go.tmpl", Path: "internal/infrastructure/persistence/postgres/pool.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"postgres-api": true}, Persistence: map[string]bool{"gorm": true}},
	{Source: "templates/gorm/transaction.go.tmpl", Path: "internal/infrastructure/persistence/postgres/transaction.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"postgres-api": true}, Persistence: map[string]bool{"gorm": true}},
	{Source: "templates/gorm/order_repository.go.tmpl", Path: "internal/infrastructure/persistence/postgres/order_repository.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"postgres-api": true}, Persistence: map[string]bool{"gorm": true}},
	{Source: "templates/gorm/integration_test.go.tmpl", Path: "tests/integration/order_repository_test.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"postgres-api": true}, Persistence: map[string]bool{"gorm": true}},
	{Source: "templates/postgres/order_handler.go.tmpl", Path: "internal/infrastructure/http/order_handler.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"postgres-api": true, "production-api": true, "event-driven-api": true}, Routers: map[string]bool{"chi": true}},
	{Source: "templates/postgres/order_handler_test.go.tmpl", Path: "internal/infrastructure/http/order_handler_test.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"postgres-api": true, "production-api": true, "event-driven-api": true}, Routers: map[string]bool{"chi": true}},
	{Source: "templates/postgres/order_handler_echo.go.tmpl", Path: "internal/infrastructure/http/order_handler.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"postgres-api": true}, Routers: map[string]bool{"echo": true}},
	{Source: "templates/postgres/order_handler_echo_test.go.tmpl", Path: "internal/infrastructure/http/order_handler_test.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"postgres-api": true}, Routers: map[string]bool{"echo": true}},
	{Source: "templates/postgres/http_errors.go.tmpl", Path: "internal/infrastructure/http/errors.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"postgres-api": true, "production-api": true, "event-driven-api": true}, Routers: map[string]bool{"chi": true}},
	{Source: "templates/postgres/http_errors_echo.go.tmpl", Path: "internal/infrastructure/http/errors.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"postgres-api": true}, Routers: map[string]bool{"echo": true}},
	{Source: "templates/postgres/oapi-codegen.yaml.tmpl", Path: "api/oapi-codegen.yaml", Mode: 0o644, Ownership: OwnershipGenerated, Presets: map[string]bool{"postgres-api": true, "production-api": true, "event-driven-api": true}},

	{Source: "templates/production/auth_domain.go.tmpl", Path: "internal/domain/auth.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"production-api": true, "event-driven-api": true}},
	{Source: "templates/production/auth_ports.go.tmpl", Path: "internal/ports/auth.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"production-api": true, "event-driven-api": true}},
	{Source: "templates/production/auth_service.go.tmpl", Path: "internal/application/auth_service.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"production-api": true, "event-driven-api": true}},
	{Source: "templates/production/auth_service_test.go.tmpl", Path: "internal/application/auth_service_test.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"production-api": true, "event-driven-api": true}},
	{Source: "templates/production/jwt.go.tmpl", Path: "internal/infrastructure/auth/jwt.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"production-api": true, "event-driven-api": true}},
	{Source: "templates/production/jwt_test.go.tmpl", Path: "internal/infrastructure/auth/jwt_test.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"production-api": true, "event-driven-api": true}},
	{Source: "templates/production/password.go.tmpl", Path: "internal/infrastructure/auth/password.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"production-api": true, "event-driven-api": true}},
	{Source: "templates/production/hash_password.go.tmpl", Path: "cmd/hash-password/main.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"production-api": true, "event-driven-api": true}},
	{Source: "templates/production/auth_repository.go.tmpl", Path: "internal/infrastructure/persistence/postgres/auth_repository.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"production-api": true, "event-driven-api": true}},
	{Source: "templates/production/auth_middleware.go.tmpl", Path: "internal/infrastructure/http/auth_middleware.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"production-api": true, "event-driven-api": true}},
	{Source: "templates/production/auth_handler.go.tmpl", Path: "internal/infrastructure/http/auth_handler.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"production-api": true, "event-driven-api": true}},
	{Source: "templates/production/auth_handler_test.go.tmpl", Path: "internal/infrastructure/http/auth_handler_test.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"production-api": true, "event-driven-api": true}},
	{Source: "templates/production/security_observability_test.go.tmpl", Path: "internal/infrastructure/http/security_observability_test.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"production-api": true, "event-driven-api": true}},
	{Source: "templates/production/rate_limit.go.tmpl", Path: "internal/infrastructure/http/rate_limit.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"production-api": true, "event-driven-api": true}},
	{Source: "templates/production/metrics.go.tmpl", Path: "internal/infrastructure/http/metrics.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"production-api": true, "event-driven-api": true}},
	{Source: "templates/production/logging.go.tmpl", Path: "internal/infrastructure/http/logging.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"production-api": true, "event-driven-api": true}},
	{Source: "templates/production/observability.go.tmpl", Path: "internal/observability/observability.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"production-api": true, "event-driven-api": true}},
	{Source: "templates/production/auth_bootstrap.go.tmpl", Path: "internal/bootstrap/auth.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"production-api": true, "event-driven-api": true}},
	{Source: "templates/production/auth_migration_up.sql.tmpl", Path: "db/migrations/900001_create_auth.up.sql", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"production-api": true, "event-driven-api": true}},
	{Source: "templates/production/auth_migration_down.sql.tmpl", Path: "db/migrations/900001_create_auth.down.sql", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"production-api": true, "event-driven-api": true}},
	{Source: "templates/production/prometheus.yml.tmpl", Path: "deployments/observability/prometheus.yml", Mode: 0o644, Ownership: OwnershipGenerated, Presets: map[string]bool{"production-api": true, "event-driven-api": true}},
	{Source: "templates/production/otel-collector.yaml.tmpl", Path: "deployments/observability/otel-collector.yaml", Mode: 0o644, Ownership: OwnershipGenerated, Presets: map[string]bool{"production-api": true, "event-driven-api": true}},

	{Source: "templates/distributed/event.go.tmpl", Path: "internal/domain/event.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"event-driven-api": true}},
	{Source: "templates/distributed/events_ports.go.tmpl", Path: "internal/ports/events.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"event-driven-api": true}},
	{Source: "templates/distributed/outbox_worker.go.tmpl", Path: "internal/application/outbox_worker.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"event-driven-api": true}},
	{Source: "templates/distributed/outbox_worker_test.go.tmpl", Path: "internal/application/outbox_worker_test.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"event-driven-api": true}},
	{Source: "templates/distributed/redis.go.tmpl", Path: "internal/infrastructure/cache/redis/store.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"event-driven-api": true}},
	{Source: "templates/distributed/redis_test.go.tmpl", Path: "internal/infrastructure/cache/redis/store_test.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"event-driven-api": true}},
	{Source: "templates/distributed/kafka.go.tmpl", Path: "internal/infrastructure/messaging/kafka/publisher.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"event-driven-api": true}},
	{Source: "templates/distributed/consumer.go.tmpl", Path: "internal/infrastructure/messaging/kafka/consumer.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"event-driven-api": true}},
	{Source: "templates/distributed/consumer_test.go.tmpl", Path: "internal/infrastructure/messaging/kafka/consumer_test.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"event-driven-api": true}},
	{Source: "templates/distributed/distributed_integration_test.go.tmpl", Path: "tests/integration/distributed_test.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"event-driven-api": true}},
	{Source: "templates/distributed/outbox_repository.go.tmpl", Path: "internal/infrastructure/persistence/postgres/outbox_repository.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"event-driven-api": true}},
	{Source: "templates/distributed/outbox_migration_up.sql.tmpl", Path: "db/migrations/900100_create_outbox.up.sql", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"event-driven-api": true}},
	{Source: "templates/distributed/outbox_migration_down.sql.tmpl", Path: "db/migrations/900100_create_outbox.down.sql", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"event-driven-api": true}},
	{Source: "templates/distributed/worker_main.go.tmpl", Path: "cmd/worker/main.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"event-driven-api": true}},
	{Source: "templates/distributed/worker_bootstrap.go.tmpl", Path: "internal/bootstrap/worker.go", Mode: 0o644, Ownership: OwnershipUser, Presets: map[string]bool{"event-driven-api": true}},
	{Source: "templates/distributed/Dockerfile.migrate.tmpl", Path: "Dockerfile.migrate", Mode: 0o644, Ownership: OwnershipGenerated, Presets: map[string]bool{"event-driven-api": true}},
	{Source: "templates/k8s/namespace.yaml.tmpl", Path: "deployments/k8s/00-namespace.yaml", Mode: 0o644, Ownership: OwnershipGenerated, Presets: map[string]bool{"event-driven-api": true}},
	{Source: "templates/k8s/configmap.yaml.tmpl", Path: "deployments/k8s/10-configmap.yaml", Mode: 0o644, Ownership: OwnershipGenerated, Presets: map[string]bool{"event-driven-api": true}},
	{Source: "templates/k8s/secret.yaml.tmpl", Path: "deployments/k8s/11-secret.example", Mode: 0o600, Ownership: OwnershipUser, Presets: map[string]bool{"event-driven-api": true}},
	{Source: "templates/k8s/api-deployment.yaml.tmpl", Path: "deployments/k8s/20-api-deployment.yaml", Mode: 0o644, Ownership: OwnershipGenerated, Presets: map[string]bool{"event-driven-api": true}},
	{Source: "templates/k8s/worker-deployment.yaml.tmpl", Path: "deployments/k8s/21-worker-deployment.yaml", Mode: 0o644, Ownership: OwnershipGenerated, Presets: map[string]bool{"event-driven-api": true}},
	{Source: "templates/k8s/service.yaml.tmpl", Path: "deployments/k8s/30-service.yaml", Mode: 0o644, Ownership: OwnershipGenerated, Presets: map[string]bool{"event-driven-api": true}},
	{Source: "templates/k8s/migration-job.yaml.tmpl", Path: "deployments/k8s/40-migration-job.yaml", Mode: 0o644, Ownership: OwnershipGenerated, Presets: map[string]bool{"event-driven-api": true}},
	{Source: "templates/k8s/network-policy.yaml.tmpl", Path: "deployments/k8s/50-network-policy.yaml", Mode: 0o644, Ownership: OwnershipGenerated, Presets: map[string]bool{"event-driven-api": true}},
}

func Render(config project.Config, definition preset.Definition, resources []resource.Definition) ([]Artifact, error) {
	goVersion := resolvedGoVersion(config.GoLanguageVersion())
	goToolchain := config.GoToolchainVersion()
	if goToolchain == "" {
		goToolchain = project.PreferredToolchain(goVersion)
	}
	data := TemplateData{
		Config:          config,
		Preset:          definition,
		GoVersion:       goVersion,
		GoToolchain:     goToolchain,
		DockerGoVersion: project.RequiredRuntimeGoVersionWithToolchain(goVersion, goToolchain),
	}
	artifacts := make([]Artifact, 0, len(templateSpecs))
	seen := make(map[string]struct{}, len(templateSpecs))
	for _, spec := range templateSpecs {
		if len(spec.Presets) > 0 && !spec.Presets[definition.Name] {
			continue
		}
		if len(spec.Routers) > 0 && !spec.Routers[config.API.Router] {
			continue
		}
		if len(spec.Persistence) > 0 && !spec.Persistence[config.Database.CodeGeneration] {
			continue
		}
		if _, exists := seen[spec.Path]; exists {
			return nil, fmt.Errorf("duplicate artifact path %q", spec.Path)
		}
		seen[spec.Path] = struct{}{}

		source, err := templateFS.ReadFile(spec.Source)
		if err != nil {
			return nil, fmt.Errorf("read template %q: %w", spec.Source, err)
		}
		tmpl, err := template.New(spec.Source).Option("missingkey=error").Parse(string(source))
		if err != nil {
			return nil, fmt.Errorf("parse template %q: %w", spec.Source, err)
		}
		var output bytes.Buffer
		if err := tmpl.Execute(&output, data); err != nil {
			return nil, fmt.Errorf("render template %q: %w", spec.Source, err)
		}
		content := output.Bytes()
		if strings.HasSuffix(spec.Path, ".go") {
			content, err = format.Source(content)
			if err != nil {
				return nil, fmt.Errorf("format generated Go file %q: %w", spec.Path, err)
			}
		}
		artifact := Artifact{
			Path:      spec.Path,
			Content:   append([]byte(nil), content...),
			Mode:      fileMode(spec.Mode),
			Ownership: spec.Ownership,
		}
		if err := artifact.Validate(); err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	managed, err := renderManagedResources(config, resources)
	if err != nil {
		return nil, err
	}
	for _, artifact := range managed {
		if _, exists := seen[artifact.Path]; exists {
			return nil, fmt.Errorf("duplicate artifact path %q", artifact.Path)
		}
		seen[artifact.Path] = struct{}{}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, nil
}

func resolvedGoVersion(value string) string {
	if value != "" && value != "auto" {
		return value
	}
	version := strings.TrimPrefix(runtime.Version(), "go")
	parts := strings.Split(version, ".")
	if len(parts) >= 2 && isDigits(parts[0]) && isDigits(parts[1]) {
		return parts[0] + "." + parts[1]
	}
	return "1.23"
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
