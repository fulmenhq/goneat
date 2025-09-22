package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulmenhq/goneat/internal/guardian"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func TestRunHooksGenerate_GuardianAutoInstall(t *testing.T) {
	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	if err := os.MkdirAll(".goneat", 0o750); err != nil {
		t.Fatalf("mkdir .goneat failed: %v", err)
	}
	manifest := `version: "1.0.0"
hooks:
  pre-commit:
    - command: "assess"
      args: ["--categories", "format"]
  pre-push:
    - command: "assess"
      args: ["--categories", "format"]
`
	if err := os.WriteFile(".goneat/hooks.yaml", []byte(manifest), 0o600); err != nil {
		t.Fatalf("write hooks.yaml failed: %v", err)
	}

	homeDir := filepath.Join(tmp, "home")
	t.Setenv("GONEAT_HOME", homeDir)

	cfgPath, err := guardian.EnsureConfig()
	if err != nil {
		t.Fatalf("EnsureConfig failed: %v", err)
	}

	cfgBytes, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read guardian config failed: %v", err)
	}
	var cfg guardian.ConfigRoot
	if err := yaml.Unmarshal(cfgBytes, &cfg); err != nil {
		t.Fatalf("unmarshal guardian config failed: %v", err)
	}
	cfg.Guardian.Integrations.Hooks.AutoInstall = true
	updated, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal guardian config failed: %v", err)
	}
	if err := os.WriteFile(cfgPath, updated, 0o600); err != nil {
		t.Fatalf("write guardian config failed: %v", err)
	}

	hooksGuardian = false
	cmd := &cobra.Command{Use: "generate"}
	cmd.Flags().BoolVar(&hooksGuardian, "with-guardian", false, "")

	if err := runHooksGenerate(cmd, nil); err != nil {
		t.Fatalf("runHooksGenerate failed: %v", err)
	}

	prePushPath := filepath.Join(".goneat", "hooks", "pre-push")
	data, err := os.ReadFile(prePushPath)
	if err != nil {
		t.Fatalf("read pre-push failed: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "guardian check \"$GUARDIAN_SCOPE\" \"$GUARDIAN_OPERATION\"") {
		t.Fatalf("guardian command not embedded in pre-push hook:\n%s", content)
	}
	if !strings.Contains(content, "Risk level: critical") {
		t.Errorf("expected risk level rendering in hook")
	}
}

func TestRunHooksGenerate_NoGuardianByDefault(t *testing.T) {
	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	if err := os.MkdirAll(".goneat", 0o750); err != nil {
		t.Fatalf("mkdir .goneat failed: %v", err)
	}
	manifest := `version: "1.0.0"
hooks:
  pre-commit:
    - command: "assess"
      args: ["--categories", "format"]
  pre-push:
    - command: "assess"
      args: ["--categories", "format"]
`
	if err := os.WriteFile(".goneat/hooks.yaml", []byte(manifest), 0o600); err != nil {
		t.Fatalf("write hooks.yaml failed: %v", err)
	}

	homeDir := filepath.Join(tmp, "home")
	t.Setenv("GONEAT_HOME", homeDir)

	hooksGuardian = false
	cmd := &cobra.Command{Use: "generate"}
	cmd.Flags().BoolVar(&hooksGuardian, "with-guardian", false, "")

	if err := runHooksGenerate(cmd, nil); err != nil {
		t.Fatalf("runHooksGenerate failed: %v", err)
	}

	prePushPath := filepath.Join(".goneat", "hooks", "pre-push")
	data, err := os.ReadFile(prePushPath)
	if err != nil {
		t.Fatalf("read pre-push failed: %v", err)
	}
	if strings.Contains(string(data), "goneat guardian check") {
		t.Fatalf("unexpected guardian integration in pre-push hook:\n%s", string(data))
	}
}

func TestRunHooksInstall_CreatesGuardianConfig(t *testing.T) {
	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	if err := os.MkdirAll(".goneat/hooks", 0o750); err != nil {
		t.Fatalf("mkdir hooks failed: %v", err)
	}
	script := "#!/bin/sh\necho test\ngoneat guardian check git push\n"
	if err := os.WriteFile(".goneat/hooks/pre-commit", []byte(script), 0o700); err != nil {
		t.Fatalf("write pre-commit failed: %v", err)
	}
	if err := os.WriteFile(".goneat/hooks/pre-push", []byte(script), 0o700); err != nil {
		t.Fatalf("write pre-push failed: %v", err)
	}

	if err := os.MkdirAll(".git/hooks", 0o750); err != nil {
		t.Fatalf("mkdir .git/hooks failed: %v", err)
	}

	homeDir := filepath.Join(tmp, "home")
	t.Setenv("GONEAT_HOME", homeDir)

	hooksGuardian = false
	cmd := &cobra.Command{Use: "install"}

	if err := runHooksInstall(cmd, nil); err != nil {
		t.Fatalf("runHooksInstall failed: %v", err)
	}

	cfgPath, err := guardian.ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath failed: %v", err)
	}
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("expected guardian config to exist: %v", err)
	}
}
