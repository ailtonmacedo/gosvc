package acceptance

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/ailtonmacedo/gosvc/internal/generator"
	"github.com/ailtonmacedo/gosvc/internal/preset"
	"github.com/ailtonmacedo/gosvc/internal/project"
	"github.com/ailtonmacedo/gosvc/internal/projectcheck"
	"github.com/ailtonmacedo/gosvc/internal/resource"
)

// Options controls the built-in acceptance matrix.
type Options struct {
	WorkDir          string
	Keep             bool
	JSON             bool
	Output           io.Writer
	FrameworkVersion string
}

// Report summarizes acceptance results across all built-in presets.
type Report struct {
	SchemaVersion int            `json:"schema_version"`
	GeneratedAt   time.Time      `json:"generated_at"`
	WorkDir       string         `json:"work_dir,omitempty"`
	Presets       []PresetResult `json:"presets"`
	Passed        int            `json:"passed"`
	Failed        int            `json:"failed"`
	DurationMS    int64          `json:"duration_ms"`
}

// PresetResult captures the checks performed for one preset.
type PresetResult struct {
	Preset               string   `json:"preset"`
	Project              string   `json:"project"`
	Files                int      `json:"files"`
	Resources            int      `json:"resources"`
	ArchitectureFiles    int      `json:"architecture_files"`
	FirstGeneration      int      `json:"first_generation_changes"`
	SecondGeneration     int      `json:"second_generation_changes"`
	ResourceChanges      int      `json:"resource_changes,omitempty"`
	SecondResourceChange int      `json:"second_resource_changes,omitempty"`
	Checks               []string `json:"checks"`
	Status               string   `json:"status"`
	Error                string   `json:"error,omitempty"`
	DurationMS           int64    `json:"duration_ms"`
}

// Run generates every built-in preset, validates it, adds a representative
// UUID resource to database-backed presets, and verifies idempotency.
func Run(options Options) (Report, error) {
	if options.Output == nil {
		options.Output = io.Discard
	}
	started := time.Now()
	root, cleanup, err := resolveWorkDir(options.WorkDir, options.Keep)
	if err != nil {
		return Report{}, err
	}
	defer cleanup()

	report := Report{SchemaVersion: 1, GeneratedAt: time.Now().UTC()}
	if options.WorkDir != "" || options.Keep {
		report.WorkDir = root
	}
	for _, name := range preset.Names() {
		result := runPreset(root, name, options.FrameworkVersion)
		report.Presets = append(report.Presets, result)
		if result.Status == "pass" {
			report.Passed++
		} else {
			report.Failed++
		}
	}
	report.DurationMS = time.Since(started).Milliseconds()

	if options.JSON {
		encoder := json.NewEncoder(options.Output)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return Report{}, fmt.Errorf("encode acceptance report: %w", err)
		}
	} else {
		for _, result := range report.Presets {
			if result.Status == "pass" {
				fmt.Fprintf(options.Output, "PASS    %-18s files=%d resources=%d architecture=%d duration=%dms\n",
					result.Preset, result.Files, result.Resources, result.ArchitectureFiles, result.DurationMS)
				continue
			}
			fmt.Fprintf(options.Output, "FAIL    %-18s %s\n", result.Preset, result.Error)
		}
		fmt.Fprintf(options.Output, "Acceptance matrix: passed=%d failed=%d duration=%dms\n", report.Passed, report.Failed, report.DurationMS)
	}
	if report.Failed > 0 {
		return report, fmt.Errorf("acceptance matrix failed for %d preset(s)", report.Failed)
	}
	return report, nil
}

func resolveWorkDir(requested string, keep bool) (string, func(), error) {
	if requested != "" {
		absolute, err := filepath.Abs(requested)
		if err != nil {
			return "", func() {}, fmt.Errorf("resolve acceptance workdir: %w", err)
		}
		if err := os.MkdirAll(absolute, 0o755); err != nil {
			return "", func() {}, fmt.Errorf("create acceptance workdir: %w", err)
		}
		entries, err := os.ReadDir(absolute)
		if err != nil {
			return "", func() {}, fmt.Errorf("read acceptance workdir: %w", err)
		}
		if len(entries) != 0 {
			return "", func() {}, fmt.Errorf("acceptance workdir %q must be empty", absolute)
		}
		return absolute, func() {}, nil
	}
	root, err := os.MkdirTemp("", "gosvc-acceptance-")
	if err != nil {
		return "", func() {}, fmt.Errorf("create acceptance workdir: %w", err)
	}
	cleanup := func() {
		if !keep {
			_ = os.RemoveAll(root)
		}
	}
	return root, cleanup, nil
}

func runPreset(root, name, frameworkVersion string) (result PresetResult) {
	started := time.Now()
	result = PresetResult{Preset: name, Project: "acceptance-" + name, Status: "fail"}
	defer func() { result.DurationMS = time.Since(started).Milliseconds() }()

	config := project.DefaultConfigForPreset(name)
	config.Project.Name = result.Project
	config.Project.Module = "github.com/gosvc/acceptance/" + name
	if name == "production-api" || name == "event-driven-api" {
		config.Auth.AccessToken.Issuer = result.Project
		config.Auth.AccessToken.Audience = result.Project + "-api"
	}
	if name == "event-driven-api" {
		config.Deployment.Namespace = result.Project
	}
	projectDir := filepath.Join(root, name)
	first, err := generator.Generate(generator.Request{
		Config: config, Destination: projectDir, FrameworkVersion: frameworkVersion,
	})
	if err != nil {
		result.Error = "initial generation: " + err.Error()
		return result
	}
	result.FirstGeneration = countWrites(first.Changes)
	result.Checks = append(result.Checks, "initial-generation")

	generatedConfig, loadErr := project.Load(filepath.Join(projectDir, "project.yaml"))
	if loadErr != nil {
		result.Error = "load generated project contract: " + loadErr.Error()
		return result
	}
	if generatedConfig.Project.Preset != name {
		result.Error = fmt.Sprintf("preset contract mismatch: got %s want %s", generatedConfig.Project.Preset, name)
		return result
	}
	result.Checks = append(result.Checks, "preset-contract")
	if isDatabasePreset(name) {
		if generatedConfig.GoLanguageVersion() != "1.25.0" || generatedConfig.GoToolchainVersion() != "go1.25.12" {
			result.Error = fmt.Sprintf("runtime Go policy mismatch: language=%s toolchain=%s", generatedConfig.GoLanguageVersion(), generatedConfig.GoToolchainVersion())
			return result
		}
		result.Checks = append(result.Checks, "runtime-go-policy")
	}

	if isDatabasePreset(name) {
		definition, parseErr := resource.Parse("product", "id:uuid,name:string,price:decimal,active:bool,released_at:datetime")
		if parseErr != nil {
			result.Error = "parse resource: " + parseErr.Error()
			return result
		}
		addedResult, added, addErr := generator.AddResource(generator.AddResourceRequest{
			ProjectDir: projectDir, Definition: definition, FrameworkVersion: frameworkVersion,
		})
		if addErr != nil {
			result.Error = "add resource: " + addErr.Error()
			return result
		}
		if !added {
			result.Error = "representative resource was not added"
			return result
		}
		result.ResourceChanges = countWrites(addedResult.Changes)
		result.Checks = append(result.Checks, "resource-generation")

		repeatedResource, addedAgain, repeatErr := generator.AddResource(generator.AddResourceRequest{
			ProjectDir: projectDir, Definition: definition, FrameworkVersion: frameworkVersion,
		})
		if repeatErr != nil {
			result.Error = "repeat resource generation: " + repeatErr.Error()
			return result
		}
		if addedAgain || generator.HasWrites(repeatedResource.Changes) || repeatedResource.Applied {
			result.Error = "resource generation is not idempotent"
			return result
		}
		result.SecondResourceChange = countWrites(repeatedResource.Changes)
		result.Checks = append(result.Checks, "resource-idempotency")
	}

	validation, err := projectcheck.Check(projectDir)
	if err != nil {
		result.Error = "structural validation: " + err.Error()
		return result
	}
	if err := validation.Error(); err != nil {
		result.Error = "structural validation: " + err.Error()
		return result
	}
	result.Resources = validation.ResourcesChecked
	result.ArchitectureFiles = validation.ArchitectureChecked
	result.Checks = append(result.Checks, "project-validation", "architecture-boundaries")

	second, err := generator.Generate(generator.Request{
		Config: config, Destination: projectDir, FrameworkVersion: frameworkVersion,
	})
	if err != nil {
		result.Error = "repeat generation: " + err.Error()
		return result
	}
	result.SecondGeneration = countWrites(second.Changes)
	if generator.HasWrites(second.Changes) || second.Applied {
		result.Error = "project generation is not idempotent"
		return result
	}
	result.Checks = append(result.Checks, "project-idempotency")

	files, err := countFiles(projectDir)
	if err != nil {
		result.Error = "count generated files: " + err.Error()
		return result
	}
	result.Files = files
	result.Status = "pass"
	return result
}

func isDatabasePreset(name string) bool {
	return name == "postgres-api" || name == "production-api" || name == "event-driven-api"
}

func countWrites(changes []generator.Change) int {
	count := 0
	for _, change := range changes {
		if change.Action == generator.ActionCreate || change.Action == generator.ActionUpdate {
			count++
		}
	}
	return count
}

func countFiles(root string) (int, error) {
	count := 0
	err := filepath.WalkDir(root, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			count++
		}
		return nil
	})
	return count, err
}

// StablePresetNames exposes the acceptance order for documentation and tests.
func StablePresetNames() []string {
	names := preset.Names()
	sort.Strings(names)
	return names
}
