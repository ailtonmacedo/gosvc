package upgradenotes

import (
	"fmt"
	"strings"

	versionutil "github.com/ailtonmacedo/gosvc/internal/version"
)

type Note struct {
	Version     string
	Title       string
	Breaking    bool
	Description string
}

var registry = []Note{
	{Version: "1.1.0", Title: "Manifest schema v3", Description: "Adds compatibility metadata, persistent upgrade backups, and rollback history."},
	{Version: "1.1.0", Title: "Automatic upgrade backups", Description: "gosvc upgrade creates a ZIP backup before applying managed-file and manifest changes."},
	{Version: "1.1.0", Title: "Atomic rollback", Description: "gosvc upgrade rollback restores a selected backup while preserving the backup catalog."},
}

func Between(fromRaw, toRaw string) ([]Note, error) {
	from, err := versionutil.Parse(fromRaw)
	if err != nil {
		return nil, fmt.Errorf("parse source version: %w", err)
	}
	to, err := versionutil.Parse(toRaw)
	if err != nil {
		return nil, fmt.Errorf("parse target version: %w", err)
	}
	if versionutil.Compare(to, from) < 0 {
		return nil, fmt.Errorf("target version %s is older than source version %s", to.String(), from.String())
	}
	var notes []Note
	for _, note := range registry {
		value, _ := versionutil.Parse(note.Version)
		if versionutil.Compare(value, from) > 0 && versionutil.Compare(value, to) <= 0 {
			notes = append(notes, note)
		}
	}
	return notes, nil
}

func Markdown(from, to string) (string, error) {
	notes, err := Between(from, to)
	if err != nil {
		return "", err
	}
	var output strings.Builder
	fmt.Fprintf(&output, "# gosvc upgrade notes: %s → %s\n\n", from, to)
	if len(notes) == 0 {
		output.WriteString("No registered migration notes for this version range.\n")
		return output.String(), nil
	}
	for _, note := range notes {
		marker := ""
		if note.Breaking {
			marker = " **Breaking:**"
		}
		fmt.Fprintf(&output, "## %s%s\n\n%s\n\n", note.Title, marker, note.Description)
	}
	return output.String(), nil
}
