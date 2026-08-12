package certification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ailtonmacedo/gosvc/internal/acceptance"
	"github.com/ailtonmacedo/gosvc/internal/generator"
	"github.com/ailtonmacedo/gosvc/internal/preset"
	"github.com/ailtonmacedo/gosvc/internal/project"
	"github.com/ailtonmacedo/gosvc/internal/projectcheck"
	"github.com/ailtonmacedo/gosvc/internal/resource"
)

const ReportSchemaVersion = 1

type Status string

const (
	StatusPass    Status = "pass"
	StatusFail    Status = "fail"
	StatusBlocked Status = "blocked"
	StatusSkipped Status = "skipped"
)

type Mode string

const (
	ModeStatic Mode = "static"
	ModeReal   Mode = "real"
)

type Options struct {
	Mode             Mode
	WorkDir          string
	Keep             bool
	JSON             bool
	Output           io.Writer
	FrameworkVersion string
	RequireReal      bool
	CommandTimeout   time.Duration
}

type Report struct {
	SchemaVersion int            `json:"schema_version"`
	GeneratedAt   time.Time      `json:"generated_at"`
	Mode          Mode           `json:"mode"`
	Host          Host           `json:"host"`
	Prerequisites []Prerequisite `json:"prerequisites"`
	Presets       []PresetResult `json:"presets"`
	Passed        int            `json:"passed"`
	Failed        int            `json:"failed"`
	Blocked       int            `json:"blocked"`
	Skipped       int            `json:"skipped"`
	DurationMS    int64          `json:"duration_ms"`
	WorkDir       string         `json:"work_dir,omitempty"`
}

type Host struct {
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	GoVersion string `json:"go_version"`
}

type Prerequisite struct {
	Name     string `json:"name"`
	Command  string `json:"command"`
	Required bool   `json:"required"`
	Status   Status `json:"status"`
	Version  string `json:"version,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

type PresetResult struct {
	Preset     string        `json:"preset"`
	Status     Status        `json:"status"`
	ProjectDir string        `json:"project_dir,omitempty"`
	Checks     []CheckResult `json:"checks"`
	DurationMS int64         `json:"duration_ms"`
	Error      string        `json:"error,omitempty"`
}

type CheckResult struct {
	Name       string `json:"name"`
	Status     Status `json:"status"`
	DurationMS int64  `json:"duration_ms"`
	Detail     string `json:"detail,omitempty"`
	Output     string `json:"output,omitempty"`
}

// realCapabilities is derived from the generated project configuration rather
// than from preset names. This keeps real certification correct for non-HTTP
// presets and for component variants such as postgres-api + GORM.
type realCapabilities struct {
	API       bool
	OpenAPI   bool
	Database  bool
	SQLC      bool
	Compose   bool
	Cache     bool
	Messaging bool
	Outbox    bool
	Worker    bool
	Docker    bool
}

func realCapabilitiesFor(config project.Config) realCapabilities {
	return realCapabilities{
		API:       config.API.Enabled,
		OpenAPI:   config.OpenAPI.Enabled,
		Database:  config.Database.Enabled,
		SQLC:      config.Database.Enabled && config.Database.CodeGeneration == "sqlc",
		Compose:   config.Deployment.Compose,
		Cache:     config.Cache.Enabled,
		Messaging: config.Messaging.Enabled,
		Outbox:    config.Outbox.Enabled,
		Worker:    config.Project.Preset == "worker",
		Docker:    config.Deployment.Docker,
	}
}

func Run(options Options) (Report, error) {
	if options.Mode == "" {
		options.Mode = ModeStatic
	}
	if options.Mode != ModeStatic && options.Mode != ModeReal {
		return Report{}, fmt.Errorf("unsupported certification mode %q", options.Mode)
	}
	if options.Output == nil {
		options.Output = io.Discard
	}
	if options.CommandTimeout <= 0 {
		options.CommandTimeout = 2 * time.Minute
	}

	started := time.Now()
	root, cleanup, err := resolveWorkDir(options.WorkDir, options.Keep)
	if err != nil {
		return Report{}, err
	}
	defer cleanup()

	prerequisites := inspectPrerequisites(options.Mode)
	report := Report{
		SchemaVersion: ReportSchemaVersion,
		GeneratedAt:   time.Now().UTC(),
		Mode:          options.Mode,
		Host:          Host{OS: runtime.GOOS, Arch: runtime.GOARCH, GoVersion: activeGoVersion(prerequisites)},
		Prerequisites: prerequisites,
	}
	if options.WorkDir != "" || options.Keep {
		report.WorkDir = root
	}

	for _, name := range preset.Names() {
		result := certifyPreset(root, name, options)
		report.Presets = append(report.Presets, result)
		switch result.Status {
		case StatusPass:
			report.Passed++
		case StatusFail:
			report.Failed++
		case StatusBlocked:
			report.Blocked++
		case StatusSkipped:
			report.Skipped++
		}
	}
	report.DurationMS = time.Since(started).Milliseconds()

	if options.JSON {
		encoder := json.NewEncoder(options.Output)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return report, fmt.Errorf("encode certification report: %w", err)
		}
	} else {
		printHuman(options.Output, report)
	}

	if report.Failed > 0 {
		return report, fmt.Errorf("certification failed for %d preset(s)", report.Failed)
	}
	if options.RequireReal && report.Blocked > 0 {
		return report, fmt.Errorf("real certification blocked for %d preset(s)", report.Blocked)
	}
	return report, nil
}

func inspectPrerequisites(mode Mode) []Prerequisite {
	specs := []struct {
		name     string
		command  string
		required bool
		args     []string
	}{
		{"Go", "go", true, []string{"version"}},
		{"Git", "git", true, []string{"--version"}},
	}
	if mode == ModeReal {
		specs = append(specs,
			struct {
				name, command string
				required      bool
				args          []string
			}{"Docker", "docker", true, []string{"--version"}},
			struct {
				name, command string
				required      bool
				args          []string
			}{"Docker Compose", "docker", true, []string{"compose", "version"}},
			struct {
				name, command string
				required      bool
				args          []string
			}{"sqlc", "sqlc", true, []string{"version"}},
			struct {
				name, command string
				required      bool
				args          []string
			}{"oapi-codegen", "oapi-codegen", true, []string{"-version"}},
			struct {
				name, command string
				required      bool
				args          []string
			}{"golang-migrate", "migrate", true, []string{"-version"}},
			struct {
				name, command string
				required      bool
				args          []string
			}{"golangci-lint", "golangci-lint", true, []string{"version"}},
			struct {
				name, command string
				required      bool
				args          []string
			}{"govulncheck", "govulncheck", true, []string{"-version"}},
			struct {
				name, command string
				required      bool
				args          []string
			}{"kubectl", "kubectl", false, []string{"version", "--client"}},
		)
	}
	out := make([]Prerequisite, 0, len(specs))
	for _, spec := range specs {
		item := Prerequisite{Name: spec.name, Command: spec.command, Required: spec.required, Status: StatusBlocked}
		path, err := exec.LookPath(spec.command)
		if err != nil {
			item.Detail = "command not found"
			out = append(out, item)
			continue
		}
		item.Command = path
		cmd := exec.Command(path, spec.args...)
		raw, err := cmd.CombinedOutput()
		if err != nil {
			item.Detail = strings.TrimSpace(string(raw))
			if item.Detail == "" {
				item.Detail = err.Error()
			}
			out = append(out, item)
			continue
		}
		item.Status = StatusPass
		item.Version = firstLine(string(raw))
		out = append(out, item)
	}
	return out
}

func certifyPreset(root, name string, options Options) (result PresetResult) {
	started := time.Now()
	result = PresetResult{Preset: name, Status: StatusFail, ProjectDir: filepath.Join(root, name)}
	defer func() { result.DurationMS = time.Since(started).Milliseconds() }()

	config := project.DefaultConfigForPreset(name)
	config.Project.Name = "cert-" + name
	config.Project.Module = "github.com/gosvc/certification/" + name
	if name == "production-api" || name == "event-driven-api" {
		config.Auth.AccessToken.Issuer = config.Project.Name
		config.Auth.AccessToken.Audience = config.Project.Name + "-api"
	}
	if name == "event-driven-api" {
		config.Deployment.Namespace = config.Project.Name
	}

	if err := runCheck(&result, "generate-project", func() (string, error) {
		generated, err := generator.Generate(generator.Request{Config: config, Destination: result.ProjectDir, FrameworkVersion: options.FrameworkVersion})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("changes=%d", len(generated.Changes)), nil
	}); err != nil {
		result.Error = err.Error()
		return result
	}

	if isDatabasePreset(name) {
		if err := runCheck(&result, "add-uuid-resource", func() (string, error) {
			definition, err := resource.Parse("product", "id:uuid,name:string,price:decimal,active:bool,released_at:datetime")
			if err != nil {
				return "", err
			}
			generated, added, err := generator.AddResource(generator.AddResourceRequest{ProjectDir: result.ProjectDir, Definition: definition, FrameworkVersion: options.FrameworkVersion})
			if err != nil {
				return "", err
			}
			if !added {
				return "", fmt.Errorf("representative resource was not added")
			}
			return fmt.Sprintf("changes=%d", len(generated.Changes)), nil
		}); err != nil {
			result.Error = err.Error()
			return result
		}
	}

	if err := runCheck(&result, "structural-validation", func() (string, error) {
		validation, err := projectcheck.Check(result.ProjectDir)
		if err != nil {
			return "", err
		}
		if err := validation.Error(); err != nil {
			return "", err
		}
		return fmt.Sprintf("resources=%d architecture_files=%d", validation.ResourcesChecked, validation.ArchitectureChecked), nil
	}); err != nil {
		result.Error = err.Error()
		return result
	}

	if err := runCheck(&result, "generator-idempotency", func() (string, error) {
		generated, err := generator.Generate(generator.Request{Config: config, Destination: result.ProjectDir, FrameworkVersion: options.FrameworkVersion})
		if err != nil {
			return "", err
		}
		if generator.HasWrites(generated.Changes) || generated.Applied {
			return "", fmt.Errorf("repeat generation produced writes")
		}
		return "no changes", nil
	}); err != nil {
		result.Error = err.Error()
		return result
	}

	if options.Mode == ModeStatic {
		result.Status = StatusPass
		return result
	}

	capabilities := realCapabilitiesFor(config)
	blockers := realBlockers(capabilities, project.RequiredRuntimeGoVersionWithToolchain(config.GoLanguageVersion(), config.GoToolchainVersion()), activeGoVersion(nil))
	if len(blockers) > 0 {
		result.Status = StatusBlocked
		result.Error = strings.Join(blockers, "; ")
		result.Checks = append(result.Checks, CheckResult{Name: "real-environment", Status: StatusBlocked, Detail: result.Error})
		return result
	}

	if err := runCommandCheck(&result, options.CommandTimeout, result.ProjectDir, "go-mod-download", nil, "go", "mod", "download"); err != nil {
		result.Error = err.Error()
		return result
	}
	if capabilities.OpenAPI {
		if err := runCommandCheck(&result, options.CommandTimeout, result.ProjectDir, "oapi-codegen-real", nil, "oapi-codegen", "--config", "api/oapi-codegen.yaml", "api/openapi.yaml"); err != nil {
			result.Error = err.Error()
			return result
		}
	} else {
		appendSkippedCheck(&result, "oapi-codegen-real", "not applicable: OpenAPI is disabled")
	}
	if capabilities.SQLC {
		if err := runCommandCheck(&result, options.CommandTimeout, result.ProjectDir, "sqlc-real", nil, "sqlc", "generate"); err != nil {
			result.Error = err.Error()
			return result
		}
	} else {
		appendSkippedCheck(&result, "sqlc-real", "not applicable: sqlc code generation is disabled")
	}
	// go mod download verifies that dependencies are reachable, but it does not
	// necessarily populate every module content checksum required by compilation.
	// Run tidy after code generation so go.sum reflects imports emitted by
	// oapi-codegen/sqlc as well as handwritten source before the quality gates.
	if err := runCommandCheck(&result, options.CommandTimeout, result.ProjectDir, "go-mod-tidy", nil, "go", "mod", "tidy"); err != nil {
		result.Error = err.Error()
		return result
	}
	// Run independent quality gates as a batch so one lint/security failure does
	// not hide the remaining diagnostics. Runtime checks may continue whenever
	// the project still builds successfully.
	var qualityFailures []string
	buildOK := true
	runQuality := func(name, command string, args ...string) {
		if err := runCommandCheck(&result, options.CommandTimeout, result.ProjectDir, name, nil, command, args...); err != nil {
			qualityFailures = append(qualityFailures, name+": "+err.Error())
			if name == "go-build" {
				buildOK = false
			}
		}
	}
	runQuality("go-test", "go", "test", "./...")
	runQuality("go-vet", "go", "vet", "./...")
	runQuality("go-build", "go", "build", "./...")
	runQuality("golangci-lint", "golangci-lint", "run")
	runQuality("govulncheck", "govulncheck", "./...")
	if !buildOK {
		result.Error = strings.Join(qualityFailures, "; ")
		return result
	}
	if capabilities.Docker {
		if err := runCommandCheck(&result, options.CommandTimeout, result.ProjectDir, "docker-build", nil, "make", "docker-build"); err != nil {
			qualityFailures = append(qualityFailures, "docker-build: "+err.Error())
			result.Error = strings.Join(qualityFailures, "; ")
			return result
		}
	} else {
		appendSkippedCheck(&result, "docker-build", "not applicable: Docker deployment is disabled")
	}

	if capabilities.Database {
		dbURL := "postgres://postgres:postgres@127.0.0.1:5432/" + config.Project.Name + "?sslmode=disable"
		env := []string{"DATABASE_URL=" + dbURL}
		services := []string{"postgres"}
		if capabilities.Cache {
			env = append(env, "REDIS_ADDRESS=127.0.0.1:6379")
			services = append(services, "redis")
		}
		if capabilities.Messaging {
			env = append(env, "KAFKA_BROKERS=127.0.0.1:9092")
			services = append(services, "kafka")
		}
		// Docker Compose has first-class readiness semantics. --wait blocks until
		// services with healthchecks are healthy (and services without one are
		// running), which avoids racing PostgreSQL while it is still starting.
		waitSeconds := int(options.CommandTimeout.Seconds())
		if waitSeconds < 1 {
			waitSeconds = 1
		}
		args := []string{"compose", "up", "-d", "--wait", "--wait-timeout", strconv.Itoa(waitSeconds)}
		args = append(args, services...)
		if err := runCommandCheck(&result, options.CommandTimeout, result.ProjectDir, "compose-dependencies-up", nil, "docker", args...); err != nil {
			result.Error = err.Error()
			return result
		}
		defer func() {
			_ = runCommandCheck(&result, options.CommandTimeout, result.ProjectDir, "compose-down", nil, "docker", "compose", "down", "-v")
		}()
		for _, service := range services {
			if err := verifyComposeServiceReady(result.ProjectDir, service); err != nil {
				result.Checks = append(result.Checks, CheckResult{Name: service + "-health", Status: StatusFail, Detail: err.Error()})
				result.Error = err.Error()
				return result
			}
			result.Checks = append(result.Checks, CheckResult{Name: service + "-health", Status: StatusPass})
		}
		if err := runMigrateUpWithRetry(&result, options.CommandTimeout, result.ProjectDir, env, dbURL); err != nil {
			appendComposeDiagnostics(&result, options.CommandTimeout, result.ProjectDir, "postgres")
			result.Error = err.Error()
			return result
		}
		if capabilities.Messaging {
			if err := provisionKafkaTopics(&result, options.CommandTimeout, result.ProjectDir, config); err != nil {
				appendComposeDiagnostics(&result, options.CommandTimeout, result.ProjectDir, "kafka")
				result.Error = err.Error()
				return result
			}
		}
		if err := runCommandCheck(&result, options.CommandTimeout, result.ProjectDir, "integration-tests", env, "go", "test", "-tags=integration", "./tests/integration/..."); err != nil {
			result.Error = err.Error()
			return result
		}
		if capabilities.API {
			if err := smokeAPI(&result, options.CommandTimeout, result.ProjectDir, name, env, config); err != nil {
				result.Error = err.Error()
				return result
			}
		} else {
			appendSkippedCheck(&result, "api-smoke", "not applicable: HTTP API is disabled")
		}
		if capabilities.Outbox {
			if err := smokeOutbox(&result, options.CommandTimeout, result.ProjectDir, env, config); err != nil {
				result.Error = err.Error()
				return result
			}
		}
		if err := runCommandCheck(&result, options.CommandTimeout, result.ProjectDir, "migrate-down", env, "migrate", "-path", "db/migrations", "-database", dbURL, "down", "1"); err != nil {
			result.Error = err.Error()
			return result
		}
		if err := runCommandCheck(&result, options.CommandTimeout, result.ProjectDir, "migrate-up-again", env, "migrate", "-path", "db/migrations", "-database", dbURL, "up"); err != nil {
			result.Error = err.Error()
			return result
		}
	} else if capabilities.API {
		if err := smokeAPI(&result, options.CommandTimeout, result.ProjectDir, name, nil, config); err != nil {
			result.Error = err.Error()
			return result
		}
	} else if capabilities.Worker {
		if err := smokeStandaloneWorker(&result, options.CommandTimeout, result.ProjectDir); err != nil {
			result.Error = err.Error()
			return result
		}
	} else {
		if err := runCommandCheck(&result, options.CommandTimeout, result.ProjectDir, "app-smoke", nil, "go", "run", "./cmd/app"); err != nil {
			result.Error = err.Error()
			return result
		}
	}

	if len(qualityFailures) > 0 {
		result.Error = strings.Join(qualityFailures, "; ")
		return result
	}

	result.Status = StatusPass
	return result
}

func appendSkippedCheck(result *PresetResult, name, detail string) {
	result.Checks = append(result.Checks, CheckResult{Name: name, Status: StatusSkipped, Detail: detail})
}

func smokeStandaloneWorker(result *PresetResult, timeout time.Duration, dir string) error {
	started := time.Now()
	binary := filepath.Join(dir, ".gosvc-cert-worker")
	defer os.Remove(binary)

	buildCtx, buildCancel := context.WithTimeout(context.Background(), timeout)
	build := exec.CommandContext(buildCtx, "go", "build", "-o", binary, "./cmd/worker")
	build.Dir = dir
	buildOutput, buildErr := build.CombinedOutput()
	buildCancel()
	if buildErr != nil {
		check := CheckResult{Name: "worker-smoke", Status: StatusFail, DurationMS: time.Since(started).Milliseconds(), Detail: "build worker: " + buildErr.Error(), Output: truncate(strings.TrimSpace(string(buildOutput)), 4000)}
		result.Checks = append(result.Checks, check)
		return buildErr
	}

	smokeTimeout := 2 * time.Second
	if timeout > 0 && timeout < smokeTimeout {
		smokeTimeout = timeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), smokeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary)
	cmd.Dir = dir
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	text := truncate(strings.TrimSpace(output.String()), 4000)
	check := CheckResult{Name: "worker-smoke", Status: StatusPass, DurationMS: time.Since(started).Milliseconds(), Output: text}
	if ctx.Err() == context.DeadlineExceeded && strings.Contains(text, "worker started") {
		check.Detail = "worker started and remained alive until certification timeout"
		result.Checks = append(result.Checks, check)
		return nil
	}
	if err == nil && strings.Contains(text, "worker started") {
		check.Detail = "worker started and exited cleanly"
		result.Checks = append(result.Checks, check)
		return nil
	}
	if err == nil {
		err = fmt.Errorf("worker exited without startup signal")
	}
	check.Status = StatusFail
	check.Detail = err.Error()
	result.Checks = append(result.Checks, check)
	return err
}

func certificationKafkaTopics(cfg project.Config) []string {
	return []string{
		"certification.integration",
		"certification.integration" + cfg.Messaging.DLQSuffix,
		cfg.Messaging.TopicPrefix + ".certification",
		cfg.Messaging.TopicPrefix + ".certification" + cfg.Messaging.DLQSuffix,
	}
}

func provisionKafkaTopics(result *PresetResult, timeout time.Duration, dir string, cfg project.Config) error {
	topics := certificationKafkaTopics(cfg)
	args := []string{"compose", "exec", "-T", "kafka", "rpk", "topic", "create"}
	args = append(args, topics...)
	args = append(args, "-p", "1", "-r", "1")
	if err := runCommandCheck(result, timeout, dir, "kafka-topics", nil, "docker", args...); err != nil {
		return fmt.Errorf("provision Kafka certification topics: %w", err)
	}
	return nil
}

func runCheck(result *PresetResult, name string, fn func() (string, error)) error {
	started := time.Now()
	detail, err := fn()
	check := CheckResult{Name: name, Status: StatusPass, Detail: detail, DurationMS: time.Since(started).Milliseconds()}
	if err != nil {
		check.Status = StatusFail
		check.Detail = err.Error()
	}
	result.Checks = append(result.Checks, check)
	return err
}

func runCommandCheck(result *PresetResult, timeout time.Duration, dir, name string, extraEnv []string, command string, args ...string) error {
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), extraEnv...)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	text := truncate(strings.TrimSpace(output.String()), 8000)
	check := CheckResult{Name: name, Status: StatusPass, DurationMS: time.Since(started).Milliseconds(), Output: text}
	if ctx.Err() == context.DeadlineExceeded {
		err = fmt.Errorf("%s timed out after %s", name, timeout)
	}
	if err != nil {
		check.Status = StatusFail
		check.Detail = err.Error()
	}
	result.Checks = append(result.Checks, check)
	return err
}

func realBlockers(capabilities realCapabilities, requiredGo, actualGo string) []string {
	var blockers []string
	if requiredGo != "" && requiredGo != "auto" && !goVersionAtLeast(actualGo, requiredGo) {
		blockers = append(blockers, fmt.Sprintf("Go %s or newer required; host has %s", requiredGo, actualGo))
	}
	required := []string{"go", "git", "golangci-lint", "govulncheck"}
	if capabilities.Docker || capabilities.Compose {
		required = append(required, "docker")
	}
	if capabilities.OpenAPI {
		required = append(required, "oapi-codegen")
	}
	if capabilities.SQLC {
		required = append(required, "sqlc")
	}
	if capabilities.Database {
		required = append(required, "migrate")
	}
	for _, command := range required {
		if _, err := exec.LookPath(command); err != nil {
			blockers = append(blockers, command+" not found")
		}
	}
	if capabilities.Compose {
		if _, err := exec.LookPath("docker"); err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			cmd := exec.CommandContext(ctx, "docker", "compose", "version")
			if err := cmd.Run(); err != nil {
				blockers = append(blockers, "docker compose is unavailable")
			}
			cancel()
		}
	}
	if !networkResolvable("proxy.golang.org") {
		blockers = append(blockers, "proxy.golang.org is not resolvable")
	}
	sort.Strings(blockers)
	return blockers
}

func networkResolvable(host string) bool {
	// Avoid introducing a long network dependency here. getent is available on
	// common Linux CI images; on other platforms the caller will still surface
	// toolchain failures during go mod download.
	if runtime.GOOS != "linux" {
		return true
	}
	path, err := exec.LookPath("getent")
	if err != nil {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "hosts", host)
	return cmd.Run() == nil
}

func activeGoVersion(prerequisites []Prerequisite) string {
	for _, item := range prerequisites {
		if item.Name == "Go" && item.Status == StatusPass {
			if version := parseGoVersionOutput(item.Version); version != "" {
				return version
			}
		}
	}
	path, err := exec.LookPath("go")
	if err == nil {
		if raw, err := exec.Command(path, "version").CombinedOutput(); err == nil {
			if version := parseGoVersionOutput(string(raw)); version != "" {
				return version
			}
		}
	}
	return runtime.Version()
}

func parseGoVersionOutput(value string) string {
	for _, field := range strings.Fields(strings.TrimSpace(value)) {
		if strings.HasPrefix(field, "go1.") {
			return field
		}
	}
	return ""
}

func goVersionAtLeast(actualRuntime, required string) bool {
	actual := strings.TrimSpace(strings.TrimPrefix(actualRuntime, "go"))
	if strings.Contains(actual, "devel") {
		return true
	}
	actualParts := goVersionParts(actual)
	requiredParts := goVersionParts(required)
	for i := range actualParts {
		if actualParts[i] != requiredParts[i] {
			return actualParts[i] > requiredParts[i]
		}
	}
	return true
}

func goVersionParts(value string) [3]int {
	value = strings.TrimSpace(strings.TrimPrefix(value, "go"))
	parts := strings.Split(value, ".")
	var parsed [3]int
	for i := 0; i < len(parts) && i < len(parsed); i++ {
		parsed[i], _ = strconv.Atoi(numericPrefix(parts[i]))
	}
	return parsed
}

func numericPrefix(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r < '0' || r > '9' {
			break
		}
		b.WriteRune(r)
	}
	return b.String()
}

type composePSStatus struct {
	Service  string `json:"Service"`
	State    string `json:"State"`
	Health   string `json:"Health"`
	ExitCode int    `json:"ExitCode"`
}

func verifyComposeServiceReady(dir, service string) error {
	cmd := exec.Command("docker", "compose", "ps", "--format", "json", service)
	cmd.Dir = dir
	raw, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("inspect docker compose service %s: %w", service, err)
	}
	statuses, err := parseComposePS(raw)
	if err != nil {
		return fmt.Errorf("parse docker compose status for %s: %w", service, err)
	}
	if len(statuses) == 0 {
		return fmt.Errorf("docker compose service %s has no running container", service)
	}
	for _, status := range statuses {
		if err := composeStatusReady(status, true); err != nil {
			return fmt.Errorf("docker compose service %s: %w", service, err)
		}
	}
	return nil
}

func composeStatusReady(status composePSStatus, requireHealthy bool) error {
	state := strings.ToLower(strings.TrimSpace(status.State))
	health := strings.ToLower(strings.TrimSpace(status.Health))
	if state != "running" {
		return fmt.Errorf("state=%s exit_code=%d", status.State, status.ExitCode)
	}
	// When a healthcheck exists, "running" is not sufficient: the service
	// must explicitly report healthy. This prevents the startup race that can
	// otherwise expose a published PostgreSQL port before the server is ready.
	if requireHealthy && health != "healthy" {
		if health == "" {
			return fmt.Errorf("health status is empty; expected healthy")
		}
		return fmt.Errorf("health=%s", status.Health)
	}
	if !requireHealthy && health != "" && health != "healthy" {
		return fmt.Errorf("health=%s", status.Health)
	}
	return nil
}

func parseComposePS(raw []byte) ([]composePSStatus, error) {
	var statuses []composePSStatus
	for _, line := range bytes.Split(raw, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var status composePSStatus
		if err := json.Unmarshal(line, &status); err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func runMigrateUpWithRetry(result *PresetResult, timeout time.Duration, dir string, env []string, dbURL string) error {
	started := time.Now()
	deadline := time.Now().Add(timeout)
	if retryLimit := time.Now().Add(15 * time.Second); retryLimit.Before(deadline) {
		deadline = retryLimit
	}
	var lastOutput string
	for attempt := 1; ; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		cmd := exec.CommandContext(ctx, "migrate", "-path", "db/migrations", "-database", dbURL, "up")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), env...)
		raw, err := cmd.CombinedOutput()
		cancel()
		lastOutput = strings.TrimSpace(string(raw))
		if err == nil {
			result.Checks = append(result.Checks, CheckResult{
				Name: "migrate-up", Status: StatusPass,
				Detail: fmt.Sprintf("attempts=%d", attempt), DurationMS: time.Since(started).Milliseconds(),
				Output: lastOutput,
			})
			return nil
		}
		if !isTransientDatabaseStartupError(lastOutput) || time.Now().After(deadline) {
			result.Checks = append(result.Checks, CheckResult{
				Name: "migrate-up", Status: StatusFail, Detail: err.Error(),
				DurationMS: time.Since(started).Milliseconds(), Output: lastOutput,
			})
			return err
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func appendComposeDiagnostics(result *PresetResult, timeout time.Duration, dir, service string) {
	commands := []struct {
		name string
		args []string
	}{
		{name: service + "-ps-diagnostic", args: []string{"compose", "ps", "--all", "--format", "json", service}},
		{name: service + "-logs-diagnostic", args: []string{"compose", "logs", "--no-color", "--tail", "200", service}},
	}
	for _, diagnostic := range commands {
		started := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		cmd := exec.CommandContext(ctx, "docker", diagnostic.args...)
		cmd.Dir = dir
		raw, err := cmd.CombinedOutput()
		cancel()
		check := CheckResult{
			Name: diagnostic.name, Status: StatusPass,
			DurationMS: time.Since(started).Milliseconds(), Output: truncate(strings.TrimSpace(string(raw)), 12000),
		}
		if err != nil {
			check.Status = StatusFail
			check.Detail = err.Error()
		}
		result.Checks = append(result.Checks, check)
	}
}

func isTransientDatabaseStartupError(output string) bool {
	text := strings.ToLower(output)
	transient := []string{
		"connection reset by peer",
		"connection refused",
		"the database system is starting up",
		"server closed the connection unexpectedly",
		"unexpected eof",
		"dial tcp",
	}
	for _, marker := range transient {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func smokeAPI(result *PresetResult, timeout time.Duration, dir, presetName string, baseEnv []string, cfg project.Config) error {
	started := time.Now()
	port, err := freePort()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := exec.CommandContext(ctx, "go", "run", "./cmd/api")
	command.Dir = dir
	env := append([]string{}, baseEnv...)
	env = append(env, "APP_ENV=development", fmt.Sprintf("HTTP_PORT=%d", port), "OTEL_TRACING_ENABLED=false", "AUTH_COOKIE_SECURE=false")
	if presetName == "production-api" || presetName == "event-driven-api" {
		env = append(env, "JWT_SECRET=certification-secret-0123456789-abcdefghijklmnopqrstuvwxyz", "JWT_ISSUER="+cfg.Auth.AccessToken.Issuer, "JWT_AUDIENCE="+cfg.Auth.AccessToken.Audience)
	}
	if presetName == "event-driven-api" {
		env = append(env, "REDIS_ADDRESS=127.0.0.1:6379", "KAFKA_BROKERS=127.0.0.1:9092")
	}
	command.Env = append(os.Environ(), env...)
	var logs bytes.Buffer
	command.Stdout, command.Stderr = &logs, &logs
	if err := command.Start(); err != nil {
		return fmt.Errorf("start API: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			if command.Process != nil {
				_ = command.Process.Kill()
			}
		}
	}()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	if err := waitHTTP(timeout, baseURL+"/health/live", http.StatusOK); err != nil {
		return fmt.Errorf("API liveness: %w; logs=%s", err, truncate(logs.String(), 3000))
	}
	if err := waitHTTP(timeout, baseURL+"/health/ready", http.StatusOK); err != nil {
		return fmt.Errorf("API readiness: %w; logs=%s", err, truncate(logs.String(), 3000))
	}
	checks := []string{"liveness", "readiness"}
	if presetName == "postgres-api" {
		if err := smokeCRUD(baseURL, "", nil); err != nil {
			return err
		}
		checks = append(checks, "orders-crud")
	}
	if presetName == "production-api" || presetName == "event-driven-api" {
		protectedBody := []byte(`{"status":"created","total_cents":1250}`)
		if err := expectStatus(http.DefaultClient, http.MethodPost, baseURL+"/orders", protectedBody, map[string]string{"Content-Type": "application/json"}, http.StatusUnauthorized); err != nil {
			return fmt.Errorf("missing bearer token smoke: %w", err)
		}
		if err := expectStatus(http.DefaultClient, http.MethodPost, baseURL+"/orders", protectedBody, map[string]string{"Content-Type": "application/json", "Authorization": "Bearer invalid-certification-token"}, http.StatusUnauthorized); err != nil {
			return fmt.Errorf("invalid bearer token smoke: %w", err)
		}
		if err := seedAdmin(dir, timeout, cfg.Project.Name); err != nil {
			return err
		}
		token, jar, err := login(baseURL)
		if err != nil {
			return err
		}
		if err := smokeCRUD(baseURL, token, jar); err != nil {
			return err
		}
		if err := expectStatus(http.DefaultClient, http.MethodGet, baseURL+"/admin/ping", nil, map[string]string{"Authorization": "Bearer " + token}, http.StatusOK); err != nil {
			return fmt.Errorf("admin RBAC smoke: %w", err)
		}
		client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
		if err := expectStatus(client, http.MethodPost, baseURL+"/auth/refresh", nil, nil, http.StatusOK); err != nil {
			return fmt.Errorf("refresh token smoke: %w", err)
		}
		if err := expectStatus(client, http.MethodPost, baseURL+"/auth/logout", nil, nil, http.StatusNoContent); err != nil {
			return fmt.Errorf("logout smoke: %w", err)
		}
		if err := expectStatus(http.DefaultClient, http.MethodGet, baseURL+cfg.Observability.Metrics.Endpoint, nil, nil, http.StatusOK); err != nil {
			return fmt.Errorf("metrics smoke: %w", err)
		}
		checks = append(checks, "auth-required", "invalid-jwt", "login", "jwt", "rbac", "refresh", "logout", "metrics", "orders-crud")
	}
	result.Checks = append(result.Checks, CheckResult{Name: "api-smoke", Status: StatusPass, Detail: strings.Join(checks, ","), DurationMS: time.Since(started).Milliseconds()})
	return nil
}

func seedAdmin(dir string, timeout time.Duration, database string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	hashCmd := exec.CommandContext(ctx, "go", "run", "./cmd/hash-password", "change-this-password")
	hashCmd.Dir = dir
	rawHash, err := hashCmd.Output()
	if err != nil {
		return fmt.Errorf("hash certification password: %w", err)
	}
	hash := strings.TrimSpace(string(rawHash))
	sql := fmt.Sprintf("INSERT INTO users (id,email,password_hash,roles,active) VALUES ('00000000-0000-4000-8000-000000000001','admin@example.com','%s',ARRAY['admin'],TRUE) ON CONFLICT (email) DO UPDATE SET password_hash=EXCLUDED.password_hash,roles=EXCLUDED.roles,active=TRUE;", strings.ReplaceAll(hash, "'", "''"))
	cmd := exec.CommandContext(ctx, "docker", "compose", "exec", "-T", "postgres", "psql", "-U", "postgres", "-d", database, "-v", "ON_ERROR_STOP=1", "-c", sql)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("seed admin: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func login(baseURL string) (string, http.CookieJar, error) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	body := []byte(`{"email":"admin@example.com","password":"change-this-password"}`)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/auth/login", bytes.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("login request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(response.Body)
		return "", nil, fmt.Errorf("login status=%d body=%s", response.StatusCode, truncate(string(raw), 1000))
	}
	var payload struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", nil, fmt.Errorf("decode login: %w", err)
	}
	if payload.AccessToken == "" {
		return "", nil, fmt.Errorf("login returned empty access token")
	}
	return payload.AccessToken, jar, nil
}

func smokeCRUD(baseURL, token string, jar http.CookieJar) error {
	client := &http.Client{Timeout: 10 * time.Second, Jar: jar}
	headers := map[string]string{"Content-Type": "application/json"}
	if token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	createBody := []byte(`{"status":"created","total_cents":1250}`)
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/orders", bytes.NewReader(createBody))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("create order: %w", err)
	}
	raw, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		return fmt.Errorf("create order status=%d body=%s", response.StatusCode, truncate(string(raw), 1000))
	}
	var created struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(raw, &created); err != nil || created.ID < 1 {
		return fmt.Errorf("create order returned invalid id: %s", string(raw))
	}
	orderURL := fmt.Sprintf("%s/orders/%d", baseURL, created.ID)
	if err := expectStatus(client, http.MethodGet, orderURL, nil, authHeader(token), http.StatusOK); err != nil {
		return fmt.Errorf("get order: %w", err)
	}
	update := []byte(`{"status":"paid","total_cents":1500}`)
	updateHeaders := authHeader(token)
	updateHeaders["Content-Type"] = "application/json"
	if err := expectStatus(client, http.MethodPut, orderURL, update, updateHeaders, http.StatusOK); err != nil {
		return fmt.Errorf("update order: %w", err)
	}
	if err := expectStatus(client, http.MethodDelete, orderURL, nil, authHeader(token), http.StatusNoContent); err != nil {
		return fmt.Errorf("delete order: %w", err)
	}
	if err := expectStatus(client, http.MethodGet, orderURL, nil, authHeader(token), http.StatusNotFound); err != nil {
		return fmt.Errorf("deleted order lookup: %w", err)
	}
	return nil
}

func authHeader(token string) map[string]string {
	h := map[string]string{}
	if token != "" {
		h["Authorization"] = "Bearer " + token
	}
	return h
}

func expectStatus(client *http.Client, method, url string, body []byte, headers map[string]string, want int) error {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != want {
		raw, _ := io.ReadAll(response.Body)
		return fmt.Errorf("%s %s status=%d want=%d body=%s", method, url, response.StatusCode, want, truncate(string(raw), 1200))
	}
	return nil
}

func waitHTTP(timeout time.Duration, url string, want int) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: time.Second}
	var last error
	for time.Now().Before(deadline) {
		response, err := client.Get(url)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == want {
				return nil
			}
			last = fmt.Errorf("status=%d", response.StatusCode)
		} else {
			last = err
		}
		time.Sleep(250 * time.Millisecond)
	}
	if last == nil {
		last = fmt.Errorf("no response")
	}
	return last
}

func freePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("allocate HTTP port: %w", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func smokeOutbox(result *PresetResult, timeout time.Duration, dir string, baseEnv []string, cfg project.Config) error {
	started := time.Now()
	topic := cfg.Messaging.TopicPrefix + ".certification"
	eventID := "00000000-0000-4000-8000-000000000099"
	sql := fmt.Sprintf("INSERT INTO outbox_events (id,aggregate_type,aggregate_id,event_type,topic,payload,occurred_at) VALUES ('%s','certification','1','certification.created','%s','{\"ok\":true}'::jsonb,NOW()) ON CONFLICT (id) DO UPDATE SET published_at=NULL,failed_at=NULL,locked_at=NULL,attempts=0,last_error=NULL;", eventID, topic)
	cmd := exec.Command("docker", "compose", "exec", "-T", "postgres", "psql", "-U", "postgres", "-d", cfg.Project.Name, "-v", "ON_ERROR_STOP=1", "-c", sql)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("seed outbox: %w: %s", err, string(output))
	}
	ctx, cancel := context.WithCancel(context.Background())
	worker := exec.CommandContext(ctx, "go", "run", "./cmd/worker")
	worker.Dir = dir
	env := append([]string{}, baseEnv...)
	env = append(env, "KAFKA_BROKERS=127.0.0.1:9092", "REDIS_ADDRESS=127.0.0.1:6379", "OUTBOX_POLL_INTERVAL=250ms")
	worker.Env = append(os.Environ(), env...)
	var logs bytes.Buffer
	worker.Stdout = &logs
	worker.Stderr = &logs
	if err := worker.Start(); err != nil {
		cancel()
		return fmt.Errorf("start outbox worker: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- worker.Wait() }()
	workerExited := false
	defer func() {
		cancel()
		if workerExited {
			return
		}
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			if worker.Process != nil {
				_ = worker.Process.Kill()
			}
		}
	}()
	deadline := time.Now().Add(timeout)
	published := false
	for time.Now().Before(deadline) {
		select {
		case workerErr := <-done:
			workerExited = true
			detail := "worker exited before publishing the outbox event"
			if workerErr != nil {
				detail += ": " + workerErr.Error()
			}
			result.Checks = append(result.Checks, CheckResult{Name: "outbox-worker", Status: StatusFail, Detail: detail, Output: truncate(logs.String(), 3000), DurationMS: time.Since(started).Milliseconds()})
			return fmt.Errorf("%s; worker logs=%s", detail, truncate(logs.String(), 3000))
		default:
		}
		check := exec.Command("docker", "compose", "exec", "-T", "postgres", "psql", "-U", "postgres", "-d", cfg.Project.Name, "-tAc", "SELECT published_at IS NOT NULL FROM outbox_events WHERE id='"+eventID+"';")
		check.Dir = dir
		raw, err := check.Output()
		if err == nil && strings.TrimSpace(string(raw)) == "t" {
			published = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !published {
		return fmt.Errorf("outbox event was not marked published; worker logs=%s", truncate(logs.String(), 3000))
	}
	ctxConsume, cancelConsume := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelConsume()
	consume := exec.CommandContext(ctxConsume, "docker", "compose", "exec", "-T", "kafka", "rpk", "topic", "consume", topic, "-n", "1", "--format", "%v")
	consume.Dir = dir
	raw, err := consume.CombinedOutput()
	if err != nil {
		return fmt.Errorf("consume certification event: %w: %s", err, string(raw))
	}
	if !isExpectedOutboxPayload(raw) {
		result.Checks = append(result.Checks, CheckResult{Name: "outbox-kafka-smoke", Status: StatusFail, Detail: topic, Output: truncate(string(raw), 1000), DurationMS: time.Since(started).Milliseconds()})
		return fmt.Errorf("Kafka payload not observed: %s", truncate(string(raw), 1000))
	}
	result.Checks = append(result.Checks, CheckResult{Name: "outbox-kafka-smoke", Status: StatusPass, Detail: topic, DurationMS: time.Since(started).Milliseconds()})
	return nil
}

func isExpectedOutboxPayload(raw []byte) bool {
	var payload map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(raw), &payload); err != nil {
		return false
	}
	ok, exists := payload["ok"].(bool)
	return exists && ok
}

func resolveWorkDir(requested string, keep bool) (string, func(), error) {
	if requested != "" {
		absolute, err := filepath.Abs(requested)
		if err != nil {
			return "", func() {}, fmt.Errorf("resolve certification workdir: %w", err)
		}
		if err := os.MkdirAll(absolute, 0o755); err != nil {
			return "", func() {}, fmt.Errorf("create certification workdir: %w", err)
		}
		entries, err := os.ReadDir(absolute)
		if err != nil {
			return "", func() {}, fmt.Errorf("read certification workdir: %w", err)
		}
		if len(entries) != 0 {
			return "", func() {}, fmt.Errorf("certification workdir %q must be empty", absolute)
		}
		return absolute, func() {}, nil
	}
	root, err := os.MkdirTemp("", "gosvc-certification-")
	if err != nil {
		return "", func() {}, fmt.Errorf("create certification workdir: %w", err)
	}
	return root, func() {
		if !keep {
			_ = os.RemoveAll(root)
		}
	}, nil
}

func printHuman(w io.Writer, report Report) {
	fmt.Fprintf(w, "Certification mode: %s\n", report.Mode)
	fmt.Fprintf(w, "Host: %s/%s %s\n", report.Host.OS, report.Host.Arch, report.Host.GoVersion)
	if len(report.Prerequisites) > 0 {
		fmt.Fprintln(w, "Prerequisites:")
		for _, item := range report.Prerequisites {
			value := item.Version
			if value == "" {
				value = item.Detail
			}
			fmt.Fprintf(w, "  %-8s %-18s %s\n", strings.ToUpper(string(item.Status)), item.Name, value)
		}
	}
	fmt.Fprintln(w, "Presets:")
	for _, result := range report.Presets {
		fmt.Fprintf(w, "  %-8s %-18s checks=%d duration=%dms", strings.ToUpper(string(result.Status)), result.Preset, len(result.Checks), result.DurationMS)
		if result.Error != "" {
			fmt.Fprintf(w, " — %s", result.Error)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "Certification: passed=%d failed=%d blocked=%d skipped=%d duration=%dms\n", report.Passed, report.Failed, report.Blocked, report.Skipped, report.DurationMS)
}

func isDatabasePreset(name string) bool {
	return name == "postgres-api" || name == "production-api" || name == "event-driven-api"
}

func firstLine(value string) string {
	value = strings.TrimSpace(value)
	if index := strings.IndexByte(value, '\n'); index >= 0 {
		value = value[:index]
	}
	return strings.TrimSpace(value)
}

func truncate(value string, maximum int) string {
	if len(value) <= maximum {
		return value
	}
	return value[:maximum] + "\n... output truncated ..."
}

// AcceptanceSmoke makes the relationship between certification and the
// existing acceptance matrix explicit for callers and release evidence.
func AcceptanceSmoke(output io.Writer, frameworkVersion string) error {
	_, err := acceptance.Run(acceptance.Options{Output: output, FrameworkVersion: frameworkVersion})
	return err
}
