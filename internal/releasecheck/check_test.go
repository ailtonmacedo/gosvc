package releasecheck

import (
	"os"
	"path/filepath"
	"testing"
)

const placeholderModule = "github.com/" + "example/gosvc"

func TestCheckReportsPlaceholderAndMissingFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+placeholderModule+"\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := Check(Options{Root: root, Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Err() == nil {
		t.Fatal("expected release issues")
	}
}

func TestCheckAcceptsCompleteFixtureWithPlaceholderOverride(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                                     "module " + placeholderModule + "\n\ngo 1.23\n",
		"README.md":                                  "release-evidence-ready\n",
		"LICENSE":                                    "MIT\n",
		"CHANGELOG.md":                               "## [1.0.0]\n",
		"CONTRIBUTING.md":                            "Contributing\n",
		"SECURITY.md":                                "Security\n",
		"CODE_OF_CONDUCT.md":                         "Code\n",
		"docs/INSTALLATION.md":                       "Install\n",
		"docs/RELEASES.md":                           "Release\n",
		"docs/ACCEPTANCE.md":                         "Acceptance\n",
		"docs/CERTIFICATION.md":                      "Certification\n",
		"docs/COMPATIBILITY.md":                      "Compatibility\n",
		"docs/RELEASE_EVIDENCE.md":                   "Evidence\n",
		"docs/GITHUB_PUBLISHING.md":                  "GitHub publication\n",
		"schema/manifest.schema.json":                "{}\n",
		"schema/plugin.schema.json":                  "{}\n",
		"schema/compatibility-matrix.json":           "{}\n",
		"schema/acceptance-report.schema.json":       "{}\n",
		"schema/certification-report.schema.json":    "{}\n",
		"schema/release-evidence.schema.json":        "{}\n",
		"schema/github-publication-plan.schema.json": "{}\n",
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
	report, err := Check(Options{Root: root, Version: "1.0.0", Repository: "acme/gosvc", AllowPlaceholder: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := report.Err(); err != nil {
		t.Fatal(err)
	}
}
