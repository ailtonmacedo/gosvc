package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const gosvcSourceModule = "github.com/ailtonmacedo/gosvc"

// resolveNewDestination keeps the historical ./<project-name> default for
// normal use, but when gosvc is executed from its own source tree it places
// generated projects beside that repository. This prevents the generated
// project's Git state from being accidentally inherited from gosvc.
func resolveNewDestination(projectName, explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return explicit, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get current directory: %w", err)
	}
	return defaultNewDestination(cwd, projectName)
}

func defaultNewDestination(cwd, projectName string) (string, error) {
	root, found, err := findGoSvcSourceRoot(cwd)
	if err != nil {
		return "", err
	}
	if !found {
		return projectName, nil
	}

	target := filepath.Join(filepath.Dir(root), projectName)
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve gosvc source root: %w", err)
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolve destination: %w", err)
	}
	if filepath.Clean(rootAbs) == filepath.Clean(targetAbs) {
		return "", fmt.Errorf("default destination would overwrite the gosvc source repository; choose another project name or use --output")
	}

	relative, err := filepath.Rel(cwd, targetAbs)
	if err != nil {
		return targetAbs, nil
	}
	return relative, nil
}

// legacyNestedDestination reports an older project directory that still exists
// inside the gosvc source checkout while the new default destination points
// outside it. Surfacing this prevents users from accidentally `cd`-ing into a
// stale scaffold after the safe-destination behavior was introduced.
func legacyNestedDestination(projectName, resolved string) (string, bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	root, found, err := findGoSvcSourceRoot(cwd)
	if err != nil || !found {
		return "", false
	}
	legacy := filepath.Join(root, projectName)
	legacyInfo, err := os.Stat(legacy)
	if err != nil || !legacyInfo.IsDir() {
		return "", false
	}
	resolvedAbs, err := filepath.Abs(resolved)
	if err != nil {
		return "", false
	}
	legacyAbs, err := filepath.Abs(legacy)
	if err != nil || filepath.Clean(resolvedAbs) == filepath.Clean(legacyAbs) {
		return "", false
	}
	relative, err := filepath.Rel(cwd, legacyAbs)
	if err != nil {
		return legacyAbs, true
	}
	return relative, true
}

func findGoSvcSourceRoot(start string) (string, bool, error) {
	current, err := filepath.Abs(start)
	if err != nil {
		return "", false, fmt.Errorf("resolve current directory: %w", err)
	}
	for {
		modulePath, ok, err := moduleFromGoMod(filepath.Join(current, "go.mod"))
		if err != nil {
			return "", false, err
		}
		if ok && modulePath == gosvcSourceModule {
			return current, true, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false, nil
		}
		current = parent
	}
}

func moduleFromGoMod(path string) (string, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			module := strings.TrimSpace(strings.TrimPrefix(line, "module "))
			return module, module != "", nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", false, fmt.Errorf("read %s: %w", path, err)
	}
	return "", false, nil
}
