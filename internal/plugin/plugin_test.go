package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverIncludesBuiltInsAndExternalManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, ".gosvc", "plugins", "audit")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{
  "schema_version": 1,
  "name": "audit",
  "version": "1.0.0",
  "description": "Adds audit artifacts",
  "minimum_gosvc_version": "1.0.0",
  "capabilities": ["artifacts", "validation"],
  "entrypoint": "./bin/audit"
}`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	plugins, err := Discover(dir, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range plugins {
		if item.Name == "audit" && !item.BuiltIn {
			found = true
		}
	}
	if !found {
		t.Fatalf("external plugin not found: %+v", plugins)
	}
}

func TestLoadMetadataRejectsIncompatibleVersion(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "plugin.json")
	content := `{"schema_version":1,"name":"audit","version":"1.0.0","description":"Audit","minimum_gosvc_version":"2.0.0","capabilities":["artifacts"]}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadMetadata(path, "1.0.0")
	if err == nil || !strings.Contains(err.Error(), "older than required") {
		t.Fatalf("LoadMetadata() error = %v", err)
	}
}

func TestMetadataRejectsUnknownCapability(t *testing.T) {
	t.Parallel()
	metadata := Metadata{SchemaVersion: 1, Name: "audit", Version: "1.0.0", Description: "Audit", MinimumGosvcVersion: "1.0.0", Capabilities: []Capability{"runtime"}}
	if err := metadata.Validate("1.0.0"); err == nil {
		t.Fatal("expected capability validation error")
	}
}
