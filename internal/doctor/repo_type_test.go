package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectRepoType_Go(t *testing.T) {
	// Create temporary directory with go.mod
	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module example.com/test\n"), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	repoType := DetectRepoType(tmpDir)
	if repoType != RepoTypeGo {
		t.Errorf("Expected Go repository, got %s", repoType)
	}
}

func TestDetectRepoType_Python(t *testing.T) {
	tests := []struct {
		name   string
		marker string
	}{
		{"pyproject.toml", "pyproject.toml"},
		{"uv.lock", "uv.lock"},
		{"requirements.txt", "requirements.txt"},
		{"setup.py", "setup.py"},
		{"Pipfile", "Pipfile"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			markerPath := filepath.Join(tmpDir, tt.marker)
			if err := os.WriteFile(markerPath, []byte{}, 0644); err != nil {
				t.Fatalf("Failed to create %s: %v", tt.marker, err)
			}

			repoType := DetectRepoType(tmpDir)
			if repoType != RepoTypePython {
				t.Errorf("Expected Python repository for %s, got %s", tt.marker, repoType)
			}
		})
	}
}

func TestDetectRepoType_TypeScript(t *testing.T) {
	tests := []struct {
		name   string
		marker string
	}{
		{"tsconfig.json", "tsconfig.json"},
		{"package.json", "package.json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			markerPath := filepath.Join(tmpDir, tt.marker)
			if err := os.WriteFile(markerPath, []byte("{}"), 0644); err != nil {
				t.Fatalf("Failed to create %s: %v", tt.marker, err)
			}

			repoType := DetectRepoType(tmpDir)
			if repoType != RepoTypeTypeScript {
				t.Errorf("Expected TypeScript repository for %s, got %s", tt.marker, repoType)
			}
		})
	}
}

func TestDetectRepoType_Rust(t *testing.T) {
	tests := []struct {
		name   string
		marker string
	}{
		{"Cargo.toml", "Cargo.toml"},
		{"Cargo.lock", "Cargo.lock"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			markerPath := filepath.Join(tmpDir, tt.marker)
			if err := os.WriteFile(markerPath, []byte(""), 0644); err != nil {
				t.Fatalf("Failed to create %s: %v", tt.marker, err)
			}

			repoType := DetectRepoType(tmpDir)
			if repoType != RepoTypeRust {
				t.Errorf("Expected Rust repository for %s, got %s", tt.marker, repoType)
			}
		})
	}
}

func TestDetectRepoType_CSharp(t *testing.T) {
	tests := []struct {
		name   string
		marker string
	}{
		{"csproj", "test.csproj"},
		{"sln", "test.sln"},
		{"fsproj", "test.fsproj"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			markerPath := filepath.Join(tmpDir, tt.marker)
			if err := os.WriteFile(markerPath, []byte(""), 0644); err != nil {
				t.Fatalf("Failed to create %s: %v", tt.marker, err)
			}

			repoType := DetectRepoType(tmpDir)
			if repoType != RepoTypeCSharp {
				t.Errorf("Expected C# repository for %s, got %s", tt.marker, repoType)
			}
		})
	}
}

func TestDetectRepoType_Unknown(t *testing.T) {
	tmpDir := t.TempDir()
	// Empty directory with no marker files

	repoType := DetectRepoType(tmpDir)
	if repoType != RepoTypeUnknown {
		t.Errorf("Expected Unknown repository, got %s", repoType)
	}
}

func TestDetectRepoType_Priority(t *testing.T) {
	// Test that Go has higher priority than Python
	tmpDir := t.TempDir()

	// Create both go.mod and requirements.txt
	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module test\n"), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	reqPath := filepath.Join(tmpDir, "requirements.txt")
	if err := os.WriteFile(reqPath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create requirements.txt: %v", err)
	}

	repoType := DetectRepoType(tmpDir)
	if repoType != RepoTypeGo {
		t.Errorf("Expected Go repository (higher priority), got %s", repoType)
	}
}

func TestRepoType_String(t *testing.T) {
	tests := []struct {
		repoType RepoType
		expected string
	}{
		{RepoTypeGo, "go"},
		{RepoTypePython, "python"},
		{RepoTypeTypeScript, "typescript"},
		{RepoTypeRust, "rust"},
		{RepoTypeCSharp, "csharp"},
		{RepoTypeUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.repoType.String() != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.repoType.String())
			}
		})
	}
}

func TestRepoType_IsLanguageNative(t *testing.T) {
	tests := []struct {
		repoType RepoType
		expected bool
	}{
		{RepoTypeGo, true},
		{RepoTypePython, true},
		{RepoTypeTypeScript, true},
		{RepoTypeRust, true},
		{RepoTypeCSharp, false},
		{RepoTypeUnknown, false},
	}

	for _, tt := range tests {
		t.Run(tt.repoType.String(), func(t *testing.T) {
			if tt.repoType.IsLanguageNative() != tt.expected {
				t.Errorf("Expected IsLanguageNative()=%v for %s, got %v",
					tt.expected, tt.repoType, tt.repoType.IsLanguageNative())
			}
		})
	}
}

func TestRepoType_GetLanguageNativePackageManager(t *testing.T) {
	tests := []struct {
		repoType RepoType
		expected string
	}{
		{RepoTypeGo, "go-install"},
		{RepoTypePython, "uv"},
		{RepoTypeTypeScript, "npm"},
		{RepoTypeRust, "cargo"},
		{RepoTypeCSharp, ""},
		{RepoTypeUnknown, ""},
	}

	for _, tt := range tests {
		t.Run(tt.repoType.String(), func(t *testing.T) {
			result := tt.repoType.GetLanguageNativePackageManager()
			if result != tt.expected {
				t.Errorf("Expected %s for %s, got %s",
					tt.expected, tt.repoType, result)
			}
		})
	}
}

func TestDetectCurrentRepoType(t *testing.T) {
	// Find the repo root by walking up from current directory
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	// Walk up to find go.mod (repo root)
	repoRoot := cwd
	for !fileExists(filepath.Join(repoRoot, "go.mod")) {
		parent := filepath.Dir(repoRoot)
		if parent == repoRoot {
			// Reached filesystem root without finding go.mod
			t.Skip("Running outside of goneat repository, skipping test")
		}
		repoRoot = parent
	}

	// Test detection from repo root
	repoType := DetectRepoType(repoRoot)
	if repoType != RepoTypeGo {
		t.Errorf("Expected Go repository for goneat project, got %s", repoType)
	}
}
