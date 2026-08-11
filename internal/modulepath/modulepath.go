package modulepath

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ailtonmacedo/gosvc/internal/repository"
)

const maxTextFileSize = 4 << 20

type Change struct {
	Path         string
	Replacements int
	content      []byte
	mode         fs.FileMode
}

type Plan struct {
	Root       string
	Repository repository.GitHub
	OldModule  string
	NewModule  string
	Changes    []Change
}

func Prepare(root, repositoryValue string) (Plan, error) {
	if root == "" {
		root = "."
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return Plan{}, fmt.Errorf("resolve repository root: %w", err)
	}
	repo, err := repository.Parse(repositoryValue)
	if err != nil {
		return Plan{}, err
	}
	oldModule, err := readModule(filepath.Join(absolute, "go.mod"))
	if err != nil {
		return Plan{}, err
	}
	plan := Plan{Root: absolute, Repository: repo, OldModule: oldModule, NewModule: repo.Module()}
	legacyRepositoryUpper := "OWNER/" + "gosvc"
	legacyRepositoryLower := "owner/" + "gosvc"
	replacements := [][2]string{
		{oldModule, plan.NewModule},
		{legacyRepositoryUpper, repo.Slug()},
		{legacyRepositoryLower, repo.Slug()},
	}
	err = filepath.WalkDir(absolute, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(absolute, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if relative != "." && excludedDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if !entry.Type().IsRegular() || !candidate(path, entry.Name()) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > maxTextFileSize {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.IndexByte(string(content), 0) >= 0 {
			return nil
		}
		updated := string(content)
		count := 0
		for _, replacement := range replacements {
			if replacement[0] == "" || replacement[0] == replacement[1] {
				continue
			}
			n := strings.Count(updated, replacement[0])
			if n > 0 {
				updated = strings.ReplaceAll(updated, replacement[0], replacement[1])
				count += n
			}
		}
		if count == 0 {
			return nil
		}
		plan.Changes = append(plan.Changes, Change{
			Path: filepath.ToSlash(relative), Replacements: count,
			content: []byte(updated), mode: info.Mode().Perm(),
		})
		return nil
	})
	if err != nil {
		return Plan{}, fmt.Errorf("scan repository: %w", err)
	}
	sort.Slice(plan.Changes, func(i, j int) bool { return plan.Changes[i].Path < plan.Changes[j].Path })
	return plan, nil
}

func Apply(plan Plan) error {
	if len(plan.Changes) == 0 {
		return nil
	}
	stagedFiles := make([]staged, 0, len(plan.Changes))
	cleanup := func() {
		for _, item := range stagedFiles {
			_ = os.Remove(item.temp)
		}
	}
	defer cleanup()

	for index, change := range plan.Changes {
		target := filepath.Join(plan.Root, filepath.FromSlash(change.Path))
		file, err := os.CreateTemp(filepath.Dir(target), ".gosvc-module-*")
		if err != nil {
			return fmt.Errorf("stage %s: %w", change.Path, err)
		}
		temp := file.Name()
		if _, err := file.Write(change.content); err != nil {
			file.Close()
			return fmt.Errorf("write staged %s: %w", change.Path, err)
		}
		if err := file.Chmod(change.mode); err != nil {
			file.Close()
			return fmt.Errorf("chmod staged %s: %w", change.Path, err)
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("close staged %s: %w", change.Path, err)
		}
		stagedFiles = append(stagedFiles, staged{change: change, temp: temp, backup: fmt.Sprintf("%s.gosvc-module-backup-%d", target, index)})
	}

	applied := 0
	for index := range stagedFiles {
		item := &stagedFiles[index]
		target := filepath.Join(plan.Root, filepath.FromSlash(item.change.Path))
		_ = os.Remove(item.backup)
		if err := os.Rename(target, item.backup); err != nil {
			rollback(plan.Root, stagedFiles, applied)
			return fmt.Errorf("backup %s: %w", item.change.Path, err)
		}
		if err := os.Rename(item.temp, target); err != nil {
			_ = os.Rename(item.backup, target)
			rollback(plan.Root, stagedFiles, applied)
			return fmt.Errorf("activate %s: %w", item.change.Path, err)
		}
		item.temp = ""
		applied++
	}
	for _, item := range stagedFiles {
		if err := os.Remove(item.backup); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove backup for %s: %w", item.change.Path, err)
		}
	}
	return nil
}

func rollback(root string, files []staged, applied int) {
	for index := applied - 1; index >= 0; index-- {
		item := files[index]
		target := filepath.Join(root, filepath.FromSlash(item.change.Path))
		_ = os.Remove(target)
		_ = os.Rename(item.backup, target)
	}
}

// staged is duplicated at package level so rollback can remain small and testable.
type staged struct {
	change Change
	temp   string
	backup string
}

func excludedDirectory(name string) bool {
	switch name {
	case ".git", "dist", "bin", "vendor", "node_modules", ".idea", ".vscode":
		return true
	default:
		return false
	}
}

func candidate(path, name string) bool {
	switch name {
	case "Makefile", "Dockerfile", ".gitignore", ".dockerignore":
		return true
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".mod", ".sum", ".md", ".yaml", ".yml", ".json", ".tmpl", ".sh", ".ps1", ".txt", ".toml", ".ini", ".xml":
		return true
	default:
		return false
	}
}

func readModule(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open go.mod: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	return "", fmt.Errorf("go.mod does not declare a module")
}
