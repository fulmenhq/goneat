package safeio

import (
    "errors"
    "os"
    "path/filepath"
    "strings"
)

// CleanUserPath cleans a user-provided path and rejects traversal attempts.
func CleanUserPath(p string) (string, error) {
    c := filepath.Clean(p)
    if strings.Contains(c, "..") {
        return "", errors.New("path traversal detected")
    }
    return c, nil
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
