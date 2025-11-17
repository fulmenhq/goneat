package doctor

import (
	"os"
	"path/filepath"
)

// RepoType represents the primary language/type of a repository
type RepoType string

const (
	// RepoTypeGo indicates a Go repository
	RepoTypeGo RepoType = "go"

	// RepoTypePython indicates a Python repository
	RepoTypePython RepoType = "python"

	// RepoTypeTypeScript indicates a TypeScript repository
	RepoTypeTypeScript RepoType = "typescript"

	// RepoTypeRust indicates a Rust repository
	RepoTypeRust RepoType = "rust"

	// RepoTypeCSharp indicates a C# repository
	RepoTypeCSharp RepoType = "csharp"

	// RepoTypeUnknown indicates the repository type could not be determined
	RepoTypeUnknown RepoType = "unknown"
)

// DetectRepoType examines the given directory for language marker files
// and returns the detected repository type.
//
// Detection priority (first match wins):
// 1. Go (go.mod, go.sum)
// 2. Python (pyproject.toml, uv.lock, requirements.txt, setup.py, Pipfile)
// 3. TypeScript (tsconfig.json, package.json)
// 4. Rust (Cargo.toml, Cargo.lock)
// 5. C# (*.csproj, *.sln, *.fsproj)
//
// This priority order matches the Crucible language taxonomy priority
// as defined in foundation-package-managers.yaml.
func DetectRepoType(rootDir string) RepoType {
	// Priority 1: Go
	if fileExists(filepath.Join(rootDir, "go.mod")) ||
		fileExists(filepath.Join(rootDir, "go.sum")) {
		return RepoTypeGo
	}

	// Priority 2: Python
	pyMarkers := []string{
		"pyproject.toml",
		"uv.lock",
		"requirements.txt",
		"setup.py",
		"Pipfile",
	}
	for _, marker := range pyMarkers {
		if fileExists(filepath.Join(rootDir, marker)) {
			return RepoTypePython
		}
	}

	// Priority 3: TypeScript (check tsconfig.json first, then package.json)
	if fileExists(filepath.Join(rootDir, "tsconfig.json")) {
		return RepoTypeTypeScript
	}

	// package.json could be JS or TS, treat as TS-compatible
	if fileExists(filepath.Join(rootDir, "package.json")) {
		return RepoTypeTypeScript
	}

	// Priority 4: Rust
	if fileExists(filepath.Join(rootDir, "Cargo.toml")) ||
		fileExists(filepath.Join(rootDir, "Cargo.lock")) {
		return RepoTypeRust
	}

	// Priority 5: C# (check for common project files)
	csharpMarkers := []string{
		"*.csproj",
		"*.sln",
		"*.fsproj",
	}
	for _, pattern := range csharpMarkers {
		matches, err := filepath.Glob(filepath.Join(rootDir, pattern))
		if err == nil && len(matches) > 0 {
			return RepoTypeCSharp
		}
	}

	return RepoTypeUnknown
}

// DetectCurrentRepoType detects the repository type in the current working directory
func DetectCurrentRepoType() RepoType {
	cwd, err := os.Getwd()
	if err != nil {
		return RepoTypeUnknown
	}
	return DetectRepoType(cwd)
}

// String returns the string representation of the RepoType
func (r RepoType) String() string {
	return string(r)
}

// IsLanguageNative returns true if the repo type has a native package manager
// (go-install for Go, pip/uv for Python, npm for TypeScript, cargo for Rust)
func (r RepoType) IsLanguageNative() bool {
	switch r {
	case RepoTypeGo, RepoTypePython, RepoTypeTypeScript, RepoTypeRust:
		return true
	default:
		return false
	}
}

// GetLanguageNativePackageManager returns the native package manager for this repo type
func (r RepoType) GetLanguageNativePackageManager() string {
	switch r {
	case RepoTypeGo:
		return "go-install"
	case RepoTypePython:
		return "uv" // Prefer uv over pip
	case RepoTypeTypeScript:
		return "npm" // Or bun if available
	case RepoTypeRust:
		return "cargo"
	default:
		return ""
	}
}

// fileExists checks if a file exists at the given path
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
