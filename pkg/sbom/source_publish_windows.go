//go:build windows

package sbom

import "golang.org/x/sys/windows"

func replaceSourceOutput(staged, destination string) error {
	from, err := windows.UTF16PtrFromString(staged)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	// Both names are in the destination directory. Do not allow COPY_ALLOWED:
	// a cross-volume copy/delete fallback would violate atomic publication.
	return windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}
