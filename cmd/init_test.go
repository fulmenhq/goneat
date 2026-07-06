package cmd

import "testing"

func TestGeneratePatternsIncludesGeneratedToolingDefaults(t *testing.T) {
	patterns, err := generatePatterns(nil)
	if err != nil {
		t.Fatalf("generatePatterns failed: %v", err)
	}

	got := map[string]bool{}
	for _, pattern := range patterns {
		got[pattern] = true
	}

	for _, want := range []string{".cache/", "bin/", "dist/", "sbom/", "vendor/"} {
		if !got[want] {
			t.Fatalf("generated patterns missing %q: %v", want, patterns)
		}
	}
}
