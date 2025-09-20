package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestVersionValidateCmd tests the version validate command
func TestVersionValidateCmd(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		expectError bool
		description string
	}{
		// Valid versions
		{"valid_basic", "1.2.3", false, "basic semver"},
		{"valid_with_v", "v1.2.3", false, "with v prefix"},
		{"valid_prerelease", "1.0.0-alpha.1", false, "with prerelease"},
		{"valid_build", "1.0.0+build.123", false, "with build metadata"},
		{"valid_complex", "1.0.0-rc.1+build.456", false, "prerelease and build"},

		// Invalid versions
		{"invalid_empty", "", true, "empty string"},
		{"invalid_format", "1.2", true, "missing patch"},
		{"invalid_leading_zero", "01.2.3", true, "leading zero in major"},
		{"invalid_chars", "1.2.3-beta!", true, "invalid characters"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{}

			// Capture output
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			err := runVersionValidate(cmd, []string{tc.version})

			if tc.expectError {
				if err == nil {
					t.Errorf("%s: expected error for invalid version %s", tc.description, tc.version)
				}
			} else {
				if err != nil {
					t.Errorf("%s: unexpected error for valid version %s: %v", tc.description, tc.version, err)
				}
			}
		})
	}
}

// TestVersionBumpCmd tests the version bump command
func TestVersionBumpCmd(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "goneat-version-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Change to temp directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	tests := []struct {
		name        string
		initial     string
		bumpType    string
		expected    string
		expectError bool
		description string
	}{
		// Valid bump operations
		{"bump_patch", "1.2.3", "patch", "1.2.4", false, "basic patch bump"},
		{"bump_minor", "1.2.3", "minor", "1.3.0", false, "basic minor bump"},
		{"bump_major", "1.2.3", "major", "2.0.0", false, "basic major bump"},
		{"bump_patch_with_v", "v1.2.3", "patch", "v1.2.4", false, "patch bump preserves v prefix"},
		{"bump_prerelease_clear", "1.2.3-alpha.1", "patch", "1.2.4", false, "bump clears prerelease"},
		{"bump_build_clear", "1.2.3+build.123", "minor", "1.3.0", false, "bump clears build metadata"},

		// Invalid cases
		{"invalid_bump_type", "1.2.3", "invalid", "", true, "invalid bump type"},
		{"missing_version_file", "", "patch", "", true, "no VERSION file"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup VERSION file if needed
			if tc.initial != "" {
				err := writeVersionToFile("VERSION", tc.initial)
				if err != nil {
					t.Fatalf("failed to write VERSION file: %v", err)
				}
			} else {
				// Remove VERSION file for missing file test
				_ = os.Remove("VERSION")
			}

			cmd := &cobra.Command{}

			// Capture output
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			err := runVersionBump(cmd, []string{tc.bumpType})

			if tc.expectError {
				if err == nil {
					t.Errorf("%s: expected error", tc.description)
				}
				return
			}

			if err != nil {
				t.Errorf("%s: unexpected error: %v", tc.description, err)
				return
			}

			// Read the updated VERSION file
			updated, err := readVersionFromFile("VERSION")
			if err != nil {
				t.Fatalf("failed to read updated VERSION file: %v", err)
			}

			if updated != tc.expected {
				t.Errorf("%s: expected %s, got %s", tc.description, tc.expected, updated)
			}
		})
	}
}

// TestVersionSetCmd tests the version set command
func TestVersionSetCmd(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "goneat-version-set-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Change to temp directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	tests := []struct {
		name        string
		version     string
		expectError bool
		description string
	}{
		// Valid versions
		{"set_basic", "2.1.0", false, "set basic version"},
		{"set_with_v", "v2.1.0", false, "set version with v prefix"},
		{"set_prerelease", "2.0.0-beta.1", false, "set prerelease version"},
		{"set_build", "2.0.0+build.789", false, "set version with build metadata"},

		// Invalid versions
		{"set_invalid", "invalid", true, "set invalid version"},
		{"set_empty", "", true, "set empty version"},
		{"set_leading_zero", "02.1.0", true, "set version with leading zero"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{}

			// Capture output
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			err := runVersionSet(cmd, []string{tc.version})

			if tc.expectError {
				if err == nil {
					t.Errorf("%s: expected error for invalid version %s", tc.description, tc.version)
				}
				return
			}

			if err != nil {
				t.Errorf("%s: unexpected error: %v", tc.description, err)
				return
			}

			// Read the VERSION file
			set, err := readVersionFromFile("VERSION")
			if err != nil {
				t.Fatalf("failed to read VERSION file: %v", err)
			}

			if set != tc.version {
				t.Errorf("%s: expected %s, got %s", tc.description, tc.version, set)
			}
		})
	}
}

// TestCLIParityWithOldImplementation tests that CLI commands produce identical results to before
func TestCLIParityWithOldImplementation(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "goneat-cli-parity-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Change to temp directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	// Test cases that should match old behavior
	testCases := []struct {
		name         string
		initial      string
		command      string
		args         []string
		expectedFile string
		description  string
	}{
		{"validate_basic", "", "validate", []string{"1.2.3"}, "", "validate basic version"},
		{"validate_with_v", "", "validate", []string{"v1.2.3"}, "", "validate version with v prefix"},
		{"validate_prerelease", "", "validate", []string{"1.0.0-alpha.1"}, "", "validate prerelease version"},
		{"set_basic", "", "set", []string{"1.2.3"}, "1.2.3", "set basic version"},
		{"set_with_v", "", "set", []string{"v1.2.3"}, "v1.2.3", "set version with v prefix"},
		{"bump_patch", "1.2.3", "bump", []string{"patch"}, "1.2.4", "bump patch version"},
		{"bump_minor", "1.2.3", "bump", []string{"minor"}, "1.3.0", "bump minor version"},
		{"bump_major", "1.2.3", "bump", []string{"major"}, "2.0.0", "bump major version"},
		{"bump_with_v", "v1.2.3", "bump", []string{"patch"}, "v1.2.4", "bump preserves v prefix"},
		{"bump_clears_prerelease", "1.2.3-alpha.1", "bump", []string{"patch"}, "1.2.4", "bump clears prerelease"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup initial VERSION file if needed
			if tc.initial != "" {
				err := writeVersionToFile("VERSION", tc.initial)
				if err != nil {
					t.Fatalf("failed to setup initial VERSION file: %v", err)
				}
			}

			cmd := &cobra.Command{}

			// Capture output
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			var err error
			switch tc.command {
			case "validate":
				err = runVersionValidate(cmd, tc.args)
			case "set":
				err = runVersionSet(cmd, tc.args)
			case "bump":
				err = runVersionBump(cmd, tc.args)
			default:
				t.Fatalf("unknown command: %s", tc.command)
			}

			if tc.expectedFile == "" {
				// For validate commands, just check no error
				if err != nil {
					t.Errorf("%s: unexpected error: %v", tc.description, err)
				}
			} else {
				// For set/bump commands, check the file content
				if err != nil {
					t.Errorf("%s: unexpected error: %v", tc.description, err)
					return
				}

				content, err := readVersionFromFile("VERSION")
				if err != nil {
					t.Fatalf("failed to read VERSION file: %v", err)
				}

				if content != tc.expectedFile {
					t.Errorf("%s: expected VERSION file to contain %s, got %s", tc.description, tc.expectedFile, content)
				}
			}
		})
	}
}

// TestVersionInitCmd tests the version init command
func TestVersionInitCmd(t *testing.T) {
	tests := []struct {
		name        string
		template    string
		initialVer  string
		expectError bool
		description string
	}{
		{"init_basic", "basic", "0.1.0", false, "initialize basic version management"},
		{"init_custom_version", "basic", "2.1.0", false, "initialize with custom version"},
		{"init_invalid_template", "invalid", "0.1.0", true, "invalid template should error"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create temporary directory for testing
			tempDir, initErr := os.MkdirTemp("", "goneat-init-test")
			if initErr != nil {
				t.Fatalf("failed to create temp dir: %v", initErr)
			}
			defer func() { _ = os.RemoveAll(tempDir) }()

			// Change to temp directory
			oldWd, err := os.Getwd()
			if err != nil {
				t.Fatalf("failed to get working dir: %v", err)
			}
			if err := os.Chdir(tempDir); err != nil {
				t.Fatalf("failed to chdir: %v", err)
			}
			defer func() { _ = os.Chdir(oldWd) }()

			cmd := &cobra.Command{}
			// Set up the flags that runVersionInit expects
			cmd.Flags().String("initial-version", tc.initialVer, "")
			cmd.Flags().Bool("dry-run", false, "")
			cmd.Flags().Bool("force", true, "") // Force to avoid conflicts

			// Capture output
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			args := []string{}
			if tc.template != "" {
				args = append(args, tc.template)
			}

			err = runVersionInit(cmd, args)

			if tc.expectError {
				if err == nil {
					t.Errorf("%s: expected error", tc.description)
				}
				return
			}

			if err != nil {
				t.Errorf("%s: unexpected error: %v", tc.description, err)
				return
			}

			// Check VERSION file was created
			content, err := readVersionFromFile("VERSION")
			if err != nil {
				t.Fatalf("VERSION file not created: %v", err)
			}

			if content != tc.initialVer {
				t.Errorf("%s: expected VERSION file to contain %s, got %s", tc.description, tc.initialVer, content)
			}
		})
	}
}

// TestVersionCheckConsistencyCmd tests the version check-consistency command
func TestVersionCheckConsistencyCmd(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "goneat-consistency-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Change to temp directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	tests := []struct {
		name        string
		version     string
		expectError bool
		description string
	}{
		{"consistency_basic", "1.2.3", false, "check consistency with VERSION file"},
		{"consistency_missing", "", true, "missing VERSION file should error"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup VERSION file if needed
			if tc.version != "" {
				err := writeVersionToFile("VERSION", tc.version)
				if err != nil {
					t.Fatalf("failed to write VERSION file: %v", err)
				}
			} else {
				_ = os.Remove("VERSION")
			}

			cmd := &cobra.Command{}

			// Capture output
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			err := runVersionCheckConsistency(cmd, []string{})

			if tc.expectError {
				if err == nil {
					t.Errorf("%s: expected error", tc.description)
				}
				return
			}

			if err != nil {
				t.Errorf("%s: unexpected error: %v", tc.description, err)
				return
			}

			// Check output contains expected information
			output := buf.String()
			if !strings.Contains(output, tc.version) {
				t.Errorf("%s: output should contain version %s", tc.description, tc.version)
			}
		})
	}
}

// TestVersionCmdIntegration tests end-to-end integration of version commands
func TestVersionCmdIntegration(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "goneat-integration-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Change to temp directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	// Test full workflow: init -> validate -> bump -> set -> check-consistency
	t.Run("full_workflow", func(t *testing.T) {
		// 1. Initialize version management
		initCmd := &cobra.Command{}
		initCmd.Flags().String("initial-version", "0.1.0", "")
		initCmd.Flags().Bool("dry-run", false, "")
		initCmd.Flags().Bool("force", true, "") // Force to avoid conflicts
		var buf bytes.Buffer
		initCmd.SetOut(&buf)
		initCmd.SetErr(&buf)

		err := runVersionInit(initCmd, []string{"basic"})
		if err != nil {
			t.Fatalf("init failed: %v", err)
		}

		// Verify VERSION file
		content, err := readVersionFromFile("VERSION")
		if err != nil || content != "0.1.0" {
			t.Fatalf("init did not create correct VERSION file: %v", err)
		}

		// 2. Validate the version
		validateCmd := &cobra.Command{}
		buf.Reset()
		validateCmd.SetOut(&buf)
		validateCmd.SetErr(&buf)

		err = runVersionValidate(validateCmd, []string{"0.1.0"})
		if err != nil {
			t.Fatalf("validate failed: %v", err)
		}

		// 3. Bump patch version
		bumpCmd := &cobra.Command{}
		buf.Reset()
		bumpCmd.SetOut(&buf)
		bumpCmd.SetErr(&buf)

		err = runVersionBump(bumpCmd, []string{"patch"})
		if err != nil {
			t.Fatalf("bump patch failed: %v", err)
		}

		// Verify version was bumped
		content, err = readVersionFromFile("VERSION")
		if err != nil || content != "0.1.1" {
			t.Fatalf("bump patch did not update VERSION file correctly: got %s, want 0.1.1", content)
		}

		// 4. Set specific version
		setCmd := &cobra.Command{}
		buf.Reset()
		setCmd.SetOut(&buf)
		setCmd.SetErr(&buf)

		err = runVersionSet(setCmd, []string{"1.0.0"})
		if err != nil {
			t.Fatalf("set version failed: %v", err)
		}

		// Verify version was set
		content, err = readVersionFromFile("VERSION")
		if err != nil || content != "1.0.0" {
			t.Fatalf("set version did not update VERSION file correctly: got %s, want 1.0.0", content)
		}

		// 5. Check consistency
		checkCmd := &cobra.Command{}
		buf.Reset()
		checkCmd.SetOut(&buf)
		checkCmd.SetErr(&buf)

		err = runVersionCheckConsistency(checkCmd, []string{})
		if err != nil {
			t.Fatalf("check consistency failed: %v", err)
		}

		// Verify output contains version
		output := buf.String()
		if !strings.Contains(output, "1.0.0") {
			t.Errorf("consistency check output should contain version: %s", output)
		}
	})
}

// TestVersionCmdErrorHandling tests error handling in version commands
func TestVersionCmdErrorHandling(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "goneat-error-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Change to temp directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	tests := []struct {
		name        string
		command     string
		args        []string
		setup       func()
		expectError bool
		description string
	}{
		{
			name:        "bump_no_version_file",
			command:     "bump",
			args:        []string{"patch"},
			setup:       func() { _ = os.Remove("VERSION") },
			expectError: true,
			description: "bump should fail without VERSION file",
		},
		{
			name:        "set_invalid_version",
			command:     "set",
			args:        []string{"invalid.version"},
			setup:       func() {},
			expectError: true,
			description: "set should reject invalid version",
		},
		{
			name:        "validate_invalid_version",
			command:     "validate",
			args:        []string{"1.2"},
			setup:       func() {},
			expectError: true,
			description: "validate should reject malformed version",
		},
		{
			name:        "bump_invalid_type",
			command:     "bump",
			args:        []string{"invalid"},
			setup:       func() { _ = writeVersionToFile("VERSION", "1.0.0") },
			expectError: true,
			description: "bump should reject invalid bump type",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			tc.setup()

			cmd := &cobra.Command{}

			// Capture output
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			var err error
			switch tc.command {
			case "validate":
				err = runVersionValidate(cmd, tc.args)
			case "set":
				err = runVersionSet(cmd, tc.args)
			case "bump":
				err = runVersionBump(cmd, tc.args)
			default:
				t.Fatalf("unknown command: %s", tc.command)
			}

			if tc.expectError {
				if err == nil {
					t.Errorf("%s: expected error but got none", tc.description)
				}
			} else {
				if err != nil {
					t.Errorf("%s: unexpected error: %v", tc.description, err)
				}
			}
		})
	}
}

// TestVersionCmdPathTraversalProtection tests path traversal protection
func TestVersionCmdPathTraversalProtection(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "goneat-path-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Change to temp directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	// Test path traversal in filename
	invalidPath := filepath.Join("..", "..", "etc", "passwd")

	// This should fail due to path traversal protection
	err = writeVersionToFile(invalidPath, "1.0.0")
	if err == nil {
		t.Error("writeVersionToFile should reject path traversal attempts")
	}

	// This should also fail
	_, err = readVersionFromFile(invalidPath)
	if err == nil {
		t.Error("readVersionFromFile should reject path traversal attempts")
	}
}
