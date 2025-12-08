/*
Copyright © 2025 3 Leaps <info@3leaps.com>
*/
package tools

import (
	"context"
	"os"
	"testing"
)

func TestGetModeFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     ExecutionMode
	}{
		{"empty defaults to auto", "", ModeAuto},
		{"local mode", "local", ModeLocal},
		{"docker mode", "docker", ModeDocker},
		{"auto mode", "auto", ModeAuto},
		{"case insensitive", "LOCAL", ModeLocal},
		{"unknown defaults to auto", "unknown", ModeAuto},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore env
			orig := os.Getenv("GONEAT_TOOL_MODE")
			defer func() {
				if orig == "" {
					_ = os.Unsetenv("GONEAT_TOOL_MODE")
				} else {
					_ = os.Setenv("GONEAT_TOOL_MODE", orig)
				}
			}()

			if tt.envValue == "" {
				_ = os.Unsetenv("GONEAT_TOOL_MODE")
			} else {
				_ = os.Setenv("GONEAT_TOOL_MODE", tt.envValue)
			}

			got := getModeFromEnv()
			if got != tt.want {
				t.Errorf("getModeFromEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewExecutor(t *testing.T) {
	tests := []struct {
		name string
		mode ExecutionMode
		want string
	}{
		{"local mode", ModeLocal, "local"},
		{"docker mode", ModeDocker, "docker"},
		{"auto mode", ModeAuto, "auto"},
		{"empty defaults to auto", "", "auto"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear env for consistent tests
			orig := os.Getenv("GONEAT_TOOL_MODE")
			_ = os.Unsetenv("GONEAT_TOOL_MODE")
			defer func() {
				if orig != "" {
					_ = os.Setenv("GONEAT_TOOL_MODE", orig)
				}
			}()

			executor := NewExecutor(tt.mode)
			if executor.Name() != tt.want {
				t.Errorf("NewExecutor(%v).Name() = %v, want %v", tt.mode, executor.Name(), tt.want)
			}
		})
	}
}

func TestLocalExecutor_IsAvailable(t *testing.T) {
	executor := NewLocalExecutor()

	// "go" should be available in most dev environments
	// Skip if not available
	if !executor.IsAvailable("go") {
		t.Skip("go not available, skipping")
	}

	if !executor.IsAvailable("go") {
		t.Error("expected 'go' to be available")
	}

	// Nonexistent tool should not be available
	if executor.IsAvailable("nonexistent-tool-xyz-123") {
		t.Error("expected nonexistent tool to not be available")
	}
}

func TestLocalExecutor_Execute(t *testing.T) {
	executor := NewLocalExecutor()

	// Skip if echo not available (should be available on all systems)
	if !executor.IsAvailable("echo") {
		t.Skip("echo not available, skipping")
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, ExecuteOptions{
		Tool: "echo",
		Args: []string{"hello", "world"},
	})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %v, want 0", result.ExitCode)
	}

	if result.Executor != "local" {
		t.Errorf("Execute() executor = %v, want local", result.Executor)
	}

	// Check output contains expected text
	output := string(result.Stdout)
	if output != "hello world\n" {
		t.Errorf("Execute() stdout = %q, want %q", output, "hello world\n")
	}
}

func TestDockerExecutor_isToolInImage(t *testing.T) {
	executor := NewDockerExecutor()

	tests := []struct {
		tool string
		want bool
	}{
		{"prettier", true},
		{"yamlfmt", true},
		{"jq", true},
		{"yq", true},
		{"rg", true},
		{"ripgrep", true},
		{"git", true},
		{"bash", true},
		{"nonexistent", false},
		{"npm", false}, // Not in minimal image
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			got := executor.isToolInImage(tt.tool)
			if got != tt.want {
				t.Errorf("isToolInImage(%q) = %v, want %v", tt.tool, got, tt.want)
			}
		})
	}
}

func TestDockerExecutor_GetImage(t *testing.T) {
	// Test default image
	orig := os.Getenv(ToolsImageEnvVar)
	_ = os.Unsetenv(ToolsImageEnvVar)
	defer func() {
		if orig != "" {
			_ = os.Setenv(ToolsImageEnvVar, orig)
		}
	}()

	executor := NewDockerExecutor()
	if executor.GetImage() != DefaultToolsImage {
		t.Errorf("GetImage() = %v, want %v", executor.GetImage(), DefaultToolsImage)
	}

	// Test custom image
	_ = os.Setenv(ToolsImageEnvVar, "custom/image:v1")
	executor = NewDockerExecutor()
	if executor.GetImage() != "custom/image:v1" {
		t.Errorf("GetImage() = %v, want custom/image:v1", executor.GetImage())
	}
}

func TestAutoExecutor_IsAvailable(t *testing.T) {
	executor := NewAutoExecutor()

	// Should be available if either local or docker can handle it
	// "prettier" is in docker image, so should be available if docker is installed
	// or if prettier is locally installed

	// "go" should be available locally
	if executor.local.IsAvailable("go") {
		if !executor.IsAvailable("go") {
			t.Error("expected 'go' to be available via auto executor")
		}
	}
}

func TestIsInCI(t *testing.T) {
	// Save original env
	origCI := os.Getenv("CI")
	origGH := os.Getenv("GITHUB_ACTIONS")
	defer func() {
		if origCI == "" {
			_ = os.Unsetenv("CI")
		} else {
			_ = os.Setenv("CI", origCI)
		}
		if origGH == "" {
			_ = os.Unsetenv("GITHUB_ACTIONS")
		} else {
			_ = os.Setenv("GITHUB_ACTIONS", origGH)
		}
	}()

	// Clear CI vars
	_ = os.Unsetenv("CI")
	_ = os.Unsetenv("GITHUB_ACTIONS")

	if isInCI() {
		t.Error("expected isInCI() = false when CI vars not set")
	}

	// Set CI=true
	_ = os.Setenv("CI", "true")
	if !isInCI() {
		t.Error("expected isInCI() = true when CI=true")
	}

	_ = os.Unsetenv("CI")
	_ = os.Setenv("GITHUB_ACTIONS", "true")
	if !isInCI() {
		t.Error("expected isInCI() = true when GITHUB_ACTIONS=true")
	}
}
