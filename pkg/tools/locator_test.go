package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveBinary(t *testing.T) {
	// Create a temporary directory to simulate GONEAT_HOME
	tempDir := t.TempDir()
	originalGoneatHome := os.Getenv("GONEAT_HOME")
	defer func() {
		if originalGoneatHome != "" {
			_ = os.Setenv("GONEAT_HOME", originalGoneatHome) // Ignore error in test cleanup
		} else {
			_ = os.Unsetenv("GONEAT_HOME") // Ignore error in test cleanup
		}
	}()

	// Set GONEAT_HOME to our temp directory
	_ = os.Setenv("GONEAT_HOME", tempDir) // Ignore error in test setup

	// Create managed bin directory structure
	binDir := filepath.Join(tempDir, "tools", "bin")
	if err := os.MkdirAll(binDir, 0750); err != nil {
		t.Fatalf("Failed to create bin dir: %v", err)
	}

	// Create a fake syft binary in managed location
	syftVersionDir := filepath.Join(binDir, "syft@1.0.0")
	if err := os.MkdirAll(syftVersionDir, 0750); err != nil {
		t.Fatalf("Failed to create syft version dir: %v", err)
	}

	syftBinaryName := "syft"
	if runtime.GOOS == "windows" {
		syftBinaryName += ".exe"
	}
	syftBinaryPath := filepath.Join(syftVersionDir, syftBinaryName)

	// Create a fake binary (just a script that does nothing)
	fakeBinaryContent := "#!/bin/bash\necho 'fake syft'\n"
	if runtime.GOOS == "windows" {
		fakeBinaryContent = "echo fake syft"
	}
	if err := os.WriteFile(syftBinaryPath, []byte(fakeBinaryContent), 0750); err != nil {
		t.Fatalf("Failed to create fake binary: %v", err)
	}

	tests := []struct {
		name           string
		toolName       string
		opts           ResolveOptions
		envSetup       func()
		expectError    bool
		expectedInPath string // substring that should be in the returned path
	}{
		{
			name:     "env override takes precedence when path exists",
			toolName: "syft",
			opts: ResolveOptions{
				EnvOverride: "TEST_SYFT_PATH",
				AllowPath:   true,
			},
			envSetup: func() {
				// Create a fake binary at the override path
				overridePath := "/tmp/test-syft-override"
				fakeBinaryContent := "#!/bin/bash\necho 'override syft'\n"
				if runtime.GOOS == "windows" {
					fakeBinaryContent = "echo override syft"
					overridePath += ".exe"
				}
				_ = os.WriteFile(overridePath, []byte(fakeBinaryContent), 0750) // Ignore error in test setup
				_ = os.Setenv("TEST_SYFT_PATH", overridePath)                   // Ignore error in test setup
			},
			expectError:    false,
			expectedInPath: "/tmp/test-syft-override",
		},
		{
			name:     "managed binary found when no env override",
			toolName: "syft",
			opts: ResolveOptions{
				AllowPath: true,
			},
			envSetup:       func() {},
			expectError:    false,
			expectedInPath: syftBinaryPath,
		},
		{
			name:     "path fallback when managed not found and allow path true",
			toolName: "go", // 'go' should be in PATH
			opts: ResolveOptions{
				AllowPath: true,
			},
			envSetup:       func() {},
			expectError:    false,
			expectedInPath: "go",
		},
		{
			name:     "error when not found and allow path false",
			toolName: "nonexistent-tool-12345",
			opts: ResolveOptions{
				AllowPath: false,
			},
			envSetup:    func() {},
			expectError: true,
		},
		{
			name:     "env override ignored if path doesn't exist",
			toolName: "syft",
			opts: ResolveOptions{
				EnvOverride: "TEST_SYFT_PATH",
				AllowPath:   true,
			},
			envSetup: func() {
				_ = os.Setenv("TEST_SYFT_PATH", "/nonexistent/path") // Ignore error in test setup
			},
			expectError:    false,
			expectedInPath: syftBinaryPath, // Should fall back to managed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up env
			if tt.opts.EnvOverride != "" {
				_ = os.Unsetenv(tt.opts.EnvOverride) // Ignore error in test cleanup
			}

			// Setup environment
			tt.envSetup()

			// Run resolution
			path, err := ResolveBinary(tt.toolName, tt.opts)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.expectedInPath != "" && !contains(path, tt.expectedInPath) {
				t.Errorf("Expected path to contain %q, got %q", tt.expectedInPath, path)
			}
		})
	}
}

func TestResolveBinary_ErrorMessages(t *testing.T) {
	tempDir := t.TempDir()
	originalGoneatHome := os.Getenv("GONEAT_HOME")
	defer func() {
		if originalGoneatHome != "" {
			_ = os.Setenv("GONEAT_HOME", originalGoneatHome) // Ignore error in test cleanup
		} else {
			_ = os.Unsetenv("GONEAT_HOME") // Ignore error in test cleanup
		}
	}()
	_ = os.Setenv("GONEAT_HOME", tempDir) // Ignore error in test setup

	tests := []struct {
		name            string
		toolName        string
		opts            ResolveOptions
		expectedInError string
	}{
		{
			name:     "error includes env override suggestion",
			toolName: "missing-tool",
			opts: ResolveOptions{
				EnvOverride: "GONEAT_TOOL_MISSING",
				AllowPath:   false,
			},
			expectedInError: "set GONEAT_TOOL_MISSING=/path/to/missing-tool",
		},
		{
			name:     "error includes doctor install suggestion",
			toolName: "missing-tool",
			opts: ResolveOptions{
				AllowPath: false,
			},
			expectedInError: "run 'goneat doctor tools --scope sbom --install'",
		},
		{
			name:     "error includes PATH suggestion when allowed",
			toolName: "missing-tool",
			opts: ResolveOptions{
				AllowPath: true,
			},
			expectedInError: "install missing-tool and ensure it's in your PATH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ResolveBinary(tt.toolName, tt.opts)
			if err == nil {
				t.Errorf("Expected error, got nil")
				return
			}

			if !contains(err.Error(), tt.expectedInError) {
				t.Errorf("Expected error to contain %q, got %q", tt.expectedInError, err.Error())
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsAt(s, substr)))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
