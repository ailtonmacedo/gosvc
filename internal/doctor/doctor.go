package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ailtonmacedo/gosvc/internal/project"
)

type Status string

const (
	StatusOK      Status = "ok"
	StatusMissing Status = "missing"
	StatusError   Status = "error"
)

type Tool struct {
	Name     string
	Command  string
	Args     []string
	Required bool
	Local    bool
	Minimum  string
}

type Result struct {
	Tool    Tool
	Status  Status
	Path    string
	Version string
	Error   string
}

type Report struct {
	ProjectDir string
	Preset     string
	Results    []Result
}

func (r Report) HasMissingRequired() bool {
	for _, result := range r.Results {
		if result.Tool.Required && result.Status != StatusOK {
			return true
		}
	}
	return false
}

func (r Report) Err() error {
	if !r.HasMissingRequired() {
		return nil
	}
	missing := make([]string, 0)
	for _, result := range r.Results {
		if result.Tool.Required && result.Status != StatusOK {
			missing = append(missing, result.Tool.Name)
		}
	}
	return fmt.Errorf("required development tools are unavailable: %s", strings.Join(missing, ", "))
}

type LookPathFunc func(string) (string, error)
type RunFunc func(context.Context, string, ...string) ([]byte, error)

func Check(projectDir string) (Report, error) {
	return CheckWith(projectDir, exec.LookPath, func(ctx context.Context, command string, args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, command, args...).CombinedOutput()
	})
}

func CheckWith(projectDir string, lookup LookPathFunc, run RunFunc) (Report, error) {
	if projectDir == "" {
		projectDir = "."
	}
	absolute, err := filepath.Abs(projectDir)
	if err != nil {
		return Report{}, fmt.Errorf("resolve project directory: %w", err)
	}
	report := Report{ProjectDir: absolute}
	preset := ""
	requiredGo := ""
	if _, statErr := os.Stat(filepath.Join(absolute, "project.yaml")); statErr == nil {
		config, loadErr := project.Load(filepath.Join(absolute, "project.yaml"))
		if loadErr != nil {
			return Report{}, fmt.Errorf("load project configuration: %w", loadErr)
		}
		preset = config.Project.Preset
		report.Preset = preset
		if config.Project.GoVersion != "auto" {
			requiredGo = config.Project.GoVersion
		}
	}
	tools := []Tool{
		{Name: "Go", Command: "go", Args: []string{"version"}, Required: true, Minimum: requiredGo},
		{Name: "Git", Command: "git", Args: []string{"--version"}, Required: true},
		{Name: "Docker", Command: "docker", Args: []string{"--version"}, Required: preset != ""},
		{Name: "Docker Compose", Command: "docker", Args: []string{"compose", "version"}, Required: (preset == "postgres-api" || preset == "production-api" || preset == "event-driven-api")},
	}
	if preset == "postgres-api" || preset == "production-api" || preset == "event-driven-api" {
		tools = append(tools,
			Tool{Name: "sqlc", Command: "sqlc", Args: []string{"version"}, Required: true, Local: true},
			Tool{Name: "oapi-codegen", Command: "oapi-codegen", Args: []string{"--version"}, Required: true, Local: true},
			Tool{Name: "golang-migrate", Command: "migrate", Args: []string{"-version"}, Required: true, Local: true},
		)
	}
	if preset == "event-driven-api" {
		tools = append(tools, Tool{Name: "kubectl", Command: "kubectl", Args: []string{"version", "--client"}, Required: true})
	}
	tools = append(tools,
		Tool{Name: "golangci-lint", Command: "golangci-lint", Args: []string{"version"}, Required: true, Local: true},
		Tool{Name: "govulncheck", Command: "govulncheck", Args: []string{"-version"}, Required: true, Local: true},
	)
	for _, tool := range tools {
		result := Result{Tool: tool}
		path, findErr := resolveTool(absolute, tool, lookup)
		if findErr != nil {
			result.Status = StatusMissing
			result.Error = findErr.Error()
			report.Results = append(report.Results, result)
			continue
		}
		result.Path = path
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		output, runErr := run(ctx, path, tool.Args...)
		cancel()
		result.Version = firstLine(string(output))
		if runErr != nil {
			result.Status = StatusError
			result.Error = runErr.Error()
		} else if tool.Minimum != "" && !versionAtLeast(tool.Command, result.Version, tool.Minimum) {
			result.Status = StatusError
			result.Error = fmt.Sprintf("requires %s %s or newer", tool.Name, tool.Minimum)
		} else {
			result.Status = StatusOK
		}
		report.Results = append(report.Results, result)
	}
	return report, nil
}

func resolveTool(projectDir string, tool Tool, lookup LookPathFunc) (string, error) {
	if tool.Local {
		candidate := filepath.Join(projectDir, "bin", tool.Command)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	path, err := lookup(tool.Command)
	if err != nil {
		return "", fmt.Errorf("%s not found in project bin or PATH", tool.Command)
	}
	return path, nil
}

func firstLine(value string) string {
	value = strings.TrimSpace(value)
	if index := strings.IndexByte(value, '\n'); index >= 0 {
		value = value[:index]
	}
	return value
}

func versionAtLeast(command, output, minimum string) bool {
	if command != "go" {
		return true
	}
	const marker = "go version go"
	index := strings.Index(output, marker)
	if index < 0 {
		return false
	}
	value := output[index+len(marker):]
	if end := strings.IndexAny(value, " \t\r\n"); end >= 0 {
		value = value[:end]
	}
	current, ok := numericVersion(value)
	if !ok {
		return false
	}
	required, ok := numericVersion(minimum)
	if !ok {
		return false
	}
	for len(current) < len(required) {
		current = append(current, 0)
	}
	for len(required) < len(current) {
		required = append(required, 0)
	}
	for index := range current {
		if current[index] > required[index] {
			return true
		}
		if current[index] < required[index] {
			return false
		}
	}
	return true
}

func numericVersion(value string) ([]int, bool) {
	parts := strings.Split(value, ".")
	result := make([]int, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return nil, false
		}
		number := 0
		for _, char := range part {
			if char < '0' || char > '9' {
				break
			}
			number = number*10 + int(char-'0')
		}
		result = append(result, number)
	}
	return result, len(result) > 0
}
