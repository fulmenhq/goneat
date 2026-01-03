package assess

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFakeCargo writes a fake cargo binary for tests and prepends it to PATH.
func writeFakeCargo(t *testing.T, repo string, script string) string {
	t.Helper()

	binDir := filepath.Join(repo, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}

	cargoPath := filepath.Join(binDir, "cargo")
	if err := os.WriteFile(cargoPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write cargo script: %v", err)
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+oldPath)

	return binDir
}
