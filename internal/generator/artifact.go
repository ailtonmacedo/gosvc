package generator

import (
	"fmt"
	"io/fs"
	"path"
	"strings"
)

type Ownership string

const (
	OwnershipGenerated Ownership = "generated"
	OwnershipUser      Ownership = "user"
)

type Artifact struct {
	Path      string
	Content   []byte
	Mode      fs.FileMode
	Ownership Ownership
	Producer  string
}

func (a Artifact) Validate() error {
	if strings.TrimSpace(a.Path) == "" {
		return fmt.Errorf("artifact path is required")
	}
	if strings.ContainsRune(a.Path, '\\') {
		return fmt.Errorf("artifact path %q must use forward slashes", a.Path)
	}
	if path.IsAbs(a.Path) || strings.HasPrefix(a.Path, "/") {
		return fmt.Errorf("artifact path %q must be relative", a.Path)
	}
	clean := path.Clean(a.Path)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || clean != a.Path {
		return fmt.Errorf("artifact path %q is unsafe or not clean", a.Path)
	}
	if a.Ownership != OwnershipGenerated && a.Ownership != OwnershipUser {
		return fmt.Errorf("artifact %q has invalid ownership %q", a.Path, a.Ownership)
	}
	if a.Mode == 0 {
		return fmt.Errorf("artifact %q requires file mode", a.Path)
	}
	if a.Mode.Perm() != a.Mode {
		return fmt.Errorf("artifact %q mode must contain permission bits only", a.Path)
	}
	return nil
}
