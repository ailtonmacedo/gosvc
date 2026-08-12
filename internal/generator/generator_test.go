package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ailtonmacedo/gosvc/internal/manifest"
	"github.com/ailtonmacedo/gosvc/internal/project"
	"github.com/ailtonmacedo/gosvc/internal/resource"
)

func TestGenerateCreatesCompilableProjectAndIsIdempotent(t *testing.T) {
	t.Parallel()

	destination := filepath.Join(t.TempDir(), "order-service")
	config := testConfig("minimal-api")

	first, err := Generate(Request{
		Config:           config,
		Destination:      destination,
		FrameworkVersion: "0.2.0-test",
	})
	if err != nil {
		t.Fatalf("Generate() first error = %v", err)
	}
	if !first.Applied {
		t.Fatal("Generate() first Applied = false, want true")
	}
	for _, path := range []string{
		"go.mod",
		"project.yaml",
		".golangci.yml",
		"scripts/check-coverage.sh",
		"cmd/api/main.go",
		"internal/domain/doc.go",
		manifest.Path,
	} {
		if _, err := os.Stat(filepath.Join(destination, filepath.FromSlash(path))); err != nil {
			t.Fatalf("generated file %q missing: %v", path, err)
		}
	}

	second, err := Generate(Request{
		Config:           config,
		Destination:      destination,
		FrameworkVersion: "0.2.0-test",
	})
	if err != nil {
		t.Fatalf("Generate() second error = %v", err)
	}
	if second.Applied {
		t.Fatal("Generate() second Applied = true, want false")
	}
	for _, change := range second.Changes {
		if change.Action != ActionSkip {
			t.Fatalf("second generation action for %s = %s, want SKIP", change.Artifact.Path, change.Action)
		}
	}
	ci, err := os.ReadFile(filepath.Join(destination, ".github/workflows/ci.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(ci), "make generate") {
		t.Fatalf("minimal-api CI contains unavailable generator targets: %s", ci)
	}
	coverageScript, err := os.Stat(filepath.Join(destination, "scripts/check-coverage.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if coverageScript.Mode().Perm()&0o111 == 0 {
		t.Fatalf("coverage script mode = %v, want executable", coverageScript.Mode())
	}
	compileGeneratedProject(t, destination, false)
}

func TestGenerateProtectsUserFiles(t *testing.T) {
	t.Parallel()

	destination := filepath.Join(t.TempDir(), "order-service")
	config := testConfig("minimal-api")
	if _, err := Generate(Request{Config: config, Destination: destination, FrameworkVersion: "dev"}); err != nil {
		t.Fatal(err)
	}

	readme := filepath.Join(destination, "README.md")
	custom := []byte("# My custom documentation\n")
	if err := os.WriteFile(readme, custom, 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Generate(Request{Config: config, Destination: destination, FrameworkVersion: "dev"})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if result.Applied {
		t.Fatal("Generate() Applied = true, want false when only user content differs")
	}
	foundProtect := false
	for _, change := range result.Changes {
		if change.Artifact.Path == "README.md" && change.Action == ActionProtect {
			foundProtect = true
		}
	}
	if !foundProtect {
		t.Fatalf("README.md was not protected: %+v", result.Changes)
	}
	content, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != string(custom) {
		t.Fatalf("README.md was overwritten: %q", content)
	}
}

func TestGenerateRejectsModifiedGeneratedFileUnlessForced(t *testing.T) {
	t.Parallel()

	destination := filepath.Join(t.TempDir(), "order-service")
	config := testConfig("minimal-api")
	if _, err := Generate(Request{Config: config, Destination: destination, FrameworkVersion: "dev"}); err != nil {
		t.Fatal(err)
	}

	makefile := filepath.Join(destination, "Makefile")
	if err := os.WriteFile(makefile, []byte("custom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Generate(Request{Config: config, Destination: destination, FrameworkVersion: "dev"})
	if err == nil || !strings.Contains(err.Error(), "modified") {
		t.Fatalf("Generate() error = %v, want modified generated file conflict", err)
	}

	result, err := Generate(Request{Config: config, Destination: destination, FrameworkVersion: "dev", Force: true})
	if err != nil {
		t.Fatalf("forced Generate() error = %v", err)
	}
	if !result.Applied {
		t.Fatal("forced Generate() Applied = false, want true")
	}
	content, err := os.ReadFile(makefile)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) == "custom\n" {
		t.Fatal("forced Generate() did not restore generated Makefile")
	}
}

func TestGenerateDryRunDoesNotWrite(t *testing.T) {
	t.Parallel()

	destination := filepath.Join(t.TempDir(), "dry-run")
	result, err := Generate(Request{
		Config:           testConfig("minimal-api"),
		Destination:      destination,
		DryRun:           true,
		FrameworkVersion: "dev",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied {
		t.Fatal("dry run Applied = true")
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("dry run destination exists or stat failed: %v", err)
	}
}

func TestGeneratePostgresPresetCreatesDatabaseLayout(t *testing.T) {
	t.Parallel()

	destination := filepath.Join(t.TempDir(), "postgres-service")
	if _, err := Generate(Request{
		Config:           testConfig("postgres-api"),
		Destination:      destination,
		FrameworkVersion: "dev",
	}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"db/migrations/000001_create_orders.up.sql",
		"internal/application/order_service_test.go",
		"db/migrations/000001_create_orders.down.sql",
		"db/queries/orders.sql",
		"sqlc.yaml",
		"compose.yaml",
		"internal/infrastructure/persistence/postgres/pool.go",
		"internal/generated/sqlc/orders.sql.go",
	} {
		if _, err := os.Stat(filepath.Join(destination, filepath.FromSlash(path))); err != nil {
			t.Fatalf("postgres layout file %q missing: %v", path, err)
		}
	}
	sqlcConfig, err := os.ReadFile(filepath.Join(destination, "sqlc.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"schema: db/migrations", "queries: db/queries", "github.com/google/uuid"} {
		if !strings.Contains(string(sqlcConfig), expected) {
			t.Fatalf("sqlc.yaml missing %q: %s", expected, sqlcConfig)
		}
	}
	if strings.Contains(string(sqlcConfig), "pg_catalog.timestamptz") {
		t.Fatalf("sqlc.yaml should preserve pgx/v5 native timestamptz mapping: %s", sqlcConfig)
	}
	models, err := os.ReadFile(filepath.Join(destination, "internal/generated/sqlc/models.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(models), "pgtype.Timestamptz") {
		t.Fatalf("sqlc snapshot should match pgx/v5 native timestamptz mapping: %s", models)
	}
	timestampHelper, err := os.ReadFile(filepath.Join(destination, "internal/infrastructure/persistence/postgres/timestamp.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"timeFromDB", "timeToDB", "pgtype.Timestamptz", "//nolint:unused // Used by generated resources that declare datetime fields."} {
		if !strings.Contains(string(timestampHelper), expected) {
			t.Fatalf("timestamp helper missing %q: %s", expected, timestampHelper)
		}
	}
	ci, err := os.ReadFile(filepath.Join(destination, ".github/workflows/ci.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ci), "make generate") {
		t.Fatalf("postgres CI does not validate generators: %s", ci)
	}
	mod, err := os.ReadFile(filepath.Join(destination, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"go 1.25.0", "toolchain go1.25.12"} {
		if !strings.Contains(string(mod), expected) {
			t.Fatalf("postgres go.mod missing secure toolchain invariant %q: %s", expected, mod)
		}
	}
	dockerfile, err := os.ReadFile(filepath.Join(destination, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dockerfile), "FROM golang:1.25.12-bookworm AS builder") {
		t.Fatalf("postgres Dockerfile does not use patched Go builder: %s", dockerfile)
	}
	compileGeneratedProject(t, destination, true)
}

func TestGenerateRejectsUnknownNonEmptyDirectory(t *testing.T) {
	t.Parallel()

	destination := filepath.Join(t.TempDir(), "existing")
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "notes.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Generate(Request{Config: testConfig("minimal-api"), Destination: destination})
	if err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("Generate() error = %v, want non-empty directory error", err)
	}
}

func testConfig(preset string) project.Config {
	config := project.DefaultConfigForPreset(preset)
	config.Project.Name = "order-service"
	config.Project.Module = "github.com/acme/order-service"
	if preset == "production-api" || preset == "event-driven-api" {
		config.Auth.AccessToken.Issuer = "order-service"
		config.Auth.AccessToken.Audience = "order-service-api"
	}
	return config
}

func TestAddResourceGeneratesCRUDAndIsIdempotent(t *testing.T) {
	t.Parallel()
	destination := filepath.Join(t.TempDir(), "catalog-service")
	config := testConfig("postgres-api")
	config.Project.Name = "catalog-service"
	config.Project.Module = "github.com/acme/catalog-service"
	if _, err := Generate(Request{Config: config, Destination: destination, FrameworkVersion: "dev"}); err != nil {
		t.Fatal(err)
	}
	definition, err := resource.Parse("product", "id:uuid,name:string,price:decimal,active:bool,released_at:datetime")
	if err != nil {
		t.Fatal(err)
	}
	first, added, err := AddResource(AddResourceRequest{ProjectDir: destination, Definition: definition, FrameworkVersion: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if !added || !first.Applied {
		t.Fatalf("added=%t applied=%t", added, first.Applied)
	}
	for _, path := range []string{
		"internal/domain/product.go",
		"internal/application/product_service.go",
		"internal/ports/product_repository.go",
		"internal/infrastructure/http/product_handler.go",
		"internal/infrastructure/http/product_handler_test.go",
		"internal/infrastructure/persistence/postgres/product_repository.go",
		"db/queries/products.sql",
		"db/migrations/000002_create_products.up.sql",
		"api/openapi.yaml",
	} {
		if _, err := os.Stat(filepath.Join(destination, filepath.FromSlash(path))); err != nil {
			t.Fatalf("missing %s: %v", path, err)
		}
	}
	migration, err := os.ReadFile(filepath.Join(destination, "db/migrations/000002_create_products.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(migration), "CREATE EXTENSION IF NOT EXISTS pgcrypto") {
		t.Fatalf("uuid migration does not enable pgcrypto: %s", migration)
	}

	second, added, err := AddResource(AddResourceRequest{ProjectDir: destination, Definition: definition, FrameworkVersion: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if added || second.Applied {
		t.Fatalf("second added=%t applied=%t", added, second.Applied)
	}
	compileGeneratedProject(t, destination, true)
}

func TestGenerateProductionPresetCreatesSecurityAndObservability(t *testing.T) {
	t.Parallel()

	destination := filepath.Join(t.TempDir(), "production-service")
	config := testConfig("production-api")
	if _, err := Generate(Request{
		Config:           config,
		Destination:      destination,
		FrameworkVersion: "dev",
	}); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{
		"internal/domain/auth.go",
		"internal/application/auth_service.go",
		"internal/infrastructure/auth/jwt.go",
		"internal/infrastructure/auth/password.go",
		"internal/infrastructure/http/auth_handler.go",
		"internal/infrastructure/http/auth_middleware.go",
		"internal/infrastructure/http/rate_limit.go",
		"internal/infrastructure/http/logging.go",
		"internal/infrastructure/http/metrics.go",
		"internal/observability/observability.go",
		"internal/bootstrap/auth.go",
		"cmd/hash-password/main.go",
		"db/migrations/900001_create_auth.up.sql",
		"deployments/observability/prometheus.yml",
		"deployments/observability/otel-collector.yaml",
	} {
		if _, err := os.Stat(filepath.Join(destination, filepath.FromSlash(path))); err != nil {
			t.Fatalf("production layout file %q missing: %v", path, err)
		}
	}

	openapi, err := os.ReadFile(filepath.Join(destination, "api/openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"/auth/login", "/auth/refresh", "/auth/logout", "/admin/ping", "bearerAuth"} {
		if !strings.Contains(string(openapi), expected) {
			t.Fatalf("OpenAPI contract missing %q: %s", expected, openapi)
		}
	}

	contract := string(openapi)
	for _, operation := range []string{"operationId: login", "operationId: refreshToken", "operationId: logout"} {
		idx := strings.Index(contract, operation)
		if idx < 0 {
			t.Fatalf("OpenAPI contract missing %q", operation)
		}
		end := idx + 180
		if end > len(contract) {
			end = len(contract)
		}
		if !strings.Contains(contract[idx:end], "security: []") {
			t.Fatalf("public auth operation %q must override global bearer security: %s", operation, contract[idx:end])
		}
	}

	definition, err := resource.Parse("product", "id:uuid,name:string,price:decimal,active:bool,released_at:datetime")
	if err != nil {
		t.Fatal(err)
	}
	if _, added, err := AddResource(AddResourceRequest{ProjectDir: destination, Definition: definition, FrameworkVersion: "dev"}); err != nil {
		t.Fatal(err)
	} else if !added {
		t.Fatal("production resource was not added")
	}
	if _, err := os.Stat(filepath.Join(destination, "db/migrations/000002_create_products.up.sql")); err != nil {
		t.Fatalf("production resource migration missing: %v", err)
	}

	mod, err := os.ReadFile(filepath.Join(destination, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"github.com/golang-jwt/jwt/v5",
		"github.com/prometheus/client_golang",
		"go.opentelemetry.io/otel",
		"golang.org/x/crypto",
		"golang.org/x/time",
	} {
		if !strings.Contains(string(mod), expected) {
			t.Fatalf("go.mod missing %q: %s", expected, mod)
		}
	}

	compileGeneratedProject(t, destination, true, true)
}

func TestGenerateEventDrivenPresetCreatesDistributedRuntime(t *testing.T) {
	t.Parallel()
	destination := filepath.Join(t.TempDir(), "event-service")
	config := testConfig("event-driven-api")
	if _, err := Generate(Request{Config: config, Destination: destination, FrameworkVersion: "dev"}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"cmd/worker/main.go",
		"internal/application/outbox_worker.go",
		"internal/infrastructure/cache/redis/store.go",
		"internal/infrastructure/messaging/kafka/publisher.go",
		"internal/infrastructure/messaging/kafka/consumer.go",
		"internal/infrastructure/persistence/postgres/outbox_repository.go",
		"db/migrations/900100_create_outbox.up.sql",
		"deployments/k8s/20-api-deployment.yaml",
		"deployments/k8s/21-worker-deployment.yaml",
		"deployments/k8s/40-migration-job.yaml",
	} {
		if _, err := os.Stat(filepath.Join(destination, filepath.FromSlash(path))); err != nil {
			t.Fatalf("event-driven file %q missing: %v", path, err)
		}
	}
	mod, err := os.ReadFile(filepath.Join(destination, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for _, dependency := range []string{"github.com/redis/go-redis/v9", "github.com/twmb/franz-go"} {
		if !strings.Contains(string(mod), dependency) {
			t.Fatalf("go.mod missing %s", dependency)
		}
	}
	compose, err := os.ReadFile(filepath.Join(destination, "compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, service := range []string{"redis:", "kafka:", "worker:"} {
		if !strings.Contains(string(compose), service) {
			t.Fatalf("compose missing %s", service)
		}
	}
	compileGeneratedProject(t, destination, true, true, true)
}

func TestGenerateForceStillProtectsUserFiles(t *testing.T) {
	t.Parallel()
	destination := filepath.Join(t.TempDir(), "service")
	config := testConfig("minimal-api")
	if _, err := Generate(Request{Config: config, Destination: destination, FrameworkVersion: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	custom := []byte("# custom readme\n")
	if err := os.WriteFile(filepath.Join(destination, "README.md"), custom, 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Generate(Request{Config: config, Destination: destination, FrameworkVersion: "1.0.0", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range result.Changes {
		if change.Artifact.Path == "README.md" && change.Action != ActionProtect {
			t.Fatalf("README action = %s, want PROTECT", change.Action)
		}
	}
	content, err := os.ReadFile(filepath.Join(destination, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != string(custom) {
		t.Fatalf("README overwritten: %s", content)
	}
}

func TestGeneratedHTTPHelpersMatchPresetFeatures(t *testing.T) {
	t.Parallel()

	minimalDir := filepath.Join(t.TempDir(), "minimal")
	if _, err := Generate(Request{Config: testConfig("minimal-api"), Destination: minimalDir, FrameworkVersion: "dev"}); err != nil {
		t.Fatal(err)
	}
	minimalRouter, err := os.ReadFile(filepath.Join(minimalDir, "internal/infrastructure/http/router.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, unexpected := range []string{"func serveRedoc", "func writeJSON"} {
		if strings.Contains(string(minimalRouter), unexpected) {
			t.Fatalf("minimal router unexpectedly contains %q", unexpected)
		}
	}

	postgresDir := filepath.Join(t.TempDir(), "postgres")
	if _, err := Generate(Request{Config: testConfig("postgres-api"), Destination: postgresDir, FrameworkVersion: "dev"}); err != nil {
		t.Fatal(err)
	}
	postgresRouter, err := os.ReadFile(filepath.Join(postgresDir, "internal/infrastructure/http/router.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"func serveRedoc", "func writeJSON"} {
		if !strings.Contains(string(postgresRouter), expected) {
			t.Fatalf("postgres router missing %q", expected)
		}
	}
}

func TestGeneratedProtectedOpenAPIRoutesConfigureBearerAuthentication(t *testing.T) {
	t.Parallel()

	for _, presetName := range []string{"production-api", "event-driven-api"} {
		presetName := presetName
		t.Run(presetName, func(t *testing.T) {
			t.Parallel()
			destination := filepath.Join(t.TempDir(), presetName)
			if _, err := Generate(Request{Config: testConfig(presetName), Destination: destination, FrameworkVersion: "dev"}); err != nil {
				t.Fatal(err)
			}

			router, err := os.ReadFile(filepath.Join(destination, "internal/infrastructure/http/router.go"))
			if err != nil {
				t.Fatal(err)
			}
			routerText := string(router)
			for _, expected := range []string{
				"openapi3filter.Options{AuthenticationFunc: openAPIBearerAuthentication}",
				"func openAPIBearerAuthentication",
				`SecuritySchemeName != "bearerAuth"`,
				`Header.Get("Authorization")`,
			} {
				if !strings.Contains(routerText, expected) {
					t.Fatalf("generated %s router missing OpenAPI bearer authentication contract %q", presetName, expected)
				}
			}
		})
	}

	postgresDir := filepath.Join(t.TempDir(), "postgres-api")
	if _, err := Generate(Request{Config: testConfig("postgres-api"), Destination: postgresDir, FrameworkVersion: "dev"}); err != nil {
		t.Fatal(err)
	}
	postgresRouter, err := os.ReadFile(filepath.Join(postgresDir, "internal/infrastructure/http/router.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(postgresRouter), "openAPIBearerAuthentication") {
		t.Fatal("postgres-api must not generate bearer authentication when the preset has no security feature")
	}
}

func TestGeneratedProjectsAvoidKnownRealE2ELintRegressions(t *testing.T) {
	t.Parallel()

	for _, presetName := range []string{"minimal-api", "postgres-api", "production-api", "event-driven-api"} {
		presetName := presetName
		t.Run(presetName, func(t *testing.T) {
			t.Parallel()
			destination := filepath.Join(t.TempDir(), presetName)
			if _, err := Generate(Request{Config: testConfig(presetName), Destination: destination, FrameworkVersion: "dev"}); err != nil {
				t.Fatal(err)
			}

			router, err := os.ReadFile(filepath.Join(destination, "internal/infrastructure/http/router.go"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(router), "middleware.RealIP") {
				t.Fatal("generated router must not use deprecated/insecure middleware.RealIP")
			}

			bootstrap, err := os.ReadFile(filepath.Join(destination, "internal/bootstrap/app.go"))
			if err != nil {
				t.Fatal(err)
			}
			bootstrapText := string(bootstrap)
			if presetName != "minimal-api" {
				if strings.Contains(bootstrapText, "ReadinessCheck(func(context.Context) error { return nil })") {
					t.Fatal("database preset must derive readiness directly from postgres pool")
				}
				if strings.Contains(bootstrapText, "var register httpserver.RouteRegistrar") {
					t.Fatal("database preset must initialize route registrar at declaration")
				}
			}

			if presetName == "production-api" || presetName == "event-driven-api" {
				authService, err := os.ReadFile(filepath.Join(destination, "internal/application/auth_service.go"))
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(authService), "if err != nil {\n\t\treturn nil\n\t}") {
					t.Fatal("logout must not silently return nil from an explicit non-nil error branch")
				}
			}
		})
	}
}

func TestGeneratedEventWorkerUsesWorkerScopedConfiguration(t *testing.T) {
	t.Parallel()
	destination := filepath.Join(t.TempDir(), "event-driven-api")
	if _, err := Generate(Request{Config: testConfig("event-driven-api"), Destination: destination, FrameworkVersion: "dev"}); err != nil {
		t.Fatal(err)
	}

	configSource, err := os.ReadFile(filepath.Join(destination, "internal/config/config.go"))
	if err != nil {
		t.Fatal(err)
	}
	configText := string(configSource)
	for _, expected := range []string{"func LoadWorker()", "func (c Config) ValidateWorker() error"} {
		if !strings.Contains(configText, expected) {
			t.Fatalf("generated worker configuration missing %q", expected)
		}
	}

	workerBootstrap, err := os.ReadFile(filepath.Join(destination, "internal/bootstrap/worker.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(workerBootstrap), "config.LoadWorker()") {
		t.Fatal("event worker must load worker-scoped configuration")
	}
	if strings.Contains(string(workerBootstrap), "config.Load()") {
		t.Fatal("event worker must not use API-wide configuration validation")
	}

	composeSource, err := os.ReadFile(filepath.Join(destination, "compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	composeText := string(composeSource)
	workerIndex := strings.Index(composeText, "\n  worker:\n")
	if workerIndex < 0 {
		t.Fatal("compose worker service missing")
	}
	workerBlock := composeText[workerIndex:]
	if next := strings.Index(workerBlock[1:], "\n  prometheus:\n"); next >= 0 {
		workerBlock = workerBlock[:next+1]
	}
	for _, unexpected := range []string{"JWT_SECRET:", "REDIS_ADDRESS:"} {
		if strings.Contains(workerBlock, unexpected) {
			t.Fatalf("worker compose environment must not contain unused API/cache setting %q", unexpected)
		}
	}
}
