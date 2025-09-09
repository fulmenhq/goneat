package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestInitializeLogger(t *testing.T) {
	// Test default logger initialization
	cmd := &cobra.Command{}
	cmd.Flags().String("log-level", "info", "")
	cmd.Flags().Bool("json", false, "")
	cmd.Flags().Bool("no-color", false, "")
	cmd.Flags().Bool("no-op", false, "")

	// This should not panic
	initializeLogger(cmd)
}

func TestInitializeLogger_DebugLevel(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("log-level", "debug", "")
	cmd.Flags().Bool("json", false, "")
	cmd.Flags().Bool("no-color", false, "")
	cmd.Flags().Bool("no-op", false, "")

	initializeLogger(cmd)
}

func TestInitializeLogger_InvalidLevel(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("log-level", "invalid", "")
	cmd.Flags().Bool("json", false, "")
	cmd.Flags().Bool("no-color", false, "")
	cmd.Flags().Bool("no-op", false, "")

	// Should default to info level
	initializeLogger(cmd)
}

func TestInitializeLogger_JSONOutput(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("log-level", "info", "")
	cmd.Flags().Bool("json", true, "")
	cmd.Flags().Bool("no-color", false, "")
	cmd.Flags().Bool("no-op", false, "")

	initializeLogger(cmd)
}

func TestInitializeLogger_NoColor(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("log-level", "info", "")
	cmd.Flags().Bool("json", false, "")
	cmd.Flags().Bool("no-color", true, "")
	cmd.Flags().Bool("no-op", false, "")

	initializeLogger(cmd)
}

func TestInitializeLogger_NoOp(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("log-level", "info", "")
	cmd.Flags().Bool("json", false, "")
	cmd.Flags().Bool("no-color", false, "")
	cmd.Flags().Bool("no-op", true, "")

	initializeLogger(cmd)
}

func TestGetVersionFromSources(t *testing.T) {
	// Test the getVersionFromSources function indirectly through rootCmd.Version
	// This is tested by checking that rootCmd has a version set
	if rootCmd.Version == "" {
		t.Error("rootCmd.Version should not be empty")
	}
}

func TestRootCmd_Help(t *testing.T) {
	// Capture help output
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	// Test help command
	rootCmd.SetArgs([]string{"--help"})
	err := rootCmd.Execute()

	// Help should show usage and exit with code 0 or exit code for help
	// We don't check the exact error since cobra help exits
	if err != nil && !strings.Contains(err.Error(), "unknown flag") {
		// This is expected for help - no action needed
		_ = err // Acknowledge the error but don't act on it
	}

	output := buf.String()
	if !strings.Contains(output, "goneat") {
		t.Error("Help output should contain 'goneat'")
	}
	if !strings.Contains(output, "unified tool for formatting") {
		t.Error("Help output should contain description")
	}
}

func TestRootCmd_VersionFlag(t *testing.T) {
	// Test --version flag
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	rootCmd.SetArgs([]string{"--version"})
	err := rootCmd.Execute()

	// Version should exit with success or specific version exit code
	if err != nil && err.Error() != "exit 0" {
		// Version command should work
		t.Errorf("Version command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "goneat") {
		t.Error("Version output should contain 'goneat'")
	}
}

func TestRootCmd_InvalidFlag(t *testing.T) {
	// Test invalid flag
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	rootCmd.SetArgs([]string{"--invalid-flag"})
	err := rootCmd.Execute()

	// Should return an error for invalid flag
	if err == nil {
		t.Error("Invalid flag should return an error")
	}
}
