package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGoLicensesSelectedRootWrapper(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Bash build tooling runs on Unix hosts")
	}
	dir := t.TempDir()
	root := filepath.Join(dir, "selected toolchain")
	if err := os.MkdirAll(filepath.Join(root, "src"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, script := range map[string]string{
		"go":          "#!/bin/sh\n[ \"$*\" = 'env GOROOT' ] || exit 9\n[ \"$FIXTURE_MODE\" != failed ] || exit 7\n[ \"$FIXTURE_MODE\" != empty ] || exit 0\nprintf '%s\\n' \"$FIXTURE_ROOT\"\n",
		"go-licenses": "#!/bin/sh\n[ \"$GOROOT\" = \"$FIXTURE_ROOT\" ] || exit 8\nprintf '%s\\n' \"$*\"\n[ \"$FIXTURE_MODE\" != scanner_failed ] || exit 6\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, mode := range []string{"complete", "failed", "empty", "scanner_failed"} {
		t.Run(mode, func(t *testing.T) {
			cmd := exec.Command("bash", "go-licenses.sh", "csv", ".")
			cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "FIXTURE_ROOT="+root, "FIXTURE_MODE="+mode)
			out, err := cmd.CombinedOutput()
			if mode == "complete" {
				if err != nil || strings.TrimSpace(string(out)) != "csv ." {
					t.Fatalf("selected root or args lost: %s %v", out, err)
				}
			} else if err == nil {
				t.Fatalf("%s incorrectly passed: %s", mode, out)
			}
		})
	}
}
