package githubpublish

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ailtonmacedo/gosvc/internal/repository"
	"github.com/ailtonmacedo/gosvc/internal/version"
)

const SchemaVersion = 1

type Status string

const (
	StatusPass Status = "pass"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
)

type Options struct {
	Root       string
	Repository string
	Version    string
}

type Check struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
	Detail string `json:"detail"`
}

type Step struct {
	Order       int    `json:"order"`
	Description string `json:"description"`
	Command     string `json:"command"`
}

type Plan struct {
	SchemaVersion int     `json:"schema_version"`
	Repository    string  `json:"repository"`
	Module        string  `json:"module"`
	Version       string  `json:"version"`
	GitOrigin     string  `json:"git_origin,omitempty"`
	CurrentBranch string  `json:"current_branch,omitempty"`
	Checks        []Check `json:"checks"`
	Steps         []Step  `json:"steps"`
	Passed        int     `json:"passed"`
	Warnings      int     `json:"warnings"`
	Failed        int     `json:"failed"`
}

func (p Plan) Ready() bool { return p.Failed == 0 }

func (p Plan) EncodeJSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

func Build(options Options) (Plan, error) {
	root := options.Root
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return Plan{}, fmt.Errorf("resolve repository root: %w", err)
	}
	repo, err := repository.Parse(options.Repository)
	if err != nil {
		return Plan{}, err
	}
	parsedVersion, err := version.Parse(options.Version)
	if err != nil || parsedVersion.Development {
		if err == nil {
			err = fmt.Errorf("publication version cannot be %q", options.Version)
		}
		return Plan{}, err
	}
	module, err := readModule(filepath.Join(absolute, "go.mod"))
	if err != nil {
		return Plan{}, err
	}

	plan := Plan{
		SchemaVersion: SchemaVersion,
		Repository:    repo.Slug(),
		Module:        module,
		Version:       parsedVersion.String(),
	}
	plan.addCheck("module-path", module == repo.Module(), StatusFail,
		fmt.Sprintf("module=%s expected=%s", module, repo.Module()))

	gitDir := filepath.Join(absolute, ".git")
	_, gitErr := os.Stat(gitDir)
	gitRepo := gitErr == nil
	if gitRepo {
		plan.GitOrigin = gitOrigin(absolute)
		plan.CurrentBranch = gitBranch(absolute)
		if plan.GitOrigin == "" {
			plan.add(StatusWarn, "git-origin", "git repository has no origin remote")
		} else if originRepo, err := repository.Parse(plan.GitOrigin); err != nil {
			plan.add(StatusFail, "git-origin", "cannot parse origin remote: "+plan.GitOrigin)
		} else if originRepo.Slug() != repo.Slug() {
			plan.add(StatusFail, "git-origin", fmt.Sprintf("origin=%s expected=%s", originRepo.Slug(), repo.Slug()))
		} else {
			plan.add(StatusPass, "git-origin", "origin matches publication repository")
		}
		if plan.CurrentBranch == "" {
			plan.add(StatusWarn, "git-branch", "current branch could not be determined")
		} else if plan.CurrentBranch != "main" {
			plan.add(StatusWarn, "git-branch", "current branch is "+plan.CurrentBranch+"; publication plan targets main")
		} else {
			plan.add(StatusPass, "git-branch", "current branch is main")
		}
	} else {
		plan.add(StatusWarn, "git-repository", "directory is not initialized as a Git repository")
	}

	workflowChecks := []struct {
		name     string
		path     string
		required []string
	}{
		{"ci-workflow", ".github/workflows/ci.yml", []string{"pull_request", "go test"}},
		{"acceptance-workflow", ".github/workflows/acceptance.yml", []string{"acceptance"}},
		{"certification-workflow", ".github/workflows/certification.yml", []string{"certify", "--require-real", "workflow_dispatch"}},
		{"release-workflow", ".github/workflows/release.yml", []string{"tags:", "v*.*.*", "release check", "certify --mode real", "gh release create"}},
	}
	for _, item := range workflowChecks {
		validateWorkflow(&plan, absolute, item.name, item.path, item.required)
	}

	changelogPath := filepath.Join(absolute, "CHANGELOG.md")
	if body, err := os.ReadFile(changelogPath); err != nil {
		plan.add(StatusFail, "changelog", "CHANGELOG.md is missing")
	} else if !strings.Contains(string(body), "## ["+plan.Version+"]") {
		plan.add(StatusFail, "changelog", "CHANGELOG.md has no ["+plan.Version+"] section")
	} else {
		plan.add(StatusPass, "changelog", "release version is documented")
	}

	securityFiles := []string{"SECURITY.md", "CODE_OF_CONDUCT.md", "CONTRIBUTING.md", "LICENSE"}
	missing := make([]string, 0)
	for _, name := range securityFiles {
		if info, err := os.Stat(filepath.Join(absolute, name)); err != nil || info.IsDir() {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		plan.add(StatusPass, "repository-governance", "security and contribution documents are present")
	} else {
		plan.add(StatusFail, "repository-governance", "missing: "+strings.Join(missing, ", "))
	}

	plan.Steps = publicationSteps(repo, plan.Version, gitRepo, plan.GitOrigin)
	return plan, nil
}

func (p *Plan) addCheck(name string, ok bool, failStatus Status, detail string) {
	if ok {
		p.add(StatusPass, name, detail)
		return
	}
	p.add(failStatus, name, detail)
}

func (p *Plan) add(status Status, name, detail string) {
	p.Checks = append(p.Checks, Check{Name: name, Status: status, Detail: detail})
	switch status {
	case StatusPass:
		p.Passed++
	case StatusWarn:
		p.Warnings++
	case StatusFail:
		p.Failed++
	}
}

func validateWorkflow(plan *Plan, root, name, relative string, required []string) {
	path := filepath.Join(root, filepath.FromSlash(relative))
	body, err := os.ReadFile(path)
	if err != nil {
		plan.add(StatusFail, name, relative+" is missing")
		return
	}
	text := string(body)
	missing := make([]string, 0)
	for _, needle := range required {
		if !strings.Contains(text, needle) {
			missing = append(missing, needle)
		}
	}
	if len(missing) > 0 {
		plan.add(StatusFail, name, "missing required markers: "+strings.Join(missing, ", "))
		return
	}
	plan.add(StatusPass, name, relative+" contains required publication gates")
}

func publicationSteps(repo repository.GitHub, version string, gitRepo bool, origin string) []Step {
	commands := make([]Step, 0, 10)
	add := func(description, command string) {
		commands = append(commands, Step{Order: len(commands) + 1, Description: description, Command: command})
	}
	if !gitRepo {
		add("Initialize Git repository", "git init")
		add("Use main as the default branch", "git branch -M main")
	}
	remote := repo.URL() + ".git"
	if origin == "" {
		add("Configure GitHub origin", "git remote add origin "+remote)
	} else {
		add("Align GitHub origin", "git remote set-url origin "+remote)
	}
	add("Review repository changes", "git status --short")
	add("Stage the release-ready source", "git add .")
	add("Create the publication commit", fmt.Sprintf("git commit -m \"chore: prepare gosvc v%s\"", version))
	add("Push main", "git push -u origin main")
	add("Run real certification in GitHub Actions", "gh workflow run certification.yml --ref main")
	add("Create the annotated release tag after CI is green", fmt.Sprintf("git tag -a v%s -m \"gosvc v%s\"", version, version))
	add("Push the release tag", fmt.Sprintf("git push origin v%s", version))
	return commands
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

func gitOrigin(root string) string {
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	cmd.Dir = root
	raw, err := cmd.Output()
	if err != nil {
		return ""
	}
	value := strings.TrimSpace(string(raw))
	value = strings.TrimPrefix(value, "git@github.com:")
	value = strings.TrimPrefix(value, "ssh://git@github.com/")
	return value
}

func gitBranch(root string) string {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = root
	raw, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func SortedFailures(plan Plan) []string {
	out := make([]string, 0)
	for _, check := range plan.Checks {
		if check.Status == StatusFail {
			out = append(out, check.Name+": "+check.Detail)
		}
	}
	sort.Strings(out)
	return out
}
