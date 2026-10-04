package dependencies

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRequestedLicenseAssurance(t *testing.T) {
	for _, language := range []Language{LanguageGo, LanguageRust, LanguageTypeScript, LanguagePython, LanguageCSharp} {
		for _, requested := range []bool{false, true} {
			result := &AnalysisResult{Dependencies: []Dependency{}, Issues: []Issue{}, Passed: true}
			enforceRequestedLicenseAssurance(result, AnalysisConfig{CheckLicenses: requested}, language)
			if requested && (result.Passed || !hasLicenseCollectionError(result.Issues)) {
				t.Fatalf("%s empty/stub license assurance passed", language)
			}
			if !requested && (!result.Passed || len(result.Issues) != 0) {
				t.Fatalf("%s behavior changed without license request", language)
			}
		}
	}
	completeGo := &AnalysisResult{Dependencies: []Dependency{{Module: Module{Name: "example.test/dep"}, License: &License{Type: "MIT"}}}, Passed: true}
	enforceRequestedLicenseAssurance(completeGo, AnalysisConfig{CheckLicenses: true}, LanguageGo)
	if !completeGo.Passed || len(completeGo.Issues) != 0 {
		t.Fatal("undetected language affected complete Go result")
	}
	rustWithMissingTool := &AnalysisResult{Dependencies: completeGo.Dependencies, Issues: []Issue{{Type: "configuration", Message: "cargo-deny not installed"}}, Passed: true}
	enforceRequestedLicenseAssurance(rustWithMissingTool, AnalysisConfig{CheckLicenses: true}, LanguageRust)
	if rustWithMissingTool.Passed || !hasLicenseCollectionError(rustWithMissingTool.Issues) {
		t.Fatal("missing Rust tool soft-passed even with diagnostic dependency rows")
	}
}

func TestUnsupportedLicenseCollectors(t *testing.T) {
	for _, tc := range []struct {
		language Language
		analyzer Analyzer
	}{
		{LanguageTypeScript, NewTypeScriptAnalyzer()},
		{LanguagePython, NewPythonAnalyzer()},
		{LanguageCSharp, NewCSharpAnalyzer()},
	} {
		t.Run(string(tc.language), func(t *testing.T) {
			result, err := RunCoolingAwareAnalysis(context.Background(), t.TempDir(), AnalysisConfig{CheckLicenses: true}, tc.language, tc.analyzer, nil)
			if err != nil || result.Passed || !hasLicenseCollectionError(result.Issues) || !strings.Contains(result.Issues[0].Message, "unsupported") {
				t.Fatalf("stub did not report unsupported inventory: %+v %v", result, err)
			}
		})
	}
}

func TestRustLicenseMissingToolsFailsClosed(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \"fixture\"\nversion = \"0.1.0\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := RunCoolingAwareAnalysis(context.Background(), dir, AnalysisConfig{CheckLicenses: true}, LanguageRust, NewRustAnalyzer(), nil)
	if err != nil || result.Passed || !hasLicenseCollectionError(result.Issues) {
		t.Fatalf("missing cargo/cargo-deny did not fail requested licenses: %+v %v", result, err)
	}
}
