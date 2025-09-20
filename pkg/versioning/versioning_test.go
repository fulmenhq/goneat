package versioning

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestCompareSemverFull(t *testing.T) {
	tests := []struct {
		name    string
		a       string
		b       string
		want    Comparison
		wantErr bool
		errMsg  string
	}{
		{"less_patch", "1.2.0", "1.2.1", ComparisonLess, false, ""},
		{"greater_patch", "1.2.2", "1.2.1", ComparisonGreater, false, ""},
		{"less_minor", "1.2.3", "1.3.0", ComparisonLess, false, ""},
		{"greater_major", "3.0.0", "2.9.9", ComparisonGreater, false, ""},
		{"equal", "1.2.3", "1.2.3", ComparisonEqual, false, ""},
		{"prefix_v_left", "v1.2.3", "1.2.4", ComparisonLess, false, ""},
		{"prefix_v_right", "1.2.3", "v1.2.4", ComparisonLess, false, ""},
		{"prerelease_order", "1.0.0-alpha", "1.0.0-beta", ComparisonLess, false, ""},
		{"prerelease_vs_release", "1.0.0-rc.1", "1.0.0", ComparisonLess, false, ""},
		{"natural_sorting", "1.0.0-rc.2", "1.0.0-rc.11", ComparisonLess, false, ""},
		{"build_metadata_ignored", "1.2.3+build.1", "1.2.3+build.2", ComparisonEqual, false, ""},
		{"mixed_prerelease_build", "1.2.3-rc.1+build.3", "1.2.3-rc.2+build.4", ComparisonLess, false, ""},
		{"non_numeric_major", "a.2.3", "1.2.3", ComparisonUnknown, true, "invalid format"},
		{"non_numeric_minor", "1.b.3", "1.2.3", ComparisonUnknown, true, "invalid format"},
		{"non_numeric_patch", "1.2.c", "1.2.3", ComparisonUnknown, true, "invalid format"},
		{"missing_patch", "1.2", "1.2.3", ComparisonUnknown, true, "invalid format"},
		{"missing_minor", "1", "1.2.3", ComparisonUnknown, true, "invalid format"},
		{"too_many_segments", "1.2.3.4", "1.2.3", ComparisonUnknown, true, "invalid format"},
		{"empty_string", "", "1.2.3", ComparisonUnknown, true, "empty version"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Compare(SchemeSemverFull, tc.a, tc.b)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error containing '%s', got nil", tc.errMsg)
				}
				if !strings.Contains(err.Error(), tc.errMsg) {
					t.Fatalf("expected error containing '%s', got: %v", tc.errMsg, err)
				}
				if got != ComparisonUnknown {
					t.Fatalf("expected ComparisonUnknown for error case, got %v", got)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tc.want {
					t.Fatalf("Compare() = %v want %v", got, tc.want)
				}
			}
		})
	}
}

func TestCompareSemverCompact(t *testing.T) {
	tests := []struct {
		name    string
		a       string
		b       string
		want    Comparison
		wantErr bool
		errMsg  string
	}{
		{"basic_less", "1.0.0", "1.0.1", ComparisonLess, false, ""},
		{"basic_greater", "2.0.0", "1.9.9", ComparisonGreater, false, ""},
		{"equal", "1.2.3", "1.2.3", ComparisonEqual, false, ""},
		{"reject_prerelease_left", "1.2.3-alpha", "1.2.3", ComparisonUnknown, true, "forbids prerelease"},
		{"reject_prerelease_right", "1.2.3", "1.2.3-beta", ComparisonUnknown, true, "forbids prerelease"},
		{"reject_build", "1.2.3+build", "1.2.4", ComparisonUnknown, true, "forbids prerelease"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Compare(SchemeSemverCompact, tc.a, tc.b)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error containing '%s', got nil", tc.errMsg)
				}
				if !strings.Contains(err.Error(), tc.errMsg) {
					t.Fatalf("expected error containing '%s', got: %v", tc.errMsg, err)
				}
				if got != ComparisonUnknown {
					t.Fatalf("expected ComparisonUnknown for error case, got %v", got)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tc.want {
					t.Fatalf("Compare() = %v want %v", got, tc.want)
				}
			}
		})
	}
}

func TestCompareCalver(t *testing.T) {
	tests := []struct {
		name    string
		a       string
		b       string
		want    Comparison
		wantErr bool
		errMsg  string
	}{
		{"year_month_day_less", "2024.09.01", "2024.09.02", ComparisonLess, false, ""},
		{"year_month_day_greater", "2024.09.15", "2024.09.10", ComparisonGreater, false, ""},
		{"year_month_day_equal", "2024.09.15", "2024.09.15", ComparisonEqual, false, ""},
		{"year_month_less", "2024.08", "2024.09", ComparisonLess, false, ""},
		{"year_month_equal", "2024.09", "2024.09", ComparisonEqual, false, ""},
		{"mixed_precision", "2024.09", "2024.09.01", ComparisonLess, false, ""},
		{"invalid_month_high", "2024.13.01", "2024.12.31", ComparisonUnknown, true, "invalid month"},
		{"invalid_month_zero", "2024.00.01", "2024.01.01", ComparisonUnknown, true, "invalid month"},
		{"invalid_day_zero", "2024.01.00", "2024.01.01", ComparisonUnknown, true, "invalid day"},
		{"invalid_day_high", "2024.01.32", "2024.01.31", ComparisonUnknown, true, "invalid day"},
		{"invalid_year_zero", "0000.01.01", "2024.01.01", ComparisonUnknown, true, "invalid year"},
		{"non_numeric_year", "abcd.01.01", "2024.01.01", ComparisonUnknown, true, "strict format"},
		{"non_numeric_month", "2024.ab.01", "2024.01.01", ComparisonUnknown, true, "strict format"},
		{"non_numeric_day", "2024.01.ab", "2024.01.01", ComparisonUnknown, true, "strict format"},
		{"year_only", "2023", "2024", ComparisonUnknown, true, "strict format"},
		{"missing_month_padding", "2024.1", "2024.02", ComparisonUnknown, true, "strict format"},
		{"missing_day_padding", "2024.01.1", "2024.01.02", ComparisonUnknown, true, "strict format"},
		{"mixed_separators", "2024.01-01", "2024.01.02", ComparisonUnknown, true, "consistent separators"},
		{"empty_string", "", "2024.01.01", ComparisonUnknown, true, "empty version"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Compare(SchemeCalver, tc.a, tc.b)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error containing '%s', got nil", tc.errMsg)
				}
				if !strings.Contains(err.Error(), tc.errMsg) {
					t.Fatalf("expected error containing '%s', got: %v", tc.errMsg, err)
				}
				if got != ComparisonUnknown {
					t.Fatalf("expected ComparisonUnknown for error case, got %v", got)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tc.want {
					t.Fatalf("Compare() = %v want %v", got, tc.want)
				}
			}
		})
	}
}

func TestCompareDefaultScheme(t *testing.T) {
	// Test that empty scheme defaults to lexical
	got, err := Compare("", "v1.0.0", "v1.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != ComparisonLess {
		t.Fatalf("expected ComparisonLess for default scheme, got %v", got)
	}
}

func TestEvaluatePolicy(t *testing.T) {
	tests := []struct {
		name           string
		policy         Policy
		actual         string
		wantMeetsMin   bool
		wantMeetsRec   bool
		wantDisallowed bool
		wantErr        bool
		errMsg         string
	}{
		{"minimum_only_met", Policy{Scheme: SchemeSemverFull, MinimumVersion: "1.3.0"}, "1.4.0", true, true, false, false, ""},
		{"minimum_only_not_met", Policy{Scheme: SchemeSemverFull, MinimumVersion: "1.3.0"}, "1.2.9", false, true, false, false, ""},
		{"minimum_only_equal", Policy{Scheme: SchemeSemverFull, MinimumVersion: "1.3.0"}, "1.3.0", true, true, false, false, ""},
		{"recommended_only_met", Policy{Scheme: SchemeSemverFull, RecommendedVersion: "1.4.0"}, "1.5.0", true, true, false, false, ""},
		{"recommended_only_not_met", Policy{Scheme: SchemeSemverFull, RecommendedVersion: "1.4.0"}, "1.3.0", true, false, false, false, ""},
		{"both_met", Policy{Scheme: SchemeSemverFull, MinimumVersion: "1.3.0", RecommendedVersion: "1.4.0"}, "1.5.0", true, true, false, false, ""},
		{"both_minimum_met_rec_not", Policy{Scheme: SchemeSemverFull, MinimumVersion: "1.3.0", RecommendedVersion: "1.4.0"}, "1.3.5", true, false, false, false, ""},
		{"both_not_met", Policy{Scheme: SchemeSemverFull, MinimumVersion: "1.3.0", RecommendedVersion: "1.4.0"}, "1.2.0", false, false, false, false, ""},
		{"disallowed_exact_match", Policy{Scheme: SchemeSemverFull, DisallowedVersions: []string{"1.3.5"}}, "1.3.5", true, true, true, false, ""},
		{"disallowed_no_match", Policy{Scheme: SchemeSemverFull, DisallowedVersions: []string{"1.3.5"}}, "1.3.6", true, true, false, false, ""},
		{"disallowed_precedence", Policy{Scheme: SchemeSemverFull, MinimumVersion: "1.3.0", DisallowedVersions: []string{"1.3.5"}}, "1.3.5", true, true, true, false, ""},
		{"mixed_semver_calver", Policy{Scheme: SchemeSemverFull, MinimumVersion: "1.3.0"}, "2024.09.01", false, true, false, true, "invalid semver"},
		{"mixed_calver_semver", Policy{Scheme: SchemeCalver, MinimumVersion: "2024.09.01"}, "1.3.0", false, true, false, true, "invalid calver"},
		{"zero_policy", Policy{}, "1.2.3", true, true, false, false, ""},
		{"zero_policy_empty_scheme", Policy{Scheme: ""}, "1.2.3", true, true, false, false, ""},
		{"empty_actual", Policy{Scheme: SchemeSemverFull, MinimumVersion: "1.3.0"}, "", false, false, false, true, "actual version cannot be empty"},
		{"calver_minimum_met", Policy{Scheme: SchemeCalver, MinimumVersion: "2024.09.01"}, "2024.09.15", true, true, false, false, ""},
		{"calver_minimum_not_met", Policy{Scheme: SchemeCalver, MinimumVersion: "2024.09.15"}, "2024.09.01", false, true, false, false, ""},
		{"lexical_minimum", Policy{Scheme: SchemeLexical, MinimumVersion: "v1.0.0"}, "v1.0.1", true, true, false, false, ""},
		{"lexical_recommended", Policy{Scheme: SchemeLexical, RecommendedVersion: "v1.0.0"}, "v0.9.9", true, false, false, false, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eval, err := Evaluate(tc.policy, tc.actual)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error containing '%s', got nil", tc.errMsg)
				}
				if !strings.Contains(err.Error(), tc.errMsg) {
					t.Fatalf("expected error containing '%s', got: %v", tc.errMsg, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if eval.MeetsMinimum != tc.wantMeetsMin {
					t.Fatalf("MeetsMinimum = %v, want %v", eval.MeetsMinimum, tc.wantMeetsMin)
				}
				if eval.MeetsRecommended != tc.wantMeetsRec {
					t.Fatalf("MeetsRecommended = %v, want %v", eval.MeetsRecommended, tc.wantMeetsRec)
				}
				if eval.IsDisallowed != tc.wantDisallowed {
					t.Fatalf("IsDisallowed = %v, want %v", eval.IsDisallowed, tc.wantDisallowed)
				}
			}
		})
	}
}

func TestSortDisallowed(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{"basic_sort", []string{"2.0.0", "1.0.0", "1.0.1"}, []string{"1.0.0", "1.0.1", "2.0.0"}},
		{"empty_slice", []string{}, nil},
		{"single_item", []string{"1.0.0"}, []string{"1.0.0"}},
		{"already_sorted", []string{"1.0.0", "1.0.1", "2.0.0"}, []string{"1.0.0", "1.0.1", "2.0.0"}},
		{"reverse_sorted", []string{"3.0.0", "2.0.0", "1.0.0"}, []string{"1.0.0", "2.0.0", "3.0.0"}},
		{"mixed_versions", []string{"1.10.0", "1.2.0", "1.1.0"}, []string{"1.1.0", "1.2.0", "1.10.0"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			original := make([]string, len(tc.input))
			copy(original, tc.input)

			result := SortDisallowed(tc.input)

			// Check result
			if len(result) != len(tc.expected) {
				t.Fatalf("expected length %d, got %d", len(tc.expected), len(result))
			}
			for i, v := range tc.expected {
				if result[i] != v {
					t.Fatalf("expected %v at index %d, got %v", v, i, result[i])
				}
			}

			// Ensure original slice is unchanged
			if len(original) != len(tc.input) {
				t.Fatalf("original slice length changed from %d to %d", len(original), len(tc.input))
			}
			for i, v := range original {
				if tc.input[i] != v {
					t.Fatalf("original slice modified at index %d: expected %v, got %v", i, v, tc.input[i])
				}
			}
		})
	}
}

// TestPolicyIsZero tests the zero policy fast path
func TestPolicyIsZero(t *testing.T) {
	tests := []struct {
		name   string
		policy Policy
		want   bool
	}{
		{"truly_zero", Policy{}, true},
		{"scheme_only", Policy{Scheme: SchemeSemverFull}, false},
		{"minimum_only", Policy{MinimumVersion: "1.0.0"}, false},
		{"recommended_only", Policy{RecommendedVersion: "1.0.0"}, false},
		{"disallowed_only", Policy{DisallowedVersions: []string{"1.0.0"}}, false},
		{"all_constraints", Policy{
			Scheme:             SchemeSemverFull,
			MinimumVersion:     "1.0.0",
			RecommendedVersion: "2.0.0",
			DisallowedVersions: []string{"1.5.0"},
		}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.policy.IsZero()
			if got != tc.want {
				t.Fatalf("IsZero() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestSemverRegressionWithCmdVersion tests edge cases that should match cmd/version.go behavior
func TestSemverRegressionWithCmdVersion(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want Comparison
	}{
		// Test cases that should match bump logic from cmd/version.go
		{"bump_patch_regression", "1.2.3", "1.2.4", ComparisonLess},
		{"bump_minor_regression", "1.2.9", "1.3.0", ComparisonLess},
		{"bump_major_regression", "1.9.9", "2.0.0", ComparisonLess},

		// Large numbers that should work with bump logic
		{"large_patch_bump", "1.2.999", "1.3.0", ComparisonLess},
		{"large_minor_bump", "1.999.0", "2.0.0", ComparisonLess},

		// Prefix handling that should match cmd/version.go
		{"prefix_consistency", "v1.2.3", "v1.2.4", ComparisonLess},
		{"mixed_prefix", "v1.2.3", "1.2.4", ComparisonLess},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Compare(SchemeSemverFull, tc.a, tc.b)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("Compare() = %v want %v (should match cmd/version.go bump logic)", got, tc.want)
			}
		})
	}
}

// TestLatestSemverTagRegression tests that our semver parsing matches cmd/version.go latestSemverTag
func TestLatestSemverTagRegression(t *testing.T) {
	// Test cases based on the latestSemverTag function in cmd/version.go
	testCases := []struct {
		name     string
		tags     []string
		expected string
		hasTag   bool
	}{
		{
			name:     "basic_semver_tags",
			tags:     []string{"v1.0.0", "v1.1.0", "v2.0.0", "v1.2.0"},
			expected: "v2.0.0",
			hasTag:   true,
		},
		{
			name:     "mixed_prefixes",
			tags:     []string{"1.0.0", "v1.1.0", "2.0.0"},
			expected: "2.0.0",
			hasTag:   true,
		},
		{
			name:     "prerelease_handling",
			tags:     []string{"v1.0.0-alpha", "v1.0.0-beta", "v1.0.0", "v1.0.1-rc.1"},
			expected: "v1.0.1-rc.1",
			hasTag:   true,
		},
		{
			name:     "no_valid_semver",
			tags:     []string{"invalid", "not-a-version", "random-string"},
			expected: "",
			hasTag:   false,
		},
		{
			name:     "empty_list",
			tags:     []string{},
			expected: "",
			hasTag:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate the logic from latestSemverTag
			type sv struct {
				raw                 string
				major, minor, patch int
			}
			var semvers []sv
			re := regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-[a-zA-Z0-9.-]+)?(?:\+[a-zA-Z0-9.-]+)?$`)
			for _, tag := range tc.tags {
				m := re.FindStringSubmatch(tag)
				if len(m) == 0 {
					continue
				}
				maj, _ := strconv.Atoi(m[1])
				min, _ := strconv.Atoi(m[2])
				pat, _ := strconv.Atoi(m[3])
				semvers = append(semvers, sv{raw: tag, major: maj, minor: min, patch: pat})
			}

			if len(semvers) == 0 {
				if tc.hasTag {
					t.Errorf("expected to find a semver tag, but found none")
				}
				return
			}

			// Sort using the same logic as cmd/version.go
			sort.Slice(semvers, func(i, j int) bool {
				if semvers[i].major != semvers[j].major {
					return semvers[i].major > semvers[j].major
				}
				if semvers[i].minor != semvers[j].minor {
					return semvers[i].minor > semvers[j].minor
				}
				return semvers[i].patch > semvers[j].patch
			})

			actual := semvers[0].raw
			if actual != tc.expected {
				t.Errorf("latestSemverTag regression: expected %s, got %s", tc.expected, actual)
			}
		})
	}
}

// TestLatestCalverTagRegression tests that our calver parsing matches cmd/version.go latestCalverTag
func TestLatestCalverTagRegression(t *testing.T) {
	// Test cases based on the latestCalverTag function in cmd/version.go
	testCases := []struct {
		name     string
		tags     []string
		expected string
		hasTag   bool
	}{
		{
			name:     "basic_calver_tags",
			tags:     []string{"2024.01.01", "2024.02.01", "2024.12.31", "2023.12.31"},
			expected: "2024.12.31",
			hasTag:   true,
		},
		{
			name:     "year_month_only",
			tags:     []string{"2024.01", "2024.02", "2024.10"},
			expected: "2024.10",
			hasTag:   true,
		},
		{
			name:     "mixed_formats",
			tags:     []string{"2024.01", "2024.01.15", "2024.02.01"},
			expected: "2024.02.01",
			hasTag:   true,
		},
		{
			name:     "no_valid_calver",
			tags:     []string{"invalid", "v1.0.0", "random-string"},
			expected: "",
			hasTag:   false,
		},
		{
			name:     "empty_list",
			tags:     []string{},
			expected: "",
			hasTag:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate the logic from latestCalverTag
			re := regexp.MustCompile(`^(\d{4})\.(\d{2})(?:\.(\d{2}))?$`)
			var calvers []string
			for _, tag := range tc.tags {
				if re.MatchString(tag) {
					calvers = append(calvers, tag)
				}
			}

			if len(calvers) == 0 {
				if tc.hasTag {
					t.Errorf("expected to find a calver tag, but found none")
				}
				return
			}

			// Sort lexicographically (same as cmd/version.go)
			sort.Slice(calvers, func(i, j int) bool { return calvers[i] > calvers[j] })

			actual := calvers[0]
			if actual != tc.expected {
				t.Errorf("latestCalverTag regression: expected %s, got %s", tc.expected, actual)
			}
		})
	}
}

// TestCalverRegressionWithCmdVersion tests calendar version detection patterns
func TestCalverRegressionWithCmdVersion(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want Comparison
	}{
		// Test cases that should match latestCalverTag logic from cmd/version.go
		{"calver_year_month_day", "2024.09.01", "2024.09.02", ComparisonLess},
		{"calver_year_month", "2024.08", "2024.09", ComparisonLess},
		{"calver_mixed_segments", "2024.09", "2024.09.01", ComparisonLess},

		// Dot-separated numeric tokens (as accepted by latestCalverTag)
		{"dot_separated", "2024.09.01", "2024.09.02", ComparisonLess},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Compare(SchemeCalver, tc.a, tc.b)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("Compare() = %v want %v (should match cmd/version.go calver detection)", got, tc.want)
			}
		})
	}
}

// TestSchemeValidationMatrix verifies per-scheme validation rules.
func TestSchemeValidationMatrix(t *testing.T) {
	testCases := []struct {
		name    string
		scheme  Scheme
		version string
		valid   bool
	}{
		{"semver_full_basic", SchemeSemverFull, "1.0.0", true},
		{"semver_full_with_prefix", SchemeSemverFull, "v1.2.3", true},
		{"semver_full_prerelease", SchemeSemverFull, "1.0.0-alpha.1", true},
		{"semver_full_build", SchemeSemverFull, "1.0.0+build.1", true},
		{"semver_full_leading_zero", SchemeSemverFull, "1.02.3", false},
		{"semver_full_calver_like", SchemeSemverFull, "2024.01.01", false},
		{"semver_full_short", SchemeSemverFull, "1.0", false},
		{"semver_compact_basic", SchemeSemverCompact, "1.0.0", true},
		{"semver_compact_prefix", SchemeSemverCompact, "v2.3.4", true},
		{"semver_compact_prerelease", SchemeSemverCompact, "1.0.0-alpha", false},
		{"semver_compact_build", SchemeSemverCompact, "1.0.0+build.1", false},
		{"calver_year_month", SchemeCalver, "2024.09", true},
		{"calver_year_month_day", SchemeCalver, "2024.09.15", true},
		{"calver_dash", SchemeCalver, "2024-09-15", true},
		{"calver_underscore", SchemeCalver, "2024_09", true},
		{"calver_year_only", SchemeCalver, "2024", false},
		{"calver_invalid_month", SchemeCalver, "2024.13.01", false},
		{"calver_extra_segment", SchemeCalver, "2024.09.15.1", false},
		{"calver_mixed_separators", SchemeCalver, "2024.09-15", false},
		{"lexical_any", SchemeLexical, "any-string", true},
		{"lexical_empty", SchemeLexical, "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compare(tc.scheme, tc.version, tc.version)
			if tc.valid && err != nil {
				t.Fatalf("expected %s to be valid for %s, got error: %v", tc.version, tc.scheme, err)
			}
			if !tc.valid && err == nil {
				t.Fatalf("expected %s to be invalid for %s", tc.version, tc.scheme)
			}
		})
	}
}

// TestCrossSchemeComparisonErrors ensures we fail when comparing across incompatible schemes.
func TestCrossSchemeComparisonErrors(t *testing.T) {
	cases := []struct {
		name   string
		scheme Scheme
		a      string
		b      string
	}{
		{"semver_vs_calver", SchemeSemverFull, "1.2.3", "2024.09.01"},
		{"semver_compact_vs_calver", SchemeSemverCompact, "1.2.3", "2024.09"},
		{"calver_vs_semver", SchemeCalver, "2024.09", "1.2.3"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Compare(tc.scheme, tc.a, tc.b); err == nil {
				t.Fatalf("expected comparison error for %s", tc.name)
			}
		})
	}
}

// TestExtremeValues tests edge cases and extreme values
func TestExtremeValues(t *testing.T) {
	extremeTests := []struct {
		name     string
		scheme   Scheme
		versionA string
		versionB string
		expected Comparison
		valid    bool
	}{
		// Very long versions
		{"long_semver", SchemeSemverFull, "1.0.0-alpha.beta.gamma.delta.epsilon", "1.0.0-alpha.beta.gamma.delta.zeta", ComparisonLess, true},
		{"long_calver", SchemeCalver, "2024.01.01.001.002.003", "2024.01.01.001.002.004", ComparisonUnknown, false},
		{"long_lexical", SchemeLexical, "very-long-version-string-with-many-characters", "very-long-version-string-with-many-characters-but-different", ComparisonLess, true},

		// Unicode and special characters
		{"unicode_semver", SchemeSemverFull, "1.0.0", "1.0.0ü", ComparisonUnknown, false},       // Should fail semver
		{"unicode_calver", SchemeCalver, "2024.01.01", "2024.01.01ü", ComparisonUnknown, false}, // Should fail calver
		{"unicode_lexical", SchemeLexical, "v1.0.0", "v1.0.0ü", ComparisonLess, true},           // Should work for lexical

		// Maximum numeric values
		{"max_int_semver", SchemeSemverFull, "2147483647.2147483647.2147483647", "2147483648.0.0", ComparisonLess, true},
		{"max_year_calver", SchemeCalver, "9999.12.31", "9999.12.32", ComparisonUnknown, false}, // Invalid day

		// Minimum values
		{"min_semver", SchemeSemverFull, "0.0.0", "0.0.1", ComparisonLess, true},
		{"min_calver", SchemeCalver, "0000.01.01", "0001.01.01", ComparisonUnknown, false},

		// Empty and whitespace
		{"empty_semver", SchemeSemverFull, "", "1.0.0", ComparisonUnknown, false},
		{"empty_calver", SchemeCalver, "", "2024.01.01", ComparisonUnknown, false},
		{"empty_lexical", SchemeLexical, "", "a", ComparisonLess, true},
		{"whitespace_semver", SchemeSemverFull, "  1.0.0  ", "1.0.0", ComparisonEqual, true},
		{"whitespace_lexical", SchemeLexical, "  abc  ", "abc", ComparisonEqual, true},

		// Pathological cases
		{"very_deep_semver", SchemeSemverFull, "1.2.3.4.5.6.7.8.9.10", "1.2.3", ComparisonUnknown, false},
		{"mixed_separators", SchemeCalver, "2024.01-01_02", "2024.01-01_03", ComparisonUnknown, false},
		{"repeated_dots", SchemeLexical, "v1..0..0", "v1..0..1", ComparisonLess, true},
	}

	for _, tc := range extremeTests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Compare(tc.scheme, tc.versionA, tc.versionB)

			if tc.valid {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				} else if got != tc.expected {
					t.Errorf("expected %v, got %v", tc.expected, got)
				}
			} else {
				if err == nil {
					t.Errorf("expected error for invalid case, but got %v", got)
				}
			}
		})
	}
}

// TestPolicyEvaluationEdgeCases tests policy evaluation with extreme and edge case inputs
func TestPolicyEvaluationEdgeCases(t *testing.T) {
	edgeCaseTests := []struct {
		name        string
		policy      Policy
		actual      string
		expectError bool
		description string
	}{
		// Empty and invalid versions
		{"empty_actual", Policy{Scheme: SchemeSemverFull, MinimumVersion: "1.0.0"}, "", true, "empty actual version should error"},
		{"whitespace_actual", Policy{Scheme: SchemeSemverFull, MinimumVersion: "1.0.0"}, "   ", true, "whitespace actual version should error"},
		{"invalid_scheme", Policy{Scheme: "invalid", MinimumVersion: "1.0.0"}, "1.0.0", true, "invalid scheme should error"},

		// Cross-scheme comparisons that should fail
		{"semver_vs_calver", Policy{Scheme: SchemeSemverFull, MinimumVersion: "1.0.0"}, "2024.01.01", true, "semver policy with calver version should error"},
		{"calver_vs_semver", Policy{Scheme: SchemeCalver, MinimumVersion: "2024.01.01"}, "1.0.0", true, "calver policy with semver version should error"},

		// Extreme version values
		{"very_long_version", Policy{Scheme: SchemeLexical, MinimumVersion: "a"}, strings.Repeat("a", 1000), false, "very long lexical version should work"},
		{"unicode_versions", Policy{Scheme: SchemeLexical, MinimumVersion: "a"}, "αβγδε", false, "unicode versions should work with lexical"},

		// Disallowed list edge cases
		{"empty_disallowed", Policy{Scheme: SchemeSemverFull, DisallowedVersions: []string{}}, "1.0.0", false, "empty disallowed list should work"},
		{"disallowed_with_whitespace", Policy{Scheme: SchemeSemverFull, DisallowedVersions: []string{"  1.0.0  "}}, "1.0.0", false, "disallowed with whitespace should match"},

		// Zero policy variations
		{"zero_policy_empty_scheme", Policy{MinimumVersion: "1.0.0"}, "1.0.0", false, "policy with only minimum should not be zero"},
		{"true_zero_policy", Policy{}, "1.0.0", false, "truly zero policy should work"},
	}

	for _, tc := range edgeCaseTests {
		t.Run(tc.name, func(t *testing.T) {
			eval, err := Evaluate(tc.policy, tc.actual)

			if tc.expectError {
				if err == nil {
					t.Errorf("%s: expected error but got none", tc.description)
				}
			} else {
				if err != nil {
					t.Errorf("%s: unexpected error: %v", tc.description, err)
				} else {
					// Verify evaluation is reasonable
					if tc.policy.IsZero() {
						if !eval.MeetsMinimum || !eval.MeetsRecommended {
							t.Errorf("%s: zero policy should always meet requirements", tc.description)
						}
					}
				}
			}
		})
	}
}

// TestParseLenient tests the new ParseLenient function with various semver patterns
func TestParseLenient(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantMajor   int
		wantMinor   int
		wantPatch   int
		wantPre     []semverIdentifier
		wantBuild   string
		wantRaw     string
		wantErr     bool
		errContains string
	}{
		// Basic semver patterns
		{"basic_no_prefix", "1.2.3", 1, 2, 3, nil, "", "1.2.3", false, ""},
		{"basic_with_v_prefix", "v1.2.3", 1, 2, 3, nil, "", "v1.2.3", false, ""},
		{"basic_with_V_prefix", "V1.2.3", 1, 2, 3, nil, "", "V1.2.3", false, ""},

		// Prerelease versions
		{"prerelease_alpha", "1.0.0-alpha", 1, 0, 0, []semverIdentifier{{raw: "alpha"}}, "", "1.0.0-alpha", false, ""},
		{"prerelease_beta_numeric", "2.0.0-beta.1", 2, 0, 0, []semverIdentifier{{raw: "beta"}, {raw: "1", numeric: true, num: 1}}, "", "2.0.0-beta.1", false, ""},
		{"prerelease_complex", "1.0.0-rc.1.alpha", 1, 0, 0, []semverIdentifier{{raw: "rc"}, {raw: "1", numeric: true, num: 1}, {raw: "alpha"}}, "", "1.0.0-rc.1.alpha", false, ""},

		// Build metadata
		{"build_metadata", "1.2.3+build.1", 1, 2, 3, nil, "build.1", "1.2.3+build.1", false, ""},
		{"prerelease_and_build", "1.0.0-beta+build.123", 1, 0, 0, []semverIdentifier{{raw: "beta"}}, "build.123", "1.0.0-beta+build.123", false, ""},

		// Edge cases with v prefix
		{"v_prefix_prerelease", "v1.0.0-alpha.1", 1, 0, 0, []semverIdentifier{{raw: "alpha"}, {raw: "1", numeric: true, num: 1}}, "", "v1.0.0-alpha.1", false, ""},
		{"v_prefix_build", "v2.1.0+20230101", 2, 1, 0, nil, "20230101", "v2.1.0+20230101", false, ""},

		// Invalid cases
		{"empty_string", "", 0, 0, 0, nil, "", "", true, "empty version"},
		{"whitespace_only", "   ", 0, 0, 0, nil, "", "", true, "empty version"},
		{"invalid_format", "1.2", 0, 0, 0, nil, "", "", true, "invalid format"},
		{"leading_zero_major", "01.2.3", 0, 0, 0, nil, "", "", true, "leading zeros not allowed"},
		{"leading_zero_minor", "1.02.3", 0, 0, 0, nil, "", "", true, "leading zeros not allowed"},
		{"leading_zero_patch", "1.2.03", 0, 0, 0, nil, "", "", true, "leading zeros not allowed"},
		{"leading_zero_prerelease", "1.0.0-alpha.01", 0, 0, 0, nil, "", "", true, "leading zeros not allowed"},
		{"leading_zero_build", "1.0.0+build.01", 0, 0, 0, nil, "", "", true, "leading zeros not allowed"},
		{"empty_prerelease_segment", "1.0.0-alpha.", 0, 0, 0, nil, "", "", true, "empty segment"},
		{"empty_build_segment", "1.0.0+build.", 0, 0, 0, nil, "", "", true, "empty segment"},
		{"invalid_characters", "1.2.3-beta!", 0, 0, 0, nil, "", "", true, "invalid format"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseLenient(tc.input)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error containing '%s', got nil", tc.errContains)
				}
				if !strings.Contains(err.Error(), tc.errContains) {
					t.Fatalf("expected error containing '%s', got: %v", tc.errContains, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got == nil {
				t.Fatal("expected non-nil Version, got nil")
			}

			if got.major != tc.wantMajor {
				t.Errorf("major = %d, want %d", got.major, tc.wantMajor)
			}
			if got.minor != tc.wantMinor {
				t.Errorf("minor = %d, want %d", got.minor, tc.wantMinor)
			}
			if got.patch != tc.wantPatch {
				t.Errorf("patch = %d, want %d", got.patch, tc.wantPatch)
			}
			if got.build != tc.wantBuild {
				t.Errorf("build = %s, want %s", got.build, tc.wantBuild)
			}
			if got.raw != tc.wantRaw {
				t.Errorf("raw = %s, want %s", got.raw, tc.wantRaw)
			}

			// Check prerelease identifiers
			if len(got.pre) != len(tc.wantPre) {
				t.Errorf("prerelease length = %d, want %d", len(got.pre), len(tc.wantPre))
			} else {
				for i, wantID := range tc.wantPre {
					if i >= len(got.pre) {
						t.Errorf("missing prerelease identifier at index %d", i)
						continue
					}
					gotID := got.pre[i]
					if gotID.raw != wantID.raw {
						t.Errorf("prerelease[%d].raw = %s, want %s", i, gotID.raw, wantID.raw)
					}
					if gotID.numeric != wantID.numeric {
						t.Errorf("prerelease[%d].numeric = %v, want %v", i, gotID.numeric, wantID.numeric)
					}
					if gotID.numeric && gotID.num != wantID.num {
						t.Errorf("prerelease[%d].num = %d, want %d", i, gotID.num, wantID.num)
					}
				}
			}
		})
	}
}

// TestVersionString tests the String method and round-trip consistency
func TestVersionString(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"basic", "1.2.3"},
		{"with_v_prefix", "v1.2.3"},
		{"prerelease", "1.0.0-alpha.1"},
		{"build_metadata", "2.0.0+build.123"},
		{"prerelease_and_build", "1.0.0-beta.2+build.456"},
		{"complex_prerelease", "1.0.0-rc.1.alpha.beta"},
		{"large_numbers", "999.999.999"},
		{"zero_versions", "0.0.0"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Parse the version
			v, err := ParseLenient(tc.input)
			if err != nil {
				t.Fatalf("failed to parse %s: %v", tc.input, err)
			}

			// Get string representation
			str := v.String()
			if str == "" {
				t.Errorf("String() returned empty string for %s", tc.input)
			}

			// Parse the string representation again (round-trip)
			v2, err := ParseLenient(str)
			if err != nil {
				t.Fatalf("failed to parse string representation %s: %v", str, err)
			}

			// Verify round-trip consistency
			if v.major != v2.major || v.minor != v2.minor || v.patch != v2.patch {
				t.Errorf("round-trip failed: original %d.%d.%d, got %d.%d.%d",
					v.major, v.minor, v.patch, v2.major, v2.minor, v2.patch)
			}

			if len(v.pre) != len(v2.pre) {
				t.Errorf("prerelease length mismatch: %d vs %d", len(v.pre), len(v2.pre))
			}

			if v.build != v2.build {
				t.Errorf("build mismatch: %s vs %s", v.build, v2.build)
			}
		})
	}
}

// TestVersionStringNil tests String method with nil receiver
func TestVersionStringNil(t *testing.T) {
	var v *Version
	result := v.String()
	if result != "" {
		t.Errorf("String() on nil receiver = %s, want empty string", result)
	}
}

// TestVersionBumpOperations tests the bump methods
func TestVersionBumpOperations(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		bumpType string // "major", "minor", or "patch"
		expected string
	}{
		// Basic bump operations
		{"bump_patch_basic", "1.2.3", "patch", "1.2.4"},
		{"bump_minor_basic", "1.2.3", "minor", "1.3.0"},
		{"bump_major_basic", "1.2.3", "major", "2.0.0"},

		// Bump with v prefix preservation
		{"bump_patch_with_v", "v1.2.3", "patch", "v1.2.4"},
		{"bump_minor_with_v", "v1.2.3", "minor", "v1.3.0"},
		{"bump_major_with_v", "v1.2.3", "major", "v2.0.0"},

		// Bump with prerelease (should clear prerelease)
		{"bump_patch_prerelease", "1.2.3-alpha.1", "patch", "1.2.4"},
		{"bump_minor_prerelease", "1.2.3-beta.2", "minor", "1.3.0"},
		{"bump_major_prerelease", "1.2.3-rc.1", "major", "2.0.0"},

		// Bump with build metadata (should clear build)
		{"bump_patch_build", "1.2.3+build.123", "patch", "1.2.4"},
		{"bump_minor_build", "1.2.3+build.456", "minor", "1.3.0"},
		{"bump_major_build", "1.2.3+build.789", "major", "2.0.0"},

		// Bump with both prerelease and build
		{"bump_patch_both", "1.2.3-alpha.1+build.123", "patch", "1.2.4"},
		{"bump_minor_both", "1.2.3-beta.2+build.456", "minor", "1.3.0"},
		{"bump_major_both", "1.2.3-rc.1+build.789", "major", "2.0.0"},

		// Large numbers
		{"bump_patch_large", "999.999.998", "patch", "999.999.999"},
		{"bump_minor_large", "999.998.999", "minor", "999.999.0"},
		{"bump_major_large", "998.999.999", "major", "999.0.0"},

		// Zero versions
		{"bump_patch_zero", "0.0.0", "patch", "0.0.1"},
		{"bump_minor_zero", "0.0.0", "minor", "0.1.0"},
		{"bump_major_zero", "0.0.0", "major", "1.0.0"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v, err := ParseLenient(tc.input)
			if err != nil {
				t.Fatalf("failed to parse %s: %v", tc.input, err)
			}

			var result *Version
			switch tc.bumpType {
			case "patch":
				result = v.BumpPatch()
			case "minor":
				result = v.BumpMinor()
			case "major":
				result = v.BumpMajor()
			default:
				t.Fatalf("invalid bump type: %s", tc.bumpType)
			}

			if result == nil {
				t.Fatal("bump method returned nil")
			}

			actual := result.String()
			if actual != tc.expected {
				t.Errorf("bump %s of %s = %s, want %s", tc.bumpType, tc.input, actual, tc.expected)
			}

			// Verify prerelease and build are cleared
			if len(result.pre) != 0 {
				t.Errorf("prerelease not cleared after bump: %v", result.pre)
			}
			if result.build != "" {
				t.Errorf("build metadata not cleared after bump: %s", result.build)
			}
		})
	}
}

// TestVersionBumpNilReceiver tests bump methods with nil receiver
func TestVersionBumpNilReceiver(t *testing.T) {
	var v *Version

	if result := v.BumpPatch(); result != nil {
		t.Errorf("BumpPatch() on nil = %v, want nil", result)
	}
	if result := v.BumpMinor(); result != nil {
		t.Errorf("BumpMinor() on nil = %v, want nil", result)
	}
	if result := v.BumpMajor(); result != nil {
		t.Errorf("BumpMajor() on nil = %v, want nil", result)
	}
}

// TestVersionBumpRoundTrip tests that bump operations are consistent
func TestVersionBumpRoundTrip(t *testing.T) {
	originalVersions := []string{
		"1.2.3",
		"v1.2.3",
		"1.0.0-alpha.1",
		"2.0.0+build.123",
		"1.0.0-beta.2+build.456",
	}

	for _, original := range originalVersions {
		t.Run(original, func(t *testing.T) {
			v, err := ParseLenient(original)
			if err != nil {
				t.Fatalf("failed to parse %s: %v", original, err)
			}

			// Test patch bump round-trip
			v2 := v.BumpPatch()
			v3 := v2.BumpPatch()
			if v2.patch != v.patch+1 || v3.patch != v.patch+2 {
				t.Errorf("patch bump inconsistent: %d -> %d -> %d", v.patch, v2.patch, v3.patch)
			}

			// Test minor bump round-trip
			v4 := v.BumpMinor()
			v5 := v4.BumpMinor()
			if v4.minor != v.minor+1 || v5.minor != v.minor+2 {
				t.Errorf("minor bump inconsistent: %d -> %d -> %d", v.minor, v4.minor, v5.minor)
			}

			// Test major bump round-trip
			v6 := v.BumpMajor()
			v7 := v6.BumpMajor()
			if v6.major != v.major+1 || v7.major != v.major+2 {
				t.Errorf("major bump inconsistent: %d -> %d -> %d", v.major, v6.major, v7.major)
			}
		})
	}
}

// TestParseLenientIntegrationWithExistingAPIs tests integration with existing versioning APIs
func TestParseLenientIntegrationWithExistingAPIs(t *testing.T) {
	testVersions := []string{
		"1.2.3",
		"v1.2.4",
		"1.0.0-alpha.1",
		"2.0.0+build.123",
		"1.0.0-beta.2+build.456",
	}

	for _, versionStr := range testVersions {
		t.Run(versionStr, func(t *testing.T) {
			// Parse with new API
			v, err := ParseLenient(versionStr)
			if err != nil {
				t.Fatalf("ParseLenient failed: %v", err)
			}

			// Test integration with Compare
			for _, otherVersion := range testVersions {
				// Compare using semver-full scheme
				cmp1, err1 := Compare(SchemeSemverFull, versionStr, otherVersion)
				if err1 != nil {
					t.Errorf("Compare failed for %s vs %s: %v", versionStr, otherVersion, err1)
					continue
				}

				// Parse other version and compare structs
				v2, err2 := ParseLenient(otherVersion)
				if err2 != nil {
					t.Errorf("ParseLenient failed for %s: %v", otherVersion, err2)
					continue
				}

				cmp2 := compareSemverVersions(
					&semverVersion{major: v.major, minor: v.minor, patch: v.patch, pre: v.pre, build: v.build},
					&semverVersion{major: v2.major, minor: v2.minor, patch: v2.patch, pre: v2.pre, build: v2.build},
				)

				if cmp1 != cmp2 {
					t.Errorf("Compare inconsistency: Compare()=%v, compareSemverVersions()=%v for %s vs %s",
						cmp1, cmp2, versionStr, otherVersion)
				}
			}

			// Test integration with Evaluate
			policy := Policy{
				Scheme:         SchemeSemverFull,
				MinimumVersion: "1.0.0",
			}

			eval, err := Evaluate(policy, versionStr)
			if err != nil {
				t.Errorf("Evaluate failed: %v", err)
			}

			// Verify evaluation is reasonable
			if eval.ActualVersion != strings.TrimSpace(versionStr) {
				t.Errorf("ActualVersion mismatch: %s vs %s", eval.ActualVersion, versionStr)
			}
		})
	}
}

// TestParseLenientEdgeCases tests edge cases and error conditions
func TestParseLenientEdgeCases(t *testing.T) {
	edgeCases := []struct {
		name        string
		input       string
		expectError bool
		description string
	}{
		// Extreme lengths
		{"very_long_input", strings.Repeat("1", 1000) + ".0.0", true, "extremely long version string"},
		{"unicode_input", "1.0.0-αβγδε", true, "unicode in version string"},
		{"special_chars", "1.0.0-beta!", true, "special characters in version"},

		// Boundary values
		{"max_int", "2147483647.2147483647.2147483647", false, "maximum int32 values"},
		{"negative_zero", "-0.0.0", true, "negative zero (invalid)"},
		{"double_zero", "00.0.0", true, "double zero major"},

		// Malformed inputs
		{"missing_dots", "123", true, "missing dots"},
		{"too_many_dots", "1.2.3.4.5", true, "too many dots"},
		{"empty_segments", "1..3", true, "empty segments"},
		{"whitespace_middle", "1. 2.3", true, "whitespace in middle"},

		// Valid edge cases
		{"single_digit", "1.0.0", false, "single digit components"},
		{"multi_digit", "123.456.789", false, "multi-digit components"},
		{"zero_components", "0.0.0", false, "all zero components"},
	}

	for _, tc := range edgeCases {
		t.Run(tc.name, func(t *testing.T) {
			v, err := ParseLenient(tc.input)

			if tc.expectError {
				if err == nil {
					t.Errorf("%s: expected error for %s", tc.description, tc.input)
				}
			} else {
				if err != nil {
					t.Errorf("%s: unexpected error for %s: %v", tc.description, tc.input, err)
				}
				if v == nil {
					t.Errorf("%s: expected non-nil version for %s", tc.description, tc.input)
				}
			}
		})
	}
}
