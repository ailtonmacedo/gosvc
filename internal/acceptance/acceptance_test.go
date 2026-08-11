package acceptance

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunPassesAllBuiltInPresets(t *testing.T) {
	root := filepath.Join(t.TempDir(), "matrix")
	var output bytes.Buffer
	report, err := Run(Options{WorkDir: root, Output: &output, FrameworkVersion: "test"})
	if err != nil {
		t.Fatalf("Run() error = %v\n%s", err, output.String())
	}
	if report.Passed != len(StablePresetNames()) || report.Failed != 0 {
		t.Fatalf("report = %+v", report)
	}
	for _, result := range report.Presets {
		if result.Status != "pass" {
			t.Fatalf("preset result = %+v", result)
		}
		if result.SecondGeneration != 0 || result.SecondResourceChange != 0 {
			t.Fatalf("preset %s is not idempotent: %+v", result.Preset, result)
		}
		if _, err := os.Stat(filepath.Join(root, result.Preset, "project.yaml")); err != nil {
			t.Fatalf("preset %s project.yaml: %v", result.Preset, err)
		}
	}
}

func TestRunProducesJSON(t *testing.T) {
	var output bytes.Buffer
	report, err := Run(Options{JSON: true, Output: &output, FrameworkVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	var decoded Report
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("decode report: %v\n%s", err, output.String())
	}
	if decoded.Passed != report.Passed || decoded.SchemaVersion != 1 {
		t.Fatalf("decoded = %+v report = %+v", decoded, report)
	}
}

func TestRunRejectsNonEmptyWorkDir(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "existing.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(Options{WorkDir: root}); err == nil {
		t.Fatal("Run() error = nil")
	}
}
