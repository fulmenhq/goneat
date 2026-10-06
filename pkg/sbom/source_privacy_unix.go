//go:build !windows

package sbom

import (
	"fmt"
	"io/fs"
	"os"
)

func makeSourcePrivate(name string) error {
	return os.Chmod(name, 0o700)
}

func setSourceChildOwner(file *os.File) error {
	return nil // Unix-created children already belong to the creating user.
}

func verifySourcePrivacy(name string, info fs.FileInfo, root bool) error {
	want := fs.FileMode(0o600)
	if info.IsDir() {
		want = 0o700
	}
	if info.Mode().Perm() != want {
		return fmt.Errorf("source SBOM private capture permissions changed at %q", name)
	}
	return nil
}
