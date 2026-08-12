package certification

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/ailtonmacedo/gosvc/internal/project"
)

func TestStaticCertificationPassesAllPresets(t *testing.T) {
	root := filepath.Join(t.TempDir(), "matrix")
	var output bytes.Buffer
	report, err := Run(Options{Mode: ModeStatic, WorkDir: root, Output: &output, FrameworkVersion: "test"})
	if err != nil {
		t.Fatalf("Run() error = %v\n%s", err, output.String())
	}
	if report.Passed != 6 || report.Failed != 0 || report.Blocked != 0 {
		t.Fatalf("report = %+v", report)
	}
	for _, result := range report.Presets {
		if result.Status != StatusPass {
			t.Fatalf("result = %+v", result)
		}
		if _, err := os.Stat(filepath.Join(root, result.Preset, "project.yaml")); err != nil {
			t.Fatalf("%s project missing: %v", result.Preset, err)
		}
	}
}

func TestCertificationJSON(t *testing.T) {
	var output bytes.Buffer
	report, err := Run(Options{Mode: ModeStatic, JSON: true, Output: &output, FrameworkVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	var decoded Report
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("decode: %v\n%s", err, output.String())
	}
	if decoded.SchemaVersion != ReportSchemaVersion || decoded.Passed != report.Passed {
		t.Fatalf("decoded=%+v report=%+v", decoded, report)
	}
}

func TestGoVersionAtLeast(t *testing.T) {
	tests := []struct {
		actual, required string
		want             bool
	}{
		{"go1.23.2", "1.23", true},
		{"go1.23.2", "1.25", false},
		{"go1.25.0", "1.25", true},
		{"go1.25.4", "1.25.12", false},
		{"go1.25.12", "1.25.12", true},
		{"go1.25.13", "1.25.12", true},
		{"go1.26.1", "1.25.12", true},
	}
	for _, test := range tests {
		if got := goVersionAtLeast(test.actual, test.required); got != test.want {
			t.Fatalf("goVersionAtLeast(%q,%q)=%v want %v", test.actual, test.required, got, test.want)
		}
	}
}

func TestParseGoVersionOutput(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"go version go1.25.12 linux/amd64", "go1.25.12"},
		{"go1.26.0", "go1.26.0"},
		{"unexpected", ""},
	}
	for _, test := range tests {
		if got := parseGoVersionOutput(test.input); got != test.want {
			t.Fatalf("parseGoVersionOutput(%q)=%q want %q", test.input, got, test.want)
		}
	}
}

func TestActiveGoVersionUsesPrerequisite(t *testing.T) {
	prerequisites := []Prerequisite{{
		Name: "Go", Status: StatusPass, Version: "go version go1.25.12 linux/amd64",
	}}
	if got := activeGoVersion(prerequisites); got != "go1.25.12" {
		t.Fatalf("activeGoVersion()=%q want %q", got, "go1.25.12")
	}
}

func TestStaticCertificationGeneratedMakefilesUseCompatibleSQLC(t *testing.T) {
	root := filepath.Join(t.TempDir(), "matrix")
	report, err := Run(Options{Mode: ModeStatic, WorkDir: root, FrameworkVersion: "test"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, result := range report.Presets {
		if result.Preset != "postgres-api" && result.Preset != "production-api" && result.Preset != "event-driven-api" {
			continue
		}
		contents, err := os.ReadFile(filepath.Join(root, result.Preset, "Makefile"))
		if err != nil {
			t.Fatalf("read %s Makefile: %v", result.Preset, err)
		}
		if !bytes.Contains(contents, []byte("SQLC_VERSION := v1.30.0")) {
			t.Fatalf("%s Makefile does not pin sqlc v1.30.0", result.Preset)
		}
		if bytes.Contains(contents, []byte("SQLC_VERSION := v1.31.0")) {
			t.Fatalf("%s Makefile still references sqlc v1.31.0", result.Preset)
		}
	}
}

func TestParseComposePSRequiresHealthyWhenHealthcheckExists(t *testing.T) {
	starting := []byte(`{"Service":"postgres","State":"running","Health":"starting","ExitCode":0}` + "\n")
	statuses, err := parseComposePS(starting)
	if err != nil {
		t.Fatalf("parseComposePS(starting): %v", err)
	}
	if len(statuses) != 1 || statuses[0].State != "running" || statuses[0].Health != "starting" {
		t.Fatalf("statuses=%+v", statuses)
	}

	if err := composeStatusReady(statuses[0], true); err == nil {
		t.Fatal("running/starting service must not be accepted as ready")
	}

	healthy := []byte(`{"Service":"postgres","State":"running","Health":"healthy","ExitCode":0}` + "\n")
	statuses, err = parseComposePS(healthy)
	if err != nil {
		t.Fatalf("parseComposePS(healthy): %v", err)
	}
	if len(statuses) != 1 || statuses[0].Health != "healthy" {
		t.Fatalf("statuses=%+v", statuses)
	}
	if err := composeStatusReady(statuses[0], true); err != nil {
		t.Fatalf("healthy service rejected: %v", err)
	}

	withoutHealthcheck := composePSStatus{Service: "plain", State: "running"}
	if err := composeStatusReady(withoutHealthcheck, false); err != nil {
		t.Fatalf("running service without healthcheck rejected: %v", err)
	}
	if err := composeStatusReady(withoutHealthcheck, true); err == nil {
		t.Fatal("healthchecked service with empty health must not be accepted")
	}
}

func TestTransientDatabaseStartupErrors(t *testing.T) {
	transient := []string{
		"read: connection reset by peer",
		"connect: connection refused",
		"FATAL: the database system is starting up",
		"server closed the connection unexpectedly",
		"unexpected EOF",
	}
	for _, message := range transient {
		if !isTransientDatabaseStartupError(message) {
			t.Fatalf("expected transient: %q", message)
		}
	}
	if isTransientDatabaseStartupError("migration failed: syntax error at or near TABLE") {
		t.Fatal("SQL syntax error must not be retried as startup failure")
	}
}

func TestCertificationKafkaTopicsAreExplicitAndDeterministic(t *testing.T) {
	cfg := project.DefaultConfigForPreset("event-driven-api")
	cfg.Messaging.TopicPrefix = "events"
	cfg.Messaging.DLQSuffix = ".dlq"
	want := []string{
		"certification.integration",
		"certification.integration.dlq",
		"events.certification",
		"events.certification.dlq",
	}
	if got := certificationKafkaTopics(cfg); !reflect.DeepEqual(got, want) {
		t.Fatalf("certificationKafkaTopics()=%v want %v", got, want)
	}
}

func TestExpectedOutboxPayloadUsesJSONSemantics(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "compact", raw: `{"ok":true}`, want: true},
		{name: "postgres jsonb formatting", raw: `{"ok": true}`, want: true},
		{name: "whitespace and newline", raw: "  {\"ok\": true}\n", want: true},
		{name: "false", raw: `{"ok":false}`, want: false},
		{name: "missing field", raw: `{"other":true}`, want: false},
		{name: "string is not bool", raw: `{"ok":"true"}`, want: false},
		{name: "invalid json", raw: `not-json`, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isExpectedOutboxPayload([]byte(test.raw)); got != test.want {
				t.Fatalf("isExpectedOutboxPayload(%q)=%v want %v", test.raw, got, test.want)
			}
		})
	}
}

func TestRealCapabilitiesFollowEffectiveConfiguration(t *testing.T) {
	tests := []struct {
		name string
		cfg  project.Config
		want realCapabilities
	}{
		{
			name: "bare has no http database or generators",
			cfg:  project.DefaultConfigForPreset("bare"),
			want: realCapabilities{Docker: true},
		},
		{
			name: "worker is non-http long running",
			cfg:  project.DefaultConfigForPreset("worker"),
			want: realCapabilities{Worker: true, Docker: true},
		},
		{
			name: "postgres sqlc",
			cfg:  project.DefaultConfigForPreset("postgres-api"),
			want: realCapabilities{API: true, OpenAPI: true, Database: true, SQLC: true, Compose: true, Docker: true},
		},
	}
	gorm := project.DefaultConfigForPreset("postgres-api")
	gorm.Database.CodeGeneration = "gorm"
	gorm.Database.Driver = "gorm-postgres"
	gorm.Database.Pool = "database/sql"
	tests = append(tests, struct {
		name string
		cfg  project.Config
		want realCapabilities
	}{
		name: "postgres gorm does not require sqlc",
		cfg:  gorm,
		want: realCapabilities{API: true, OpenAPI: true, Database: true, Compose: true, Docker: true},
	})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := realCapabilitiesFor(test.cfg); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("realCapabilitiesFor()=%+v want %+v", got, test.want)
			}
		})
	}
}

func TestNotApplicableCodegenChecksAreRecordedAsSkipped(t *testing.T) {
	result := PresetResult{Preset: "bare"}
	appendSkippedCheck(&result, "oapi-codegen-real", "not applicable: OpenAPI is disabled")
	appendSkippedCheck(&result, "sqlc-real", "not applicable: sqlc code generation is disabled")
	if len(result.Checks) != 2 {
		t.Fatalf("checks=%d want 2", len(result.Checks))
	}
	for _, check := range result.Checks {
		if check.Status != StatusSkipped || !bytes.Contains([]byte(check.Detail), []byte("not applicable")) {
			t.Fatalf("check=%+v", check)
		}
	}
}

func TestRealBlockerToolRequirementsAreCapabilityDriven(t *testing.T) {
	bare := realCapabilitiesFor(project.DefaultConfigForPreset("bare"))
	gormCfg := project.DefaultConfigForPreset("postgres-api")
	gormCfg.Database.CodeGeneration = "gorm"
	gormCfg.Database.Driver = "gorm-postgres"
	gormCfg.Database.Pool = "database/sql"
	gorm := realCapabilitiesFor(gormCfg)
	if bare.OpenAPI || bare.SQLC || bare.Database {
		t.Fatalf("bare capabilities=%+v", bare)
	}
	if !gorm.Database || !gorm.OpenAPI || gorm.SQLC {
		t.Fatalf("gorm capabilities=%+v", gorm)
	}
}

func TestRealCertificationNonHTTPPresetsSkipAPICodegen(t *testing.T) {
	bin := t.TempDir()
	for _, name := range []string{"docker", "golangci-lint", "govulncheck", "getent"} {
		path := filepath.Join(bin, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	for _, presetName := range []string{"bare", "worker"} {
		t.Run(presetName, func(t *testing.T) {
			result := certifyPreset(t.TempDir(), presetName, Options{Mode: ModeReal, FrameworkVersion: "test", CommandTimeout: 20 * time.Second})
			if result.Status != StatusPass {
				t.Fatalf("result=%+v", result)
			}
			statuses := map[string]Status{}
			for _, check := range result.Checks {
				statuses[check.Name] = check.Status
			}
			if statuses["oapi-codegen-real"] != StatusSkipped || statuses["sqlc-real"] != StatusSkipped {
				t.Fatalf("codegen statuses=%v", statuses)
			}
		})
	}
}
