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
