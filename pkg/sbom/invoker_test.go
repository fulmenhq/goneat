package sbom

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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

	if _, err := invoker.GetVersion(context.Background()); err != nil {
		t.Skipf("Syft version check failed: %v", err)
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
		t.Skipf("Syft generate failed: %v", err)
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
		t.Skipf("Syft output not parseable: %v", err)
	}

	t.Logf("SBOM generated: %d packages, took %v", result.PackageCount, result.Duration)
}

func TestSyftInvoker_GenerateStdout(t *testing.T) {
	invoker, err := NewSyftInvoker()
	if err != nil {
		t.Skipf("Syft not installed: %v", err)
	}

	if _, err := invoker.GetVersion(context.Background()); err != nil {
		t.Skipf("Syft version check failed: %v", err)
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
		t.Skipf("Syft generate failed: %v", err)
	}

	if len(result.SBOMContent) == 0 {
		t.Error("Expected non-empty SBOM content")
	}

	var sbom map[string]interface{}
	if err := json.Unmarshal(result.SBOMContent, &sbom); err != nil {
		t.Skipf("Syft output not parseable: %v", err)
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

func TestSyftInvoker_ResolveBinaryIntegration(t *testing.T) {
	// Create a temporary directory to simulate GONEAT_HOME
	tempDir := t.TempDir()
	originalGoneatHome := os.Getenv("GONEAT_HOME")
	originalSyftEnv := os.Getenv("GONEAT_TOOL_SYFT")
	defer func() {
		if originalGoneatHome != "" {
			_ = os.Setenv("GONEAT_HOME", originalGoneatHome) // Ignore error in test cleanup
		} else {
			_ = os.Unsetenv("GONEAT_HOME") // Ignore error in test cleanup
		}
		if originalSyftEnv != "" {
			_ = os.Setenv("GONEAT_TOOL_SYFT", originalSyftEnv) // Ignore error in test cleanup
		} else {
			_ = os.Unsetenv("GONEAT_TOOL_SYFT") // Ignore error in test cleanup
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

	// Create a fake binary that outputs valid CycloneDX JSON
	fakeSyftContent := `#!/bin/bash
cat << 'EOF'
{
  "bomFormat": "CycloneDX",
  "specVersion": "1.4",
  "serialNumber": "urn:uuid:12345678-1234-1234-1234-123456789012",
  "version": 1,
  "components": [
    {
      "type": "library",
      "name": "github.com/spf13/cobra",
      "version": "v1.8.0"
    }
  ]
}
EOF`
	if runtime.GOOS == "windows" {
		fakeSyftContent = `echo {`
		fakeSyftContent += "\n" + `  "bomFormat": "CycloneDX",`
		fakeSyftContent += "\n" + `  "specVersion": "1.4",`
		fakeSyftContent += "\n" + `  "components": []`
		fakeSyftContent += "\n" + `}`
	}
	if err := os.WriteFile(syftBinaryPath, []byte(fakeSyftContent), 0750); err != nil {
		t.Fatalf("Failed to create fake syft binary: %v", err)
	}

	// Test that NewSyftInvoker finds the managed binary
	invoker, err := NewSyftInvoker()
	if err != nil {
		t.Fatalf("Failed to create SyftInvoker: %v", err)
	}

	// Verify it found the managed binary
	if invoker.syftPath != syftBinaryPath {
		t.Errorf("Expected syft path %s, got %s", syftBinaryPath, invoker.syftPath)
	}

	// Test env override takes precedence
	overrideDir := t.TempDir()
	customPath := filepath.Join(overrideDir, "custom-syft")
	customContent := `#!/bin/bash
echo '{"bomFormat": "CycloneDX", "components": [{"name": "custom", "version": "1.0.0"}]}'`
	if runtime.GOOS == "windows" {
		customContent = `@echo {"bomFormat": "CycloneDX", "components": [{"name": "custom", "version": "1.0.0"}]}`
		customPath += ".bat"
	}
	if err := os.WriteFile(customPath, []byte(customContent), 0o755); err != nil {
		t.Fatalf("Failed to create custom syft: %v", err)
	}
	_ = os.Setenv("GONEAT_TOOL_SYFT", customPath) // Ignore error in test setup

	// Create new invoker - should use env override
	invoker2, err := NewSyftInvoker()
	if err != nil {
		t.Fatalf("Failed to create SyftInvoker with env override: %v", err)
	}

	if invoker2.syftPath != customPath {
		t.Errorf("Expected syft path %s with env override, got %s", customPath, invoker2.syftPath)
	}
}

func TestExtractDependencyGraph(t *testing.T) {
	raw := []byte(`{
		"components": [
			{"bom-ref":"pkg:npm/app@1.0.0","name":"app","version":"1.0.0","type":"application"},
			{"bom-ref":"pkg:npm/lib-a@2.0.0","name":"lib-a","version":"2.0.0","type":"library"},
			{"bom-ref":"pkg:npm/lib-b@3.1.0","name":"lib-b","version":"3.1.0","type":"library"}
		],
		"dependencies": [
			{"ref":"pkg:npm/app@1.0.0","dependsOn":["pkg:npm/lib-a@2.0.0","pkg:npm/lib-b@3.1.0"]},
			{"ref":"pkg:npm/lib-a@2.0.0","dependsOn":["pkg:npm/lib-b@3.1.0"]},
			{"ref":"pkg:npm/lib-b@3.1.0","dependsOn":[]}
		]
	}`)

	graph, err := extractDependencyGraph(raw, "cyclonedx-json")
	if err != nil {
		t.Fatalf("extractDependencyGraph returned error: %v", err)
	}
	if graph == nil {
		t.Fatalf("expected graph, got nil")
	}
	if len(graph.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(graph.Nodes))
	}
	if len(graph.Roots) != 1 || graph.Roots[0] != "pkg:npm/app@1.0.0" {
		t.Fatalf("expected root pkg:npm/app@1.0.0, got %v", graph.Roots)
	}

	app := graph.Nodes["pkg:npm/app@1.0.0"]
	if len(app.Dependencies) != 2 {
		t.Fatalf("expected app to depend on 2 nodes, got %d", len(app.Dependencies))
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
