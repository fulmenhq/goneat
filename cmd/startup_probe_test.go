package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// startupProbeChild re-enters this test binary as goneat when set.
const startupProbeChild = "GONEAT_TEST_STARTUP_PROBE_CHILD"

func TestMain(m *testing.M) {
	if os.Getenv(startupProbeChild) == "1" {
		rootCmd.SetArgs(os.Args[1:])
		if err := rootCmd.Execute(); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// Startup must not run package manager detection commands: pure-Go commands
// such as version and validate data are called thousands of times by
// consumers, and each probe is a child process.
func TestStartupSpawnsNoPackageManagerProbes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub executables are POSIX shell scripts")
	}
	stubs := t.TempDir()
	marker := filepath.Join(t.TempDir(), "spawned")
	for _, name := range []string{"bun", "mise", "brew", "go", "uv", "pip", "npm", "cargo", "apt-get", "pacman", "dnf", "scoop", "winget"} {
		script := "#!/bin/sh\necho " + name + " >> '" + marker + "'\necho " + name + " 1.0.0\n"
		if err := os.WriteFile(filepath.Join(stubs, name), []byte(script), 0o755); err != nil { // #nosec G306 -- test stub must be executable
			t.Fatal(err)
		}
	}
	work := t.TempDir()
	schema := filepath.Join(work, "schema.json")
	data := filepath.Join(work, "data.json")
	if err := os.WriteFile(schema, []byte(`{"$schema":"http://json-schema.org/draft-07/schema#","type":"object"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(data, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"version"},
		{"validate", "data", "--schema-file", schema, "--data", data},
	} {
		t.Run(args[0], func(t *testing.T) {
			_ = os.Remove(marker)
			// #nosec G204 -- re-executes this test binary with fixed arguments
			child := exec.Command(os.Args[0], args...)
			child.Dir = work
			child.Env = append(os.Environ(), startupProbeChild+"=1", "PATH="+stubs, "HOME="+work)
			if out, err := child.CombinedOutput(); err != nil {
				t.Fatalf("goneat %v failed: %v\n%s", args, err, out)
			}
			if spawned, err := os.ReadFile(marker); err == nil {
				t.Fatalf("goneat %v ran package manager probes:\n%s", args, spawned)
			}
		})
	}
}
