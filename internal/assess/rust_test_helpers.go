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
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}

	cargoPath := filepath.Join(binDir, "cargo")
	if err := os.WriteFile(cargoPath, []byte(script), 0o600); err != nil {
		t.Fatalf("write cargo script: %v", err)
	}
	// #nosec G302 -- test helper needs executable permissions
	if err := os.Chmod(cargoPath, 0o700); err != nil {
		t.Fatalf("chmod cargo script: %v", err)
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+oldPath)

	return binDir
}
