package githubpublish

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildPassesPreparedFixtureAndWarnsWithoutGit(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "github.com/acme/gosvc", "1.1.0")
	plan, err := Build(Options{Root: root, Repository: "acme/gosvc", Version: "1.1.0"})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Ready() {
		t.Fatalf("plan should be ready: %+v", SortedFailures(plan))
	}
	if plan.Warnings == 0 {
		t.Fatal("expected a warning for missing .git repository")
	}
	if len(plan.Steps) == 0 || !strings.Contains(plan.Steps[len(plan.Steps)-1].Command, "git push origin v1.1.0") {
		t.Fatalf("unexpected steps: %+v", plan.Steps)
	}
}

func TestBuildRejectsModuleMismatch(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "github.com/ailtonmacedo/gosvc", "1.1.0")
	plan, err := Build(Options{Root: root, Repository: "acme/gosvc", Version: "1.1.0"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Ready() {
		t.Fatal("expected module mismatch to fail readiness")
	}
	failures := strings.Join(SortedFailures(plan), "\n")
	if !strings.Contains(failures, "module-path") {
		t.Fatalf("failures=%q", failures)
	}
}

func TestBuildRejectsIncompleteReleaseWorkflow(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "github.com/acme/gosvc", "1.1.0")
	if err := os.WriteFile(filepath.Join(root, ".github/workflows/release.yml"), []byte("name: Release\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := Build(Options{Root: root, Repository: "acme/gosvc", Version: "1.1.0"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Ready() {
		t.Fatal("expected incomplete release workflow to fail")
	}
}

func writeFixture(t *testing.T, root, module, version string) {
	t.Helper()
	files := map[string]string{
		"go.mod":                              moduleLine(module),
		"CHANGELOG.md":                        "# Changelog\n\n## [" + version + "]\n",
		"SECURITY.md":                         "security\n",
		"CODE_OF_CONDUCT.md":                  "code\n",
		"CONTRIBUTING.md":                     "contrib\n",
		"LICENSE":                             "MIT\n",
		".github/workflows/ci.yml":            "name: CI\non: pull_request\njobs:\n  test:\n    steps:\n      - run: go test ./...\n",
		".github/workflows/acceptance.yml":    "name: Acceptance\n# acceptance\n",
		".github/workflows/certification.yml": "name: Certification\non: workflow_dispatch\n# certify --require-real\n",
		".github/workflows/release.yml":       "name: Release\non:\n  push:\n    tags:\n      - 'v*.*.*'\n# release check\n# certify --mode real\n# gh release create\n",
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
}

func moduleLine(module string) string { return "module " + module + "\n\ngo 1.23\n" }
