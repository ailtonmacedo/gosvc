package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncodeIsDeterministicAndUsesCurrentSchema(t *testing.T) {
	t.Parallel()
	value := Manifest{
		FrameworkVersion: "1.0.0",
		Project:          Project{Name: "orders", Module: "github.com/acme/orders", ConfigSchemaVersion: 1},
		Preset:           "minimal-api",
		PresetVersion:    "1.1.0",
		Features:         []string{"z", "a"},
		Plugins:          []PluginReference{{Name: "z", Version: "1.0.0"}, {Name: "a", Version: "1.0.0"}},
		Files:            []File{{Path: "z", Checksum: "2"}, {Path: "a", Checksum: "1"}},
	}
	data, err := Encode(value)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, `"schema_version": 4`) {
		t.Fatalf("encoded manifest = %s", text)
	}
	if strings.Index(text, `"a"`) > strings.Index(text, `"z"`) {
		t.Fatalf("manifest entries are not sorted: %s", text)
	}
}

func TestLoadDocumentMigratesSchemaOneInMemory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, filepath.FromSlash(Path))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := `{
  "framework_version": "0.7.0",
  "schema_version": 1,
  "preset": "minimal-api",
  "features": ["base"],
  "files": [{"path":"go.mod","ownership":"user","checksum":"sha256:test"}]
}`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	document, err := LoadDocument(dir)
	if err != nil {
		t.Fatal(err)
	}
	if document.SourceSchemaVersion != 1 {
		t.Fatalf("source schema = %d", document.SourceSchemaVersion)
	}
	if document.Manifest.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("migrated schema = %d", document.Manifest.SchemaVersion)
	}
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "requires upgrade") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestDecodeDocumentRejectsUnknownSchema(t *testing.T) {
	t.Parallel()
	_, err := DecodeDocument([]byte(`{"schema_version":99}`))
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("DecodeDocument() error = %v", err)
	}
}

func TestChecksumIsStable(t *testing.T) {
	t.Parallel()
	if Checksum([]byte("value")) != Checksum([]byte("value")) {
		t.Fatal("checksum is not stable")
	}
}

func TestLoadDocumentMigratesSchemaTwoInMemory(t *testing.T) {
	t.Parallel()
	legacy := `{
  "framework_version": "1.0.0",
  "schema_version": 2,
  "project": {"name":"orders","module":"github.com/acme/orders","config_schema_version":1},
  "preset": "minimal-api",
  "features": ["base"],
  "files": [{"path":"go.mod","ownership":"user","checksum":"sha256:test"}]
}`
	document, err := DecodeDocument([]byte(legacy))
	if err != nil {
		t.Fatal(err)
	}
	if document.SourceSchemaVersion != 2 || document.Manifest.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("document=%+v", document)
	}
}
