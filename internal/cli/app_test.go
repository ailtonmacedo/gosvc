package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ailtonmacedo/gosvc/internal/clierror"
	"github.com/ailtonmacedo/gosvc/internal/plugin"
)

func TestRunVersion(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)

	code := app.Run([]string{"version"})
	if code != 0 {
		t.Fatalf("Run() code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "gosvc version") {
		t.Fatalf("stdout = %q, want version information", stdout.String())
	}
	if !strings.Contains(stdout.String(), "executable:") {
		t.Fatalf("stdout = %q, want executable path", stdout.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)

	code := app.Run([]string{"unknown"})
	if code != int(clierror.CodeGeneral) {
		t.Fatalf("Run() code = %d, want %d", code, clierror.CodeGeneral)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr = %q, want unknown command message", stderr.String())
	}
}

func TestRunValidateConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "project.yaml")
	content := []byte(`schema_version: 1
project:
  name: order-service
  module: github.com/acme/order-service
  preset: minimal-api
api:
  enabled: true
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	code := app.Run([]string{"validate-config", "--print", path})
	if code != 0 {
		t.Fatalf("Run() code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Configuration is valid") {
		t.Fatalf("stdout = %q, want validation success", stdout.String())
	}
	if !strings.Contains(stdout.String(), "port=8080") {
		t.Fatalf("stdout = %q, want applied default port", stdout.String())
	}
}

func TestRunValidateConfigRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "project.yaml")
	content := []byte(`schema_version: 1
project:
  name: order-service
  module: github.com/acme/order-service
  preset: minimal-api
unknown: true
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	code := app.Run([]string{"validate-config", path})
	if code != int(clierror.CodeInvalidConfig) {
		t.Fatalf("Run() code = %d, want %d", code, clierror.CodeInvalidConfig)
	}
	if !strings.Contains(stderr.String(), "field unknown not found") {
		t.Fatalf("stderr = %q, want unknown field error", stderr.String())
	}
}

func TestRunNewDryRunAndCreate(t *testing.T) {
	t.Parallel()

	destination := filepath.Join(t.TempDir(), "order-service")
	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)

	code := app.Run([]string{
		"new", "order-service",
		"--module", "github.com/acme/order-service",
		"--output", destination,
		"--dry-run",
	})
	if code != 0 {
		t.Fatalf("dry-run code = %d, stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote destination: %v", err)
	}
	if !strings.Contains(stdout.String(), "CREATE") {
		t.Fatalf("dry-run stdout = %q, want CREATE plan", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = app.Run([]string{
		"new", "order-service",
		"--module", "github.com/acme/order-service",
		"--output", destination,
	})
	if code != 0 {
		t.Fatalf("create code = %d, stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(destination, "go.mod")); err != nil {
		t.Fatalf("generated go.mod missing: %v", err)
	}
	if !strings.Contains(stdout.String(), "Next: cd "+destination) {
		t.Fatalf("stdout = %q, want exact next-directory instruction", stdout.String())
	}
}

func TestRunAddResource(t *testing.T) {
	t.Parallel()
	destination := filepath.Join(t.TempDir(), "catalog-service")
	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	code := app.Run([]string{"new", "catalog-service", "--module", "github.com/acme/catalog-service", "--preset", "postgres-api", "--output", destination})
	if code != 0 {
		t.Fatalf("new code = %d, stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = app.Run([]string{"add", "resource", "product", "--fields", "id:uuid,name:string,price:decimal", "--crud", "--project", destination})
	if code != 0 {
		t.Fatalf("add code = %d, stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(destination, "internal/domain/product.go")); err != nil {
		t.Fatalf("product domain missing: %v", err)
	}
	if !strings.Contains(stdout.String(), "Resource product added") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunValidateAndVerifyStatic(t *testing.T) {
	t.Parallel()
	destination := filepath.Join(t.TempDir(), "validated-service")
	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	code := app.Run([]string{"new", "validated-service", "--module", "github.com/acme/validated-service", "--output", destination})
	if code != 0 {
		t.Fatalf("new code=%d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = app.Run([]string{"validate", "--project", destination})
	if code != 0 {
		t.Fatalf("validate code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Project is valid") {
		t.Fatalf("stdout=%q", stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = app.Run([]string{"verify", "--project", destination, "--static"})
	if code != 0 {
		t.Fatalf("verify code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Verification completed successfully") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestRunCheckArchitectureDetectsViolation(t *testing.T) {
	t.Parallel()
	destination := filepath.Join(t.TempDir(), "invalid-service")
	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	code := app.Run([]string{"new", "invalid-service", "--module", "github.com/acme/invalid-service", "--output", destination})
	if code != 0 {
		t.Fatalf("new code=%d stderr=%s", code, stderr.String())
	}
	path := filepath.Join(destination, "internal", "domain", "forbidden.go")
	if err := os.WriteFile(path, []byte("package domain\nimport _ \"github.com/jackc/pgx/v5\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	code = app.Run([]string{"check", "architecture", "--project", destination})
	if code != int(clierror.CodeInvalidProject) {
		t.Fatalf("code=%d want=%d stderr=%s", code, clierror.CodeInvalidProject, stderr.String())
	}
	if !strings.Contains(stdout.String(), "domain independence") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestRunUpgradeDryRun(t *testing.T) {
	t.Parallel()
	destination := filepath.Join(t.TempDir(), "upgrade-service")
	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	code := app.Run([]string{"new", "upgrade-service", "--module", "github.com/acme/upgrade-service", "--output", destination})
	if code != 0 {
		t.Fatalf("new code=%d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = app.Run([]string{"upgrade", "--project", destination, "--dry-run"})
	if code != 0 {
		t.Fatalf("upgrade code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Manifest schema") || !strings.Contains(stdout.String(), "dry run") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestRunPluginsList(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	code := app.Run([]string{"plugins", "list", "--project", t.TempDir()})
	if code != 0 {
		t.Fatalf("plugins code=%d stderr=%s", code, stderr.String())
	}
	for _, name := range []string{"core", "postgres", "distributed"} {
		if !strings.Contains(stdout.String(), name) {
			t.Fatalf("stdout=%q missing %s", stdout.String(), name)
		}
	}
}

func TestParsePluginRunOptionsAllowsFlagsInAnyOrder(t *testing.T) {
	t.Parallel()
	options, err := parsePluginRunOptions([]string{"--dry-run", "audit", "--project", "/tmp/project", "--timeout", "5s"})
	if err != nil {
		t.Fatal(err)
	}
	if options.name != "audit" || options.projectDir != "/tmp/project" || !options.dryRun || options.timeout.String() != "5s" {
		t.Fatalf("options = %+v", options)
	}
}

func TestRunExternalPlugin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	destination := filepath.Join(t.TempDir(), "plugin-service")
	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	if code := app.Run([]string{"new", "plugin-service", "--module", "github.com/acme/plugin-service", "--output", destination}); code != 0 {
		t.Fatalf("new code=%d stderr=%s", code, stderr.String())
	}
	root := filepath.Join(destination, ".gosvc", "plugins", "audit")
	entrypoint := filepath.Join(root, "bin", "plugin")
	if err := os.MkdirAll(filepath.Dir(entrypoint), 0o755); err != nil {
		t.Fatal(err)
	}
	script := []byte(`#!/bin/sh
read request
case "$GOSVC_PLUGIN_ACTION" in
 validate) printf '%s\n' '{"protocol_version":1}' ;;
 contribute) printf '%s\n' '{"protocol_version":1,"contribution":{"artifacts":[{"path":"internal/audit/audit.go","content":"package audit\n","mode":420,"ownership":"generated"}]}}' ;;
esac
`)
	if err := os.WriteFile(entrypoint, script, 0o755); err != nil {
		t.Fatal(err)
	}
	metadata := plugin.Metadata{
		SchemaVersion: 2, ProtocolVersion: 1, Name: "audit", Version: "1.0.0",
		Description: "Audit", MinimumGosvcVersion: "1.0.0",
		Capabilities: []plugin.Capability{plugin.CapabilityValidation, plugin.CapabilityArtifacts},
		Entrypoint:   "bin/plugin", Checksum: plugin.Checksum(script),
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plugin.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	code := app.Run([]string{"plugins", "run", "audit", "--project", destination, "--timeout", "2s"})
	if code != 0 {
		t.Fatalf("plugin run code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Plugin audit 1.0.0 applied") {
		t.Fatalf("stdout=%q", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(destination, "internal", "audit", "audit.go")); err != nil {
		t.Fatalf("plugin artifact missing: %v", err)
	}
}

func TestRunCompletion(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	code := app.Run([]string{"completion", "bash"})
	if code != 0 {
		t.Fatalf("completion code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "complete -F") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestRunReleaseCheckAllowsLocalPlaceholder(t *testing.T) {
	t.Parallel()
	root := filepath.Clean(filepath.Join("..", ".."))
	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	code := app.Run([]string{"release", "check", "--project", root, "--version", "1.0.0", "--allow-placeholder"})
	if code != 0 {
		t.Fatalf("release check code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Release preflight passed") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestRunAcceptanceJSON(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	code := app.Run([]string{"acceptance", "--json"})
	if code != 0 {
		t.Fatalf("acceptance code=%d stderr=%s", code, stderr.String())
	}
	var report struct {
		SchemaVersion int `json:"schema_version"`
		Passed        int `json:"passed"`
		Failed        int `json:"failed"`
		Presets       []struct {
			Status string `json:"status"`
		} `json:"presets"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode acceptance output: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != 1 || report.Passed != 6 || report.Failed != 0 || len(report.Presets) != 6 {
		t.Fatalf("report=%+v", report)
	}
}

func TestRunReleaseSnapshotRejectsInvalidParallelism(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	code := app.Run([]string{"release", "snapshot", "--version", "1.0.0", "--parallel", "-1", "--allow-placeholder"})
	if code == 0 {
		t.Fatal("expected invalid parallelism to fail")
	}
	if !strings.Contains(stderr.String(), "--parallel must be between 0 and 32") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestRunUpgradeBackupCatalogAndRollback(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "rollback-service")
	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	if code := app.Run([]string{"new", "rollback-service", "--module", "github.com/acme/rollback-service", "--output", destination}); code != 0 {
		t.Fatalf("new code=%d stderr=%s", code, stderr.String())
	}
	original, err := os.ReadFile(filepath.Join(destination, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := app.Run([]string{"upgrade", "--project", destination, "--to", "1.1.0"}); code != 0 {
		t.Fatalf("upgrade code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Backup: .gosvc/backups/") {
		t.Fatalf("stdout=%q", stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := app.Run([]string{"upgrade", "backups", "--project", destination}); code != 0 {
		t.Fatalf("backups code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), ".gosvc/backups/") {
		t.Fatalf("stdout=%q", stdout.String())
	}
	if err := os.WriteFile(filepath.Join(destination, "README.md"), []byte("changed after upgrade\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := app.Run([]string{"upgrade", "rollback", "--project", destination, "--backup", "latest"}); code != 0 {
		t.Fatalf("rollback code=%d stderr=%s", code, stderr.String())
	}
	restored, err := os.ReadFile(filepath.Join(destination, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != string(original) {
		t.Fatalf("README was not restored: %q", restored)
	}
}

func TestRunUpgradeNotes(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	if code := app.Run([]string{"upgrade", "notes", "--from", "1.0.0", "--to", "1.1.0"}); code != 0 {
		t.Fatalf("notes code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Manifest schema v3") || !strings.Contains(stdout.String(), "Atomic rollback") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestRunCertifyStatic(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	code := app.Run([]string{"certify", "--mode", "static"})
	if code != 0 {
		t.Fatalf("certify code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Certification: passed=6 failed=0 blocked=0") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestRunCertifyRejectsMode(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	code := app.Run([]string{"certify", "--mode", "unknown"})
	if code != int(clierror.CodeInvalidProject) {
		t.Fatalf("certify code=%d want=%d stderr=%s", code, clierror.CodeInvalidProject, stderr.String())
	}
	if !strings.Contains(stderr.String(), "unsupported certification mode") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestRunReleaseGitHubPlan(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                              "module github.com/acme/gosvc\n\ngo 1.23\n",
		"CHANGELOG.md":                        "# Changelog\n\n## [1.1.0]\n",
		"SECURITY.md":                         "Security\n",
		"CODE_OF_CONDUCT.md":                  "Code\n",
		"CONTRIBUTING.md":                     "Contributing\n",
		"LICENSE":                             "MIT\n",
		".github/workflows/ci.yml":            "on: pull_request\n# go test\n",
		".github/workflows/acceptance.yml":    "# acceptance\n",
		".github/workflows/certification.yml": "on: workflow_dispatch\n# certify --require-real\n",
		".github/workflows/release.yml":       "tags:\n  - 'v*.*.*'\n# release check\n# certify --mode real\n# gh release create\n",
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	code := app.Run([]string{"release", "github-plan", "--project", root, "--repository", "acme/gosvc", "--version", "1.1.0"})
	if code != 0 {
		t.Fatalf("github-plan code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Readiness:") || !strings.Contains(stdout.String(), "git push origin v1.1.0") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestRunReleaseGitHubPlanJSON(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                              "module github.com/acme/gosvc\n\ngo 1.23\n",
		"CHANGELOG.md":                        "## [1.1.0]\n",
		"SECURITY.md":                         "Security\n",
		"CODE_OF_CONDUCT.md":                  "Code\n",
		"CONTRIBUTING.md":                     "Contributing\n",
		"LICENSE":                             "MIT\n",
		".github/workflows/ci.yml":            "pull_request\ngo test\n",
		".github/workflows/acceptance.yml":    "acceptance\n",
		".github/workflows/certification.yml": "workflow_dispatch\ncertify --require-real\n",
		".github/workflows/release.yml":       "tags:\nv*.*.*\nrelease check\ncertify --mode real\ngh release create\n",
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	code := app.Run([]string{"release", "github-plan", "--project", root, "--repository", "acme/gosvc", "--version", "1.1.0", "--json"})
	if code != 0 {
		t.Fatalf("github-plan code=%d stderr=%s", code, stderr.String())
	}
	var report struct {
		SchemaVersion int    `json:"schema_version"`
		Repository    string `json:"repository"`
		Failed        int    `json:"failed"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode publication plan: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != 1 || report.Repository != "acme/gosvc" || report.Failed != 0 {
		t.Fatalf("report=%+v", report)
	}
}
