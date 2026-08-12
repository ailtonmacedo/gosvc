package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckDetectsForbiddenImports(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, filepath.Join(root, "internal/domain/order.go"), `package domain
import "github.com/jackc/pgx/v5"
`)
	write(t, filepath.Join(root, "internal/application/service.go"), `package application
import "github.com/acme/service/internal/infrastructure/http"
`)
	report, err := Check(root, "github.com/acme/service")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Violations) != 2 {
		t.Fatalf("violations=%d want=2: %+v", len(report.Violations), report.Violations)
	}
	if report.Err() == nil {
		t.Fatal("Report.Err() = nil")
	}
}

func TestCheckAllowsDomainStandardLibrary(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, filepath.Join(root, "internal/domain/order.go"), `package domain
import "errors"
var Err = errors.New("x")
`)
	report, err := Check(root, "github.com/acme/service")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Violations) != 0 {
		t.Fatalf("violations=%+v", report.Violations)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
