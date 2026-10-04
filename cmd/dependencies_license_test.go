package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulmenhq/goneat/pkg/dependencies"
	"github.com/spf13/cobra"
)

type licenseResultAnalyzer struct {
	result      *dependencies.AnalysisResult
	err         error
	wantCooling bool
	t           *testing.T
}

func (a licenseResultAnalyzer) Analyze(_ context.Context, _ string, cfg dependencies.AnalysisConfig) (*dependencies.AnalysisResult, error) {
	if !cfg.CheckLicenses || cfg.CheckCooling != a.wantCooling {
		a.t.Fatalf("explicit gates were not threaded: %+v", cfg)
	}
	return a.result, a.err
}

func (a licenseResultAnalyzer) DetectLanguages(string) ([]dependencies.Language, error) {
	return []dependencies.Language{dependencies.LanguageGo}, nil
}

func TestDependenciesLicenseOutputAndExit(t *testing.T) {
	for _, tc := range []struct {
		name        string
		issues      []dependencies.Issue
		analysisErr error
		cooling     bool
		wantFailure bool
	}{
		{"complete", nil, nil, false, false},
		{"collector_failure", nil, errors.New("collector failed"), false, true},
		{"empty", []dependencies.Issue{{Type: "license_error", Severity: "critical", Message: "license inventory is empty"}}, nil, false, true},
		{"partial", []dependencies.Issue{{Type: "license_error", Severity: "critical", Message: "partial coverage"}}, nil, false, true},
		{"unresolved", []dependencies.Issue{{Type: "license", Severity: "critical", Message: "unresolved required license"}}, nil, false, true},
		{"cooling_pass_license_error", []dependencies.Issue{{Type: "license_error", Severity: "critical", Message: "license collection degraded"}}, nil, true, true},
		{"license_pass_cooling_failure", []dependencies.Issue{{Type: "age_violation", Severity: "high", Message: "cooling age violation"}}, nil, true, true},
		{"both_pass", nil, nil, true, false},
	} {
		for _, format := range []string{"json", "text"} {
			t.Run(tc.name+"/"+format, func(t *testing.T) {
				dir := t.TempDir()
				if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test/license-cli\ngo 1.26.0\n"), 0600); err != nil {
					t.Fatal(err)
				}
				output := filepath.Join(dir, "result."+format)
				command := &cobra.Command{Use: "dependencies"}
				command.Flags().Bool("licenses", true, "")
				command.Flags().Bool("cooling", tc.cooling, "")
				command.Flags().String("policy", "", "")
				command.Flags().String("format", format, "")
				command.Flags().String("output", output, "")
				command.Flags().String("fail-on", "critical", "")
				analyzer := licenseResultAnalyzer{
					result: &dependencies.AnalysisResult{Dependencies: []dependencies.Dependency{{Module: dependencies.Module{Name: "example.test/licensed"}, License: &dependencies.License{Type: "MIT"}}}, Issues: tc.issues, Passed: !tc.wantFailure},
					err:    tc.analysisErr, wantCooling: tc.cooling, t: t,
				}
				err := runDependenciesWithGoAnalyzer(command, []string{dir}, analyzer)
				if (err != nil) != tc.wantFailure {
					t.Fatalf("exit contract: err=%v wantFailure=%v", err, tc.wantFailure)
				}
				data, readErr := os.ReadFile(output)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if format == "json" {
					var result dependencies.AnalysisResult
					if err := json.Unmarshal(data, &result); err != nil {
						t.Fatalf("machine output invalid: %v\n%s", err, data)
					}
					if result.Passed == tc.wantFailure || (tc.wantFailure && len(result.Issues) == 0) {
						t.Fatalf("JSON false pass or missing diagnostic: %s", data)
					}
				} else {
					if tc.wantFailure && !strings.Contains(string(data), "Passed: false") {
						t.Fatalf("human false pass: %s", data)
					}
					if len(tc.issues) > 0 && strings.HasPrefix(tc.issues[0].Type, "license") && !strings.Contains(string(data), tc.issues[0].Message) {
						t.Fatalf("human diagnostic omitted: %s", data)
					}
					if tc.analysisErr != nil && !strings.Contains(string(data), tc.analysisErr.Error()) {
						t.Fatalf("collector error omitted: %s", data)
					}
				}
			})
		}
	}
}

func TestDependenciesCollectionErrorCannotBeThresholdedAway(t *testing.T) {
	for _, threshold := range []string{"critical", "high", "medium", "low", "any", "none"} {
		if !shouldFailDependencies(&dependencies.AnalysisResult{Passed: false, Issues: []dependencies.Issue{{Type: "license_error", Severity: "critical"}}}, threshold) {
			t.Fatalf("%s cleared collection error", threshold)
		}
		if !shouldFailDependencies(&dependencies.AnalysisResult{Passed: false, Issues: []dependencies.Issue{{Type: "age_violation", Severity: "high"}}}, threshold) {
			t.Fatalf("%s cleared combined gate failure", threshold)
		}
	}
}
