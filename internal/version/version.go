package version

import (
	"fmt"
	"strconv"
	"strings"
)

type Value struct {
	Raw         string
	Major       int
	Minor       int
	Patch       int
	Prerelease  string
	Development bool
}

func Parse(raw string) (Value, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Value{}, fmt.Errorf("version cannot be empty")
	}
	if raw == "dev" || raw == "current" {
		return Value{Raw: raw, Development: true}, nil
	}
	trimmed := strings.TrimPrefix(raw, "v")
	main, prerelease, _ := strings.Cut(trimmed, "-")
	parts := strings.Split(main, ".")
	if len(parts) != 3 {
		return Value{}, fmt.Errorf("version %q must use semantic versioning (for example 1.0.0)", raw)
	}
	values := make([]int, 3)
	for index, part := range parts {
		if part == "" {
			return Value{}, fmt.Errorf("version %q contains an empty component", raw)
		}
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 {
			return Value{}, fmt.Errorf("version %q contains invalid component %q", raw, part)
		}
		values[index] = number
	}
	return Value{Raw: raw, Major: values[0], Minor: values[1], Patch: values[2], Prerelease: prerelease}, nil
}

func (v Value) String() string {
	if v.Development {
		return v.Raw
	}
	base := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		return base + "-" + v.Prerelease
	}
	return base
}

func Compare(left, right Value) int {
	if left.Development || right.Development {
		switch {
		case left.Raw == right.Raw:
			return 0
		case left.Development && !right.Development:
			return 1
		default:
			return -1
		}
	}
	for _, pair := range [][2]int{{left.Major, right.Major}, {left.Minor, right.Minor}, {left.Patch, right.Patch}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	if left.Prerelease == right.Prerelease {
		return 0
	}
	if left.Prerelease == "" {
		return 1
	}
	if right.Prerelease == "" {
		return -1
	}
	if left.Prerelease < right.Prerelease {
		return -1
	}
	return 1
}

func AtLeast(actualRaw, minimumRaw string) error {
	actual, err := Parse(actualRaw)
	if err != nil {
		return fmt.Errorf("parse actual version: %w", err)
	}
	minimum, err := Parse(minimumRaw)
	if err != nil {
		return fmt.Errorf("parse minimum version: %w", err)
	}
	if Compare(actual, minimum) < 0 {
		return fmt.Errorf("version %s is older than required version %s", actual.String(), minimum.String())
	}
	return nil
}
