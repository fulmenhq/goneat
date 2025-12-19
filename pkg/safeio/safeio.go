package safeio

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// CleanUserPath cleans a user-provided path and rejects traversal attempts.
// Returns paths with forward slashes for cross-platform consistency.
func CleanUserPath(p string) (string, error) {
	c := filepath.Clean(p)
	if strings.Contains(c, "..") {
		return "", errors.New("path traversal detected")
	}
	// Normalize to forward slashes for cross-platform consistency
	return filepath.ToSlash(c), nil
}

// ReadFileContained reads a file only if it is contained within baseDir.
// This prevents path traversal attacks by ensuring the file path resolves
// to a location within the specified base directory.
// Returns an error if the file is outside baseDir or cannot be read.
func ReadFileContained(baseDir, filePath string) ([]byte, error) {
	// Resolve both paths to absolute
	baseDirAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, errors.New("failed to resolve base directory")
	}
	filePathAbs, err := filepath.Abs(filePath)
	if err != nil {
		return nil, errors.New("failed to resolve file path")
	}

	// Check containment using filepath.Rel
	rel, err := filepath.Rel(baseDirAbs, filePathAbs)
	if err != nil {
		return nil, errors.New("failed to compute relative path")
	}

	// Reject if relative path escapes the base directory
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return nil, errors.New("file path is outside base directory")
	}

	// Read the file (safe: path containment already verified above)
	// #nosec G304 -- filePathAbs has been verified to be contained within baseDirAbs
	return os.ReadFile(filePathAbs)
}

// WriteFilePreservePerms writes data to path preserving existing file mode when possible.
// When the file does not exist, it uses a sane default of 0644.
func WriteFilePreservePerms(path string, data []byte) error {
	var mode os.FileMode = 0o644
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode() & 0o777
		if mode == 0 {
			mode = 0o644
		}
	}
	return os.WriteFile(path, data, mode)
}
