package releasepack

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ailtonmacedo/gosvc/internal/acceptance"
)

func TestDeterministicArchivesContainPrefixedFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source := filepath.Join(root, "gosvc")
	if err := os.WriteFile(source, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	fixed := time.Unix(1_700_000_000, 0).UTC()
	files := []archiveFile{{Source: source, Name: "gosvc", Mode: 0o755}}

	tarPath := filepath.Join(root, "release.tar.gz")
	if err := writeTarGz(tarPath, "gosvc_1.0.0_linux_amd64", files, fixed); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(tarPath)
	if err != nil {
		t.Fatal(err)
	}
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	tarReader := tar.NewReader(gzipReader)
	header, err := tarReader.Next()
	if err != nil {
		t.Fatal(err)
	}
	if header.Name != "gosvc_1.0.0_linux_amd64/gosvc" || header.Mode != 0o755 {
		t.Fatalf("unexpected tar header: %+v", header)
	}
	content, err := io.ReadAll(tarReader)
	if err != nil || string(content) != "binary" {
		t.Fatalf("tar content=%q err=%v", content, err)
	}
	gzipReader.Close()
	file.Close()

	zipPath := filepath.Join(root, "release.zip")
	if err := writeZip(zipPath, "gosvc_1.0.0_windows_amd64", files, fixed); err != nil {
		t.Fatal(err)
	}
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zipReader.Close()
	if len(zipReader.File) != 1 || !strings.HasSuffix(zipReader.File[0].Name, "/gosvc") {
		t.Fatalf("unexpected zip entries: %+v", zipReader.File)
	}
}

func TestWriteReleaseNotesExtractsVersionSection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	changelog := filepath.Join(root, "CHANGELOG.md")
	content := `# Changelog

## [1.1.0] - 2026-08-06

### Added

- New release evidence.

## [1.0.0] - 2026-08-05

- Previous release.
`
	if err := os.WriteFile(changelog, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, "RELEASE_NOTES.md")
	fixed := time.Unix(1_700_000_000, 0).UTC()
	if err := writeReleaseNotes(changelog, destination, "1.1.0", fixed); err != nil {
		t.Fatal(err)
	}
	notes, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	text := string(notes)
	if !strings.Contains(text, "# gosvc 1.1.0") || !strings.Contains(text, "New release evidence") {
		t.Fatalf("unexpected release notes:\n%s", text)
	}
	if strings.Contains(text, "Previous release") {
		t.Fatalf("release notes included the next changelog section:\n%s", text)
	}
}

func TestStableAcceptanceRemovesVolatileFields(t *testing.T) {
	t.Parallel()
	report := acceptance.Report{
		Passed: 1,
		Presets: []acceptance.PresetResult{{
			Preset: "minimal-api", Status: "pass", Files: 20, ArchitectureFiles: 2,
			Checks: []string{"project-idempotency", "initial-generation"}, DurationMS: 999,
		}},
	}
	evidence := stableAcceptance(report)
	if evidence.Passed != 1 || evidence.Failed != 0 || len(evidence.Presets) != 1 {
		t.Fatalf("unexpected evidence: %+v", evidence)
	}
	checks := evidence.Presets[0].Checks
	if len(checks) != 2 || checks[0] != "initial-generation" || checks[1] != "project-idempotency" {
		t.Fatalf("checks not stable: %v", checks)
	}
}
