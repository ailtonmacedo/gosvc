package generator

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ailtonmacedo/gosvc/internal/manifest"
)

type Action string

const (
	ActionCreate  Action = "CREATE"
	ActionUpdate  Action = "UPDATE"
	ActionSkip    Action = "SKIP"
	ActionProtect Action = "PROTECT"
)

type Change struct {
	Artifact Artifact
	Action   Action
	Reason   string
}

type PlanOptions struct {
	Force bool
}

func Plan(destination string, artifacts []Artifact, existing *manifest.Manifest, options PlanOptions) ([]Change, error) {
	if err := ensureDestinationCanBeManaged(destination, existing); err != nil {
		return nil, err
	}
	changes := make([]Change, 0, len(artifacts))
	for _, artifact := range artifacts {
		if err := artifact.Validate(); err != nil {
			return nil, err
		}
		fullPath := filepath.Join(destination, filepath.FromSlash(artifact.Path))
		info, err := os.Lstat(fullPath)
		if errors.Is(err, fs.ErrNotExist) {
			changes = append(changes, Change{Artifact: artifact, Action: ActionCreate})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("inspect %q: %w", fullPath, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, conflictf("file conflict at %q: symbolic links are not managed", artifact.Path)
		}
		if !info.Mode().IsRegular() {
			return nil, conflictf("file conflict at %q: expected regular file", artifact.Path)
		}
		current, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", fullPath, err)
		}
		if manifest.Checksum(current) == manifest.Checksum(artifact.Content) {
			changes = append(changes, Change{Artifact: artifact, Action: ActionSkip, Reason: "content is unchanged"})
			continue
		}
		if artifact.Ownership == OwnershipUser {
			changes = append(changes, Change{Artifact: artifact, Action: ActionProtect, Reason: "user-owned file differs"})
			continue
		}
		if options.Force {
			changes = append(changes, Change{Artifact: artifact, Action: ActionUpdate, Reason: "forced overwrite"})
			continue
		}
		if existing == nil {
			return nil, conflictf("file conflict at %q: generated file differs and no prior manifest exists", artifact.Path)
		}
		record, ok := existing.File(artifact.Path)
		if !ok {
			return nil, conflictf("file conflict at %q: generated file is not tracked by the manifest", artifact.Path)
		}
		if manifest.Checksum(current) != record.Checksum {
			return nil, conflictf("file conflict at %q: generated file was modified; rerun with --force to overwrite", artifact.Path)
		}
		changes = append(changes, Change{Artifact: artifact, Action: ActionUpdate, Reason: "generator content changed"})
	}
	return changes, nil
}

func ensureDestinationCanBeManaged(destination string, existing *manifest.Manifest) error {
	info, err := os.Lstat(destination)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect destination %q: %w", destination, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return conflictf("destination %q must not be a symbolic link", destination)
	}
	if !info.IsDir() {
		return conflictf("destination %q is not a directory", destination)
	}
	entries, err := os.ReadDir(destination)
	if err != nil {
		return fmt.Errorf("read destination %q: %w", destination, err)
	}
	if len(entries) > 0 && existing == nil {
		return conflictf("destination %q is not empty and is not a gosvc project", destination)
	}
	return nil
}

func HasWrites(changes []Change) bool {
	for _, change := range changes {
		if change.Action == ActionCreate || change.Action == ActionUpdate {
			return true
		}
	}
	return false
}
