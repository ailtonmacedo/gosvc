package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPublishedJSONSchemasAreValid(t *testing.T) {
	t.Parallel()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "..", ".."))
	for _, name := range []string{
		"manifest.schema.json",
		"plugin.schema.json",
		"plugin-request.schema.json",
		"plugin-response.schema.json",
	} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			content, err := os.ReadFile(filepath.Join(root, "schema", name))
			if err != nil {
				t.Fatal(err)
			}
			if !json.Valid(content) {
				t.Fatalf("schema %s is not valid JSON", name)
			}
		})
	}
}
