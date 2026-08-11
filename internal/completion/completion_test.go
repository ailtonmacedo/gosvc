package completion

import (
	"strings"
	"testing"
)

func TestGenerateSupportedShells(t *testing.T) {
	t.Parallel()
	for _, shell := range []string{"bash", "zsh", "fish", "powershell", "pwsh"} {
		shell := shell
		t.Run(shell, func(t *testing.T) {
			t.Parallel()
			content, err := Generate(shell)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(content, "gosvc") {
				t.Fatalf("completion for %s does not reference gosvc", shell)
			}
			if !strings.Contains(content, "acceptance") {
				t.Fatalf("completion for %s does not include acceptance command", shell)
			}
			if !strings.Contains(content, "github-plan") {
				t.Fatalf("completion for %s does not include release github-plan", shell)
			}
		})
	}
}

func TestGenerateRejectsUnknownShell(t *testing.T) {
	t.Parallel()
	if _, err := Generate("cmd"); err == nil {
		t.Fatal("expected unsupported shell error")
	}
}
