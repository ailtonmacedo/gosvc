package releaseverify

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestVerifyFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "asset")
	content := []byte("asset")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	if err := verifyFile(path, hex.EncodeToString(digest[:]), int64(len(content))); err != nil {
		t.Fatal(err)
	}
	if err := verifyFile(path, "bad", int64(len(content))); err == nil {
		t.Fatal("expected checksum mismatch")
	}
}

func TestVerifyArchiveFindsBinary(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "gosvc_1.0.0_linux_amd64.tar.gz")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	content := []byte("binary")
	header := &tar.Header{Name: "gosvc_1.0.0_linux_amd64/gosvc", Mode: 0o755, Size: int64(len(content)), ModTime: time.Unix(0, 0)}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := verifyArchive(path, "1.0.0"); err != nil {
		t.Fatal(err)
	}
}

func TestLoadManifestRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "release-manifest.json")
	content, _ := json.Marshal(map[string]any{"schema_version": 2, "unknown": true})
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadManifest(path); err == nil {
		t.Fatal("expected unknown field to fail")
	}
}

func TestVerifyEvidenceMatchesManifest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	manifest := Manifest{
		SchemaVersion: 2, Name: "gosvc", Version: "1.0.0", Module: "github.com/acme/gosvc",
		Repository: "acme/gosvc", Commit: "abc", BuiltAt: "2026-08-06T12:00:00Z", Builder: "go1.25.0",
	}
	evidence := Evidence{
		SchemaVersion: 1, Name: manifest.Name, Version: manifest.Version, Module: manifest.Module,
		Repository: manifest.Repository, Commit: manifest.Commit, BuiltAt: manifest.BuiltAt, Builder: manifest.Builder,
		Acceptance:   EvidenceAcceptance{Passed: 1, Presets: []EvidencePresetResult{{Preset: "minimal-api", Status: "pass", Checks: []string{"project-validation"}}}},
		QualityGates: []string{"go-test"}, Reproducible: true,
	}
	content, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "release-evidence.json"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyEvidence(root, manifest); err != nil {
		t.Fatal(err)
	}
	evidence.Acceptance.Failed = 1
	content, _ = json.Marshal(evidence)
	if err := os.WriteFile(filepath.Join(root, "release-evidence.json"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyEvidence(root, manifest); err == nil {
		t.Fatal("expected failed acceptance evidence to be rejected")
	}
}

func TestVerifyReleaseNotesRequiresMatchingVersion(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "RELEASE_NOTES.md"), []byte("# gosvc 1.0.0\n\nNotes.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyReleaseNotes(root, "1.0.0"); err != nil {
		t.Fatal(err)
	}
	if err := verifyReleaseNotes(root, "2.0.0"); err == nil {
		t.Fatal("expected version mismatch")
	}
}
