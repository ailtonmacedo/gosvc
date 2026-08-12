package architecture

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Violation struct {
	File       string
	ImportPath string
	Rule       string
}

func (v Violation) Error() string {
	return fmt.Sprintf("%s: import %q violates %s", v.File, v.ImportPath, v.Rule)
}

type Report struct {
	FilesChecked int
	Violations   []Violation
}

func (r Report) Err() error {
	if len(r.Violations) == 0 {
		return nil
	}
	errs := make([]error, 0, len(r.Violations))
	for _, violation := range r.Violations {
		errs = append(errs, violation)
	}
	return ViolationsError(errs)
}

type ViolationsError []error

func (e ViolationsError) Error() string {
	messages := make([]string, 0, len(e))
	for _, err := range e {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "\n")
}

func (e ViolationsError) Unwrap() []error { return []error(e) }

func Check(projectDir, module string) (Report, error) {
	if projectDir == "" {
		projectDir = "."
	}
	internalDir := filepath.Join(projectDir, "internal")
	var report Report
	err := filepath.WalkDir(internalDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "vendor" || strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		relative, err := filepath.Rel(projectDir, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		layer := layerFor(relative)
		if layer == "" {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("parse %s: %w", relative, err)
		}
		report.FilesChecked++
		for _, item := range file.Imports {
			value, err := strconv.Unquote(item.Path.Value)
			if err != nil {
				return fmt.Errorf("parse import in %s: %w", relative, err)
			}
			if rule := forbiddenRule(layer, value, module); rule != "" {
				report.Violations = append(report.Violations, Violation{File: relative, ImportPath: value, Rule: rule})
			}
		}
		return nil
	})
	if err != nil {
		return Report{}, fmt.Errorf("check architecture: %w", err)
	}
	sort.Slice(report.Violations, func(i, j int) bool {
		if report.Violations[i].File == report.Violations[j].File {
			return report.Violations[i].ImportPath < report.Violations[j].ImportPath
		}
		return report.Violations[i].File < report.Violations[j].File
	})
	return report, nil
}

func layerFor(relative string) string {
	switch {
	case strings.HasPrefix(relative, "internal/domain/"):
		return "domain"
	case strings.HasPrefix(relative, "internal/application/"):
		return "application"
	default:
		return ""
	}
}

func forbiddenRule(layer, importPath, module string) string {
	internalApplication := module + "/internal/application"
	internalInfrastructure := module + "/internal/infrastructure"
	technicalPrefixes := []string{
		"github.com/go-chi/chi",
		"github.com/jackc/pgx",
		"github.com/golang-jwt/jwt",
		"go.opentelemetry.io/otel",
		"github.com/prometheus/client_golang",
	}
	if layer == "domain" {
		if importPath == internalApplication || strings.HasPrefix(importPath, internalApplication+"/") {
			return "domain independence: domain cannot import application"
		}
		if importPath == internalInfrastructure || strings.HasPrefix(importPath, internalInfrastructure+"/") {
			return "domain independence: domain cannot import infrastructure"
		}
		for _, prefix := range technicalPrefixes {
			if importPath == prefix || strings.HasPrefix(importPath, prefix+"/") {
				return "domain independence: domain cannot import technical adapters"
			}
		}
	}
	if layer == "application" {
		if importPath == internalInfrastructure || strings.HasPrefix(importPath, internalInfrastructure+"/") {
			return "application boundary: application cannot import infrastructure"
		}
		for _, prefix := range []string{"github.com/go-chi/chi", "github.com/jackc/pgx"} {
			if importPath == prefix || strings.HasPrefix(importPath, prefix+"/") {
				return "application boundary: application cannot import HTTP or database adapters"
			}
		}
	}
	return ""
}
