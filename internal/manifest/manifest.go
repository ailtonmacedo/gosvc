package manifest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

const (
	Path                 = ".gosvc/manifest.json"
	CurrentSchemaVersion = 3
)

type File struct {
	Path      string `json:"path"`
	Ownership string `json:"ownership"`
	Checksum  string `json:"checksum"`
	Producer  string `json:"producer,omitempty"`
}

type Project struct {
	Name                string `json:"name"`
	Module              string `json:"module"`
	ConfigSchemaVersion int    `json:"config_schema_version"`
}

type Compatibility struct {
	MinimumGosvcVersion       string `json:"minimum_gosvc_version,omitempty"`
	LastValidatedGosvcVersion string `json:"last_validated_gosvc_version,omitempty"`
}

type PluginReference struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	Source          string `json:"source"`
	Checksum        string `json:"checksum,omitempty"`
	ProtocolVersion int    `json:"protocol_version,omitempty"`
}

type UpgradeRecord struct {
	FromFrameworkVersion string `json:"from_framework_version"`
	ToFrameworkVersion   string `json:"to_framework_version"`
	FromSchemaVersion    int    `json:"from_schema_version"`
	ToSchemaVersion      int    `json:"to_schema_version"`
	AppliedAt            string `json:"applied_at"`
	Backup               string `json:"backup,omitempty"`
}

type RollbackRecord struct {
	Backup              string `json:"backup"`
	RestoredFromVersion string `json:"restored_from_version"`
	RestoredToVersion   string `json:"restored_to_version"`
	AppliedAt           string `json:"applied_at"`
}

type Manifest struct {
	FrameworkVersion string            `json:"framework_version"`
	SchemaVersion    int               `json:"schema_version"`
	Project          Project           `json:"project"`
	Compatibility    Compatibility     `json:"compatibility,omitempty"`
	Preset           string            `json:"preset"`
	Features         []string          `json:"features"`
	Plugins          []PluginReference `json:"plugins,omitempty"`
	UpgradeHistory   []UpgradeRecord   `json:"upgrade_history,omitempty"`
	RollbackHistory  []RollbackRecord  `json:"rollback_history,omitempty"`
	Files            []File            `json:"files"`
}

type Document struct {
	Manifest            Manifest
	SourceSchemaVersion int
}

type manifestV1 struct {
	FrameworkVersion string   `json:"framework_version"`
	SchemaVersion    int      `json:"schema_version"`
	Preset           string   `json:"preset"`
	Features         []string `json:"features"`
	Files            []File   `json:"files"`
}

type manifestV2 struct {
	FrameworkVersion string            `json:"framework_version"`
	SchemaVersion    int               `json:"schema_version"`
	Project          Project           `json:"project"`
	Preset           string            `json:"preset"`
	Features         []string          `json:"features"`
	Plugins          []PluginReference `json:"plugins,omitempty"`
	UpgradeHistory   []UpgradeRecord   `json:"upgrade_history,omitempty"`
	Files            []File            `json:"files"`
}

func Load(projectDir string) (Manifest, error) {
	document, err := LoadDocument(projectDir)
	if err != nil {
		return Manifest{}, err
	}
	if document.SourceSchemaVersion != CurrentSchemaVersion {
		return Manifest{}, fmt.Errorf("manifest schema version %d requires upgrade to %d; run 'gosvc upgrade --project %s'", document.SourceSchemaVersion, CurrentSchemaVersion, projectDir)
	}
	return document.Manifest, nil
}

func LoadDocument(projectDir string) (Document, error) {
	path := filepath.Join(projectDir, filepath.FromSlash(Path))
	data, err := os.ReadFile(path)
	if err != nil {
		return Document{}, fmt.Errorf("read manifest %q: %w", path, err)
	}
	return DecodeDocument(data)
}

func DecodeDocument(data []byte) (Document, error) {
	var header struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return Document{}, fmt.Errorf("decode manifest header: %w", err)
	}
	switch header.SchemaVersion {
	case 1:
		var legacy manifestV1
		if err := decodeStrict(data, &legacy); err != nil {
			return Document{}, fmt.Errorf("decode manifest schema 1: %w", err)
		}
		return Document{SourceSchemaVersion: 1, Manifest: Manifest{
			FrameworkVersion: legacy.FrameworkVersion,
			SchemaVersion:    CurrentSchemaVersion,
			Preset:           legacy.Preset,
			Features:         append([]string(nil), legacy.Features...),
			Files:            append([]File(nil), legacy.Files...),
		}}, nil
	case 2:
		var legacy manifestV2
		if err := decodeStrict(data, &legacy); err != nil {
			return Document{}, fmt.Errorf("decode manifest schema 2: %w", err)
		}
		return Document{SourceSchemaVersion: 2, Manifest: Manifest{
			FrameworkVersion: legacy.FrameworkVersion,
			SchemaVersion:    CurrentSchemaVersion,
			Project:          legacy.Project,
			Preset:           legacy.Preset,
			Features:         append([]string(nil), legacy.Features...),
			Plugins:          append([]PluginReference(nil), legacy.Plugins...),
			UpgradeHistory:   append([]UpgradeRecord(nil), legacy.UpgradeHistory...),
			Files:            append([]File(nil), legacy.Files...),
		}}, nil
	case CurrentSchemaVersion:
		var value Manifest
		if err := decodeStrict(data, &value); err != nil {
			return Document{}, fmt.Errorf("decode manifest schema %d: %w", CurrentSchemaVersion, err)
		}
		return Document{Manifest: value, SourceSchemaVersion: CurrentSchemaVersion}, nil
	default:
		return Document{}, fmt.Errorf("unsupported manifest schema version %d", header.SchemaVersion)
	}
}

func decodeStrict(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values are not allowed")
		}
		return fmt.Errorf("decode trailing data: %w", err)
	}
	return nil
}

func Encode(value Manifest) ([]byte, error) {
	value.SchemaVersion = CurrentSchemaVersion
	value.Features = append([]string(nil), value.Features...)
	value.Plugins = append([]PluginReference(nil), value.Plugins...)
	value.UpgradeHistory = append([]UpgradeRecord(nil), value.UpgradeHistory...)
	value.RollbackHistory = append([]RollbackRecord(nil), value.RollbackHistory...)
	value.Files = append([]File(nil), value.Files...)
	sort.Strings(value.Features)
	sort.Slice(value.Plugins, func(i, j int) bool {
		if value.Plugins[i].Name == value.Plugins[j].Name {
			return value.Plugins[i].Version < value.Plugins[j].Version
		}
		return value.Plugins[i].Name < value.Plugins[j].Name
	})
	sort.Slice(value.Files, func(i, j int) bool { return value.Files[i].Path < value.Files[j].Path })
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode manifest: %w", err)
	}
	return append(data, '\n'), nil
}

func Checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func (m Manifest) File(path string) (File, bool) {
	for _, file := range m.Files {
		if file.Path == path {
			return file, true
		}
	}
	return File{}, false
}
