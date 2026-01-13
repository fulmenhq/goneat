package dependencies

import (
	"context"
	"testing"
)

func TestAnalyzerInterface(t *testing.T) {
	// Basic interface compliance test
	var _ Analyzer = (*GoAnalyzer)(nil)
	var _ LanguageDetector = (*Detector)(nil)
}

func TestGoAnalyzer_Analyze(t *testing.T) {
	analyzer := NewGoAnalyzer()
	ctx := context.Background()
	config := AnalysisConfig{Target: "../.."} // Points to repo root

	result, err := analyzer.Analyze(ctx, config.Target, config)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(result.Dependencies) == 0 {
		t.Error("Expected dependencies in result")
	}

	if !result.Passed {
		t.Error("Expected analysis to pass")
	}

	if result.Duration == 0 {
		t.Error("Expected non-zero duration")
	}
}

func TestGoAnalyzer_DetectLanguages(t *testing.T) {
	analyzer := NewGoAnalyzer()

	langs, err := analyzer.DetectLanguages("../..")
	if err != nil {
		t.Fatalf("DetectLanguages failed: %v", err)
	}

	if len(langs) == 0 {
		t.Error("Expected Go language detection")
	}
}

func TestDetector_GetManifestFiles(t *testing.T) {
	detector := NewDetector(nil) // Use nil config for basic test

	files, err := detector.GetManifestFiles("../..")
	if err != nil {
		t.Fatalf("GetManifestFiles failed: %v", err)
	}

	expected := []string{"go.mod", "go.sum"}
	if len(files) != len(expected) {
		t.Errorf("Expected %d files, got %d", len(expected), len(files))
	}
}

func TestGoAnalyzer_RegistryFailureFallback(t *testing.T) {
	// This test verifies conservative fallback when registry API fails
	// Expected behavior: age_unknown=true, registry_error populated, age_days=365
	analyzer := NewGoAnalyzer()
	ctx := context.Background()
	config := AnalysisConfig{Target: "../.."} // Points to repo root

	result, err := analyzer.Analyze(ctx, config.Target, config)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(result.Dependencies) == 0 {
		t.Skip("No dependencies found - cannot test registry failure fallback")
	}

	// Check that at least some dependencies have fallback metadata
	// Note: In real execution, registry may succeed or fail depending on network
	// This test primarily documents expected fallback structure
	foundFallback := false
	for _, dep := range result.Dependencies {
		if ageUnknown, ok := dep.Metadata["age_unknown"].(bool); ok && ageUnknown {
			foundFallback = true

			// Verify conservative fallback values
			if ageDays, ok := dep.Metadata["age_days"].(int); !ok || ageDays != 365 {
				t.Errorf("Expected age_days=365 for unknown age, got %v", dep.Metadata["age_days"])
			}

			if _, ok := dep.Metadata["registry_error"].(string); !ok {
				t.Errorf("Expected registry_error to be populated when age_unknown=true")
			}

			t.Logf("Verified fallback for dependency %s: age_unknown=true, age_days=365, registry_error=%v",
				dep.Name, dep.Metadata["registry_error"])
			break
		}
	}

	if !foundFallback {
		// If registry succeeded for all deps, log that fact
		t.Logf("Registry API succeeded for all %d dependencies (no fallback triggered)", len(result.Dependencies))
	}
}

// TestParseCargoDenyList tests parsing of cargo deny list output
func TestParseCargoDenyList(t *testing.T) {
	// Actual cargo deny list output format groups crates by license:
	// LICENSE (count): crate@version, crate@version, ...
	output := `Apache-2.0 (3): anstream@0.6.18, bitflags@2.6.0, windows-sys@0.52.0
MIT (4): clap@4.5.23, bitflags@2.6.0, anstream@0.6.18, windows-sys@0.52.0
Unlicense (2): aho-corasick@1.1.3, memchr@2.7.4
`

	deps := parseCargoDenyList([]byte(output))

	// Each crate@version entry becomes a Dependency
	// Total: 3 Apache + 4 MIT + 2 Unlicense = 9 entries
	if len(deps) != 9 {
		t.Errorf("Expected 9 dependencies, got %d", len(deps))
	}

	// Check first dependency (from Apache-2.0 line)
	if deps[0].Name != "anstream" {
		t.Errorf("Expected first dep name 'anstream', got '%s'", deps[0].Name)
	}
	if deps[0].Version != "0.6.18" {
		t.Errorf("Expected first dep version '0.6.18', got '%s'", deps[0].Version)
	}
	if len(deps[0].Licenses) != 1 {
		t.Errorf("Expected 1 license for first dep, got %d", len(deps[0].Licenses))
	}
	if len(deps[0].Licenses) > 0 && deps[0].Licenses[0] != "Apache-2.0" {
		t.Errorf("Expected license 'Apache-2.0', got '%s'", deps[0].Licenses[0])
	}

	// Find an Unlicense entry
	var foundUnlicense bool
	for _, dep := range deps {
		if dep.Name == "aho-corasick" {
			foundUnlicense = true
			if dep.Version != "1.1.3" {
				t.Errorf("Expected aho-corasick version '1.1.3', got '%s'", dep.Version)
			}
			if len(dep.Licenses) != 1 || dep.Licenses[0] != "Unlicense" {
				t.Errorf("Expected Unlicense license, got %v", dep.Licenses)
			}
			break
		}
	}
	if !foundUnlicense {
		t.Error("Did not find aho-corasick in parsed dependencies")
	}
}

// TestParseLicenseExpression tests SPDX-like license expression parsing
func TestParseLicenseExpression(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected []string
	}{
		{"single", "MIT", []string{"MIT"}},
		{"dual_or", "MIT OR Apache-2.0", []string{"MIT", "Apache-2.0"}},
		{"dual_and", "MIT AND Apache-2.0", []string{"MIT", "Apache-2.0"}},
		{"triple", "MIT OR Apache-2.0 OR BSD-3-Clause", []string{"MIT", "Apache-2.0", "BSD-3-Clause"}},
		{"with_parens", "(MIT OR Apache-2.0)", []string{"MIT", "Apache-2.0"}},
		{"complex", "(MIT OR Apache-2.0) AND BSD-3-Clause", []string{"MIT", "Apache-2.0", "BSD-3-Clause"}},
		{"empty", "", nil},
		{"unlicense_or_mit", "Unlicense OR MIT", []string{"Unlicense", "MIT"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLicenseExpression(tt.expr)
			if len(got) != len(tt.expected) {
				t.Errorf("parseLicenseExpression(%q) returned %d licenses, want %d", tt.expr, len(got), len(tt.expected))
				return
			}
			for i, lic := range tt.expected {
				if got[i] != lic {
					t.Errorf("parseLicenseExpression(%q)[%d] = %q, want %q", tt.expr, i, got[i], lic)
				}
			}
		})
	}
}

// TestConvertCratesToDependencies tests conversion from cargo deny format to unified Dependency
func TestConvertCratesToDependencies(t *testing.T) {
	crates := []CargoCrateLicense{
		{Name: "serde", Version: "1.0.0", Licenses: []string{"MIT", "Apache-2.0"}},
		{Name: "tokio", Version: "1.40.0", Licenses: []string{"MIT"}},
		{Name: "unknown-crate", Version: "0.1.0", Licenses: nil},
	}

	deps := convertCratesToDependencies(crates)

	if len(deps) != 3 {
		t.Errorf("Expected 3 dependencies, got %d", len(deps))
	}

	// Check serde
	if deps[0].Name != "serde" || deps[0].Version != "1.0.0" {
		t.Errorf("Unexpected first dep: %+v", deps[0])
	}
	if deps[0].Language != LanguageRust {
		t.Errorf("Expected Language=rust, got %s", deps[0].Language)
	}
	if deps[0].License == nil || deps[0].License.Type != "MIT OR Apache-2.0" {
		t.Errorf("Unexpected license for serde: %+v", deps[0].License)
	}

	// Check tokio (single license)
	if deps[1].License == nil || deps[1].License.Type != "MIT" {
		t.Errorf("Unexpected license for tokio: %+v", deps[1].License)
	}

	// Check unknown (no license)
	if deps[2].License != nil {
		t.Errorf("Expected nil license for unknown-crate, got %+v", deps[2].License)
	}
}

// TestRustAnalyzerInterface verifies RustAnalyzer implements Analyzer interface
func TestRustAnalyzerInterface(t *testing.T) {
	var _ Analyzer = (*RustAnalyzer)(nil)
}
