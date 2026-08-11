package releasecheck

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ailtonmacedo/gosvc/internal/acceptance"
	"github.com/ailtonmacedo/gosvc/internal/completion"
	"github.com/ailtonmacedo/gosvc/internal/githubpublish"
	"github.com/ailtonmacedo/gosvc/internal/repository"
	"github.com/ailtonmacedo/gosvc/internal/version"
)

type Options struct {
	Root             string
	Version          string
	Repository       string
	AllowPlaceholder bool
}

type Report struct {
	Module     string
	Repository string
	Version    string
	Issues     []string
	Acceptance acceptance.Report
}

func (r Report) Err() error {
	if len(r.Issues) == 0 {
		return nil
	}
	return errors.New(strings.Join(r.Issues, "; "))
}

func Check(options Options) (Report, error) {
	root := options.Root
	if root == "" {
		root = "."
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return Report{}, fmt.Errorf("resolve repository root: %w", err)
	}

	parsedVersion, err := version.Parse(options.Version)
	if err != nil || parsedVersion.Development {
		if err == nil {
			err = fmt.Errorf("release version cannot be %q", options.Version)
		}
		return Report{}, err
	}

	report := Report{Version: parsedVersion.String()}
	module, err := readModule(filepath.Join(absolute, "go.mod"))
	if err != nil {
		return Report{}, err
	}
	report.Module = module
	placeholder := strings.HasPrefix(module, "github.com/example/")
	if placeholder && !options.AllowPlaceholder {
		report.Issues = append(report.Issues, "go.mod still uses placeholder module github.com/example; run 'gosvc release prepare --repository owner/name'")
	}
	repo, repoErr := resolveRepository(module, options.Repository, options.AllowPlaceholder)
	if repoErr != nil {
		report.Issues = append(report.Issues, repoErr.Error())
	} else {
		report.Repository = repo.Slug()
		if repo.Module() == module {
			publicationPlan, planErr := githubpublish.Build(githubpublish.Options{Root: absolute, Repository: repo.Slug(), Version: report.Version})
			if planErr != nil {
				report.Issues = append(report.Issues, "GitHub publication plan failed: "+planErr.Error())
			} else {
				for _, failure := range githubpublish.SortedFailures(publicationPlan) {
					report.Issues = append(report.Issues, "GitHub publication: "+failure)
				}
			}
		}
		if origin := gitOrigin(absolute); origin != "" {
			if originRepo, err := repository.Parse(origin); err == nil && originRepo.Slug() != repo.Slug() {
				report.Issues = append(report.Issues, fmt.Sprintf("git origin %s does not match release repository %s", originRepo.Slug(), repo.Slug()))
			}
		}
	}

	requiredFiles := []string{
		"README.md", "LICENSE", "CHANGELOG.md", "CONTRIBUTING.md", "SECURITY.md",
		"CODE_OF_CONDUCT.md", "docs/INSTALLATION.md", "docs/RELEASES.md",
		"docs/ACCEPTANCE.md", "docs/CERTIFICATION.md", "docs/COMPATIBILITY.md",
		"docs/RELEASE_EVIDENCE.md", "docs/GITHUB_PUBLISHING.md",
		"schema/manifest.schema.json", "schema/plugin.schema.json", "schema/compatibility-matrix.json",
		"schema/acceptance-report.schema.json", "schema/certification-report.schema.json", "schema/release-evidence.schema.json",
		"schema/github-publication-plan.schema.json",
	}
	for _, name := range requiredFiles {
		info, statErr := os.Stat(filepath.Join(absolute, filepath.FromSlash(name)))
		if statErr != nil {
			report.Issues = append(report.Issues, fmt.Sprintf("required release file %s is missing", name))
			continue
		}
		if info.IsDir() {
			report.Issues = append(report.Issues, fmt.Sprintf("required release file %s is a directory", name))
		}
	}

	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		content, completionErr := completion.Generate(shell)
		if completionErr != nil || strings.TrimSpace(content) == "" {
			report.Issues = append(report.Issues, fmt.Sprintf("cannot generate %s completion", shell))
		}
	}

	readme, readErr := os.ReadFile(filepath.Join(absolute, "README.md"))
	if readErr == nil && !strings.Contains(string(readme), "release-evidence-ready") {
		report.Issues = append(report.Issues, "README.md does not identify the release-evidence-ready state")
	}
	changelog, changeErr := os.ReadFile(filepath.Join(absolute, "CHANGELOG.md"))
	if changeErr == nil && !strings.Contains(string(changelog), "["+parsedVersion.String()+"]") {
		report.Issues = append(report.Issues, fmt.Sprintf("CHANGELOG.md has no [%s] section", parsedVersion.String()))
	}

	acceptanceReport, acceptanceErr := acceptance.Run(acceptance.Options{FrameworkVersion: parsedVersion.String()})
	report.Acceptance = acceptanceReport
	if acceptanceErr != nil {
		added := false
		for _, presetResult := range acceptanceReport.Presets {
			if presetResult.Status != "fail" {
				continue
			}
			added = true
			report.Issues = append(report.Issues, fmt.Sprintf("preset %s acceptance failed: %s", presetResult.Preset, presetResult.Error))
		}
		if !added {
			report.Issues = append(report.Issues, "built-in preset acceptance matrix failed: "+acceptanceErr.Error())
		}
	}

	return report, nil
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

func resolveRepository(module, override string, allowPlaceholder bool) (repository.GitHub, error) {
	if override != "" {
		repo, err := repository.Parse(override)
		if err != nil {
			return repository.GitHub{}, err
		}
		if !allowPlaceholder && repo.Module() != module {
			return repository.GitHub{}, fmt.Errorf("repository %s does not match module %s", repo.Slug(), module)
		}
		return repo, nil
	}
	return repository.FromModule(module)
}

func gitOrigin(root string) string {
	command := exec.Command("git", "config", "--get", "remote.origin.url")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		return ""
	}
	value := strings.TrimSpace(string(output))
	value = strings.TrimPrefix(value, "git@github.com:")
	value = strings.TrimPrefix(value, "ssh://git@github.com/")
	return value
}
