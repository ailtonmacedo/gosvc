package releaseverify

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type Options struct {
	Dist    string
	Execute bool
}

type Asset struct {
	Name   string `json:"name"`
	Target string `json:"target,omitempty"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type Manifest struct {
	SchemaVersion int     `json:"schema_version"`
	Name          string  `json:"name"`
	Version       string  `json:"version"`
	Module        string  `json:"module"`
	Repository    string  `json:"repository"`
	Commit        string  `json:"commit"`
	BuiltAt       string  `json:"built_at"`
	Builder       string  `json:"builder"`
	Assets        []Asset `json:"assets"`
}

type Report struct {
	Manifest Manifest
	Verified int
	Executed bool
}

type Evidence struct {
	SchemaVersion int                `json:"schema_version"`
	Name          string             `json:"name"`
	Version       string             `json:"version"`
	Module        string             `json:"module"`
	Repository    string             `json:"repository"`
	Commit        string             `json:"commit"`
	BuiltAt       string             `json:"built_at"`
	Builder       string             `json:"builder"`
	Acceptance    EvidenceAcceptance `json:"acceptance"`
	QualityGates  []string           `json:"quality_gates"`
	Reproducible  bool               `json:"reproducible"`
}

type EvidenceAcceptance struct {
	Passed  int                    `json:"passed"`
	Failed  int                    `json:"failed"`
	Presets []EvidencePresetResult `json:"presets"`
}

type EvidencePresetResult struct {
	Preset            string   `json:"preset"`
	Status            string   `json:"status"`
	Files             int      `json:"files"`
	Resources         int      `json:"resources"`
	ArchitectureFiles int      `json:"architecture_files"`
	Checks            []string `json:"checks"`
}

func Verify(options Options) (Report, error) {
	dist := options.Dist
	if dist == "" {
		dist = "dist"
	}
	absolute, err := filepath.Abs(dist)
	if err != nil {
		return Report{}, fmt.Errorf("resolve dist: %w", err)
	}
	manifest, err := loadManifest(filepath.Join(absolute, "release-manifest.json"))
	if err != nil {
		return Report{}, err
	}
	if manifest.SchemaVersion != 2 || manifest.Name != "gosvc" || manifest.Version == "" || manifest.Repository == "" {
		return Report{}, fmt.Errorf("release manifest is incomplete or uses an unsupported schema")
	}
	assets := make(map[string]Asset, len(manifest.Assets))
	for _, asset := range manifest.Assets {
		if _, exists := assets[asset.Name]; exists {
			return Report{}, fmt.Errorf("release manifest contains duplicate asset %s", asset.Name)
		}
		assets[asset.Name] = asset
		if err := verifyFile(filepath.Join(absolute, asset.Name), asset.SHA256, asset.Size); err != nil {
			return Report{}, fmt.Errorf("verify %s: %w", asset.Name, err)
		}
		if strings.HasSuffix(asset.Name, ".tar.gz") || strings.HasSuffix(asset.Name, ".zip") {
			if err := verifyArchive(filepath.Join(absolute, asset.Name), manifest.Version); err != nil {
				return Report{}, fmt.Errorf("verify archive %s: %w", asset.Name, err)
			}
		}
	}
	if err := verifyRequiredAssets(manifest.Version, assets); err != nil {
		return Report{}, err
	}
	if err := verifyChecksumFile(absolute); err != nil {
		return Report{}, err
	}
	if err := verifyInstallers(absolute, manifest.Repository); err != nil {
		return Report{}, err
	}
	if err := verifyPackageManagers(absolute, manifest, assets); err != nil {
		return Report{}, err
	}
	if err := verifyReleaseNotes(absolute, manifest.Version); err != nil {
		return Report{}, err
	}
	if err := verifyEvidence(absolute, manifest); err != nil {
		return Report{}, err
	}
	report := Report{Manifest: manifest, Verified: len(manifest.Assets)}
	if options.Execute {
		executed, err := executeHostBinary(absolute, manifest)
		if err != nil {
			return Report{}, err
		}
		report.Executed = executed
	}
	return report, nil
}

func verifyRequiredAssets(version string, assets map[string]Asset) error {
	required := []string{
		"RELEASE_NOTES.md", "release-evidence.json", "install.sh", "install.ps1",
		"gosvc.rb", "gosvc.json", "gosvc.bash", "_gosvc", "gosvc.fish", "gosvc.ps1",
		fmt.Sprintf("gosvc_%s_sbom.spdx.json", version),
	}
	for _, target := range []string{
		"linux_amd64.tar.gz", "linux_arm64.tar.gz", "darwin_amd64.tar.gz",
		"darwin_arm64.tar.gz", "windows_amd64.zip", "windows_arm64.zip",
	} {
		required = append(required, fmt.Sprintf("gosvc_%s_%s", version, target))
	}
	for _, name := range required {
		if _, exists := assets[name]; !exists {
			return fmt.Errorf("release manifest is missing required asset %s", name)
		}
	}
	return nil
}

func loadManifest(path string) (Manifest, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read release manifest: %w", err)
	}
	var value Manifest
	decoder := json.NewDecoder(strings.NewReader(string(content)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return Manifest{}, fmt.Errorf("decode release manifest: %w", err)
	}
	return value, nil
}

func verifyFile(path, expected string, expectedSize int64) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if int64(len(content)) != expectedSize {
		return fmt.Errorf("size mismatch: got %d want %d", len(content), expectedSize)
	}
	digest := sha256.Sum256(content)
	actual := hex.EncodeToString(digest[:])
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("checksum mismatch: got %s want %s", actual, expected)
	}
	return nil
}

func verifyChecksumFile(dist string) error {
	file, err := os.Open(filepath.Join(dist, "checksums.txt"))
	if err != nil {
		return fmt.Errorf("open checksums.txt: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			return fmt.Errorf("invalid checksums.txt line %q", scanner.Text())
		}
		path := filepath.Join(dist, filepath.Base(fields[1]))
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read checksummed asset %s: %w", fields[1], err)
		}
		digest := sha256.Sum256(content)
		if !strings.EqualFold(hex.EncodeToString(digest[:]), fields[0]) {
			return fmt.Errorf("checksums.txt mismatch for %s", fields[1])
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("checksums.txt is empty")
	}
	return nil
}

func verifyArchive(path, version string) error {
	base := filepath.Base(path)
	prefix := strings.TrimSuffix(strings.TrimSuffix(base, ".tar.gz"), ".zip") + "/"
	binary := "gosvc"
	if strings.Contains(base, "_windows_") {
		binary = "gosvc.exe"
	}
	expected := prefix + binary
	if strings.HasSuffix(path, ".zip") {
		reader, err := zip.OpenReader(path)
		if err != nil {
			return err
		}
		defer reader.Close()
		for _, file := range reader.File {
			if file.Name == expected {
				return nil
			}
		}
		return fmt.Errorf("binary %s not found", expected)
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if header.Name == expected {
			return nil
		}
	}
	return fmt.Errorf("binary %s not found for version %s", expected, version)
}

func verifyInstallers(dist, repository string) error {
	for _, name := range []string{"install.sh", "install.ps1"} {
		content, err := os.ReadFile(filepath.Join(dist, name))
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		text := string(content)
		if strings.Contains(text, "__GOSVC_REPOSITORY__") || !strings.Contains(text, repository) {
			return fmt.Errorf("%s was not rendered for repository %s", name, repository)
		}
	}
	return nil
}

func verifyPackageManagers(dist string, manifest Manifest, assets map[string]Asset) error {
	formula, err := os.ReadFile(filepath.Join(dist, "gosvc.rb"))
	if err != nil {
		return fmt.Errorf("read Homebrew formula: %w", err)
	}
	formulaText := string(formula)
	if !strings.Contains(formulaText, manifest.Repository) || !strings.Contains(formulaText, manifest.Version) {
		return fmt.Errorf("Homebrew formula does not match release identity")
	}
	for _, target := range []string{"darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64"} {
		asset, ok := findTarget(assets, target)
		if !ok || !strings.Contains(formulaText, asset.SHA256) {
			return fmt.Errorf("Homebrew formula is missing hash for %s", target)
		}
	}
	content, err := os.ReadFile(filepath.Join(dist, "gosvc.json"))
	if err != nil {
		return fmt.Errorf("read Scoop manifest: %w", err)
	}
	var scoop map[string]any
	if err := json.Unmarshal(content, &scoop); err != nil {
		return fmt.Errorf("decode Scoop manifest: %w", err)
	}
	if scoop["version"] != manifest.Version || scoop["homepage"] != "https://github.com/"+manifest.Repository {
		return fmt.Errorf("Scoop manifest does not match release identity")
	}
	return nil
}

func verifyReleaseNotes(dist, version string) error {
	content, err := os.ReadFile(filepath.Join(dist, "RELEASE_NOTES.md"))
	if err != nil {
		return fmt.Errorf("read release notes: %w", err)
	}
	text := strings.TrimSpace(string(content))
	if !strings.HasPrefix(text, "# gosvc "+version) {
		return fmt.Errorf("release notes do not match version %s", version)
	}
	if len(strings.Split(text, "\n")) < 3 {
		return fmt.Errorf("release notes are incomplete")
	}
	return nil
}

func verifyEvidence(dist string, manifest Manifest) error {
	content, err := os.ReadFile(filepath.Join(dist, "release-evidence.json"))
	if err != nil {
		return fmt.Errorf("read release evidence: %w", err)
	}
	var evidence Evidence
	decoder := json.NewDecoder(strings.NewReader(string(content)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&evidence); err != nil {
		return fmt.Errorf("decode release evidence: %w", err)
	}
	if evidence.SchemaVersion != 1 || evidence.Name != manifest.Name || evidence.Version != manifest.Version ||
		evidence.Module != manifest.Module || evidence.Repository != manifest.Repository || evidence.Commit != manifest.Commit ||
		evidence.BuiltAt != manifest.BuiltAt || evidence.Builder != manifest.Builder {
		return fmt.Errorf("release evidence does not match release manifest identity")
	}
	if !evidence.Reproducible || evidence.Acceptance.Failed != 0 || evidence.Acceptance.Passed == 0 || len(evidence.Acceptance.Presets) == 0 {
		return fmt.Errorf("release evidence does not contain a successful reproducible acceptance result")
	}
	for _, preset := range evidence.Acceptance.Presets {
		if preset.Preset == "" || preset.Status != "pass" || len(preset.Checks) == 0 {
			return fmt.Errorf("release evidence contains an invalid preset result")
		}
	}
	if len(evidence.QualityGates) == 0 {
		return fmt.Errorf("release evidence has no quality gates")
	}
	return nil
}

func findTarget(assets map[string]Asset, target string) (Asset, bool) {
	for _, asset := range assets {
		if asset.Target == target {
			return asset, true
		}
	}
	return Asset{}, false
}

func executeHostBinary(dist string, manifest Manifest) (bool, error) {
	extension := ".tar.gz"
	if runtime.GOOS == "windows" {
		extension = ".zip"
	}
	name := fmt.Sprintf("gosvc_%s_%s_%s%s", manifest.Version, runtime.GOOS, runtime.GOARCH, extension)
	path := filepath.Join(dist, name)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	temporary, err := os.MkdirTemp("", "gosvc-release-verify-*")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(temporary)
	binary, err := extractBinary(path, temporary)
	if err != nil {
		return false, err
	}
	command := exec.Command(binary, "version")
	output, err := command.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("execute released binary: %w: %s", err, strings.TrimSpace(string(output)))
	}
	expected := "gosvc version " + manifest.Version
	if !strings.Contains(string(output), expected) {
		return false, fmt.Errorf("released binary output %q does not contain %q", strings.TrimSpace(string(output)), expected)
	}
	return true, nil
}

func extractBinary(archivePath, destination string) (string, error) {
	binaryName := "gosvc"
	if strings.HasSuffix(archivePath, ".zip") {
		binaryName = "gosvc.exe"
		reader, err := zip.OpenReader(archivePath)
		if err != nil {
			return "", err
		}
		defer reader.Close()
		for _, file := range reader.File {
			if filepath.Base(file.Name) != binaryName {
				continue
			}
			input, err := file.Open()
			if err != nil {
				return "", err
			}
			defer input.Close()
			return writeExtractedBinary(input, filepath.Join(destination, binaryName))
		}
		return "", fmt.Errorf("binary not found in %s", archivePath)
	}
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if filepath.Base(header.Name) == binaryName {
			return writeExtractedBinary(tarReader, filepath.Join(destination, binaryName))
		}
	}
	return "", fmt.Errorf("binary not found in %s", archivePath)
}

func writeExtractedBinary(input io.Reader, path string) (string, error) {
	output, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(output, input); err != nil {
		output.Close()
		return "", err
	}
	if err := output.Close(); err != nil {
		return "", err
	}
	return path, nil
}
