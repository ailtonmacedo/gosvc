package backup

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ailtonmacedo/gosvc/internal/manifest"
)

const (
	Directory    = ".gosvc/backups"
	metadataName = ".gosvc-backup.json"
)

type Metadata struct {
	SchemaVersion    int    `json:"schema_version"`
	ProjectName      string `json:"project_name"`
	ProjectModule    string `json:"project_module"`
	FrameworkVersion string `json:"framework_version"`
	ManifestSchema   int    `json:"manifest_schema"`
	CreatedAt        string `json:"created_at"`
}

type Entry struct {
	Path     string
	Metadata Metadata
	Size     int64
}

func Create(projectDir string, value manifest.Manifest, now time.Time) (Entry, error) {
	if projectDir == "" {
		projectDir = "."
	}
	stamp := now.UTC().Format("20060102T150405Z")
	name := fmt.Sprintf("%s-%s-schema%d.zip", stamp, sanitize(value.FrameworkVersion), value.SchemaVersion)
	parent := filepath.Dir(projectDir)
	temp, err := os.CreateTemp(parent, ".gosvc-backup-*.zip")
	if err != nil {
		return Entry{}, fmt.Errorf("create backup staging file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	writer := zip.NewWriter(temp)
	metadata := Metadata{
		SchemaVersion: 1, ProjectName: value.Project.Name, ProjectModule: value.Project.Module,
		FrameworkVersion: value.FrameworkVersion, ManifestSchema: value.SchemaVersion,
		CreatedAt: now.UTC().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		_ = temp.Close()
		return Entry{}, fmt.Errorf("encode backup metadata: %w", err)
	}
	if err := writeBytes(writer, metadataName, append(data, '\n'), 0o600, now); err != nil {
		_ = temp.Close()
		return Entry{}, err
	}
	if err := addTree(writer, projectDir, now); err != nil {
		_ = writer.Close()
		_ = temp.Close()
		return Entry{}, err
	}
	if err := writer.Close(); err != nil {
		_ = temp.Close()
		return Entry{}, fmt.Errorf("close backup archive: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return Entry{}, fmt.Errorf("sync backup archive: %w", err)
	}
	if err := temp.Close(); err != nil {
		return Entry{}, fmt.Errorf("close backup archive file: %w", err)
	}

	destinationDir := filepath.Join(projectDir, filepath.FromSlash(Directory))
	if err := os.MkdirAll(destinationDir, 0o700); err != nil {
		return Entry{}, fmt.Errorf("create backup directory: %w", err)
	}
	destination := filepath.Join(destinationDir, name)
	if err := os.Rename(tempPath, destination); err != nil {
		return Entry{}, fmt.Errorf("activate backup: %w", err)
	}
	info, err := os.Stat(destination)
	if err != nil {
		return Entry{}, fmt.Errorf("inspect backup: %w", err)
	}
	return Entry{Path: filepath.ToSlash(filepath.Join(Directory, name)), Metadata: metadata, Size: info.Size()}, nil
}

func List(projectDir string) ([]Entry, error) {
	dir := filepath.Join(projectDir, filepath.FromSlash(Directory))
	items, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read backup directory: %w", err)
	}
	entries := make([]Entry, 0, len(items))
	for _, item := range items {
		if item.IsDir() || !strings.HasSuffix(item.Name(), ".zip") {
			continue
		}
		path := filepath.Join(dir, item.Name())
		metadata, err := readMetadata(path)
		if err != nil {
			return nil, fmt.Errorf("read backup %q: %w", item.Name(), err)
		}
		info, err := item.Info()
		if err != nil {
			return nil, fmt.Errorf("inspect backup %q: %w", item.Name(), err)
		}
		entries = append(entries, Entry{Path: filepath.ToSlash(filepath.Join(Directory, item.Name())), Metadata: metadata, Size: info.Size()})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Metadata.CreatedAt > entries[j].Metadata.CreatedAt })
	return entries, nil
}

func Resolve(projectDir, name string) (Entry, error) {
	entries, err := List(projectDir)
	if err != nil {
		return Entry{}, err
	}
	if len(entries) == 0 {
		return Entry{}, fmt.Errorf("no upgrade backups found")
	}
	if name == "" || name == "latest" {
		return entries[0], nil
	}
	clean := filepath.ToSlash(name)
	for _, entry := range entries {
		if entry.Path == clean || filepath.Base(entry.Path) == filepath.Base(clean) {
			return entry, nil
		}
	}
	return Entry{}, fmt.Errorf("backup %q was not found", name)
}

func Restore(projectDir string, entry Entry, now time.Time) error {
	current, err := manifest.LoadDocument(projectDir)
	if err != nil {
		return fmt.Errorf("load current manifest: %w", err)
	}
	if current.Manifest.Project.Name != entry.Metadata.ProjectName || current.Manifest.Project.Module != entry.Metadata.ProjectModule {
		return fmt.Errorf("backup belongs to %s (%s), not %s (%s)", entry.Metadata.ProjectName, entry.Metadata.ProjectModule, current.Manifest.Project.Name, current.Manifest.Project.Module)
	}
	archivePath := filepath.Join(projectDir, filepath.FromSlash(entry.Path))
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open backup archive: %w", err)
	}
	defer archive.Close()

	parent := filepath.Dir(projectDir)
	stage, err := os.MkdirTemp(parent, ".gosvc-rollback-")
	if err != nil {
		return fmt.Errorf("create rollback staging directory: %w", err)
	}
	defer os.RemoveAll(stage)
	for _, file := range archive.File {
		if file.Name == metadataName {
			continue
		}
		if err := extractFile(stage, file); err != nil {
			return err
		}
	}
	currentBackups := filepath.Join(projectDir, filepath.FromSlash(Directory))
	if _, err := os.Stat(currentBackups); err == nil {
		if err := copyTree(currentBackups, filepath.Join(stage, filepath.FromSlash(Directory))); err != nil {
			return fmt.Errorf("preserve backup history: %w", err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect current backups: %w", err)
	}

	document, err := manifest.LoadDocument(stage)
	if err != nil {
		return fmt.Errorf("load restored manifest: %w", err)
	}
	restored := document.Manifest
	restored.Compatibility = manifest.Compatibility{
		MinimumGosvcVersion:       restored.FrameworkVersion,
		LastValidatedGosvcVersion: restored.FrameworkVersion,
	}
	restored.RollbackHistory = append(restored.RollbackHistory, manifest.RollbackRecord{
		Backup: entry.Path, RestoredFromVersion: current.Manifest.FrameworkVersion,
		RestoredToVersion: restored.FrameworkVersion, AppliedAt: now.UTC().Format(time.RFC3339),
	})
	data, err := manifest.Encode(restored)
	if err != nil {
		return err
	}
	manifestPath := filepath.Join(stage, filepath.FromSlash(manifest.Path))
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		return fmt.Errorf("write restored manifest: %w", err)
	}
	return swapDirectory(projectDir, stage)
}

func addTree(writer *zip.Writer, root string, now time.Time) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		slash := filepath.ToSlash(relative)
		if slash == Directory || strings.HasPrefix(slash, Directory+"/") {
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
			return fmt.Errorf("backup does not support symbolic link %q", slash)
		}
		if entry.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("backup does not support file type at %q", slash)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read backup file %q: %w", slash, err)
		}
		return writeBytes(writer, slash, content, info.Mode().Perm(), now)
	})
}

func writeBytes(writer *zip.Writer, name string, content []byte, mode fs.FileMode, now time.Time) error {
	header := &zip.FileHeader{Name: filepath.ToSlash(name), Method: zip.Deflate}
	header.SetMode(mode)
	header.SetModTime(now.UTC())
	output, err := writer.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create backup entry %q: %w", name, err)
	}
	if _, err := output.Write(content); err != nil {
		return fmt.Errorf("write backup entry %q: %w", name, err)
	}
	return nil
}

func readMetadata(path string) (Metadata, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return Metadata{}, err
	}
	defer archive.Close()
	for _, file := range archive.File {
		if file.Name != metadataName {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			return Metadata{}, err
		}
		defer reader.Close()
		decoder := json.NewDecoder(io.LimitReader(reader, 1<<20))
		decoder.DisallowUnknownFields()
		var metadata Metadata
		if err := decoder.Decode(&metadata); err != nil {
			return Metadata{}, err
		}
		if metadata.SchemaVersion != 1 {
			return Metadata{}, fmt.Errorf("unsupported backup metadata schema %d", metadata.SchemaVersion)
		}
		return metadata, nil
	}
	return Metadata{}, fmt.Errorf("backup metadata is missing")
}

func extractFile(stage string, file *zip.File) error {
	clean := filepath.Clean(filepath.FromSlash(file.Name))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("unsafe backup entry %q", file.Name)
	}
	if file.Mode()&os.ModeSymlink != 0 || !file.Mode().IsRegular() {
		return fmt.Errorf("unsupported backup entry %q", file.Name)
	}
	destination := filepath.Join(stage, clean)
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	input, err := file.Open()
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, file.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, io.LimitReader(input, 128<<20))
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func copyTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return os.MkdirAll(destination, 0o700)
		}
		target := filepath.Join(destination, relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic link %q is not supported", relative)
		}
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, content, info.Mode().Perm())
	})
}

func swapDirectory(destination, stage string) error {
	backup := destination + ".gosvc-rollback-old"
	if err := os.RemoveAll(backup); err != nil {
		return err
	}
	if err := os.Rename(destination, backup); err != nil {
		return fmt.Errorf("move current project aside: %w", err)
	}
	if err := os.Rename(stage, destination); err != nil {
		_ = os.Rename(backup, destination)
		return fmt.Errorf("activate restored project: %w", err)
	}
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("remove previous project after rollback: %w", err)
	}
	return nil
}

func sanitize(value string) string {
	value = strings.TrimSpace(strings.TrimPrefix(value, "v"))
	if value == "" {
		return "unknown"
	}
	var output strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' {
			output.WriteRune(r)
		} else {
			output.WriteByte('-')
		}
	}
	return output.String()
}
