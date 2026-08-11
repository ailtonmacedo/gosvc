package repository

import (
	"fmt"
	"regexp"
	"strings"
)

var segmentPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9._-]*[A-Za-z0-9])?$`)

// GitHub identifies a GitHub repository using owner/name notation.
type GitHub struct {
	Owner string
	Name  string
}

func Parse(value string) (GitHub, error) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "https://github.com/")
	value = strings.TrimPrefix(value, "http://github.com/")
	value = strings.TrimSuffix(value, ".git")
	value = strings.Trim(value, "/")
	parts := strings.Split(value, "/")
	if len(parts) != 2 || !segmentPattern.MatchString(parts[0]) || !segmentPattern.MatchString(parts[1]) {
		return GitHub{}, fmt.Errorf("repository must use GitHub owner/name notation")
	}
	return GitHub{Owner: parts[0], Name: parts[1]}, nil
}

func FromModule(module string) (GitHub, error) {
	const prefix = "github.com/"
	if !strings.HasPrefix(module, prefix) {
		return GitHub{}, fmt.Errorf("module %q is not a GitHub module", module)
	}
	return Parse(strings.TrimPrefix(module, prefix))
}

func (r GitHub) Slug() string   { return r.Owner + "/" + r.Name }
func (r GitHub) Module() string { return "github.com/" + r.Slug() }
func (r GitHub) URL() string    { return "https://github.com/" + r.Slug() }
