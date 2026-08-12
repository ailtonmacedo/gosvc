package backup

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ailtonmacedo/gosvc/internal/generator"
	"github.com/ailtonmacedo/gosvc/internal/manifest"
	"github.com/ailtonmacedo/gosvc/internal/project"
)

func TestCreateListAndRestore(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "service")
	config := project.DefaultConfigForPreset("minimal-api")
	config.Project.Name = "service"
	config.Project.Module = "github.com/acme/service"
	if _, err := generator.Generate(generator.Request{Config: config, Destination: dir, FrameworkVersion: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	custom := []byte("# customized before upgrade\n")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), custom, 0o644); err != nil {
		t.Fatal(err)
	}
	value, err := manifest.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	entry, err := Create(dir, value, now)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Path == "" || entry.Metadata.ProjectName != "service" {
		t.Fatalf("entry = %+v", entry)
	}
	entries, err := List(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries=%+v err=%v", entries, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# changed after backup\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "temporary.txt"), []byte("remove me"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Restore(dir, entry, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != string(custom) {
		t.Fatalf("README=%q", content)
	}
	if _, err := os.Stat(filepath.Join(dir, "temporary.txt")); !os.IsNotExist(err) {
		t.Fatalf("temporary file should be removed, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(entry.Path))); err != nil {
		t.Fatalf("backup was not preserved: %v", err)
	}
	restored, err := manifest.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.RollbackHistory) != 1 || restored.RollbackHistory[0].Backup != entry.Path {
		t.Fatalf("rollback history=%+v", restored.RollbackHistory)
	}
}

func TestRestoreRejectsUnsafeEntry(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "service")
	if err := os.MkdirAll(filepath.Join(dir, filepath.FromSlash(Directory)), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, filepath.FromSlash(Directory), "unsafe.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	metadata := `{"schema_version":1,"project_name":"service","project_module":"github.com/acme/service","framework_version":"1.0.0","manifest_schema":3,"created_at":"2026-08-06T12:00:00Z"}`
	entry, _ := writer.Create(metadataName)
	_, _ = entry.Write([]byte(metadata))
	entry, _ = writer.Create("../escape.txt")
	_, _ = entry.Write([]byte("escape"))
	_ = writer.Close()
	_ = file.Close()

	resolved, err := Resolve(dir, "unsafe.zip")
	if err != nil {
		t.Fatal(err)
	}
	if err := Restore(dir, resolved, time.Now()); err == nil {
		t.Fatal("expected unsafe entry error")
	}
}
