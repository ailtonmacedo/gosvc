package plugin

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ailtonmacedo/gosvc/internal/version"
)

const (
	CurrentSchemaVersion   = 3
	CurrentProtocolVersion = 1
)

var (
	namePattern        = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
	checksumPattern    = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	dockerImagePattern = regexp.MustCompile(`^[^@\s]+@sha256:[0-9a-f]{64}$`)
)

type Capability string

const (
	CapabilityArtifacts  Capability = "artifacts"
	CapabilityValidation Capability = "validation"
	CapabilityCommands   Capability = "commands"
)

type ExecutionMode string

const (
	ExecutionNative ExecutionMode = "native"
	ExecutionDocker ExecutionMode = "docker"
)

type Execution struct {
	Mode   ExecutionMode   `json:"mode,omitempty"`
	Docker DockerExecution `json:"docker,omitempty"`
}

type DockerExecution struct {
	Image   string   `json:"image,omitempty"`
	Command []string `json:"command,omitempty"`
	Network bool     `json:"network,omitempty"`
}

type Metadata struct {
	SchemaVersion       int          `json:"schema_version"`
	ProtocolVersion     int          `json:"protocol_version,omitempty"`
	Name                string       `json:"name"`
	Version             string       `json:"version"`
	Description         string       `json:"description"`
	MinimumGosvcVersion string       `json:"minimum_gosvc_version"`
	Capabilities        []Capability `json:"capabilities"`
	Entrypoint          string       `json:"entrypoint,omitempty"`
	Checksum            string       `json:"checksum,omitempty"`
	Execution           Execution    `json:"execution,omitempty"`
	Source              string       `json:"-"`
	BuiltIn             bool         `json:"-"`
}

type Context struct {
	ProjectDir string
	Preset     string
}

type Contribution struct {
	Features []string
	Files    []File
}

type File struct {
	Path    string
	Content []byte
	Mode    fs.FileMode
}

// Plugin is the stable in-process extension contract. External plugins use the
// JSON protocol implemented by Runner; they never receive generator internals.
type Plugin interface {
	Metadata() Metadata
	Validate(Context) error
	Contribute(Context) (Contribution, error)
}

func BuiltIns() []Metadata {
	return []Metadata{
		builtin("core", "Clean Architecture, configuration, HTTP and generator lifecycle"),
		builtin("postgres", "PostgreSQL, pgxpool, migrations and sqlc"),
		builtin("openapi", "OpenAPI contract generation and request validation"),
		builtin("security", "JWT, refresh tokens, RBAC and rate limiting"),
		builtin("observability", "slog, Prometheus, OpenTelemetry and pprof"),
		builtin("distributed", "Redis, Kafka, Transactional Outbox and Kubernetes"),
	}
}

func builtin(name, description string) Metadata {
	return Metadata{
		SchemaVersion: CurrentSchemaVersion, ProtocolVersion: CurrentProtocolVersion,
		Name: name, Version: "builtin", Description: description,
		MinimumGosvcVersion: "0.1.0",
		Capabilities:        []Capability{CapabilityArtifacts, CapabilityValidation},
		Source:              "builtin", BuiltIn: true,
	}
}

func Discover(projectDir, gosvcVersion string) ([]Metadata, error) {
	plugins := BuiltIns()
	root := filepath.Join(projectDir, ".gosvc", "plugins")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return plugins, nil
		}
		return nil, fmt.Errorf("read plugin directory %q: %w", root, err)
	}
	seen := make(map[string]bool, len(plugins))
	for _, item := range plugins {
		seen[item.Name] = true
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name(), "plugin.json")
		metadata, err := LoadMetadata(path, gosvcVersion)
		if err != nil {
			return nil, err
		}
		metadata.Source = filepath.ToSlash(path)
		if seen[metadata.Name] {
			return nil, fmt.Errorf("duplicate plugin name %q", metadata.Name)
		}
		seen[metadata.Name] = true
		plugins = append(plugins, metadata)
	}
	sort.Slice(plugins, func(i, j int) bool {
		if plugins[i].BuiltIn != plugins[j].BuiltIn {
			return plugins[i].BuiltIn
		}
		return plugins[i].Name < plugins[j].Name
	})
	return plugins, nil
}

func Find(projectDir, gosvcVersion, name string) (Metadata, error) {
	plugins, err := Discover(projectDir, gosvcVersion)
	if err != nil {
		return Metadata{}, err
	}
	for _, item := range plugins {
		if item.Name == name {
			return item, nil
		}
	}
	return Metadata{}, fmt.Errorf("plugin %q was not found", name)
}

func LoadMetadata(path, gosvcVersion string) (Metadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Metadata{}, fmt.Errorf("read plugin manifest %q: %w", path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var metadata Metadata
	if err := decoder.Decode(&metadata); err != nil {
		return Metadata{}, fmt.Errorf("decode plugin manifest %q: %w", path, err)
	}
	if err := metadata.Validate(gosvcVersion); err != nil {
		return Metadata{}, fmt.Errorf("validate plugin manifest %q: %w", path, err)
	}
	return metadata, nil
}

func (m Metadata) Validate(gosvcVersion string) error {
	var issues []string
	if m.SchemaVersion < 1 || m.SchemaVersion > CurrentSchemaVersion {
		issues = append(issues, fmt.Sprintf("schema_version must be between 1 and %d", CurrentSchemaVersion))
	}
	if !namePattern.MatchString(m.Name) {
		issues = append(issues, "name must use lowercase kebab-case")
	}
	if m.Version == "" {
		issues = append(issues, "version is required")
	} else if m.Version != "builtin" {
		if _, err := version.Parse(m.Version); err != nil {
			issues = append(issues, err.Error())
		}
	}
	if strings.TrimSpace(m.Description) == "" {
		issues = append(issues, "description is required")
	}
	if m.MinimumGosvcVersion == "" {
		issues = append(issues, "minimum_gosvc_version is required")
	} else if gosvcVersion != "dev" && gosvcVersion != "" {
		if err := version.AtLeast(gosvcVersion, m.MinimumGosvcVersion); err != nil {
			issues = append(issues, err.Error())
		}
	}
	allowed := map[Capability]bool{CapabilityArtifacts: true, CapabilityValidation: true, CapabilityCommands: true}
	seen := map[Capability]bool{}
	for _, capability := range m.Capabilities {
		if !allowed[capability] {
			issues = append(issues, fmt.Sprintf("unsupported capability %q", capability))
		}
		if seen[capability] {
			issues = append(issues, fmt.Sprintf("duplicate capability %q", capability))
		}
		seen[capability] = true
	}
	if m.Entrypoint != "" {
		clean := filepath.Clean(m.Entrypoint)
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			issues = append(issues, "entrypoint must be a relative path inside the plugin directory")
		}
	}
	if !m.BuiltIn && m.SchemaVersion >= 2 {
		if m.ProtocolVersion != CurrentProtocolVersion {
			issues = append(issues, fmt.Sprintf("protocol_version must be %d", CurrentProtocolVersion))
		}
		mode := m.ExecutionMode()
		if m.SchemaVersion == CurrentSchemaVersion && m.Execution.Mode == "" {
			issues = append(issues, "execution.mode is required for schema 3 plugins")
		}
		switch mode {
		case ExecutionNative:
			if strings.TrimSpace(m.Entrypoint) == "" {
				issues = append(issues, "entrypoint is required for native plugins")
			}
			if !checksumPattern.MatchString(m.Checksum) {
				issues = append(issues, "checksum must use sha256:<64 lowercase hexadecimal characters> for native plugins")
			}
		case ExecutionDocker:
			if !dockerImagePattern.MatchString(m.Execution.Docker.Image) {
				issues = append(issues, "execution.docker.image must be pinned by sha256 digest")
			}
			if len(m.Execution.Docker.Command) == 0 {
				issues = append(issues, "execution.docker.command is required")
			}
			if m.Entrypoint != "" || m.Checksum != "" {
				issues = append(issues, "docker plugins must not declare entrypoint or checksum")
			}
		default:
			issues = append(issues, fmt.Sprintf("unsupported execution mode %q", mode))
		}
	}
	if m.SchemaVersion == 1 && (m.ProtocolVersion != 0 || m.Checksum != "" || m.Execution.Mode != "") {
		issues = append(issues, "schema 1 does not support protocol_version or checksum")
	}
	if len(issues) > 0 {
		return fmt.Errorf("%s", strings.Join(issues, "; "))
	}
	return nil
}

func (m Metadata) ExecutionMode() ExecutionMode {
	if m.SchemaVersion == 2 && m.Execution.Mode == "" {
		return ExecutionNative
	}
	return m.Execution.Mode
}

func (m Metadata) IsSandboxed() bool { return m.ExecutionMode() == ExecutionDocker }

func (m Metadata) HasCapability(capability Capability) bool {
	for _, item := range m.Capabilities {
		if item == capability {
			return true
		}
	}
	return false
}

func (m Metadata) ExecutionReady() error {
	if m.BuiltIn {
		return fmt.Errorf("built-in plugin %q is not an external executable", m.Name)
	}
	if m.SchemaVersion < 2 || m.SchemaVersion > CurrentSchemaVersion {
		return fmt.Errorf("plugin %q uses non-executable schema %d", m.Name, m.SchemaVersion)
	}
	if m.ProtocolVersion != CurrentProtocolVersion {
		return fmt.Errorf("plugin %q protocol %d is incompatible with protocol %d", m.Name, m.ProtocolVersion, CurrentProtocolVersion)
	}
	return nil
}

func (m Metadata) PluginDirectory(projectDir string) string {
	return filepath.Join(projectDir, ".gosvc", "plugins", m.Name)
}

func (m Metadata) EntrypointPath(projectDir string) (string, error) {
	if err := m.ExecutionReady(); err != nil {
		return "", err
	}
	if m.ExecutionMode() != ExecutionNative {
		return "", fmt.Errorf("plugin %q does not use native execution", m.Name)
	}
	root := m.PluginDirectory(projectDir)
	path := filepath.Join(root, filepath.Clean(m.Entrypoint))
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("plugin %q entrypoint escapes its plugin directory", m.Name)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("inspect plugin %q entrypoint: %w", m.Name, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("plugin %q entrypoint must not be a symbolic link", m.Name)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("plugin %q entrypoint must be a regular file", m.Name)
	}
	if info.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("plugin %q entrypoint is not executable", m.Name)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read plugin %q entrypoint: %w", m.Name, err)
	}
	if Checksum(content) != m.Checksum {
		return "", fmt.Errorf("plugin %q entrypoint checksum does not match plugin.json", m.Name)
	}
	return path, nil
}

func Checksum(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}
