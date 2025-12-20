package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestHooksValidatePrintsEffectiveAssessInvocation(t *testing.T) {
	tmpDir := t.TempDir()

	writeHooksFixture(t, tmpDir, `version: "1.0.0"
hooks:
  pre-commit:
    - command: "make"
      args: ["precommit"]
      priority: 5
      timeout: "60s"
    - command: "assess"
      args: ["--categories", "format,lint", "--fail-on", "high"]
      priority: 10
      timeout: "2m"
  pre-push:
    - command: "assess"
      args: ["--categories", "format,lint,security", "--fail-on", "high"]
      priority: 10
      timeout: "3m"
optimization:
  only_changed_files: false
  content_source: working
  parallel: auto
  cache_results: true
`)

	// Create generated hooks + installed hooks to keep output stable
	mustMkdirAll(t, filepath.Join(tmpDir, ".goneat", "hooks"), 0o750)
	mustWriteExec(t, filepath.Join(tmpDir, ".goneat", "hooks", "pre-commit"))
	mustWriteExec(t, filepath.Join(tmpDir, ".goneat", "hooks", "pre-push"))
	mustMkdirAll(t, filepath.Join(tmpDir, ".git", "hooks"), 0o750)
	mustWriteExec(t, filepath.Join(tmpDir, ".git", "hooks", "pre-commit"))
	mustWriteExec(t, filepath.Join(tmpDir, ".git", "hooks", "pre-push"))

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	withCwd(t, tmpDir, func() {
		err := runHooksValidate(cmd, nil)
		if err != nil {
			t.Fatalf("runHooksValidate failed: %v\noutput:\n%s", err, buf.String())
		}
	})

	out := buf.String()
	if !strings.Contains(out, "Hook wrapper: goneat assess --hook pre-commit --hook-manifest .goneat/hooks.yaml --package-mode") {
		t.Fatalf("expected effective wrapper invocation, got:\n%s", out)
	}
	if strings.Contains(out, "--staged-only") {
		t.Fatalf("did not expect --staged-only for content_source=working, got:\n%s", out)
	}
	if !strings.Contains(out, "anti-pattern: running make in hooks") {
		t.Fatalf("expected make anti-pattern warning, got:\n%s", out)
	}
	if !strings.Contains(out, "pre-commit contains external commands") {
		t.Fatalf("expected external command warning, got:\n%s", out)
	}
}

func TestHooksInspectPrintsEffectivePolicyWhenConfigPresent(t *testing.T) {
	tmpDir := t.TempDir()

	writeHooksFixture(t, tmpDir, `version: "1.0.0"
hooks:
  pre-commit:
    - command: "assess"
      args: ["--categories", "format,lint", "--fail-on", "high"]
      priority: 10
      timeout: "2m"
optimization:
  only_changed_files: true
  content_source: index
  parallel: auto
  cache_results: true
`)

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	withCwd(t, tmpDir, func() {
		err := runHooksInspect(cmd, nil)
		if err != nil {
			t.Fatalf("runHooksInspect failed: %v\noutput:\n%s", err, buf.String())
		}
	})

	out := buf.String()
	if !strings.Contains(out, "🧩 pre-commit policy") {
		t.Fatalf("expected policy section, got:\n%s", out)
	}
	if !strings.Contains(out, "--staged-only") {
		t.Fatalf("expected staged-only when content_source=index, got:\n%s", out)
	}
}

func TestHooksInspect_JSONFormatIncludesCommandAnalysis(t *testing.T) {
	tmpDir := t.TempDir()

	writeHooksFixture(t, tmpDir, `version: "1.0.0"
hooks:
  pre-commit:
    - command: "make"
      args: ["precommit"]
      priority: 5
      timeout: "60s"
    - command: "assess"
      args: ["--fix", "--categories", "format,lint", "--fail-on", "high"]
      priority: 10
      timeout: "2m"
optimization:
  only_changed_files: true
  content_source: index
  parallel: auto
  cache_results: true
`)

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.Flags().String("format", "text", "")
	_ = cmd.Flags().Set("format", "json")

	withCwd(t, tmpDir, func() {
		err := runHooksInspect(cmd, nil)
		if err != nil {
			t.Fatalf("runHooksInspect failed: %v\noutput:\n%s", err, buf.String())
		}
	})

	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("expected valid JSON output, got error %v\noutput:\n%s", err, buf.String())
	}

	hooksVal, ok := decoded["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("expected hooks object in JSON, got: %T", decoded["hooks"])
	}
	pc, ok := hooksVal["pre-commit"].(map[string]any)
	if !ok {
		t.Fatalf("expected pre-commit hook analysis, got: %T", hooksVal["pre-commit"])
	}
	commands, ok := pc["commands"].([]any)
	if !ok || len(commands) == 0 {
		t.Fatalf("expected commands array, got: %#v", pc["commands"])
	}

	foundMutator := false
	for _, cAny := range commands {
		c, _ := cAny.(map[string]any)
		if v, ok := c["is_mutator"].(bool); ok && v {
			foundMutator = true
			break
		}
	}
	if !foundMutator {
		t.Fatalf("expected at least one mutator command in JSON, got: %#v", pc["commands"])
	}
}

func writeHooksFixture(t *testing.T, dir string, content string) {
	t.Helper()
	mustMkdirAll(t, filepath.Join(dir, ".goneat"), 0o750)
	if err := os.WriteFile(filepath.Join(dir, ".goneat", "hooks.yaml"), []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write hooks.yaml: %v", err)
	}
}

func mustMkdirAll(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(path, mode); err != nil {
		t.Fatalf("failed to mkdir %s: %v", path, err)
	}
}

func mustWriteExec(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatalf("failed to write exec file %s: %v", path, err)
	}
}

func withCwd(t *testing.T, dir string, fn func()) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to getcwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	fn()
}
