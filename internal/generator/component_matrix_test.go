package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ailtonmacedo/gosvc/internal/project"
)

func TestComponentRegressionMatrix(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		preset      string
		router      string
		persistence string
		postgres    bool
	}{
		{name: "bare", preset: "bare"},
		{name: "worker", preset: "worker"},
		{name: "minimal-chi", preset: "minimal-api", router: "chi"},
		{name: "minimal-echo", preset: "minimal-api", router: "echo"},
		{name: "postgres-chi-sqlc", preset: "postgres-api", router: "chi", persistence: "sqlc", postgres: true},
		{name: "postgres-echo-sqlc", preset: "postgres-api", router: "echo", persistence: "sqlc", postgres: true},
		{name: "postgres-chi-gorm", preset: "postgres-api", router: "chi", persistence: "gorm", postgres: true},
		{name: "postgres-echo-gorm", preset: "postgres-api", router: "echo", persistence: "gorm", postgres: true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			config := project.DefaultConfigForPreset(tc.preset)
			config.Project.Name = tc.name
			config.Project.Module = "github.com/acme/" + tc.name
			if tc.router != "" {
				config.API.Router = tc.router
				if tc.router == "echo" && config.GoLanguageVersion() == "auto" {
					config.Project.GoVersion = "1.25.0"
					config.Runtime.Go.Language = "1.25.0"
					config.Runtime.Go.Toolchain = project.PreferredToolchain("1.25.0")
				}
			}
			if tc.persistence == "gorm" {
				config.Database.CodeGeneration = "gorm"
				config.Database.Driver = "gorm-postgres"
				config.Database.Pool = "database/sql"
			}
			if err := config.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			destination := filepath.Join(t.TempDir(), tc.name)
			result, err := Generate(Request{Config: config, Destination: destination, FrameworkVersion: "1.6.0"})
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}
			if !result.Applied {
				t.Fatal("Generate() did not apply")
			}
			assertComponentArtifacts(t, destination, tc.router, tc.persistence)
			compileGeneratedProject(t, destination, tc.postgres)
		})
	}
}

func assertComponentArtifacts(t *testing.T, destination, router, persistence string) {
	t.Helper()
	mod, err := os.ReadFile(filepath.Join(destination, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(mod)
	if router == "echo" {
		if !strings.Contains(text, "github.com/labstack/echo/v4") {
			t.Fatalf("Echo dependency missing:\n%s", text)
		}
		if strings.Contains(text, "github.com/go-chi/chi/v5") {
			t.Fatalf("Chi dependency leaked into Echo variant:\n%s", text)
		}
	}
	if persistence == "gorm" {
		if !strings.Contains(text, "gorm.io/gorm") || !strings.Contains(text, "gorm.io/driver/postgres") {
			t.Fatalf("GORM dependencies missing:\n%s", text)
		}
		if _, err := os.Stat(filepath.Join(destination, "sqlc.yaml")); !os.IsNotExist(err) {
			t.Fatalf("GORM variant should not generate sqlc.yaml: %v", err)
		}
	}
}
