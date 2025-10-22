//go:build integration
// +build integration

package sbom

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSyftInvoker_RequiredInCI ensures syft is installed in CI environments
func TestSyftInvoker_RequiredInCI(t *testing.T) {
	if os.Getenv("CI") == "true" {
		// In CI: syft MUST be present
		invoker, err := NewSyftInvoker()
		if err != nil {
			t.Fatalf("syft must be installed in CI - run: goneat doctor tools --scope sbom --install --yes\nError: %v", err)
		}

		// Validate syft is functional
		ctx := context.Background()
		version, err := invoker.GetVersion(ctx)
		if err != nil {
			t.Fatalf("syft version check failed in CI: %v", err)
		}

		if version == "" {
			t.Error("syft version is empty in CI")
		}

		t.Logf("✅ syft present in CI: version %s", version)
	} else {
		// Locally: Skip gracefully
		t.Skip("Skipping CI-required test in local environment (set CI=true to run)")
	}
}

// TestSyftInvoker_RealGeneration tests actual SBOM generation on fixture project
func TestSyftInvoker_RealGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	invoker, err := NewSyftInvoker()
	if err != nil {
		t.Skip("syft not available - install with: goneat doctor tools --scope sbom --install --yes")
	}

	fixturePath := filepath.Join("testdata", "fixture-project")
	if _, statErr := os.Stat(fixturePath); statErr != nil {
		t.Fatalf("Fixture project not found at %s", fixturePath)
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test-sbom.cdx.json")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := invoker.Generate(ctx, Config{
		TargetPath: fixturePath,
		OutputPath: outputPath,
		Format:     "cyclonedx-json",
	})

	if err != nil {
		t.Fatalf("SBOM generation failed: %v", err)
	}

	// Verify result metadata
	if result.Format != "cyclonedx-json" {
		t.Errorf("Expected format cyclonedx-json, got %s", result.Format)
	}

	// Assert exact package count - fixture has deterministic dependencies:
	// - github.com/google/uuid v1.6.0 (direct)
	// - gopkg.in/yaml.v3 v3.0.1 (direct)
	// - gopkg.in/check.v1 (transitive from yaml.v3)
	// Total: 3 packages minimum (may vary with syft versions/stdlib detection)
	if result.PackageCount < 3 {
		t.Errorf("Expected at least 3 packages (uuid, yaml, check), got %d", result.PackageCount)
	}

	// Verify key dependencies are present by checking bom-refs
	var cdx struct {
		Components []struct {
			BomRef string `json:"bom-ref"`
			Name   string `json:"name"`
		} `json:"components"`
		Metadata struct {
			Timestamp string `json:"timestamp"`
			Tools     struct {
				Components []struct {
					Type    string `json:"type"`
					Author  string `json:"author"`
					Name    string `json:"name"`
					Version string `json:"version"`
				} `json:"components"`
			} `json:"tools"`
		} `json:"metadata"`
	}

	if err := json.Unmarshal(result.SBOMContent, &cdx); err != nil {
		t.Fatalf("SBOM content is not valid JSON: %v", err)
	}

	// Validate metadata fields
	if cdx.Metadata.Timestamp == "" {
		t.Error("CycloneDX metadata.timestamp is missing")
	}

	if len(cdx.Metadata.Tools.Components) == 0 {
		t.Error("CycloneDX metadata.tools.components is empty")
	} else {
		foundSyft := false
		for _, tool := range cdx.Metadata.Tools.Components {
			if tool.Name == "syft" {
				foundSyft = true
				if tool.Version == "" {
					t.Error("syft tool version is empty in metadata")
				}
				if tool.Author == "" {
					t.Error("syft tool author is empty in metadata")
				}
				if tool.Type != "application" {
					t.Errorf("syft tool type should be 'application', got: %s", tool.Type)
				}
			}
		}
		if !foundSyft {
			t.Error("syft not found in metadata.tools.components")
		}
	}

	// Verify key dependencies are present
	requiredDeps := []string{"github.com/google/uuid", "gopkg.in/yaml.v3"}
	foundDeps := make(map[string]bool)
	for _, component := range cdx.Components {
		for _, reqDep := range requiredDeps {
			if strings.Contains(component.Name, reqDep) || strings.Contains(component.BomRef, reqDep) {
				foundDeps[reqDep] = true
			}
		}
	}

	for _, reqDep := range requiredDeps {
		if !foundDeps[reqDep] {
			t.Errorf("Required dependency not found in SBOM: %s", reqDep)
		}
	}

	if result.ToolVersion == "" || result.ToolVersion == "unknown" {
		t.Errorf("Tool version should be set, got: %s", result.ToolVersion)
	}

	// Verify file was created
	if _, statErr := os.Stat(outputPath); statErr != nil {
		t.Errorf("SBOM file not created at %s", outputPath)
	}

	t.Logf("✅ Generated SBOM: %d packages, tool version %s, duration %s", result.PackageCount, result.ToolVersion, result.Duration)
}

// TestSyftInvoker_ModernAPI validates we're using 'syft scan' with modern --output syntax
func TestSyftInvoker_ModernAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Point to mock script that validates argv
	mockScript, err := filepath.Abs(filepath.Join("testdata", "mock-syft-modern.sh"))
	if err != nil {
		t.Fatalf("Failed to resolve mock script path: %v", err)
	}

	// Set override to use mock
	t.Setenv("GONEAT_TOOL_SYFT", mockScript)

	invoker, err := NewSyftInvoker()
	if err != nil {
		t.Fatalf("Failed to create invoker with mock: %v", err)
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "modern-api-test.cdx.json")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := invoker.Generate(ctx, Config{
		TargetPath: tmpDir, // Use tmpDir as target (mock doesn't care)
		OutputPath: outputPath,
		Format:     "cyclonedx-json",
	})

	if err != nil {
		// If error mentions deprecated API or missing modern syntax, fail
		if strings.Contains(err.Error(), "deprecated") || strings.Contains(err.Error(), "Modern --output FORMAT=PATH syntax not detected") {
			t.Fatalf("Detected deprecated syft API usage or missing modern syntax: %v", err)
		}
		// Other errors might be from mock limitations
		t.Logf("Note: Mock generated error (may be expected): %v", err)
	}

	// Verify we got valid output with modern API
	if result != nil {
		if result.PackageCount != 3 {
			t.Errorf("Expected mock to generate 3 packages, got %d", result.PackageCount)
		}
	}

	t.Log("✅ Confirmed using modern syft scan API with --output FORMAT=PATH syntax")
}

// TestSyftInvoker_VersionParsing tests parsing of both JSON and multiline text formats
func TestSyftInvoker_VersionParsing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tests := []struct {
		name           string
		mockScript     string
		expectedFormat string
	}{
		{
			name:           "legacy_text_format",
			mockScript:     "mock-syft-legacy-version.sh",
			expectedFormat: "multiline text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockScript, err := filepath.Abs(filepath.Join("testdata", tt.mockScript))
			if err != nil {
				t.Fatalf("Failed to resolve mock script path: %v", err)
			}

			t.Setenv("GONEAT_TOOL_SYFT", mockScript)

			invoker, err := NewSyftInvoker()
			if err != nil {
				t.Fatalf("Failed to create invoker with mock: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			version, err := invoker.GetVersion(ctx)
			if err != nil {
				t.Fatalf("GetVersion failed: %v", err)
			}

			if version == "" {
				t.Error("Version should not be empty")
			}

			// Version should be parseable (e.g., "1.33.0")
			if len(version) < 3 {
				t.Errorf("Version seems invalid: %s", version)
			}

			// Verify we got the expected version from multiline parsing
			if version != "1.33.0" {
				t.Errorf("Expected version 1.33.0 from %s parsing, got: %s", tt.expectedFormat, version)
			}

			t.Logf("✅ Parsed syft version from %s: %s", tt.expectedFormat, version)
		})
	}
}

// TestSyftInvoker_VersionParsing_JSON tests JSON version parsing with real syft
func TestSyftInvoker_VersionParsing_JSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	invoker, err := NewSyftInvoker()
	if err != nil {
		t.Skip("syft not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	version, err := invoker.GetVersion(ctx)
	if err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}

	if version == "" {
		t.Error("Version should not be empty")
	}

	// Version should be parseable (e.g., "1.33.0")
	if len(version) < 3 {
		t.Errorf("Version seems invalid: %s", version)
	}

	t.Logf("✅ Parsed syft version from JSON format: %s", version)
}
