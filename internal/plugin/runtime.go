package plugin

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	DefaultTimeout       = 10 * time.Second
	DefaultMaxOutputSize = 4 << 20 // 4 MiB
)

type Action string

const (
	ActionValidate   Action = "validate"
	ActionContribute Action = "contribute"
)

type ProjectSnapshot struct {
	Root     string   `json:"root"`
	Name     string   `json:"name"`
	Module   string   `json:"module"`
	Preset   string   `json:"preset"`
	Features []string `json:"features"`
	DryRun   bool     `json:"dry_run"`
}

type ProtocolRequest struct {
	ProtocolVersion int             `json:"protocol_version"`
	Action          Action          `json:"action"`
	Plugin          PluginIdentity  `json:"plugin"`
	Project         ProjectSnapshot `json:"project"`
}

type PluginIdentity struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ProtocolResponse struct {
	ProtocolVersion int                  `json:"protocol_version"`
	Diagnostics     []Diagnostic         `json:"diagnostics,omitempty"`
	Contribution    ProtocolContribution `json:"contribution,omitempty"`
}

type Diagnostic struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Path     string `json:"path,omitempty"`
}

type ProtocolContribution struct {
	Features  []string           `json:"features,omitempty"`
	Artifacts []ProtocolArtifact `json:"artifacts,omitempty"`
}

type ProtocolArtifact struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Encoding  string `json:"encoding,omitempty"`
	Mode      uint32 `json:"mode"`
	Ownership string `json:"ownership"`
}

func (a ProtocolArtifact) Bytes() ([]byte, error) {
	switch a.Encoding {
	case "", "plain":
		return []byte(a.Content), nil
	case "base64":
		content, err := base64.StdEncoding.DecodeString(a.Content)
		if err != nil {
			return nil, fmt.Errorf("decode artifact %q base64 content: %w", a.Path, err)
		}
		return content, nil
	default:
		return nil, fmt.Errorf("artifact %q uses unsupported encoding %q", a.Path, a.Encoding)
	}
}

type Runner struct {
	Timeout        time.Duration
	MaxOutputSize  int
	RequireSandbox bool
	AllowNetwork   bool
}

func (r Runner) Run(ctx context.Context, metadata Metadata, projectDir string, project ProjectSnapshot, action Action) (ProtocolResponse, error) {
	if err := metadata.ExecutionReady(); err != nil {
		return ProtocolResponse{}, err
	}
	mode := metadata.ExecutionMode()
	if r.RequireSandbox && mode != ExecutionDocker {
		return ProtocolResponse{}, fmt.Errorf("plugin %q is not sandboxed; schema 3 docker execution is required", metadata.Name)
	}
	var workspace executionWorkspace
	var err error
	if mode == ExecutionNative {
		realEntrypoint, entryErr := metadata.EntrypointPath(projectDir)
		if entryErr != nil {
			return ProtocolResponse{}, entryErr
		}
		workspace, err = prepareExecutionWorkspace(metadata, projectDir, realEntrypoint, project)
	} else if mode == ExecutionDocker {
		workspace, err = prepareDockerWorkspace(metadata, projectDir)
	} else {
		return ProtocolResponse{}, fmt.Errorf("unsupported plugin execution mode %q", mode)
	}
	if err != nil {
		return ProtocolResponse{}, err
	}
	defer workspace.Cleanup()
	if mode == ExecutionDocker {
		project.Root = "/workspace/project"
	} else {
		project.Root = workspace.ProjectRoot
	}
	if action != ActionValidate && action != ActionContribute {
		return ProtocolResponse{}, fmt.Errorf("unsupported plugin action %q", action)
	}
	if action == ActionValidate && !metadata.HasCapability(CapabilityValidation) {
		return ProtocolResponse{}, fmt.Errorf("plugin %q does not declare validation capability", metadata.Name)
	}
	if action == ActionContribute && !metadata.HasCapability(CapabilityArtifacts) {
		return ProtocolResponse{}, fmt.Errorf("plugin %q does not declare artifacts capability", metadata.Name)
	}

	timeout := r.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	maxOutput := r.MaxOutputSize
	if maxOutput <= 0 {
		maxOutput = DefaultMaxOutputSize
	}
	request := ProtocolRequest{
		ProtocolVersion: CurrentProtocolVersion,
		Action:          action,
		Plugin:          PluginIdentity{Name: metadata.Name, Version: metadata.Version},
		Project:         project,
	}
	input, err := json.Marshal(request)
	if err != nil {
		return ProtocolResponse{}, fmt.Errorf("encode request for plugin %q: %w", metadata.Name, err)
	}

	runContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var command *exec.Cmd
	if mode == ExecutionDocker {
		args, buildErr := dockerRunArgs(metadata, workspace.ProjectRoot, r.AllowNetwork)
		if buildErr != nil {
			return ProtocolResponse{}, buildErr
		}
		command = exec.CommandContext(runContext, "docker", args...)
		command.Env = pluginEnvironment(project.Root, action)
	} else {
		command = exec.CommandContext(runContext, workspace.Entrypoint)
		command.Dir = workspace.PluginRoot
		command.Env = pluginEnvironment(project.Root, action)
	}
	command.Stdin = bytes.NewReader(append(input, '\n'))
	stdout := &cappedBuffer{limit: maxOutput}
	stderr := &cappedBuffer{limit: maxOutput}
	command.Stdout = stdout
	command.Stderr = stderr

	runErr := command.Run()
	if errors.Is(runContext.Err(), context.DeadlineExceeded) {
		return ProtocolResponse{}, fmt.Errorf("plugin %q exceeded timeout %s", metadata.Name, timeout)
	}
	if stdout.overflow {
		return ProtocolResponse{}, fmt.Errorf("plugin %q stdout exceeded %d bytes", metadata.Name, maxOutput)
	}
	if runErr != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = runErr.Error()
		}
		return ProtocolResponse{}, fmt.Errorf("plugin %q failed: %s", metadata.Name, message)
	}

	decoder := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	decoder.DisallowUnknownFields()
	var response ProtocolResponse
	if err := decoder.Decode(&response); err != nil {
		return ProtocolResponse{}, fmt.Errorf("decode response from plugin %q: %w", metadata.Name, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return ProtocolResponse{}, fmt.Errorf("plugin %q returned multiple JSON values", metadata.Name)
		}
		return ProtocolResponse{}, fmt.Errorf("decode trailing response from plugin %q: %w", metadata.Name, err)
	}
	if response.ProtocolVersion != CurrentProtocolVersion {
		return ProtocolResponse{}, fmt.Errorf("plugin %q responded with protocol %d, expected %d", metadata.Name, response.ProtocolVersion, CurrentProtocolVersion)
	}
	if err := validateDiagnostics(metadata.Name, response.Diagnostics); err != nil {
		return ProtocolResponse{}, err
	}
	if action == ActionContribute {
		if err := validateContribution(response.Contribution); err != nil {
			return ProtocolResponse{}, fmt.Errorf("plugin %q contribution is invalid: %w", metadata.Name, err)
		}
	}
	return response, nil
}

func validateDiagnostics(pluginName string, diagnostics []Diagnostic) error {
	var failures []string
	for index, diagnostic := range diagnostics {
		if strings.TrimSpace(diagnostic.Message) == "" {
			return fmt.Errorf("plugin %q diagnostic %d has an empty message", pluginName, index)
		}
		switch diagnostic.Severity {
		case "info", "warning":
		case "error":
			message := diagnostic.Message
			if diagnostic.Path != "" {
				message = diagnostic.Path + ": " + message
			}
			failures = append(failures, message)
		default:
			return fmt.Errorf("plugin %q diagnostic %d has unsupported severity %q", pluginName, index, diagnostic.Severity)
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("plugin %q validation failed: %s", pluginName, strings.Join(failures, "; "))
	}
	return nil
}

func validateContribution(contribution ProtocolContribution) error {
	seenFeatures := map[string]bool{}
	for _, feature := range contribution.Features {
		if strings.TrimSpace(feature) == "" {
			return fmt.Errorf("feature names must not be empty")
		}
		if seenFeatures[feature] {
			return fmt.Errorf("duplicate feature %q", feature)
		}
		seenFeatures[feature] = true
	}
	seenPaths := map[string]bool{}
	for _, artifact := range contribution.Artifacts {
		if seenPaths[artifact.Path] {
			return fmt.Errorf("duplicate artifact path %q", artifact.Path)
		}
		seenPaths[artifact.Path] = true
		if artifact.Mode == 0 || artifact.Mode > 0o777 {
			return fmt.Errorf("artifact %q mode must be between 0001 and 0777", artifact.Path)
		}
		if artifact.Ownership != "generated" && artifact.Ownership != "user" {
			return fmt.Errorf("artifact %q has unsupported ownership %q", artifact.Path, artifact.Ownership)
		}
		if _, err := artifact.Bytes(); err != nil {
			return err
		}
	}
	return nil
}

func pluginEnvironment(projectRoot string, action Action) []string {
	allowed := map[string]bool{
		"HOME": true, "PATH": true, "TMPDIR": true, "TEMP": true, "TMP": true,
		"LANG": true, "LC_ALL": true, "SYSTEMROOT": true,
	}
	values := make([]string, 0, len(allowed)+3)
	for _, item := range os.Environ() {
		name, _, ok := strings.Cut(item, "=")
		if ok && allowed[strings.ToUpper(name)] {
			values = append(values, item)
		}
	}
	values = append(values,
		"GOSVC_PLUGIN_PROTOCOL=1",
		"GOSVC_PLUGIN_ACTION="+string(action),
		"GOSVC_PROJECT_DIR="+projectRoot,
	)
	sort.Strings(values)
	return values
}

func dockerRunArgs(metadata Metadata, projectRoot string, allowNetwork bool) ([]string, error) {
	cfg := metadata.Execution.Docker
	if cfg.Network && !allowNetwork {
		return nil, fmt.Errorf("plugin %q requests network access; rerun with --allow-network only if you trust it", metadata.Name)
	}
	args := []string{"run", "--rm", "-i", "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges=true", "--pids-limit", "64", "--memory", "256m", "--cpus", "1", "--tmpfs", "/tmp:rw,noexec,nosuid,size=64m", "--user", "65532:65532"}
	if cfg.Network {
		args = append(args, "--network", "bridge")
	} else {
		args = append(args, "--network", "none")
	}
	args = append(args, "--mount", "type=bind,src="+projectRoot+",dst=/workspace/project,readonly", "-e", "GOSVC_PLUGIN_PROTOCOL=1", "-e", "GOSVC_PLUGIN_ACTION", "-e", "GOSVC_PROJECT_DIR=/workspace/project", "-w", "/workspace/project", cfg.Image)
	args = append(args, cfg.Command...)
	return args, nil
}

func prepareDockerWorkspace(metadata Metadata, projectDir string) (executionWorkspace, error) {
	root, err := os.MkdirTemp("", ".gosvc-plugin-docker-")
	if err != nil {
		return executionWorkspace{}, fmt.Errorf("create docker plugin workspace: %w", err)
	}
	workspace := executionWorkspace{Root: root, ProjectRoot: filepath.Join(root, "project")}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(root)
		}
	}()
	projectAbsolute, err := filepath.Abs(projectDir)
	if err != nil {
		return executionWorkspace{}, err
	}
	skip := func(relative string, entry fs.DirEntry) bool {
		clean := filepath.ToSlash(relative)
		return clean == ".git" || strings.HasPrefix(clean, ".git/") || clean == ".gosvc/plugins" || strings.HasPrefix(clean, ".gosvc/plugins/") || strings.HasSuffix(clean, ".gosvc-backup")
	}
	if err := copyWorkspaceTree(projectAbsolute, workspace.ProjectRoot, skip); err != nil {
		return executionWorkspace{}, fmt.Errorf("copy project snapshot for plugin %q: %w", metadata.Name, err)
	}
	cleanup = false
	return workspace, nil
}

type executionWorkspace struct {
	Root        string
	PluginRoot  string
	ProjectRoot string
	Entrypoint  string
}

func (w executionWorkspace) Cleanup() { _ = os.RemoveAll(w.Root) }

func prepareExecutionWorkspace(metadata Metadata, projectDir, realEntrypoint string, project ProjectSnapshot) (executionWorkspace, error) {
	root, err := os.MkdirTemp("", ".gosvc-plugin-")
	if err != nil {
		return executionWorkspace{}, fmt.Errorf("create plugin execution workspace: %w", err)
	}
	workspace := executionWorkspace{
		Root:        root,
		PluginRoot:  filepath.Join(root, "plugin"),
		ProjectRoot: filepath.Join(root, "project"),
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(root)
		}
	}()
	if err := copyWorkspaceTree(metadata.PluginDirectory(projectDir), workspace.PluginRoot, nil); err != nil {
		return executionWorkspace{}, fmt.Errorf("copy plugin %q into execution workspace: %w", metadata.Name, err)
	}
	projectAbsolute, err := filepath.Abs(projectDir)
	if err != nil {
		return executionWorkspace{}, fmt.Errorf("resolve project directory: %w", err)
	}
	skip := func(relative string, entry fs.DirEntry) bool {
		clean := filepath.ToSlash(relative)
		return clean == ".git" || strings.HasPrefix(clean, ".git/") ||
			clean == ".gosvc/plugins" || strings.HasPrefix(clean, ".gosvc/plugins/") ||
			strings.HasSuffix(clean, ".gosvc-backup")
	}
	if err := copyWorkspaceTree(projectAbsolute, workspace.ProjectRoot, skip); err != nil {
		return executionWorkspace{}, fmt.Errorf("copy project snapshot for plugin %q: %w", metadata.Name, err)
	}
	relativeEntrypoint, err := filepath.Rel(metadata.PluginDirectory(projectDir), realEntrypoint)
	if err != nil {
		return executionWorkspace{}, fmt.Errorf("resolve copied plugin entrypoint: %w", err)
	}
	workspace.Entrypoint = filepath.Join(workspace.PluginRoot, relativeEntrypoint)
	if info, err := os.Stat(workspace.Entrypoint); err != nil {
		return executionWorkspace{}, fmt.Errorf("inspect copied plugin entrypoint: %w", err)
	} else if info.Mode().Perm()&0o111 == 0 {
		return executionWorkspace{}, fmt.Errorf("copied plugin entrypoint is not executable")
	}
	cleanup = false
	return workspace, nil
}

func copyWorkspaceTree(source, destination string, skip func(string, fs.DirEntry) bool) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return os.MkdirAll(destination, 0o755)
		}
		if skip != nil && skip(relative, entry) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic link %q is not supported", filepath.ToSlash(relative))
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported file type %q", filepath.ToSlash(relative))
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			_ = input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		inputErr := input.Close()
		outputErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if inputErr != nil {
			return inputErr
		}
		return outputErr
	})
}

type cappedBuffer struct {
	data     bytes.Buffer
	limit    int
	overflow bool
}

func (b *cappedBuffer) Write(content []byte) (int, error) {
	original := len(content)
	remaining := b.limit - b.data.Len()
	if remaining <= 0 {
		b.overflow = true
		return original, nil
	}
	if len(content) > remaining {
		content = content[:remaining]
		b.overflow = true
	}
	_, _ = b.data.Write(content)
	return original, nil
}

func (b *cappedBuffer) Bytes() []byte  { return b.data.Bytes() }
func (b *cappedBuffer) String() string { return b.data.String() }

func AbsoluteProjectSnapshot(projectDir, name, module, preset string, features []string, dryRun bool) (ProjectSnapshot, error) {
	root, err := filepath.Abs(projectDir)
	if err != nil {
		return ProjectSnapshot{}, fmt.Errorf("resolve project directory: %w", err)
	}
	return ProjectSnapshot{
		Root: root, Name: name, Module: module, Preset: preset,
		Features: append([]string(nil), features...), DryRun: dryRun,
	}, nil
}
