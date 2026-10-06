//go:build !windows

package sbom

import "os"

func replaceSourceOutput(staged, destination string) error {
	return os.Rename(staged, destination)
}
