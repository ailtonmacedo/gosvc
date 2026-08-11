package releasepack

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ailtonmacedo/gosvc/internal/acceptance"
	"github.com/ailtonmacedo/gosvc/internal/completion"
	"github.com/ailtonmacedo/gosvc/internal/releasecheck"
	"github.com/ailtonmacedo/gosvc/internal/repository"
)

type Options struct {
	Root             string
	Output           string
	Version          string
	Repository       string
	AllowPlaceholder bool
	Parallel         int
}

type Asset struct {
	Name   string `json:"name"`
	Target string `json:"target,omitempty"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type Result struct {
	OutputDir string
	Version   string
	Commit    string
	Assets    []Asset
}

type releaseEvidence struct {
	SchemaVersion int                `json:"schema_version"`
	Name          string             `json:"name"`
	Version       string             `json:"version"`
	Module        string             `json:"module"`
	Repository    string             `json:"repository"`
	Commit        string             `json:"commit"`
	BuiltAt       string             `json:"built_at"`
	Builder       string             `json:"builder"`
	Acceptance    acceptanceEvidence `json:"acceptance"`
	QualityGates  []string           `json:"quality_gates"`
	Reproducible  bool               `json:"reproducible"`
}

type acceptanceEvidence struct {
	Passed  int                    `json:"passed"`
	Failed  int                    `json:"failed"`
	Presets []presetEvidenceResult `json:"presets"`
}

type presetEvidenceResult struct {
	Preset            string   `json:"preset"`
	Status            string   `json:"status"`
	Files             int      `json:"files"`
	Resources         int      `json:"resources"`
	ArchitectureFiles int      `json:"architecture_files"`
	Checks            []string `json:"checks"`
}

type target struct {
	GOOS   string
	GOARCH string
}

var targets = []target{
	{GOOS: "linux", GOARCH: "amd64"},
	{GOOS: "linux", GOARCH: "arm64"},
	{GOOS: "darwin", GOARCH: "amd64"},
	{GOOS: "darwin", GOARCH: "arm64"},
	{GOOS: "windows", GOARCH: "amd64"},
	{GOOS: "windows", GOARCH: "arm64"},
}

func Build(options Options) (Result, error) {
	root := options.Root
	if root == "" {
		root = "."
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return Result{}, fmt.Errorf("resolve repository root: %w", err)
	}
	output := options.Output
	if output == "" {
		output = filepath.Join(absoluteRoot, "dist")
	} else if !filepath.IsAbs(output) {
		output = filepath.Join(absoluteRoot, output)
	}

	report, err := releasecheck.Check(releasecheck.Options{
		Root: absoluteRoot, Version: options.Version, AllowPlaceholder: options.AllowPlaceholder,
	})
	if err != nil {
		return Result{}, err
	}
	if err := report.Err(); err != nil {
		return Result{}, fmt.Errorf("release preflight failed: %w", err)
	}

	repo, err := resolveRepository(report.Module, options.Repository, options.AllowPlaceholder)
	if err != nil {
		return Result{}, err
	}

	if err := os.RemoveAll(output); err != nil {
		return Result{}, fmt.Errorf("clean output: %w", err)
	}
	if err := os.MkdirAll(output, 0o755); err != nil {
		return Result{}, fmt.Errorf("create output: %w", err)
	}

	fixedTime := releaseTime()
	commit := gitValue(absoluteRoot, "rev-parse", "HEAD")
	if commit == "" {
		commit = "unknown"
	}
	buildTime := fixedTime.UTC().Format(time.RFC3339)

	temporary, err := os.MkdirTemp("", "gosvc-release-*")
	if err != nil {
		return Result{}, fmt.Errorf("create build workspace: %w", err)
	}
	defer os.RemoveAll(temporary)

	completionFiles, err := writeCompletions(output, fixedTime)
	if err != nil {
		return Result{}, err
	}

	created, err := buildTargetArchives(absoluteRoot, temporary, output, report.Module, report.Version, commit, buildTime, fixedTime, completionFiles, options.Parallel)
	if err != nil {
		return Result{}, err
	}

	sbomPath := filepath.Join(output, fmt.Sprintf("gosvc_%s_sbom.spdx.json", report.Version))
	if err := writeSBOM(absoluteRoot, sbomPath, report.Module, report.Version, fixedTime); err != nil {
		return Result{}, err
	}
	created = append(created, sbomPath)

	releaseNotesPath := filepath.Join(output, "RELEASE_NOTES.md")
	if err := writeReleaseNotes(filepath.Join(absoluteRoot, "CHANGELOG.md"), releaseNotesPath, report.Version, fixedTime); err != nil {
		return Result{}, err
	}
	created = append(created, releaseNotesPath)

	evidencePath := filepath.Join(output, "release-evidence.json")
	if err := writeReleaseEvidence(evidencePath, releaseEvidence{
		SchemaVersion: 1,
		Name:          "gosvc",
		Version:       report.Version,
		Module:        report.Module,
		Repository:    repo.Slug(),
		Commit:        commit,
		BuiltAt:       buildTime,
		Builder:       runtime.Version(),
		Acceptance:    stableAcceptance(report.Acceptance),
		QualityGates: []string{
			"semantic-version", "repository-identity", "required-release-files",
			"shell-completion-generation", "preset-acceptance", "cross-platform-build",
			"sbom-generation",
		},
		Reproducible: true,
	}, fixedTime); err != nil {
		return Result{}, err
	}
	created = append(created, evidencePath)

	for _, source := range []string{"scripts/install.sh", "scripts/install.ps1"} {
		destination := filepath.Join(output, filepath.Base(source))
		if err := renderRepositoryFile(filepath.Join(absoluteRoot, source), destination, repo.Slug(), fixedTime, 0o755); err != nil {
			return Result{}, err
		}
		created = append(created, destination)
	}
	created = append(created, completionFiles...)

	homebrewPath := filepath.Join(output, "gosvc.rb")
	scoopPath := filepath.Join(output, "gosvc.json")
	if err := writePackageManagers(output, homebrewPath, scoopPath, repo, report.Version, fixedTime); err != nil {
		return Result{}, err
	}
	created = append(created, homebrewPath, scoopPath)

	assets, err := collectAssets(created)
	if err != nil {
		return Result{}, err
	}
	manifestPath := filepath.Join(output, "release-manifest.json")
	manifest := struct {
		SchemaVersion int     `json:"schema_version"`
		Name          string  `json:"name"`
		Version       string  `json:"version"`
		Module        string  `json:"module"`
		Repository    string  `json:"repository"`
		Commit        string  `json:"commit"`
		BuiltAt       string  `json:"built_at"`
		Builder       string  `json:"builder"`
		Assets        []Asset `json:"assets"`
	}{
		SchemaVersion: 2, Name: "gosvc", Version: report.Version, Module: report.Module,
		Repository: repo.Slug(), Commit: commit, BuiltAt: buildTime, Builder: runtime.Version(), Assets: assets,
	}
	if err := writeJSON(manifestPath, manifest, fixedTime); err != nil {
		return Result{}, err
	}
	created = append(created, manifestPath)

	checksumsPath := filepath.Join(output, "checksums.txt")
	if err := writeChecksums(checksumsPath, created, fixedTime); err != nil {
		return Result{}, err
	}
	created = append(created, checksumsPath)

	allAssets, err := collectAssets(created)
	if err != nil {
		return Result{}, err
	}
	return Result{OutputDir: output, Version: report.Version, Commit: commit, Assets: allAssets}, nil
}

func buildTargetArchives(root, temporary, output, module, version, commit, buildTime string, fixedTime time.Time, completionFiles []string, parallel int) ([]string, error) {
	if parallel <= 0 {
		parallel = runtime.GOMAXPROCS(0)
		if parallel > 3 {
			parallel = 3
		}
	}
	if parallel > len(targets) {
		parallel = len(targets)
	}
	if parallel < 1 {
		parallel = 1
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type result struct {
		path string
		err  error
	}
	jobs := make(chan target)
	results := make(chan result, len(targets))
	var workers sync.WaitGroup
	for index := 0; index < parallel; index++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for buildTarget := range jobs {
				path, err := buildTargetArchive(ctx, root, temporary, output, module, version, commit, buildTime, fixedTime, buildTarget, completionFiles)
				results <- result{path: path, err: err}
				if err != nil {
					cancel()
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, buildTarget := range targets {
			select {
			case jobs <- buildTarget:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		workers.Wait()
		close(results)
	}()

	created := make([]string, 0, len(targets))
	for item := range results {
		if item.err != nil {
			return nil, item.err
		}
		created = append(created, item.path)
	}
	if len(created) != len(targets) {
		return nil, fmt.Errorf("cross-platform build completed %d of %d targets", len(created), len(targets))
	}
	sort.Strings(created)
	return created, nil
}

func buildTargetArchive(ctx context.Context, root, temporary, output, module, version, commit, buildTime string, fixedTime time.Time, buildTarget target, completionFiles []string) (string, error) {
	base := fmt.Sprintf("gosvc_%s_%s_%s", version, buildTarget.GOOS, buildTarget.GOARCH)
	binaryName := "gosvc"
	if buildTarget.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(temporary, base, binaryName)
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		return "", err
	}
	ldflags := fmt.Sprintf("-s -w -X %s/internal/buildinfo.Version=%s -X %s/internal/buildinfo.Commit=%s -X %s/internal/buildinfo.BuildTime=%s", module, version, module, commit, module, buildTime)
	command := exec.CommandContext(ctx, "go", "build", "-trimpath", "-ldflags", ldflags, "-o", binaryPath, "./cmd/gosvc")
	command.Dir = root
	command.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+buildTarget.GOOS, "GOARCH="+buildTarget.GOARCH)
	outputBytes, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("build %s/%s: %w: %s", buildTarget.GOOS, buildTarget.GOARCH, err, strings.TrimSpace(string(outputBytes)))
	}
	if err := os.Chtimes(binaryPath, fixedTime, fixedTime); err != nil {
		return "", err
	}

	files := []archiveFile{{Source: binaryPath, Name: binaryName, Mode: 0o755}}
	for _, name := range []string{"LICENSE", "README.md", "CHANGELOG.md"} {
		files = append(files, archiveFile{Source: filepath.Join(root, name), Name: name, Mode: 0o644})
	}
	for _, completionPath := range completionFiles {
		files = append(files, archiveFile{Source: completionPath, Name: filepath.ToSlash(filepath.Join("completions", filepath.Base(completionPath))), Mode: 0o644})
	}

	var archivePath string
	if buildTarget.GOOS == "windows" {
		archivePath = filepath.Join(output, base+".zip")
		if err := writeZip(archivePath, base, files, fixedTime); err != nil {
			return "", err
		}
	} else {
		archivePath = filepath.Join(output, base+".tar.gz")
		if err := writeTarGz(archivePath, base, files, fixedTime); err != nil {
			return "", err
		}
	}
	return archivePath, nil
}

type archiveFile struct {
	Source string
	Name   string
	Mode   os.FileMode
}

func writeTarGz(destination, prefix string, files []archiveFile, fixedTime time.Time) error {
	output, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer output.Close()
	gzipWriter, err := gzip.NewWriterLevel(output, gzip.BestCompression)
	if err != nil {
		return err
	}
	gzipWriter.Header.ModTime = fixedTime
	gzipWriter.Header.OS = 255
	tarWriter := tar.NewWriter(gzipWriter)
	for _, file := range files {
		content, readErr := os.ReadFile(file.Source)
		if readErr != nil {
			return readErr
		}
		header := &tar.Header{
			Name: filepath.ToSlash(filepath.Join(prefix, file.Name)), Mode: int64(file.Mode.Perm()),
			Size: int64(len(content)), ModTime: fixedTime, AccessTime: fixedTime, ChangeTime: fixedTime,
			Uid: 0, Gid: 0, Uname: "root", Gname: "root",
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if _, err := tarWriter.Write(content); err != nil {
			return err
		}
	}
	if err := tarWriter.Close(); err != nil {
		return err
	}
	return gzipWriter.Close()
}

func writeZip(destination, prefix string, files []archiveFile, fixedTime time.Time) error {
	output, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer output.Close()
	zipWriter := zip.NewWriter(output)
	for _, file := range files {
		content, readErr := os.ReadFile(file.Source)
		if readErr != nil {
			return readErr
		}
		header := &zip.FileHeader{Name: filepath.ToSlash(filepath.Join(prefix, file.Name)), Method: zip.Deflate}
		header.SetMode(file.Mode)
		header.SetModTime(fixedTime)
		writer, createErr := zipWriter.CreateHeader(header)
		if createErr != nil {
			return createErr
		}
		if _, err := writer.Write(content); err != nil {
			return err
		}
	}
	return zipWriter.Close()
}

func writeCompletions(output string, fixedTime time.Time) ([]string, error) {
	definitions := []struct{ shell, name string }{
		{"bash", "gosvc.bash"}, {"zsh", "_gosvc"}, {"fish", "gosvc.fish"}, {"powershell", "gosvc.ps1"},
	}
	paths := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		content, err := completion.Generate(definition.shell)
		if err != nil {
			return nil, err
		}
		path := filepath.Join(output, definition.name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return nil, err
		}
		if err := os.Chtimes(path, fixedTime, fixedTime); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}

type moduleJSON struct {
	Path    string
	Version string
	Main    bool
	Replace *moduleJSON
}

func writeSBOM(root, destination, module, version string, fixedTime time.Time) error {
	command := exec.Command("go", "list", "-m", "-json", "all")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		return fmt.Errorf("list modules for SBOM: %w", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	var packages []map[string]any
	index := 0
	for {
		var item moduleJSON
		if err := decoder.Decode(&item); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("decode module list: %w", err)
		}
		resolved := item
		if item.Replace != nil {
			resolved = *item.Replace
		}
		packageVersion := resolved.Version
		if item.Main || packageVersion == "" {
			packageVersion = version
		}
		identifier := fmt.Sprintf("SPDXRef-Package-%d", index)
		index++
		packages = append(packages, map[string]any{
			"name": item.Path, "SPDXID": identifier, "versionInfo": packageVersion,
			"downloadLocation": "NOASSERTION", "filesAnalyzed": false,
			"licenseConcluded": "NOASSERTION", "licenseDeclared": "NOASSERTION",
			"copyrightText": "NOASSERTION",
			"externalRefs": []map[string]string{{
				"referenceCategory": "PACKAGE-MANAGER", "referenceType": "purl",
				"referenceLocator": "pkg:golang/" + item.Path + "@" + packageVersion,
			}},
		})
	}
	namespaceHash := sha256.Sum256([]byte(module + "@" + version))
	document := map[string]any{
		"spdxVersion": "SPDX-2.3", "dataLicense": "CC0-1.0", "SPDXID": "SPDXRef-DOCUMENT",
		"name":              "gosvc-" + version,
		"documentNamespace": fmt.Sprintf("https://spdx.org/spdxdocs/gosvc-%s-%s", version, hex.EncodeToString(namespaceHash[:8])),
		"creationInfo": map[string]any{
			"created":  fixedTime.UTC().Format(time.RFC3339),
			"creators": []string{"Tool: gosvc-release/" + version},
		},
		"packages": packages,
	}
	return writeJSON(destination, document, fixedTime)
}

func collectAssets(paths []string) ([]Asset, error) {
	assets := make([]Asset, 0, len(paths))
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256(content)
		asset := Asset{Name: filepath.Base(path), SHA256: hex.EncodeToString(digest[:]), Size: info.Size()}
		parts := strings.Split(strings.TrimSuffix(strings.TrimSuffix(asset.Name, ".tar.gz"), ".zip"), "_")
		if len(parts) >= 4 && parts[0] == "gosvc" {
			asset.Target = parts[len(parts)-2] + "/" + parts[len(parts)-1]
		}
		assets = append(assets, asset)
	}
	sort.Slice(assets, func(i, j int) bool { return assets[i].Name < assets[j].Name })
	return assets, nil
}

func writeChecksums(destination string, paths []string, fixedTime time.Time) error {
	var lines []string
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(content)
		lines = append(lines, fmt.Sprintf("%s  %s", hex.EncodeToString(digest[:]), filepath.Base(path)))
	}
	sort.Strings(lines)
	if err := os.WriteFile(destination, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		return err
	}
	return os.Chtimes(destination, fixedTime, fixedTime)
}

func writeJSON(path string, value any, fixedTime time.Time) error {
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return err
	}
	return os.Chtimes(path, fixedTime, fixedTime)
}

func writeReleaseNotes(changelogPath, destination, version string, fixedTime time.Time) error {
	content, err := os.ReadFile(changelogPath)
	if err != nil {
		return fmt.Errorf("read changelog for release notes: %w", err)
	}
	heading := "## [" + version + "]"
	lines := strings.Split(string(content), "\n")
	found := false
	sectionLines := make([]string, 0)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !found {
			if strings.HasPrefix(trimmed, heading) {
				found = true
			}
			continue
		}
		if strings.HasPrefix(trimmed, "## [") || (strings.HasPrefix(trimmed, "[") && strings.Contains(trimmed, "]:")) {
			break
		}
		sectionLines = append(sectionLines, line)
	}
	if !found {
		return fmt.Errorf("CHANGELOG.md has no section for %s", version)
	}
	section := strings.TrimSpace(strings.Join(sectionLines, "\n"))
	if section == "" {
		return fmt.Errorf("CHANGELOG.md section for %s is empty", version)
	}
	notes := fmt.Sprintf("# gosvc %s\n\n%s\n", version, section)
	if err := os.WriteFile(destination, []byte(notes), 0o644); err != nil {
		return err
	}
	return os.Chtimes(destination, fixedTime, fixedTime)
}

func stableAcceptance(report acceptance.Report) acceptanceEvidence {
	presets := make([]presetEvidenceResult, 0, len(report.Presets))
	for _, result := range report.Presets {
		checks := append([]string(nil), result.Checks...)
		sort.Strings(checks)
		presets = append(presets, presetEvidenceResult{
			Preset: result.Preset, Status: result.Status, Files: result.Files,
			Resources: result.Resources, ArchitectureFiles: result.ArchitectureFiles,
			Checks: checks,
		})
	}
	sort.Slice(presets, func(i, j int) bool { return presets[i].Preset < presets[j].Preset })
	return acceptanceEvidence{Passed: report.Passed, Failed: report.Failed, Presets: presets}
}

func writeReleaseEvidence(destination string, evidence releaseEvidence, fixedTime time.Time) error {
	if evidence.Acceptance.Failed != 0 || evidence.Acceptance.Passed == 0 {
		return fmt.Errorf("cannot write release evidence for a failed acceptance matrix")
	}
	return writeJSON(destination, evidence, fixedTime)
}

func copyFile(source, destination string, fixedTime time.Time, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		output.Close()
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	return os.Chtimes(destination, fixedTime, fixedTime)
}

func releaseTime() time.Time {
	if raw := os.Getenv("SOURCE_DATE_EPOCH"); raw != "" {
		seconds, err := strconv.ParseInt(raw, 10, 64)
		if err == nil {
			return time.Unix(seconds, 0).UTC()
		}
	}
	return time.Now().UTC().Truncate(time.Second)
}

func gitValue(root string, args ...string) string {
	command := exec.Command("git", args...)
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
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

func renderRepositoryFile(source, destination, repositorySlug string, fixedTime time.Time, mode os.FileMode) error {
	content, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	rendered := strings.ReplaceAll(string(content), "__GOSVC_REPOSITORY__", repositorySlug)
	if strings.Contains(rendered, "__GOSVC_REPOSITORY__") {
		return fmt.Errorf("repository placeholder remains in %s", source)
	}
	if err := os.WriteFile(destination, []byte(rendered), mode); err != nil {
		return err
	}
	return os.Chtimes(destination, fixedTime, fixedTime)
}

func writePackageManagers(output, homebrewPath, scoopPath string, repo repository.GitHub, version string, fixedTime time.Time) error {
	hashes := make(map[string]string)
	for _, target := range targets {
		extension := ".tar.gz"
		if target.GOOS == "windows" {
			extension = ".zip"
		}
		name := fmt.Sprintf("gosvc_%s_%s_%s%s", version, target.GOOS, target.GOARCH, extension)
		digest, err := fileSHA256(filepath.Join(output, name))
		if err != nil {
			return err
		}
		hashes[target.GOOS+"/"+target.GOARCH] = digest
	}
	baseURL := repo.URL() + "/releases/download/v" + version
	homebrew := fmt.Sprintf(`class Gosvc < Formula
  desc "Opinionated generator for production-oriented Go services"
  homepage %q
  version %q

  on_macos do
    if Hardware::CPU.arm?
      url %q
      sha256 %q
    else
      url %q
      sha256 %q
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url %q
      sha256 %q
    else
      url %q
      sha256 %q
    end
  end

  def install
    bin.install "gosvc"
    bash_completion.install "completions/gosvc.bash" => "gosvc"
    zsh_completion.install "completions/_gosvc" => "_gosvc"
    fish_completion.install "completions/gosvc.fish"
  end

  test do
    assert_match "gosvc version #{version}", shell_output("#{bin}/gosvc version")
  end
end
`, repo.URL(), version,
		baseURL+fmt.Sprintf("/gosvc_%s_darwin_arm64.tar.gz", version), hashes["darwin/arm64"],
		baseURL+fmt.Sprintf("/gosvc_%s_darwin_amd64.tar.gz", version), hashes["darwin/amd64"],
		baseURL+fmt.Sprintf("/gosvc_%s_linux_arm64.tar.gz", version), hashes["linux/arm64"],
		baseURL+fmt.Sprintf("/gosvc_%s_linux_amd64.tar.gz", version), hashes["linux/amd64"])
	if err := os.WriteFile(homebrewPath, []byte(homebrew), 0o644); err != nil {
		return err
	}
	if err := os.Chtimes(homebrewPath, fixedTime, fixedTime); err != nil {
		return err
	}

	scoop := map[string]any{
		"version":     version,
		"description": "Opinionated generator for production-oriented Go services",
		"homepage":    repo.URL(),
		"license":     "MIT",
		"architecture": map[string]any{
			"64bit": map[string]any{
				"url":         baseURL + fmt.Sprintf("/gosvc_%s_windows_amd64.zip", version),
				"hash":        hashes["windows/amd64"],
				"extract_dir": fmt.Sprintf("gosvc_%s_windows_amd64", version),
			},
			"arm64": map[string]any{
				"url":         baseURL + fmt.Sprintf("/gosvc_%s_windows_arm64.zip", version),
				"hash":        hashes["windows/arm64"],
				"extract_dir": fmt.Sprintf("gosvc_%s_windows_arm64", version),
			},
		},
		"bin": "gosvc.exe",
	}
	return writeJSON(scoopPath, scoop, fixedTime)
}

func fileSHA256(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:]), nil
}
