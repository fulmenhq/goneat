package integration

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repoRoot() string {
	wd, _ := os.Getwd()
	// tests/integration -> repo root two levels up
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

func readPhaseFile(t *testing.T, name string) (string, bool) {
	t.Helper()
	root := repoRoot()
	path := filepath.Join(root, name)
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	// Normalize newlines
	s := strings.TrimSpace(string(b))
	// On Windows, ensure consistent casing/whitespace
	_ = runtime.GOOS
	return s, true
}

func TestLifecyclePhaseValue(t *testing.T) {
	val, ok := readPhaseFile(t, "LIFECYCLE_PHASE")
	if !ok {
		t.Fatalf("LIFECYCLE_PHASE file not found")
	}
	allowed := map[string]struct{}{
		"experimental": {},
		"alpha":        {},
		"beta":         {},
		"rc":           {},
		"ga":           {},
		"lts":          {},
	}
	if _, ok := allowed[strings.ToLower(val)]; !ok {
		t.Fatalf("invalid LIFECYCLE_PHASE value: %q", val)
	}
}

func TestReleasePhaseValue(t *testing.T) {
	val, ok := readPhaseFile(t, "RELEASE_PHASE")
	if !ok {
		// Optional for now; skip if absent
		t.Skip("RELEASE_PHASE not present; skipping")
		return
	}
	allowed := map[string]struct{}{
		"dev":     {},
		"rc":      {},
		"ga":      {},
		"release": {},
	}
	if _, ok := allowed[strings.ToLower(val)]; !ok {
		t.Fatalf("invalid RELEASE_PHASE value: %q", val)
	}
}
