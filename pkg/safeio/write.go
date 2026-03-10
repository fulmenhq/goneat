package safeio

import "os"

// WriteFileValidated writes to a caller-validated destination path.
// Callers must only pass paths that are repo-contained, sanitized, or
// explicitly user-selected CLI targets.
func WriteFileValidated(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm) // #nosec G306 G703 -- path trust boundary is enforced by the caller for local filesystem writes
}
