package upgradenotes

import (
	"strings"
	"testing"
)

func TestMarkdownIncludesRegisteredRange(t *testing.T) {
	t.Parallel()
	content, err := Markdown("1.0.0", "1.1.0")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Manifest schema v3", "Automatic upgrade backups", "Atomic rollback"} {
		if !strings.Contains(content, expected) {
			t.Fatalf("content=%q missing %q", content, expected)
		}
	}
}

func TestBetweenRejectsReverseRange(t *testing.T) {
	t.Parallel()
	if _, err := Between("1.1.0", "1.0.0"); err == nil {
		t.Fatal("expected reverse range error")
	}
}
