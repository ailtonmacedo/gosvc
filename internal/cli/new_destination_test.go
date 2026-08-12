package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultNewDestinationOutsideGosvcSourceTree(t *testing.T) {
	root := t.TempDir()
	gosvc := filepath.Join(root, "gosvc")
	if err := os.MkdirAll(filepath.Join(gosvc, "internal", "cli"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gosvc, "go.mod"), []byte("module "+gosvcSourceModule+"\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := defaultNewDestination(gosvc, "catalog-service")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join("..", "catalog-service") {
		t.Fatalf("defaultNewDestination() = %q, want %q", got, filepath.Join("..", "catalog-service"))
	}

	got, err = defaultNewDestination(filepath.Join(gosvc, "internal", "cli"), "catalog-service")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("..", "..", "..", "catalog-service")
	if got != want {
		t.Fatalf("nested defaultNewDestination() = %q, want %q", got, want)
	}
}

func TestDefaultNewDestinationKeepsNormalCurrentDirectoryBehavior(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/acme/workspace\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := defaultNewDestination(root, "catalog-service")
	if err != nil {
		t.Fatal(err)
	}
	if got != "catalog-service" {
		t.Fatalf("defaultNewDestination() = %q, want catalog-service", got)
	}
}

func TestDefaultNewDestinationProtectsGosvcSourceRepository(t *testing.T) {
	root := t.TempDir()
	gosvc := filepath.Join(root, "gosvc")
	if err := os.MkdirAll(gosvc, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gosvc, "go.mod"), []byte("module "+gosvcSourceModule+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := defaultNewDestination(gosvc, "gosvc"); err == nil {
		t.Fatal("expected protection error when default target equals gosvc source root")
	}
}

func TestLegacyNestedDestinationWarnsWhenOldNestedProjectExists(t *testing.T) {
	root := t.TempDir()
	gosvc := filepath.Join(root, "gosvc")
	legacy := filepath.Join(gosvc, "catalog-service")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gosvc, "go.mod"), []byte("module "+gosvcSourceModule+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(gosvc); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	got, ok := legacyNestedDestination("catalog-service", filepath.Join("..", "catalog-service"))
	if !ok {
		t.Fatal("expected legacy nested destination to be detected")
	}
	if got != "catalog-service" {
		t.Fatalf("legacyNestedDestination() = %q, want catalog-service", got)
	}
}
