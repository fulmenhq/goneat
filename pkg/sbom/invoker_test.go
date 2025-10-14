package sbom

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewSyftInvoker(t *testing.T) {
	_, err := NewSyftInvoker()
	if err != nil {
		t.Skipf("Syft not installed: %v", err)
	}
}

func TestSyftInvoker_GetVersion(t *testing.T) {
	invoker, err := NewSyftInvoker()
	if err != nil {
		t.Skipf("Syft not installed: %v", err)
	}

	ctx := context.Background()
	version, err := invoker.GetVersion(ctx)
	if err != nil {
		t.Fatalf("Failed to get version: %v", err)
	}

	if version == "" {
		t.Error("Expected non-empty version")
	}

	t.Logf("Syft version: %s", version)
}

func TestSyftInvoker_Generate(t *testing.T) {
	invoker, err := NewSyftInvoker()
	if err != nil {
		t.Skipf("Syft not installed: %v", err)
	}

	tmpDir := t.TempDir()
	testFixture := createTestFixture(t, tmpDir)

	outputPath := filepath.Join(tmpDir, "test.cdx.json")

	config := Config{
		TargetPath: testFixture,
		OutputPath: outputPath,
		Format:     "cyclonedx-json",
		Stdout:     false,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := invoker.Generate(ctx, config)
	if err != nil {
		t.Fatalf("Failed to generate SBOM: %v", err)
	}

	if result.OutputPath != outputPath {
		t.Errorf("Expected output path %s, got %s", outputPath, result.OutputPath)
	}

	if result.Format != "cyclonedx-json" {
		t.Errorf("Expected format cyclonedx-json, got %s", result.Format)
	}

	if result.ToolVersion == "" {
		t.Error("Expected non-empty tool version")
	}

	if result.Duration == 0 {
		t.Error("Expected non-zero duration")
	}

	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Errorf("Expected SBOM file to exist at %s", outputPath)
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read SBOM: %v", err)
	}

	var sbom map[string]interface{}
	if err := json.Unmarshal(content, &sbom); err != nil {
		t.Fatalf("Failed to parse SBOM JSON: %v", err)
	}

	t.Logf("SBOM generated: %d packages, took %v", result.PackageCount, result.Duration)
}

func TestSyftInvoker_GenerateStdout(t *testing.T) {
	invoker, err := NewSyftInvoker()
	if err != nil {
		t.Skipf("Syft not installed: %v", err)
	}

	tmpDir := t.TempDir()
	testFixture := createTestFixture(t, tmpDir)

	config := Config{
		TargetPath: testFixture,
		Format:     "cyclonedx-json",
		Stdout:     true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := invoker.Generate(ctx, config)
	if err != nil {
		t.Fatalf("Failed to generate SBOM: %v", err)
	}

	if len(result.SBOMContent) == 0 {
		t.Error("Expected non-empty SBOM content")
	}

	var sbom map[string]interface{}
	if err := json.Unmarshal(result.SBOMContent, &sbom); err != nil {
		t.Fatalf("Failed to parse SBOM JSON: %v", err)
	}

	t.Logf("SBOM generated to stdout: %d bytes, %d packages", len(result.SBOMContent), result.PackageCount)
}

func TestExtractPackageCount(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		format        string
		expectedCount int
		expectError   bool
	}{
		{
			name:          "valid cyclonedx",
			content:       `{"components": [{"name": "pkg1"}, {"name": "pkg2"}]}`,
			format:        "cyclonedx-json",
			expectedCount: 2,
			expectError:   false,
		},
		{
			name:          "empty components",
			content:       `{"components": []}`,
			format:        "cyclonedx-json",
			expectedCount: 0,
			expectError:   false,
		},
		{
			name:        "unsupported format",
			content:     `{}`,
			format:      "spdx-json",
			expectError: true,
		},
		{
			name:        "invalid json",
			content:     `{invalid}`,
			format:      "cyclonedx-json",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := extractPackageCount(json.RawMessage(tt.content), tt.format)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if count != tt.expectedCount {
				t.Errorf("Expected count %d, got %d", tt.expectedCount, count)
			}
		})
	}
}

func createTestFixture(t *testing.T, dir string) string {
	t.Helper()

	goModContent := `module example.com/test

go 1.22

require (
	github.com/spf13/cobra v1.8.0
)
`

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goModContent), 0o600); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	mainContent := `package main

import "fmt"

func main() {
	fmt.Println("test")
}
`

	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(mainContent), 0o600); err != nil {
		t.Fatalf("Failed to create main.go: %v", err)
	}

	return dir
}
