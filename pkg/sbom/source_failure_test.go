package sbom

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSourceInvokerFailurePreservesDestination(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("collector failure injector is a POSIX shell script")
	}
	for _, scenario := range []struct{ name, action, output string }{
		{"collector-error", "exit 9", ""},
		{"malformed-json", "", "not JSON"},
		{"duplicate-json-key", "", `{"bomFormat":"CycloneDX","bomFormat":"other"}`},
		{"invalid-schema", "", `{"bomFormat":"not-CycloneDX","specVersion":"1.6"}`},
		{"dangling-reference", "", `{"bomFormat":"CycloneDX","specVersion":"1.6","version":1,"dependencies":[{"ref":"missing","dependsOn":[]}]}`},
		{"snapshot-mutation", `printf changed > "$2/file"`, `{"bomFormat":"CycloneDX","specVersion":"1.6","version":1}`},
		{"snapshot-path-replaced", `mv "$2" "$2-moved"; mkdir "$2"; printf retained > "$2/user-owned"`, `{"bomFormat":"CycloneDX","specVersion":"1.6","version":1}`},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			root, work := t.TempDir(), t.TempDir()
			writeSourceFixture(t, root, "file", "original bytes")
			trace := filepath.Join(work, "captured-path")
			t.Setenv("GONEAT_TEST_CAPTURE_TRACE", trace)
			script := "#!/bin/sh\nif [ \"$1\" = version ]; then echo '{\"version\":\"test\"}'; exit 0; fi\nprintf '%s' \"$2\" > \"$GONEAT_TEST_CAPTURE_TRACE\"\n" + scenario.action + "\ncat <<'EOF'\n" + scenario.output + "\nEOF\n"
			collector := filepath.Join(work, "syft")
			if err := os.WriteFile(collector, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(work, "previous.json")
			if err := os.WriteFile(destination, []byte("previous inventory"), 0o600); err != nil {
				t.Fatal(err)
			}
			invoker := &SyftInvoker{syftPath: collector}
			result, err := invoker.Generate(context.Background(), Config{TargetPath: root, OutputPath: destination})
			if err == nil || result != nil {
				t.Fatalf("failed collection returned inventory: %v %v", result, err)
			}
			data, readErr := os.ReadFile(destination)
			if readErr != nil || string(data) != "previous inventory" {
				t.Fatalf("previous destination changed: %s %v", data, readErr)
			}
			data, readErr = os.ReadFile(filepath.Join(root, "file"))
			if readErr != nil || string(data) != "original bytes" {
				t.Fatalf("source changed: %s %v", data, readErr)
			}
			data, readErr = os.ReadFile(trace)
			if readErr != nil {
				t.Fatal(readErr)
			}
			capture := string(data)
			if scenario.name == "snapshot-path-replaced" {
				// The test deliberately moved an invocation-owned directory.
				// Remove only those two exact fixture paths after checking refusal.
				t.Cleanup(func() {
					for _, path := range []string{capture, capture + "-moved"} {
						if err := os.RemoveAll(path); err != nil {
							t.Error(err)
						}
					}
				})
				if !strings.Contains(err.Error(), capture) {
					t.Fatalf("cleanup error did not name retained data: %v", err)
				}
				if _, err := os.Stat(filepath.Join(capture, "user-owned")); err != nil {
					t.Fatal("replacement tree removed: ", err)
				}
			} else if _, err := os.Stat(capture); !os.IsNotExist(err) {
				t.Fatalf("failed collection retained private snapshot %s: %v", capture, err)
			}
		})
	}
}
