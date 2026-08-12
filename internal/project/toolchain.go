package project

import (
	"strconv"
	"strings"
)

const go125SecurityFloor = "1.25.12"

// PreferredToolchain returns an exact Go toolchain suggestion when gosvc needs
// to enforce a patched toolchain independently from the module language level.
// Go 1.25 projects use 1.25.12 as the security floor because vulnerabilities
// reachable from generated services are fixed across patch releases up to it.
func PreferredToolchain(goVersion string) string {
	version := normalizeGoVersion(goVersion)
	if version == "" {
		return ""
	}
	major, minor, patch, hasPatch, ok := parseGoVersion(version)
	if !ok {
		return ""
	}
	if major == 1 && minor == 25 {
		if !hasPatch || patch < 12 {
			return "go" + go125SecurityFloor
		}
		return "go" + version
	}
	if hasPatch {
		return "go" + version
	}
	return ""
}

// BuildGoVersion returns the Go version that should be used by concrete build
// environments such as Docker. It may be newer than the module's go directive
// when a security patch floor applies.
func BuildGoVersion(goVersion string) string {
	version := normalizeGoVersion(goVersion)
	if version == "" {
		return goVersion
	}
	major, minor, patch, hasPatch, ok := parseGoVersion(version)
	if !ok {
		return version
	}
	if major == 1 && minor == 25 && (!hasPatch || patch < 12) {
		return go125SecurityFloor
	}
	return version
}

// RequiredRuntimeGoVersion is the minimum host toolchain version required by
// doctor and real certification. It includes the security floor used for
// generated builds.
func RequiredRuntimeGoVersion(goVersion string) string {
	version := normalizeGoVersion(goVersion)
	if version == "" || version == "auto" {
		return ""
	}
	return BuildGoVersion(version)
}

func normalizeGoVersion(value string) string {
	value = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(value), "go"))
	if value == "" || value == "auto" {
		return value
	}
	return value
}

func parseGoVersion(value string) (major, minor, patch int, hasPatch, ok bool) {
	parts := strings.Split(value, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, 0, 0, false, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, false, false
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, false, false
	}
	if len(parts) == 3 {
		patch, err = strconv.Atoi(parts[2])
		if err != nil {
			return 0, 0, 0, false, false
		}
		hasPatch = true
	}
	return major, minor, patch, hasPatch, true
}

// RequiredRuntimeGoVersionWithToolchain returns the effective minimum host Go
// version after considering both the language contract and an explicit
// toolchain directive.
func RequiredRuntimeGoVersionWithToolchain(goVersion, toolchain string) string {
	base := RequiredRuntimeGoVersion(goVersion)
	tool := normalizeGoVersion(toolchain)
	if tool == "" || tool == "auto" {
		return base
	}
	if base == "" || compareGoVersions(tool, base) > 0 {
		return tool
	}
	return base
}

func compareGoVersions(left, right string) int {
	lMajor, lMinor, lPatch, _, lOK := parseGoVersion(normalizeGoVersion(left))
	rMajor, rMinor, rPatch, _, rOK := parseGoVersion(normalizeGoVersion(right))
	if !lOK || !rOK {
		return 0
	}
	if lMajor != rMajor {
		if lMajor < rMajor {
			return -1
		}
		return 1
	}
	if lMinor != rMinor {
		if lMinor < rMinor {
			return -1
		}
		return 1
	}
	if lPatch != rPatch {
		if lPatch < rPatch {
			return -1
		}
		return 1
	}
	return 0
}
