package dependencies

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fulmenhq/goneat/pkg/registry"
)

type stubCratesClient struct {
	meta map[string]*registry.Metadata
	err  map[string]error
}

func (s *stubCratesClient) GetMetadata(name, version string) (*registry.Metadata, error) {
	key := name + "@" + version
	if s.err != nil {
		if err, ok := s.err[key]; ok {
			return nil, err
		}
	}
	if s.meta != nil {
		if m, ok := s.meta[key]; ok {
			return m, nil
		}
	}
	return nil, fmt.Errorf("version %s not found in crate metadata", version)
}

func rustCoolingFixture(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("testdata", "rust-cooling"))
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func rustCoolingPolicy(t *testing.T) string {
	t.Helper()
	return filepath.Join(rustCoolingFixture(t), "policy-age.yaml")
}

func stubMeta(ageDays, downloads int) *registry.Metadata {
	return &registry.Metadata{
		PublishDate:    time.Now().Add(-time.Duration(ageDays) * 24 * time.Hour),
		TotalDownloads: downloads,
	}
}

func TestRustAnalyzer_Cooling_YoungCrateFails(t *testing.T) {
	client := &stubCratesClient{
		meta: map[string]*registry.Metadata{
			"young-crate@0.1.0":     stubMeta(1, 5000),
			"estate-helper@0.2.0":   stubMeta(1, 5000),
			"serde@1.0.195":         stubMeta(400, 250000000),
			"3leaps-internal@0.1.0": stubMeta(1, 10),
		},
	}

	analyzer := NewRustAnalyzerWithClient(client)
	result, err := analyzer.Analyze(context.Background(), rustCoolingFixture(t), AnalysisConfig{
		PolicyPath:    rustCoolingPolicy(t),
		CheckCooling:  true,
		CheckLicenses: false,
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Passed {
		t.Fatal("expected cooling to fail a crates.io package younger than min_age_days")
	}

	var youngFailed, serdeFailed, estateFailed, prefixFailed, gitFailed bool
	for _, issue := range result.Issues {
		if issue.Dependency == nil {
			continue
		}
		switch issue.Dependency.Name {
		case "young-crate":
			youngFailed = true
		case "serde":
			serdeFailed = true
		case "estate-helper":
			estateFailed = true
		case "3leaps-internal":
			prefixFailed = true
		case "git-only-dep":
			gitFailed = true
		}
	}

	if !youngFailed {
		t.Error("young-crate (1 day old) must fail cooling")
	}
	if serdeFailed {
		t.Error("serde (old enough) should not fail age cooling")
	}
	if estateFailed {
		t.Error("estate-helper exact name must be excepted")
	}
	if prefixFailed {
		t.Error("3leaps-internal must match 3leaps-* and not trip cooling")
	}
	if !gitFailed {
		t.Error("git-only-dep has no age_days and must fail closed")
	}

	// github.com/3leaps/* is in the policy; a crates.io crate must not be excepted by it.
	for _, dep := range result.Dependencies {
		if dep.Name != "young-crate" {
			continue
		}
		if _, ok := dep.Metadata["age_days"].(int); !ok {
			t.Error("young-crate should have age_days from crates.io metadata")
		}
		if dep.Metadata["recent_downloads"] != nil {
			t.Error("Rust cooling must not set recent_downloads (crates.io recent is per-version lifetime)")
		}
	}
}

func TestRustAnalyzer_Cooling_GithubOrgPatternDoesNotPassCrate(t *testing.T) {
	client := &stubCratesClient{
		meta: map[string]*registry.Metadata{
			"young-crate@0.1.0": stubMeta(1, 5000),
		},
	}
	analyzer := NewRustAnalyzerWithClient(client)
	result, err := analyzer.Analyze(context.Background(), rustCoolingFixture(t), AnalysisConfig{
		PolicyPath:    rustCoolingPolicy(t),
		CheckCooling:  true,
		CheckLicenses: false,
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	found := false
	for _, issue := range result.Issues {
		if issue.Dependency != nil && issue.Dependency.Name == "young-crate" && issue.Type == "age_violation" {
			found = true
		}
	}
	if !found {
		t.Fatal("github.com/3leaps/* must not silently pass young-crate")
	}
}

func TestRustAnalyzer_Cooling_MissingAgeDaysFails(t *testing.T) {
	client := &stubCratesClient{
		err: map[string]error{
			"young-crate@0.1.0":     fmt.Errorf("crates.io registry returned status 404"),
			"estate-helper@0.2.0":   fmt.Errorf("crates.io registry returned status 404"),
			"serde@1.0.195":         fmt.Errorf("crates.io registry returned status 404"),
			"3leaps-internal@0.1.0": fmt.Errorf("crates.io registry returned status 404"),
		},
	}
	analyzer := NewRustAnalyzerWithClient(client)
	result, err := analyzer.Analyze(context.Background(), rustCoolingFixture(t), AnalysisConfig{
		PolicyPath:    rustCoolingPolicy(t),
		CheckCooling:  true,
		CheckLicenses: false,
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Passed {
		t.Fatal("missing age_days must not pass cooling")
	}

	var serdeUnknown bool
	for _, dep := range result.Dependencies {
		if dep.Name != "serde" {
			continue
		}
		if _, ok := dep.Metadata["age_days"]; ok {
			t.Error("failed crates.io lookup must not stamp a fallback age_days")
		}
		if unknown, _ := dep.Metadata["age_unknown"].(bool); !unknown {
			t.Error("failed crates.io lookup should set age_unknown")
		}
		serdeUnknown = true
	}
	if !serdeUnknown {
		t.Fatal("expected serde in enumerated deps")
	}

	foundSerdeIssue := false
	for _, issue := range result.Issues {
		if issue.Dependency != nil && issue.Dependency.Name == "serde" && issue.Type == "age_violation" {
			foundSerdeIssue = true
		}
	}
	if !foundSerdeIssue {
		t.Error("serde with missing age_days must produce an age_violation")
	}
}

func TestAttachCratesIOMetadata_DoesNotSetRecentDownloads(t *testing.T) {
	deps := []Dependency{{
		Module: Module{Name: "serde", Version: "1.0.195", Language: LanguageRust},
		Metadata: map[string]interface{}{
			"registry": "crates.io",
		},
	}}
	client := &stubCratesClient{
		meta: map[string]*registry.Metadata{
			"serde@1.0.195": {
				PublishDate:     time.Now().Add(-400 * 24 * time.Hour),
				TotalDownloads:  250000000,
				RecentDownloads: 5000000, // crates.io version lifetime; must not be copied
			},
		},
	}
	attachCratesIOMetadata(deps, client)
	if deps[0].Metadata["recent_downloads"] != nil {
		t.Fatalf("recent_downloads must not be attached: %v", deps[0].Metadata["recent_downloads"])
	}
	if deps[0].Metadata["total_downloads"] != 250000000 {
		t.Errorf("total_downloads = %v", deps[0].Metadata["total_downloads"])
	}
}

func TestRustAnalyzer_Cooling_NoPolicyUsesSevenDayDefault(t *testing.T) {
	client := &stubCratesClient{
		meta: map[string]*registry.Metadata{
			"young-crate@0.1.0":     stubMeta(1, 5000),
			"estate-helper@0.2.0":   stubMeta(400, 5000),
			"serde@1.0.195":         stubMeta(400, 250000000),
			"3leaps-internal@0.1.0": stubMeta(400, 10),
		},
	}
	analyzer := NewRustAnalyzerWithClient(client)
	result, err := analyzer.Analyze(context.Background(), rustCoolingFixture(t), AnalysisConfig{
		PolicyPath:    filepath.Join(t.TempDir(), "missing-policy.yaml"),
		CheckCooling:  true,
		CheckLicenses: false,
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Passed {
		t.Fatal("missing policy must apply 7-day default, not vacuous-pass")
	}
	var youngFailed, configFail bool
	for _, issue := range result.Issues {
		if issue.Type == "configuration" {
			configFail = true
		}
		if issue.Dependency != nil && issue.Dependency.Name == "young-crate" && issue.Type == "age_violation" {
			youngFailed = true
		}
	}
	if configFail {
		t.Error("missing policy must not be a configuration fail")
	}
	if !youngFailed {
		t.Error("1-day crate must fail the built-in 7-day default")
	}
}

func TestApplyRustCooling_GraceTwoDayUUIDFails(t *testing.T) {
	policyPath := filepath.Join(t.TempDir(), "grace.yaml")
	if err := os.WriteFile(policyPath, []byte(`version: v1
cooling:
  enabled: true
  min_age_days: 7
  min_downloads: 0
  min_downloads_recent: 0
  grace_period_days: 3
  alert_only: false
`), 0o600); err != nil {
		t.Fatal(err)
	}

	deps := []Dependency{{
		Module: Module{Name: "uuid", Version: "1.24.1", Language: LanguageRust},
		Metadata: map[string]interface{}{
			"age_days":     2,
			"publish_date": time.Now().Add(-2 * 24 * time.Hour),
		},
	}}
	issues, passed := applyRustCooling(deps, policyPath)
	if passed {
		t.Fatal("uuid 2 days old with min_age=7 grace=3 must fail")
	}
	found := false
	for _, issue := range issues {
		if issue.Type == "age_violation" && issue.Dependency != nil && issue.Dependency.Name == "uuid" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected age_violation for uuid, issues=%v", issues)
	}
}

func TestPolyglot_GoFirstMatchStillCoolsRust(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/polyglot\n\ngo 1.25\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	src := rustCoolingFixture(t)
	for _, name := range []string{"Cargo.toml", "Cargo.lock"} {
		data, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	det := NewDetector(nil)
	lang, _, err := det.Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if lang != LanguageGo {
		t.Fatalf("expected Go first-match, got %s", lang)
	}
	if !ShouldAlsoCoolRust(dir, lang) {
		t.Fatal("Cargo.toml beside go.mod must trigger supplemental Rust cooling")
	}

	client := &stubCratesClient{
		meta: map[string]*registry.Metadata{
			"young-crate@0.1.0":     stubMeta(1, 5000),
			"estate-helper@0.2.0":   stubMeta(400, 5000),
			"serde@1.0.195":         stubMeta(400, 250000000),
			"3leaps-internal@0.1.0": stubMeta(400, 10),
		},
	}
	primary := &stubAnalyzer{result: &AnalysisResult{
		Dependencies: []Dependency{{Module: Module{Name: "example.com/polyglot", Language: LanguageGo}}},
		Issues:       []Issue{},
		Passed:       true,
	}}
	result, err := RunCoolingAwareAnalysis(context.Background(), dir, AnalysisConfig{
		PolicyPath:    rustCoolingPolicy(t),
		CheckCooling:  true,
		CheckLicenses: false,
	}, lang, primary, NewRustAnalyzerWithClient(client))
	if err != nil {
		t.Fatalf("RunCoolingAwareAnalysis: %v", err)
	}
	if result.Passed {
		t.Fatal("polyglot must not vacuous-pass: young rust crate should fail cooling")
	}
	var sawGo, sawYoung bool
	for _, dep := range result.Dependencies {
		if dep.Name == "example.com/polyglot" {
			sawGo = true
		}
		if dep.Name == "young-crate" {
			sawYoung = true
		}
	}
	if !sawGo || !sawYoung {
		t.Fatalf("expected merged Go+Rust inventory (go=%v rust=%v)", sawGo, sawYoung)
	}
}

type stubAnalyzer struct {
	result *AnalysisResult
}

func (s *stubAnalyzer) Analyze(context.Context, string, AnalysisConfig) (*AnalysisResult, error) {
	return s.result, nil
}

func (s *stubAnalyzer) DetectLanguages(string) ([]Language, error) {
	return []Language{LanguageGo}, nil
}

func TestEnumerateRustCrates_FromCargoLock(t *testing.T) {
	deps := enumerateRustCratesForCooling(context.Background(), rustCoolingFixture(t), time.Second)
	if len(deps) < 5 {
		t.Fatalf("expected Cargo.lock crates, got %d", len(deps))
	}
	var foundLocal, foundYoung bool
	for _, dep := range deps {
		if dep.Name == "cooling-fixture" {
			foundLocal = true
			if isLocal, _ := dep.Metadata["is_local"].(bool); !isLocal {
				t.Error("workspace package should be is_local")
			}
		}
		if dep.Name == "young-crate" {
			foundYoung = true
		}
	}
	if !foundLocal || !foundYoung {
		t.Fatalf("lock enumeration missing expected crates (local=%v young=%v)", foundLocal, foundYoung)
	}
}
