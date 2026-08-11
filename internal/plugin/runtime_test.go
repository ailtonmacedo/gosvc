package plugin

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunnerExecutesVersionedProtocol(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	projectDir := t.TempDir()
	metadata := writeExecutablePlugin(t, projectDir, `#!/bin/sh
read request
case "$GOSVC_PLUGIN_ACTION" in
  validate)
    printf '%s\n' '{"protocol_version":1,"diagnostics":[{"severity":"info","message":"configuration accepted"}]}'
    ;;
  contribute)
    printf '%s\n' '{"protocol_version":1,"contribution":{"features":["audit"],"artifacts":[{"path":"internal/audit/audit.go","content":"package audit\n","mode":420,"ownership":"generated"}]}}'
    ;;
esac
`)
	snapshot, err := AbsoluteProjectSnapshot(projectDir, "service", "github.com/acme/service", "minimal-api", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	runner := Runner{Timeout: time.Second}
	validated, err := runner.Run(context.Background(), metadata, projectDir, snapshot, ActionValidate)
	if err != nil {
		t.Fatal(err)
	}
	if len(validated.Diagnostics) != 1 || validated.Diagnostics[0].Message != "configuration accepted" {
		t.Fatalf("diagnostics = %+v", validated.Diagnostics)
	}
	response, err := runner.Run(context.Background(), metadata, projectDir, snapshot, ActionContribute)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Contribution.Artifacts) != 1 || response.Contribution.Artifacts[0].Path != "internal/audit/audit.go" {
		t.Fatalf("contribution = %+v", response.Contribution)
	}
}

func TestEntrypointPathRejectsChecksumMismatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	projectDir := t.TempDir()
	metadata := writeExecutablePlugin(t, projectDir, "#!/bin/sh\necho ok\n")
	path := filepath.Join(projectDir, ".gosvc", "plugins", metadata.Name, "bin", "plugin")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho changed\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := metadata.EntrypointPath(projectDir)
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("EntrypointPath() error = %v", err)
	}
}

func TestRunnerTimesOut(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	projectDir := t.TempDir()
	metadata := writeExecutablePlugin(t, projectDir, "#!/bin/sh\nsleep 2\n")
	snapshot, err := AbsoluteProjectSnapshot(projectDir, "service", "github.com/acme/service", "minimal-api", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = (Runner{Timeout: 20 * time.Millisecond}).Run(context.Background(), metadata, projectDir, snapshot, ActionValidate)
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunnerRejectsErrorDiagnostic(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	projectDir := t.TempDir()
	metadata := writeExecutablePlugin(t, projectDir, `#!/bin/sh
read request
printf '%s\n' '{"protocol_version":1,"diagnostics":[{"severity":"error","path":"project.yaml","message":"missing audit owner"}]}'
`)
	snapshot, err := AbsoluteProjectSnapshot(projectDir, "service", "github.com/acme/service", "minimal-api", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = (Runner{Timeout: time.Second}).Run(context.Background(), metadata, projectDir, snapshot, ActionValidate)
	if err == nil || !strings.Contains(err.Error(), "missing audit owner") {
		t.Fatalf("Run() error = %v", err)
	}
}

func writeExecutablePlugin(t *testing.T, projectDir, script string) Metadata {
	t.Helper()
	pluginDir := filepath.Join(projectDir, ".gosvc", "plugins", "audit")
	entrypoint := filepath.Join(pluginDir, "bin", "plugin")
	if err := os.MkdirAll(filepath.Dir(entrypoint), 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte(script)
	if err := os.WriteFile(entrypoint, content, 0o755); err != nil {
		t.Fatal(err)
	}
	metadata := Metadata{
		SchemaVersion: CurrentSchemaVersion, ProtocolVersion: CurrentProtocolVersion,
		Name: "audit", Version: "1.0.0", Description: "Audit plugin",
		MinimumGosvcVersion: "1.0.0",
		Capabilities:        []Capability{CapabilityValidation, CapabilityArtifacts},
		Entrypoint:          "bin/plugin", Checksum: Checksum(content),
		Source: filepath.Join(pluginDir, "plugin.json"),
	}
	if err := metadata.Validate("1.0.0"); err != nil {
		t.Fatal(err)
	}
	return metadata
}

func TestRunnerUsesProjectSnapshotInsteadOfRealProject(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "project.yaml"), []byte("name: real\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	metadata := writeExecutablePlugin(t, projectDir, `#!/bin/sh
read request
printf 'changed\n' > "$GOSVC_PROJECT_DIR/project.yaml"
printf 'created\n' > "$GOSVC_PROJECT_DIR/direct-write.txt"
printf '%s\n' '{"protocol_version":1}'
`)
	snapshot, err := AbsoluteProjectSnapshot(projectDir, "service", "github.com/acme/service", "minimal-api", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (Runner{Timeout: time.Second}).Run(context.Background(), metadata, projectDir, snapshot, ActionValidate); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(projectDir, "project.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "name: real\n" {
		t.Fatalf("real project was modified: %q", content)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "direct-write.txt")); !os.IsNotExist(err) {
		t.Fatalf("plugin wrote directly into real project: %v", err)
	}
}
