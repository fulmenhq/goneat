package maturity

import (
	"testing"
)

func TestReleasePhase_IsValid(t *testing.T) {
	tests := []struct {
		phase    ReleasePhase
		expected bool
	}{
		{ReleaseDev, true},
		{ReleaseRC, true},
		{ReleaseRelease, true},
		{ReleaseHotfix, true},
		{ReleasePhase("invalid"), false},
		{ReleasePhase(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			result := tt.phase.IsValid()
			if result != tt.expected {
				t.Errorf("ReleasePhase(%q).IsValid() = %v, want %v", tt.phase, result, tt.expected)
			}
		})
	}
}

func TestReleasePhase_String(t *testing.T) {
	tests := []struct {
		phase    ReleasePhase
		expected string
	}{
		{ReleaseDev, "dev"},
		{ReleaseRC, "rc"},
		{ReleaseRelease, "release"},
		{ReleaseHotfix, "hotfix"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.phase.String()
			if result != tt.expected {
				t.Errorf("ReleasePhase(%q).String() = %q, want %q", tt.phase, result, tt.expected)
			}
		})
	}
}

func TestValidReleasePhases(t *testing.T) {
	phases := ValidReleasePhases()
	expected := []ReleasePhase{ReleaseDev, ReleaseRC, ReleaseRelease, ReleaseHotfix}

	if len(phases) != len(expected) {
		t.Fatalf("ValidReleasePhases() len = %d, want %d", len(phases), len(expected))
	}

	for i, phase := range phases {
		if phase != expected[i] {
			t.Errorf("ValidReleasePhases()[%d] = %v, want %v", i, phase, expected[i])
		}
	}
}

func TestLifecyclePhase_IsValid(t *testing.T) {
	tests := []struct {
		phase    LifecyclePhase
		expected bool
	}{
		{LifecycleAlpha, true},
		{LifecycleBeta, true},
		{LifecycleGA, true},
		{LifecycleMaintenance, true},
		{LifecyclePhase("invalid"), false},
		{LifecyclePhase(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			result := tt.phase.IsValid()
			if result != tt.expected {
				t.Errorf("LifecyclePhase(%q).IsValid() = %v, want %v", tt.phase, result, tt.expected)
			}
		})
	}
}

func TestLifecyclePhase_String(t *testing.T) {
	tests := []struct {
		phase    LifecyclePhase
		expected string
	}{
		{LifecycleAlpha, "alpha"},
		{LifecycleBeta, "beta"},
		{LifecycleGA, "ga"},
		{LifecycleMaintenance, "maintenance"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.phase.String()
			if result != tt.expected {
				t.Errorf("LifecyclePhase(%q).String() = %q, want %q", tt.phase, result, tt.expected)
			}
		})
	}
}

func TestValidLifecyclePhases(t *testing.T) {
	phases := ValidLifecyclePhases()
	expected := []LifecyclePhase{LifecycleAlpha, LifecycleBeta, LifecycleGA, LifecycleMaintenance}

	if len(phases) != len(expected) {
		t.Fatalf("ValidLifecyclePhases() len = %d, want %d", len(phases), len(expected))
	}

	for i, phase := range phases {
		if phase != expected[i] {
			t.Errorf("ValidLifecyclePhases()[%d] = %v, want %v", i, phase, expected[i])
		}
	}
}

func TestPhasesConfig_Validate(t *testing.T) {
	tests := []struct {
		name     string
		config   PhasesConfig
		hasError bool
	}{
		{
			name: "valid config",
			config: PhasesConfig{
				ReleasePhases: ReleasePhasesConfig{
					ReleaseDev: PhaseRules{MinCoverage: 50},
				},
				LifecyclePhases: LifecyclePhasesConfig{
					LifecycleAlpha: PhaseRules{MinCoverage: 50},
				},
			},
			hasError: false,
		},
		{
			name: "invalid release phase key",
			config: PhasesConfig{
				ReleasePhases: ReleasePhasesConfig{
					ReleasePhase("invalid"): PhaseRules{MinCoverage: 50},
				},
			},
			hasError: true,
		},
		{
			name: "negative coverage",
			config: PhasesConfig{
				ReleasePhases: ReleasePhasesConfig{
					ReleaseDev: PhaseRules{MinCoverage: -1},
				},
			},
			hasError: true,
		},
		{
			name: "coverage over 100",
			config: PhasesConfig{
				ReleasePhases: ReleasePhasesConfig{
					ReleaseDev: PhaseRules{MinCoverage: 101},
				},
			},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.hasError {
				t.Errorf("PhasesConfig.Validate() error = %v, hasError %v", err, tt.hasError)
			}
		})
	}
}

func TestGetMinCoverageForPhase(t *testing.T) {
	tests := []struct {
		name     string
		config   PhasesConfig
		phase    LifecyclePhase
		expected int
	}{
		{
			name: "existing phase",
			config: PhasesConfig{
				LifecyclePhases: LifecyclePhasesConfig{
					LifecycleAlpha: PhaseRules{MinCoverage: 75},
				},
			},
			phase:    LifecycleAlpha,
			expected: 75,
		},
		{
			name:     "missing phase returns default",
			config:   PhasesConfig{},
			phase:    LifecycleAlpha,
			expected: 80,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetMinCoverageForPhase(tt.phase)
			if result != tt.expected {
				t.Errorf("PhasesConfig.GetMinCoverageForPhase(%v) = %d, want %d", tt.phase, result, tt.expected)
			}
		})
	}
}

func TestGetDefaultReleaseRules(t *testing.T) {
	tests := []struct {
		phase    ReleasePhase
		expected PhaseRules
	}{
		{
			ReleaseDev,
			PhaseRules{AllowedSuffixes: []string{"-dev", "-alpha"}, MinCoverage: 50, AllowDirtyGit: true, CoverageExceptions: map[string]int{"tests/**": 100}},
		},
		{
			ReleaseRC,
			PhaseRules{AllowedSuffixes: []string{"-rc.1", "-rc.2"}, MinCoverage: 75, AllowDirtyGit: false, CoverageExceptions: map[string]int{"node_modules/**": 0}},
		},
		{
			ReleaseRelease,
			PhaseRules{AllowedSuffixes: []string{}, MinCoverage: 90, AllowDirtyGit: false, CoverageExceptions: map[string]int{"docs/**": 100}},
		},
		{
			ReleaseHotfix,
			PhaseRules{AllowedSuffixes: []string{"-hotfix.1"}, MinCoverage: 80, AllowDirtyGit: false},
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			result := getDefaultReleaseRules(tt.phase)
			if result.MinCoverage != tt.expected.MinCoverage {
				t.Errorf("getDefaultReleaseRules(%v).MinCoverage = %d, want %d", tt.phase, result.MinCoverage, tt.expected.MinCoverage)
			}
			if result.AllowDirtyGit != tt.expected.AllowDirtyGit {
				t.Errorf("getDefaultReleaseRules(%v).AllowDirtyGit = %v, want %v", tt.phase, result.AllowDirtyGit, tt.expected.AllowDirtyGit)
			}
		})
	}
}

func TestGetDefaultLifecycleRules(t *testing.T) {
	tests := []struct {
		phase    LifecyclePhase
		expected PhaseRules
	}{
		{
			LifecycleAlpha,
			PhaseRules{MinCoverage: 50, CoverageExceptions: map[string]int{"prototypes/**": 0}},
		},
		{
			LifecycleBeta,
			PhaseRules{MinCoverage: 75},
		},
		{
			LifecycleGA,
			PhaseRules{MinCoverage: 90},
		},
		{
			LifecycleMaintenance,
			PhaseRules{MinCoverage: 80, SupportDuration: "P1Y"},
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			result := getDefaultLifecycleRules(tt.phase)
			if result.MinCoverage != tt.expected.MinCoverage {
				t.Errorf("getDefaultLifecycleRules(%v).MinCoverage = %d, want %d", tt.phase, result.MinCoverage, tt.expected.MinCoverage)
			}
			if result.SupportDuration != tt.expected.SupportDuration {
				t.Errorf("getDefaultLifecycleRules(%v).SupportDuration = %q, want %q", tt.phase, result.SupportDuration, tt.expected.SupportDuration)
			}
		})
	}
}

func TestApplyDefaults(t *testing.T) {
	input := PhasesConfig{
		ReleasePhases: ReleasePhasesConfig{
			ReleaseDev: PhaseRules{MinCoverage: 0}, // Should be set to default
		},
		LifecyclePhases: LifecyclePhasesConfig{
			LifecycleAlpha: PhaseRules{MinCoverage: 0}, // Should be set to default
		},
	}

	result := applyDefaults(input)

	// For now, applyDefaults just returns the config unchanged
	// This test ensures it doesn't panic and returns the expected structure
	if len(result.ReleasePhases) != len(input.ReleasePhases) {
		t.Errorf("applyDefaults() changed ReleasePhases count: got %d, want %d", len(result.ReleasePhases), len(input.ReleasePhases))
	}
	if len(result.LifecyclePhases) != len(input.LifecyclePhases) {
		t.Errorf("applyDefaults() changed LifecyclePhases count: got %d, want %d", len(result.LifecyclePhases), len(input.LifecyclePhases))
	}
}
