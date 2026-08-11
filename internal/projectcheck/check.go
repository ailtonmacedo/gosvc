package projectcheck

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ailtonmacedo/gosvc/internal/architecture"
	"github.com/ailtonmacedo/gosvc/internal/manifest"
	"github.com/ailtonmacedo/gosvc/internal/plugin"
	"github.com/ailtonmacedo/gosvc/internal/preset"
	"github.com/ailtonmacedo/gosvc/internal/project"
	"github.com/ailtonmacedo/gosvc/internal/resource"
)

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

type Issue struct {
	Severity Severity
	Path     string
	Message  string
}

func (i Issue) String() string {
	if i.Path == "" {
		return fmt.Sprintf("%s: %s", i.Severity, i.Message)
	}
	return fmt.Sprintf("%s: %s: %s", i.Severity, i.Path, i.Message)
}

type Report struct {
	Config              project.Config
	FilesChecked        int
	ResourcesChecked    int
	ArchitectureChecked int
	Issues              []Issue
}

func (r Report) HasErrors() bool {
	for _, issue := range r.Issues {
		if issue.Severity == SeverityError {
			return true
		}
	}
	return false
}

func (r Report) Error() error {
	if !r.HasErrors() {
		return nil
	}
	messages := make([]string, 0, len(r.Issues))
	for _, issue := range r.Issues {
		if issue.Severity == SeverityError {
			messages = append(messages, issue.String())
		}
	}
	return fmt.Errorf("project validation failed:\n%s", strings.Join(messages, "\n"))
}

func Check(projectDir string) (Report, error) {
	if projectDir == "" {
		projectDir = "."
	}
	absolute, err := filepath.Abs(projectDir)
	if err != nil {
		return Report{}, fmt.Errorf("resolve project directory: %w", err)
	}
	config, err := project.Load(filepath.Join(absolute, "project.yaml"))
	if err != nil {
		return Report{}, fmt.Errorf("load project.yaml: %w", err)
	}
	report := Report{Config: config}
	add := func(severity Severity, path, message string) {
		report.Issues = append(report.Issues, Issue{Severity: severity, Path: filepath.ToSlash(path), Message: message})
	}

	value, err := manifest.Load(absolute)
	if err != nil {
		add(SeverityError, manifest.Path, err.Error())
	} else {
		if value.Project.Name != config.Project.Name {
			add(SeverityError, manifest.Path, fmt.Sprintf("manifest project name %q does not match project.name %q", value.Project.Name, config.Project.Name))
		}
		if value.Project.Module != config.Project.Module {
			add(SeverityError, manifest.Path, fmt.Sprintf("manifest project module %q does not match project.module %q", value.Project.Module, config.Project.Module))
		}
		if value.Project.ConfigSchemaVersion != config.SchemaVersion {
			add(SeverityError, manifest.Path, fmt.Sprintf("manifest config schema %d does not match project schema %d", value.Project.ConfigSchemaVersion, config.SchemaVersion))
		}
		if value.Compatibility.MinimumGosvcVersion == "" {
			add(SeverityError, manifest.Path, "manifest compatibility.minimum_gosvc_version is required")
		}
		if value.Compatibility.LastValidatedGosvcVersion != value.FrameworkVersion {
			add(SeverityError, manifest.Path, fmt.Sprintf("manifest compatibility last validated version %q does not match framework version %q", value.Compatibility.LastValidatedGosvcVersion, value.FrameworkVersion))
		}
		for _, record := range value.UpgradeHistory {
			if record.Backup == "" {
				continue
			}
			backupPath := filepath.Join(absolute, filepath.FromSlash(record.Backup))
			if _, statErr := os.Stat(backupPath); statErr != nil {
				add(SeverityError, record.Backup, fmt.Sprintf("upgrade backup cannot be read: %v", statErr))
			}
		}
		definition, presetErr := preset.Resolve(config.Project.Preset)
		if presetErr != nil {
			add(SeverityError, "project.yaml", presetErr.Error())
		} else {
			if value.Preset != definition.Name {
				add(SeverityError, manifest.Path, fmt.Sprintf("manifest preset %q does not match project preset %q", value.Preset, definition.Name))
			}
			if !sameCoreFeatures(value.Features, definition.Features) {
				add(SeverityError, manifest.Path, "manifest core feature list does not match the selected preset")
			}
			for _, feature := range value.Features {
				if strings.HasPrefix(feature, "plugin:") && !pluginFeatureHasOwner(feature, value.Plugins) {
					add(SeverityError, manifest.Path, fmt.Sprintf("plugin feature %q has no installed plugin owner", feature))
				}
			}
		}
		pluginOwners := make(map[string]bool, len(value.Plugins))
		for _, reference := range value.Plugins {
			pluginOwners[reference.Name] = true
			checkPluginReference(absolute, reference, add)
		}
		for _, file := range value.Files {
			report.FilesChecked++
			if file.Producer != "" && !pluginOwners[file.Producer] {
				add(SeverityError, manifest.Path, fmt.Sprintf("file %q references missing plugin producer %q", file.Path, file.Producer))
			}
			path := filepath.Join(absolute, filepath.FromSlash(file.Path))
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				add(SeverityError, file.Path, fmt.Sprintf("tracked file cannot be read: %v", readErr))
				continue
			}
			actual := manifest.Checksum(content)
			if actual == file.Checksum {
				continue
			}
			if file.Ownership == "generated" {
				add(SeverityError, file.Path, "generated file was modified; regenerate it with gosvc")
			} else {
				add(SeverityWarning, file.Path, "user-owned file differs from the last generated snapshot")
			}
		}
	}

	module, moduleErr := readModule(filepath.Join(absolute, "go.mod"))
	if moduleErr != nil {
		add(SeverityError, "go.mod", moduleErr.Error())
	} else if module != config.Project.Module {
		add(SeverityError, "go.mod", fmt.Sprintf("module %q does not match project.module %q", module, config.Project.Module))
	}

	resources, resourcesErr := resource.Load(absolute)
	if resourcesErr != nil {
		add(SeverityError, resource.RegistryPath, resourcesErr.Error())
	} else if config.Project.Preset == "postgres-api" || config.Project.Preset == "production-api" || config.Project.Preset == "event-driven-api" {
		if len(resources) == 0 {
			add(SeverityError, resource.RegistryPath, "database-backed presets require at least one registered resource")
		}
		report.ResourcesChecked = len(resources)
		for _, definition := range resources {
			checkResourceFiles(absolute, definition, add)
		}
		checkOpenAPI(absolute, resources, add)
	}

	architectureReport, architectureErr := architecture.Check(absolute, config.Project.Module)
	if architectureErr != nil {
		add(SeverityError, "internal", architectureErr.Error())
	} else {
		report.ArchitectureChecked = architectureReport.FilesChecked
		for _, violation := range architectureReport.Violations {
			add(SeverityError, violation.File, fmt.Sprintf("import %q: %s", violation.ImportPath, violation.Rule))
		}
	}

	sort.SliceStable(report.Issues, func(i, j int) bool {
		if report.Issues[i].Severity != report.Issues[j].Severity {
			return report.Issues[i].Severity < report.Issues[j].Severity
		}
		if report.Issues[i].Path != report.Issues[j].Path {
			return report.Issues[i].Path < report.Issues[j].Path
		}
		return report.Issues[i].Message < report.Issues[j].Message
	})
	return report, nil
}

func checkResourceFiles(root string, definition resource.Definition, add func(Severity, string, string)) {
	paths := []string{
		fmt.Sprintf("db/migrations/%06d_create_%s.up.sql", definition.Migration, definition.Plural),
		fmt.Sprintf("db/migrations/%06d_create_%s.down.sql", definition.Migration, definition.Plural),
		fmt.Sprintf("db/queries/%s.sql", definition.Plural),
		fmt.Sprintf("internal/domain/%s.go", definition.Name),
		fmt.Sprintf("internal/ports/%s_repository.go", definition.Name),
		fmt.Sprintf("internal/application/%s_service.go", definition.Name),
		fmt.Sprintf("internal/infrastructure/http/%s_handler.go", definition.Name),
		fmt.Sprintf("internal/infrastructure/persistence/postgres/%s_repository.go", definition.Name),
	}
	for _, path := range paths {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
			add(SeverityError, path, "required resource artifact is missing")
		}
	}
}

func checkOpenAPI(root string, resources []resource.Definition, add func(Severity, string, string)) {
	path := filepath.Join(root, "api", "openapi.yaml")
	content, err := os.ReadFile(path)
	if err != nil {
		add(SeverityError, "api/openapi.yaml", fmt.Sprintf("cannot read contract: %v", err))
		return
	}
	text := string(content)
	if !strings.Contains(text, "openapi: 3.0.3") {
		add(SeverityError, "api/openapi.yaml", "missing supported OpenAPI version declaration")
	}
	for _, definition := range resources {
		for _, expected := range []string{"  /" + definition.Plural + ":", "  /" + definition.Plural + "/{id}:"} {
			if !strings.Contains(text, expected) {
				add(SeverityError, "api/openapi.yaml", fmt.Sprintf("missing path %s", strings.TrimSpace(expected)))
			}
		}
	}
}

func readModule(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			value := strings.TrimSpace(strings.TrimPrefix(line, "module "))
			if value == "" {
				return "", fmt.Errorf("module declaration is empty")
			}
			return value, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan go.mod: %w", err)
	}
	return "", fmt.Errorf("module declaration not found")
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	values := append([]string(nil), left...)
	expected := append([]string(nil), right...)
	sort.Strings(values)
	sort.Strings(expected)
	for index := range values {
		if values[index] != expected[index] {
			return false
		}
	}
	return true
}

func sameCoreFeatures(existing, expected []string) bool {
	core := make([]string, 0, len(existing))
	for _, feature := range existing {
		if !strings.HasPrefix(feature, "plugin:") {
			core = append(core, feature)
		}
	}
	return sameStrings(core, expected)
}

func pluginFeatureHasOwner(feature string, plugins []manifest.PluginReference) bool {
	parts := strings.SplitN(feature, ":", 3)
	if len(parts) != 3 || parts[0] != "plugin" || parts[1] == "" || parts[2] == "" {
		return false
	}
	for _, item := range plugins {
		if item.Name == parts[1] {
			return true
		}
	}
	return false
}

func checkPluginReference(root string, reference manifest.PluginReference, add func(Severity, string, string)) {
	if reference.Name == "" || reference.Source == "" {
		add(SeverityError, manifest.Path, "plugin references require name and source")
		return
	}
	clean := filepath.Clean(filepath.FromSlash(reference.Source))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		add(SeverityError, manifest.Path, fmt.Sprintf("plugin %q source must stay inside the project", reference.Name))
		return
	}
	metadata, err := plugin.LoadMetadata(filepath.Join(root, clean), "dev")
	if err != nil {
		add(SeverityError, reference.Source, err.Error())
		return
	}
	if metadata.Name != reference.Name || metadata.Version != reference.Version {
		add(SeverityError, reference.Source, fmt.Sprintf("plugin identity %s %s does not match manifest reference %s %s", metadata.Name, metadata.Version, reference.Name, reference.Version))
	}
	if reference.Checksum != "" && metadata.Checksum != reference.Checksum {
		add(SeverityError, reference.Source, "plugin checksum does not match manifest reference")
	}
	if reference.ProtocolVersion != 0 && metadata.ProtocolVersion != reference.ProtocolVersion {
		add(SeverityError, reference.Source, "plugin protocol version does not match manifest reference")
	}
	if metadata.SchemaVersion == plugin.CurrentSchemaVersion {
		if _, err := metadata.EntrypointPath(root); err != nil {
			add(SeverityError, reference.Source, err.Error())
		}
	}
}
