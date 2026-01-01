package assess

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectRustProject_StandaloneCrate(t *testing.T) {
	// Create a temporary directory with a standalone Cargo.toml
	tmpDir := t.TempDir()

	cargoContent := `[package]
name = "my-crate"
version = "0.1.0"
edition = "2021"

[dependencies]
`
	err := os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte(cargoContent), 0644)
	require.NoError(t, err)

	project := DetectRustProject(tmpDir)
	require.NotNil(t, project)

	assert.Equal(t, filepath.Join(tmpDir, "Cargo.toml"), project.CargoTomlPath)
	assert.Equal(t, tmpDir, project.RootPath)
	assert.False(t, project.IsWorkspace)
	assert.False(t, project.IsWorkspaceMember)
	assert.Equal(t, tmpDir, project.EffectiveRoot())
}

func TestDetectRustProject_WorkspaceRoot(t *testing.T) {
	// Create a workspace root
	tmpDir := t.TempDir()

	cargoContent := `[workspace]
members = ["crates/*"]
resolver = "2"
`
	err := os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte(cargoContent), 0644)
	require.NoError(t, err)

	project := DetectRustProject(tmpDir)
	require.NotNil(t, project)

	assert.True(t, project.IsWorkspace)
	assert.False(t, project.IsWorkspaceMember)
	assert.Equal(t, tmpDir, project.EffectiveRoot())
}

func TestDetectRustProject_WorkspaceMember(t *testing.T) {
	// Create a workspace with a member
	tmpDir := t.TempDir()
	memberDir := filepath.Join(tmpDir, "crates", "my-lib")
	err := os.MkdirAll(memberDir, 0755)
	require.NoError(t, err)

	// Workspace root Cargo.toml
	wsCargoContent := `[workspace]
members = ["crates/*"]
resolver = "2"
`
	err = os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte(wsCargoContent), 0644)
	require.NoError(t, err)

	// Member Cargo.toml (workspace = true in [package])
	memberCargoContent := `[package]
name = "my-lib"
version = "0.1.0"
edition = "2021"
workspace = ".."

[dependencies]
`
	err = os.WriteFile(filepath.Join(memberDir, "Cargo.toml"), []byte(memberCargoContent), 0644)
	require.NoError(t, err)

	project := DetectRustProject(memberDir)
	require.NotNil(t, project)

	assert.False(t, project.IsWorkspace)
	assert.True(t, project.IsWorkspaceMember)
	assert.Equal(t, tmpDir, project.WorkspaceRootPath)
	assert.Equal(t, tmpDir, project.EffectiveRoot(), "EffectiveRoot should return workspace root")
}

func TestDetectRustProject_NoCargoButRsFiles(t *testing.T) {
	// Create a directory with .rs files but no Cargo.toml (rare case)
	tmpDir := t.TempDir()

	// Create a .rs file
	err := os.WriteFile(filepath.Join(tmpDir, "main.rs"), []byte("fn main() {}"), 0644)
	require.NoError(t, err)

	project := DetectRustProject(tmpDir)
	require.NotNil(t, project)

	assert.Empty(t, project.CargoTomlPath)
	assert.Equal(t, tmpDir, project.RootPath)
}

func TestDetectRustProject_NotRust(t *testing.T) {
	// Create a directory with no Rust indicators
	tmpDir := t.TempDir()

	// Create a Go file instead
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)
	require.NoError(t, err)

	project := DetectRustProject(tmpDir)
	assert.Nil(t, project)
}

func TestDetectRustProject_SkipsTargetDir(t *testing.T) {
	// Ensure we don't detect .rs files in target/ directory
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target", "debug")
	err := os.MkdirAll(targetDir, 0755)
	require.NoError(t, err)

	// Create .rs file only in target dir (build artifacts)
	err = os.WriteFile(filepath.Join(targetDir, "build.rs"), []byte("fn main() {}"), 0644)
	require.NoError(t, err)

	project := DetectRustProject(tmpDir)
	assert.Nil(t, project, "Should not detect Rust project from target/ directory")
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b     string
		expected int
	}{
		{"0.14.0", "0.14.0", 0},
		{"0.14.1", "0.14.0", 1},
		{"0.14.0", "0.14.1", -1},
		{"0.15.0", "0.14.9", 1},
		{"1.0.0", "0.99.99", 1},
		{"0.18.0", "0.18.0", 0},
		{"0.21.0", "0.18.0", 1},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			result := compareVersions(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseVersionFromOutput(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"cargo-deny 0.16.2", "0.16.2"},
		{"cargo-audit 0.21.0 (abc123 2024-01-01)", "0.21.0"},
		{"clippy 0.1.85 (abc123 2024-01-01)", "0.1.85"},
		{"no version here", ""},
		{"version 1.2.3-rc1", "1.2.3"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseVersionFromOutput(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseVersionPart(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"0", 0},
		{"1", 1},
		{"14", 14},
		{"0-rc1", 0},
		{"5+build123", 5},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseVersionPart(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFindWorkspaceRoot(t *testing.T) {
	// Create a nested workspace structure
	tmpDir := t.TempDir()
	memberDir := filepath.Join(tmpDir, "crates", "deep", "nested")
	err := os.MkdirAll(memberDir, 0755)
	require.NoError(t, err)

	// Workspace root
	wsCargoContent := `[workspace]
members = ["crates/**"]
`
	err = os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte(wsCargoContent), 0644)
	require.NoError(t, err)

	// Find workspace root from deeply nested member
	root := findWorkspaceRoot(memberDir)
	assert.Equal(t, tmpDir, root)
}

func TestEffectiveRoot_Standalone(t *testing.T) {
	project := &RustProject{
		RootPath:    "/path/to/crate",
		IsWorkspace: false,
	}
	assert.Equal(t, "/path/to/crate", project.EffectiveRoot())
}

func TestEffectiveRoot_WorkspaceRoot(t *testing.T) {
	project := &RustProject{
		RootPath:    "/path/to/workspace",
		IsWorkspace: true,
	}
	assert.Equal(t, "/path/to/workspace", project.EffectiveRoot())
}

func TestEffectiveRoot_WorkspaceMember(t *testing.T) {
	project := &RustProject{
		RootPath:          "/path/to/workspace/crates/lib",
		IsWorkspaceMember: true,
		WorkspaceRootPath: "/path/to/workspace",
	}
	assert.Equal(t, "/path/to/workspace", project.EffectiveRoot())
}
