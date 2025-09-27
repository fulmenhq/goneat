package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunGuardianCheck_NoPolicy(t *testing.T) {
	// Test when no guardian policy requires approval
	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	homeDir := filepath.Join(tmp, "home")
	t.Setenv("GONEAT_HOME", homeDir)

	cmd := &cobra.Command{Use: "check"}
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	// Test with a scope/operation that has no policy
	err = runGuardianCheck(cmd, []string{"nonexistent", "scope"})
	if err != nil {
		t.Fatalf("expected no error for non-existent scope, got: %v", err)
	}

	output := stdout.String()
	if output != "" {
		t.Errorf("expected no stdout output, got: %s", output)
	}
}

func TestRunGuardianCheck_ApprovalRequired(t *testing.T) {
	// Test when guardian approval is required
	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	homeDir := filepath.Join(tmp, "home")
	t.Setenv("GONEAT_HOME", homeDir)
	t.Setenv("GONEAT_GUARDIAN_TEST_MODE", "true")

	cmd := &cobra.Command{Use: "check"}
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	// Set context to match default policy conditions (main branch, origin remote)
	guardianBranch = "main"
	guardianRemote = "origin"
	t.Cleanup(func() {
		guardianBranch = ""
		guardianRemote = ""
	})

	// Test with git push which should require approval when conditions are met
	err = runGuardianCheck(cmd, []string{"git", "push"})
	if err == nil {
		t.Fatal("expected approval required error, got nil")
	}

	// Should be an approval required error
	stderrOutput := stderr.String()
	if !strings.Contains(stderrOutput, "guardian approval required") {
		t.Errorf("expected approval message in stderr, got: %s", stderrOutput)
	}
}

func TestRunGuardianCheck_WithContext(t *testing.T) {
	// Test guardian check with branch and remote context
	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	homeDir := filepath.Join(tmp, "home")
	t.Setenv("GONEAT_HOME", homeDir)

	cmd := &cobra.Command{Use: "check"}
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	// Set context flags
	guardianBranch = "main"
	guardianRemote = "origin"
	guardianUser = "testuser"
	t.Cleanup(func() {
		guardianBranch = ""
		guardianRemote = ""
		guardianUser = ""
	})

	err = runGuardianCheck(cmd, []string{"git", "push"})
	if err == nil {
		t.Fatal("expected approval required error, got nil")
	}

	stderrOutput := stderr.String()
	if !strings.Contains(stderrOutput, "guardian approval required") {
		t.Errorf("expected approval message in stderr, got: %s", stderrOutput)
	}
}

func TestRunGuardianCheck_InvalidArgs(t *testing.T) {
	// Test with invalid arguments - simulate what happens when args are wrong
	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	homeDir := filepath.Join(tmp, "home")
	t.Setenv("GONEAT_HOME", homeDir)

	cmd := &cobra.Command{Use: "check"}
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	// Test with empty args - this should cause a panic in the current code
	// We'll catch it and verify it happens
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for insufficient args, but none occurred")
		}
	}()
	_ = runGuardianCheck(cmd, []string{}) // Error ignored as this test expects a panic
}

func TestRunGuardianApprove_InvalidArgs(t *testing.T) {
	// Test approve command with invalid arguments
	cmd := &cobra.Command{Use: "approve"}
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	// Test with too few args
	err := runGuardianApprove(cmd, []string{"git"})
	if err == nil {
		t.Fatal("expected error for insufficient args, got nil")
	}
	if !strings.Contains(err.Error(), "usage: goneat guardian approve") {
		t.Errorf("expected usage message, got: %v", err)
	}

	// Test with no command to execute (this triggers usage error since len(args) < 3)
	err = runGuardianApprove(cmd, []string{"git", "push"})
	if err == nil {
		t.Fatal("expected error for insufficient args, got nil")
	}
	if !strings.Contains(err.Error(), "usage: goneat guardian approve") {
		t.Errorf("expected usage message, got: %v", err)
	}
}

func TestRunGuardianSetup(t *testing.T) {
	// Test guardian setup command
	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	homeDir := filepath.Join(tmp, "home")
	t.Setenv("GONEAT_HOME", homeDir)

	cmd := &cobra.Command{Use: "setup"}
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err = runGuardianSetup(cmd, nil)
	if err != nil {
		t.Fatalf("runGuardianSetup failed: %v", err)
	}

	stdoutOutput := stdout.String()
	if !strings.Contains(stdoutOutput, "Guardian configuration available at") {
		t.Errorf("expected config path message, got: %s", stdoutOutput)
	}

	// Verify config file was created
	configPath := filepath.Join(homeDir, "guardian", "config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("expected guardian config file to exist: %v", err)
	}
}

func TestRunGuardianApprove_NoPolicy(t *testing.T) {
	// Test approve command when no policy requires approval
	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	homeDir := filepath.Join(tmp, "home")
	t.Setenv("GONEAT_HOME", homeDir)

	cmd := &cobra.Command{Use: "approve"}
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	// Test with a scope/operation that has no policy
	err = runGuardianApprove(cmd, []string{"nonexistent", "scope", "echo", "test"})
	if err != nil {
		t.Fatalf("expected no error for non-existent scope, got: %v", err)
	}

	stdoutOutput := stdout.String()
	if !strings.Contains(stdoutOutput, "No guardian policy requires approval") {
		t.Errorf("expected no policy message, got: %s", stdoutOutput)
	}
}

// Note: Grant and Status commands are not implemented yet, so we skip testing them
// They return inline errors that would require more complex test setup

// Integration tests require more complex cobra command setup
// For now, we've covered the core functionality with unit tests
// TODO: Add full integration tests when browser approval server is implemented
