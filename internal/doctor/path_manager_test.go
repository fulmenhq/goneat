package doctor

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGetRequiredPATHAdditions_AddsExistingShimDirWithoutProbing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub executables are POSIX shell scripts")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	shims := filepath.Join(home, ".local", "share", "mise", "shims")
	if err := os.MkdirAll(shims, 0o755); err != nil {
		t.Fatal(err)
	}
	// A mise on PATH that records any invocation.
	stubs := t.TempDir()
	marker := filepath.Join(t.TempDir(), "spawned")
	script := "#!/bin/sh\necho mise >> '" + marker + "'\necho mise 2026.1.0\n"
	if err := os.WriteFile(filepath.Join(stubs, "mise"), []byte(script), 0o755); err != nil { // #nosec G306 -- test stub must be executable
		t.Fatal(err)
	}
	t.Setenv("PATH", stubs)

	config := &PackageManagersConfig{PackageManagers: []PackageManager{{
		Name:               "mise",
		DetectionCommand:   "mise --version",
		RequiresPathUpdate: true,
		Platforms:          []string{runtime.GOOS},
	}}}
	additions := GetRequiredPATHAdditions(config)
	if len(additions) != 1 || additions[0] != shims {
		t.Fatalf("additions = %v, want [%s]", additions, shims)
	}
	if spawned, err := os.ReadFile(marker); err == nil {
		t.Fatalf("PATH additions ran a detection command: %s", spawned)
	}

	// The shim dir alone decides: it is added even when mise itself is not on PATH.
	t.Setenv("PATH", t.TempDir())
	if additions := GetRequiredPATHAdditions(config); len(additions) != 1 || additions[0] != shims {
		t.Fatalf("shim dir with mise absent from PATH: additions = %v, want [%s]", additions, shims)
	}

	if err := os.Remove(shims); err != nil {
		t.Fatal(err)
	}
	if additions := GetRequiredPATHAdditions(config); len(additions) != 0 {
		t.Fatalf("missing shim dir must not be added, got %v", additions)
	}
}
