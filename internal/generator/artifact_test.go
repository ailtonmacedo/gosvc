package generator

import "testing"

func TestArtifactValidateRejectsUnsafePaths(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"", "../secret", "/absolute", "a/../b", `a\\b`} {
		artifact := Artifact{Path: path, Mode: 0o644, Ownership: OwnershipUser}
		if err := artifact.Validate(); err == nil {
			t.Errorf("Validate(%q) error = nil, want error", path)
		}
	}
}
