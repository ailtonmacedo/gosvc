package doctor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckWithReportsMissingRequiredTools(t *testing.T) {
	t.Parallel()
	report, err := CheckWith(t.TempDir(), func(command string) (string, error) {
		if command == "go" || command == "git" {
			return "/bin/" + command, nil
		}
		return "", errors.New("missing")
	}, func(context.Context, string, ...string) ([]byte, error) {
		return []byte("version 1\n"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.HasMissingRequired() {
		t.Fatal("expected missing required lint tools")
	}
}

func TestCheckWithFindsProjectLocalTools(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "project.yaml"), `schema_version: 1
project:
  name: test-service
  module: github.com/acme/test-service
  go_version: auto
  preset: postgres-api
architecture:
  type: clean
  layout: layered
api:
  enabled: true
  router: chi
  port: 8080
  read_timeout: 10s
  write_timeout: 15s
  idle_timeout: 1m
  shutdown_timeout: 10s
  max_body_size: 1048576
openapi:
  enabled: true
  source: api/openapi.yaml
  strict_server: true
  request_validation: true
  documentation: redoc
database:
  enabled: true
  engine: postgres
  driver: pgx
  pool: pgxpool
  migrations: golang-migrate
  code_generation: sqlc
deployment:
  docker: true
  compose: true
  runtime_image: distroless
  non_root: true
quality:
  coverage:
    minimum: 80
`)
	for _, tool := range []string{"sqlc", "oapi-codegen", "migrate", "golangci-lint", "govulncheck"} {
		writeFile(t, filepath.Join(root, "bin", tool), "tool")
	}
	report, err := CheckWith(root, func(command string) (string, error) {
		return "/usr/bin/" + command, nil
	}, func(context.Context, string, ...string) ([]byte, error) {
		return []byte("tool version\n"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.HasMissingRequired() {
		t.Fatalf("results=%+v", report.Results)
	}
	for _, result := range report.Results {
		if result.Status != StatusOK {
			t.Fatalf("result=%+v", result)
		}
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestCheckWithRejectsGoOlderThanProjectRequirement(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "project.yaml"), `schema_version: 1
project:
  name: test-service
  module: github.com/acme/test-service
  go_version: 1.25
  preset: minimal-api
architecture:
  type: clean
  layout: layered
api:
  enabled: true
  router: chi
  port: 8080
  read_timeout: 10s
  write_timeout: 15s
  idle_timeout: 1m
  shutdown_timeout: 10s
  max_body_size: 1048576
deployment:
  docker: true
  compose: false
  runtime_image: distroless
  non_root: true
quality:
  coverage:
    minimum: 80
`)
	report, err := CheckWith(root, func(command string) (string, error) {
		return "/usr/bin/" + command, nil
	}, func(_ context.Context, command string, _ ...string) ([]byte, error) {
		if filepath.Base(command) == "go" {
			return []byte("go version go1.23.2 linux/amd64\n"), nil
		}
		return []byte("tool version\n"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.HasMissingRequired() {
		t.Fatal("expected old Go version to fail the environment check")
	}
	if report.Results[0].Status != StatusError {
		t.Fatalf("go result=%+v", report.Results[0])
	}
}

func TestCheckWithEnforcesGo125SecurityFloor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "project.yaml"), `schema_version: 1
project:
  name: test-service
  module: github.com/acme/test-service
  go_version: 1.25.0
  preset: postgres-api
architecture:
  type: clean
  layout: layered
api:
  enabled: true
  router: chi
  port: 8080
  read_timeout: 10s
  write_timeout: 15s
  idle_timeout: 1m
  shutdown_timeout: 10s
  max_body_size: 1048576
deployment:
  docker: true
  compose: true
  runtime_image: distroless
  non_root: true
quality:
  coverage:
    minimum: 80
`)
	report, err := CheckWith(root, func(command string) (string, error) {
		return "/usr/bin/" + command, nil
	}, func(_ context.Context, command string, _ ...string) ([]byte, error) {
		if filepath.Base(command) == "go" {
			return []byte("go version go1.25.11 linux/amd64\n"), nil
		}
		return []byte("tool version\n"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Results[0].Status != StatusError {
		t.Fatalf("expected Go 1.25.11 to fail patched toolchain floor, result=%+v", report.Results[0])
	}
}

func TestVersionAtLeast(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		output, minimum string
		want            bool
	}{
		{"go version go1.25.0 linux/amd64", "1.25", true},
		{"go version go1.25.1 linux/amd64", "1.25", true},
		{"go version go1.24.9 linux/amd64", "1.25", false},
		{"unexpected", "1.25", false},
	} {
		if got := versionAtLeast("go", test.output, test.minimum); got != test.want {
			t.Fatalf("versionAtLeast(%q, %q)=%v want %v", test.output, test.minimum, got, test.want)
		}
	}
}

func TestCheckWithPresetBeforeGenerationUsesPresetRequirements(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	report, err := CheckWithPreset(root, "postgres-api", func(command string) (string, error) {
		return "/usr/bin/" + command, nil
	}, func(_ context.Context, command string, _ ...string) ([]byte, error) {
		if filepath.Base(command) == "go" {
			return []byte("go version go1.25.12 linux/amd64\n"), nil
		}
		return []byte("tool version\n"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Preset != "postgres-api" {
		t.Fatalf("preset=%q", report.Preset)
	}
	for _, want := range []string{"Docker Compose", "sqlc", "oapi-codegen", "golang-migrate"} {
		found := false
		for _, result := range report.Results {
			if result.Tool.Name == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing prerequisite %s in report", want)
		}
	}
}

func TestCheckWithPresetBeforeGenerationDoesNotRequireBootstrapManagedTools(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	report, err := CheckWithPreset(root, "postgres-api", func(command string) (string, error) {
		switch command {
		case "go", "git", "docker":
			return "/usr/bin/" + command, nil
		default:
			return "", errors.New("missing")
		}
	}, func(_ context.Context, command string, _ ...string) ([]byte, error) {
		if filepath.Base(command) == "go" {
			return []byte("go version go1.25.12 linux/amd64\n"), nil
		}
		return []byte("tool version\n"), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.HasMissingRequired() {
		t.Fatalf("bootstrap-managed tools should be optional before generation: %+v", report.Results)
	}
}
