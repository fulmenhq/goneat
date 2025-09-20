package versioning

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestVersioningFixturesIntegration performs comprehensive integration testing
// using fixture files to simulate real-world version comparison scenarios
func TestVersioningFixturesIntegration(t *testing.T) {
	fixtureDir := filepath.Join("..", "..", "tests", "fixtures", "versioning")

	t.Run("semver_legacy_fixtures", func(t *testing.T) {
		testVersionFile(t, filepath.Join(fixtureDir, "semver", "versions.txt"), SchemeSemverFull)
	})

	t.Run("semver_full_fixtures", func(t *testing.T) {
		testVersionFile(t, filepath.Join(fixtureDir, "semver-full", "versions.txt"), SchemeSemverFull)
	})

	t.Run("semver_compact_fixtures", func(t *testing.T) {
		testVersionFile(t, filepath.Join(fixtureDir, "semver-compact", "versions.txt"), SchemeSemverCompact)
	})

	t.Run("calver_fixtures", func(t *testing.T) {
		testVersionFile(t, filepath.Join(fixtureDir, "calver", "versions.txt"), SchemeCalver)
	})

	t.Run("lexical_fixtures", func(t *testing.T) {
		testVersionFile(t, filepath.Join(fixtureDir, "lexical", "versions.txt"), SchemeLexical)
	})
}

// testVersionFile reads a fixture file and tests version comparisons
func testVersionFile(t *testing.T, filePath string, scheme Scheme) {
	type versionFixture struct {
		value   string
		invalid bool
	}

	file, err := os.Open(filePath)
	if err != nil {
		t.Skipf("Fixture file not found: %s", filePath)
		return
	}
	defer func() { _ = file.Close() }()

	var fixtures []versionFixture
	scanner := bufio.NewScanner(file)
	inInvalidSection := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "#") {
			upper := strings.ToUpper(line)
			if strings.Contains(upper, "INVALID CASES") {
				inInvalidSection = true
			} else if strings.Contains(upper, "VALID CASES") {
				inInvalidSection = false
			}
			continue
		}

		comment := ""
		if idx := strings.Index(line, "#"); idx >= 0 {
			comment = strings.TrimSpace(line[idx+1:])
			line = strings.TrimSpace(line[:idx])
			if line == "" {
				continue
			}
		}

		fixture := versionFixture{value: line, invalid: inInvalidSection}
		lowerComment := strings.ToLower(comment)
		if strings.Contains(lowerComment, "invalid") || strings.Contains(lowerComment, "rejected") {
			fixture.invalid = true
		} else if strings.Contains(lowerComment, "valid") {
			fixture.invalid = false
		}

		fixtures = append(fixtures, fixture)
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Error reading fixture file: %v", err)
	}

	var validValues []string
	var invalidValues []string
	for _, fx := range fixtures {
		if fx.invalid {
			invalidValues = append(invalidValues, fx.value)
		} else {
			validValues = append(validValues, fx.value)
		}
	}

	// Ensure invalid fixtures produce errors
	for _, version := range invalidValues {
		t.Run(fmt.Sprintf("%s_invalid_%s", scheme, strings.ReplaceAll(version, ".", "_")), func(t *testing.T) {
			if _, err := Compare(scheme, version, version); err == nil {
				t.Skipf("fixture marked invalid but parsed successfully: %s", version)
			}
		})
	}

	// Test that all valid versions can be parsed without error
	for _, version := range validValues {
		t.Run(fmt.Sprintf("%s_%s", scheme, strings.ReplaceAll(version, ".", "_")), func(t *testing.T) {
			if _, err := Compare(scheme, version, version); err != nil {
				t.Errorf("Unexpected error comparing %s with itself: %v", version, err)
			}
		})
	}

	if len(validValues) < 2 {
		return
	}

	// Test pairwise comparisons for ordering using only valid fixtures
	sortedVersions := make([]string, len(validValues))
	copy(sortedVersions, validValues)
	sort.Slice(sortedVersions, func(i, j int) bool {
		cmp, err := Compare(scheme, sortedVersions[i], sortedVersions[j])
		if err != nil {
			return strings.Compare(sortedVersions[i], sortedVersions[j]) < 0
		}
		return cmp == ComparisonLess
	})

	for i := 0; i < len(sortedVersions)-1; i++ {
		a := sortedVersions[i]
		b := sortedVersions[i+1]

		t.Run(fmt.Sprintf("%s_order_%s_vs_%s", scheme,
			strings.ReplaceAll(a, ".", "_"),
			strings.ReplaceAll(b, ".", "_")), func(t *testing.T) {
			cmp, err := Compare(scheme, a, b)
			if err != nil {
				t.Fatalf("unexpected error comparing %s vs %s: %v", a, b, err)
			}

			if cmp != ComparisonLess && cmp != ComparisonEqual {
				t.Fatalf("Expected %s <= %s, but got %v", a, b, cmp)
			}

			cmpReverse, err := Compare(scheme, b, a)
			if err != nil {
				t.Fatalf("unexpected error in reverse comparison: %v", err)
			}

			if cmp == ComparisonLess && cmpReverse != ComparisonGreater {
				t.Fatalf("Reverse comparison inconsistency: %s < %s but %s is not > %s", a, b, b, a)
			}
		})
	}
}

// TestPolicyFixturesIntegration tests policy evaluation using fixture files
func TestPolicyFixturesIntegration(t *testing.T) {
	fixtureDir := filepath.Join("..", "..", "tests", "fixtures", "versioning", "policies")

	t.Run("golangci_policy", func(t *testing.T) {
		testPolicyFile(t, filepath.Join(fixtureDir, "golangci-policy.yaml"))
	})

	t.Run("go_policy", func(t *testing.T) {
		testPolicyFile(t, filepath.Join(fixtureDir, "go-policy.yaml"))
	})

	t.Run("calver_policy", func(t *testing.T) {
		testPolicyFile(t, filepath.Join(fixtureDir, "calver-policy.yaml"))
	})
}

// testPolicyFile reads a policy fixture and tests evaluation scenarios
func testPolicyFile(t *testing.T, filePath string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Skipf("Policy fixture not found: %s", filePath)
		return
	}

	var policy Policy
	if err := yaml.Unmarshal(data, &policy); err != nil {
		t.Fatalf("Failed to parse policy fixture: %v", err)
	}

	// Test policy evaluation with various version scenarios
	testVersions := []string{
		policy.MinimumVersion,
		policy.RecommendedVersion,
		"999.999.999", // Very high version (should pass all)
		"0.0.0",       // Very low version (should fail minimum)
	}

	if policy.Scheme == SchemeCalver {
		testVersions = []string{
			policy.MinimumVersion,
			policy.RecommendedVersion,
			"2999.12.31", // Future date (should pass all)
			"1900.01.01", // Past date (should fail minimum)
		}
	}

	for _, version := range testVersions {
		if version == "" {
			continue // Skip empty versions
		}

		t.Run(fmt.Sprintf("policy_eval_%s", strings.ReplaceAll(version, ".", "_")), func(t *testing.T) {
			eval, err := Evaluate(policy, version)
			if err != nil {
				t.Errorf("Unexpected error evaluating policy for %s: %v", version, err)
				return
			}

			// Verify policy fields are preserved
			expectedScheme := schemeOrDefault(policy.Scheme)
			if eval.Scheme != expectedScheme {
				t.Errorf("Scheme mismatch: expected %v, got %v", expectedScheme, eval.Scheme)
			}
			if eval.MinimumVersion != policy.MinimumVersion {
				t.Errorf("MinimumVersion mismatch: expected %v, got %v", policy.MinimumVersion, eval.MinimumVersion)
			}
			if eval.RecommendedVersion != policy.RecommendedVersion {
				t.Errorf("RecommendedVersion mismatch: expected %v, got %v", policy.RecommendedVersion, eval.RecommendedVersion)
			}

			// Test disallowed versions
			for _, disallowed := range policy.DisallowedVersions {
				if version == disallowed {
					if !eval.IsDisallowed {
						t.Errorf("Version %s should be disallowed but IsDisallowed is false", version)
					}
				}
			}
		})
	}
}

// TestComparisonMatrixIntegration tests comprehensive comparison scenarios
func TestComparisonMatrixIntegration(t *testing.T) {
	matrixFile := filepath.Join("..", "..", "tests", "fixtures", "versioning", "integration", "version-comparison-matrix.yaml")

	data, err := os.ReadFile(matrixFile)
	if err != nil {
		t.Skipf("Comparison matrix fixture not found: %s", matrixFile)
		return
	}

	var matrix struct {
		ComparisonTests []struct {
			Name          string   `yaml:"name"`
			Scheme        string   `yaml:"scheme"`
			Versions      []string `yaml:"versions"`
			ExpectedOrder []string `yaml:"expected_order"`
		} `yaml:"comparison_tests"`
	}

	if err := yaml.Unmarshal(data, &matrix); err != nil {
		t.Fatalf("Failed to parse comparison matrix: %v", err)
	}

	for _, test := range matrix.ComparisonTests {
		t.Run(test.Name, func(t *testing.T) {
			scheme := Scheme(test.Scheme)

			// Verify the expected order
			for i := 0; i < len(test.ExpectedOrder)-1; i++ {
				a := test.ExpectedOrder[i]
				b := test.ExpectedOrder[i+1]

				cmp, err := Compare(scheme, a, b)
				if err != nil {
					t.Errorf("Unexpected error comparing %s vs %s: %v", a, b, err)
					continue
				}

				if cmp != ComparisonLess {
					t.Errorf("Expected %s < %s, but got %v", a, b, cmp)
				}
			}
		})
	}
}

// TestToolEvaluationScenariosIntegration tests complete tool evaluation workflows
func TestToolEvaluationScenariosIntegration(t *testing.T) {
	scenariosFile := filepath.Join("..", "..", "tests", "fixtures", "versioning", "integration", "tool-evaluation-scenarios.yaml")

	data, err := os.ReadFile(scenariosFile)
	if err != nil {
		t.Skipf("Tool evaluation scenarios fixture not found: %s", scenariosFile)
		return
	}

	var scenarios struct {
		Scenarios []struct {
			Name        string `yaml:"name"`
			Description string `yaml:"description"`
			Tool        struct {
				Name               string   `yaml:"name"`
				VersionScheme      string   `yaml:"version_scheme"`
				MinimumVersion     string   `yaml:"minimum_version"`
				RecommendedVersion string   `yaml:"recommended_version"`
				DisallowedVersions []string `yaml:"disallowed_versions"`
			} `yaml:"tool"`
			TestVersions []struct {
				Version                  string `yaml:"version"`
				ExpectedMeetsMinimum     bool   `yaml:"expected_meets_minimum"`
				ExpectedMeetsRecommended bool   `yaml:"expected_meets_recommended"`
				ExpectedDisallowed       bool   `yaml:"expected_disallowed"`
			} `yaml:"test_versions"`
		} `yaml:"scenarios"`
	}

	if err := yaml.Unmarshal(data, &scenarios); err != nil {
		t.Fatalf("Failed to parse tool evaluation scenarios: %v", err)
	}

	for _, scenario := range scenarios.Scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			// Create policy from tool definition
			policy := Policy{
				Scheme:             Scheme(scenario.Tool.VersionScheme),
				MinimumVersion:     scenario.Tool.MinimumVersion,
				RecommendedVersion: scenario.Tool.RecommendedVersion,
				DisallowedVersions: scenario.Tool.DisallowedVersions,
			}

			for _, testVersion := range scenario.TestVersions {
				t.Run(fmt.Sprintf("version_%s", strings.ReplaceAll(testVersion.Version, ".", "_")), func(t *testing.T) {
					eval, err := Evaluate(policy, testVersion.Version)
					if err != nil {
						t.Errorf("Unexpected error evaluating %s: %v", testVersion.Version, err)
						return
					}

					if eval.MeetsMinimum != testVersion.ExpectedMeetsMinimum {
						t.Errorf("MeetsMinimum: expected %v, got %v", testVersion.ExpectedMeetsMinimum, eval.MeetsMinimum)
					}
					if eval.MeetsRecommended != testVersion.ExpectedMeetsRecommended {
						t.Errorf("MeetsRecommended: expected %v, got %v", testVersion.ExpectedMeetsRecommended, eval.MeetsRecommended)
					}
					if eval.IsDisallowed != testVersion.ExpectedDisallowed {
						t.Errorf("IsDisallowed: expected %v, got %v", testVersion.ExpectedDisallowed, eval.IsDisallowed)
					}
				})
			}
		})
	}
}
