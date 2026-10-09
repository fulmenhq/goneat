package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestVersionProcessChild is a controlled subprocess fixture. It is inert in
// ordinary test execution and never invokes goneat or a package manager.
func TestVersionProcessChild(t *testing.T) {
	for i, arg := range os.Args {
		if arg != "--version-process-fixture" || i+1 >= len(os.Args) {
			continue
		}
		switch os.Args[i+1] {
		case "success":
			fmt.Println("Project: fixture v1.2.3")
			os.Exit(0)
		case "empty-failure":
			os.Exit(7)
		case "output-failure":
			fmt.Fprintln(os.Stderr, "fixture original failure")
			os.Exit(9)
		case "wait":
			// Only the parent context ends this fixture.
			time.Sleep(time.Minute)
			os.Exit(0)
		default:
			os.Exit(99)
		}
	}
}

func TestVersionProcessDiagnostics(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		exit int
		text string
	}{
		{"success", 0, "Project: fixture v1.2.3"},
		{"empty-failure", 7, ""},
		{"output-failure", 9, "fixture original failure"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			args := []string{"-test.run=^TestVersionProcessChild$", "--", "--version-process-fixture", tt.name}
			result, spawnErr := runVersionProcess(ctx, executable, t.TempDir(), args)
			if spawnErr != nil || result.ExitCode != tt.exit || result.Output != tt.text {
				t.Fatalf("exit/output behavior changed: result=%+v spawn=%v", result, spawnErr)
			}
			if tt.exit == 0 {
				if result.Error != "" {
					t.Fatalf("success must not gain failure diagnostics: %s", result.Error)
				}
				env := &TestEnv{}
				env.parseVersionOutput(&result)
				if result.Component != "fixture" || result.Version != "v1.2.3" {
					t.Fatalf("successful version parsing changed: %+v", result)
				}
				return
			}
			for _, want := range []string{tt.text, "executable=", "argv=", "pid=", "started=", "elapsed=", fmt.Sprintf("exit=%d", tt.exit), "exec_error=exit status", "context_error=<nil>", "sha256_before=", "sha256_after=", "pathname_bytes_equal=true", "not immutable process-image proof"} {
				if !strings.Contains(result.Error, want) {
					t.Errorf("failure diagnostics missing %q: %s", want, result.Error)
				}
			}
		})
	}
}

func TestVersionProcessDeadlineDiagnostics(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	result, _ := runVersionProcess(ctx, executable, t.TempDir(), []string{"-test.run=^TestVersionProcessChild$", "--", "--version-process-fixture", "wait"})
	if result.ExitCode == 0 || ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("deadline must remain a failure: %+v ctx=%v", result, ctx.Err())
	}
	for _, want := range []string{"context_error=context deadline exceeded", "exec_error=", "state=", "elapsed="} {
		if !strings.Contains(result.Error, want) {
			t.Errorf("deadline evidence missing %q: %s", want, result.Error)
		}
	}
}

func TestVersionProcessSpawnFailureDiagnostics(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-executable")
	result, err := runVersionProcess(context.Background(), missing, t.TempDir(), []string{"version", "--project"})
	if err == nil || result.ExitCode != -1 || !strings.Contains(result.Error, err.Error()) {
		t.Fatalf("spawn failure must retain original cause: %+v error=%v", result, err)
	}
	for _, want := range []string{missing, "sha256_before=\"unavailable:", "sha256_after=\"unavailable:", "pathname_bytes_equal=false", "process did not start"} {
		if !strings.Contains(result.Error, want) {
			t.Errorf("spawn evidence missing %q: %s", want, result.Error)
		}
	}
}

func TestVersionExecutableDigestObservations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture-bytes")
	if err := os.WriteFile(path, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	first := executableDigest(path)
	if err := os.WriteFile(path, []byte("second"), 0o600); err != nil {
		t.Fatal(err)
	}
	second := executableDigest(path)
	if len(first) != 64 || len(second) != 64 || first == second {
		t.Fatalf("changed pathname bytes not distinguished: first=%q second=%q", first, second)
	}
}
