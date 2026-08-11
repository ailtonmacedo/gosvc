package modulepath

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	legacyModule          = "github.com/" + "example/gosvc"
	legacyRepositoryUpper = "OWNER/" + "gosvc"
	legacyRepositoryLower = "owner/" + "gosvc"
)

func TestPrepareAndApply(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":           "module " + legacyModule + "\n\ngo 1.23\n",
		"cmd/main.go":      "package main\nimport _ \"" + legacyModule + "/internal/cli\"\n",
		"README.md":        "Repository " + legacyRepositoryUpper + " and " + legacyRepositoryLower + "\n",
		"dist/ignored.txt": legacyModule + "\n",
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
	plan, err := Prepare(root, "acme/gosvc")
	if err != nil {
		t.Fatal(err)
	}
	if plan.NewModule != "github.com/acme/gosvc" || len(plan.Changes) != 3 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if err := Apply(plan); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "cmd/main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "github.com/acme/gosvc/internal/cli") {
		t.Fatalf("module was not replaced: %s", content)
	}
	ignored, _ := os.ReadFile(filepath.Join(root, "dist/ignored.txt"))
	if !strings.Contains(string(ignored), legacyModule) {
		t.Fatalf("dist should be ignored: %s", ignored)
	}
}

func TestPrepareRejectsInvalidRepository(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module "+legacyModule+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(root, "invalid"); err == nil {
		t.Fatal("expected invalid repository to fail")
	}
}
