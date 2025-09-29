package assess

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSchemaRunner_GoodFixtures(t *testing.T) {
	r := NewSchemaAssessmentRunner()
	cfg := AssessmentConfig{Mode: AssessmentModeCheck, Timeout: 30 * time.Second}
	base := filepath.Join("..", "..", "tests", "fixtures", "schemas", "draft-07", "good")

	goodFiles := []string{
		filepath.Join(base, "good-config.yaml"),
		filepath.Join(base, "good-config.json"),
	}
	for _, f := range goodFiles {
		cfg.IncludeFiles = []string{f}
		res, err := r.Assess(context.Background(), ".", cfg)
		if err != nil {
			t.Fatalf("assess returned error for %s: %v", f, err)
		}
		if !res.Success {
			t.Fatalf("expected success for %s", f)
		}
		if len(res.Issues) != 0 {
			t.Fatalf("expected 0 issues for %s, got %d", f, len(res.Issues))
		}
	}
}

func TestSchemaRunner_BadFixtures(t *testing.T) {
	r := NewSchemaAssessmentRunner()
	cfg := AssessmentConfig{Mode: AssessmentModeCheck, Timeout: 30 * time.Second}
	base := filepath.Join("..", "..", "tests", "fixtures", "schemas", "draft-07", "bad")

	badFiles := []string{
		filepath.Join(base, "bad-required-wrong.yaml"),
		filepath.Join(base, "bad-additionalprops-wrong.json"),
	}
	for _, f := range badFiles {
		cfg.IncludeFiles = []string{f}
		res, err := r.Assess(context.Background(), ".", cfg)
		if err != nil {
			t.Fatalf("assess returned error for %s: %v", f, err)
		}
		if !res.Success {
			t.Fatalf("expected success flag even with issues for %s", f)
		}
		if len(res.Issues) == 0 {
			t.Fatalf("expected issues for %s, got none", f)
		}
	}
}

func TestSchemaRunner_ConfigMappingSuccess(t *testing.T) {
	r := NewSchemaAssessmentRunner()
	cfg := AssessmentConfig{
		Mode:         AssessmentModeCheck,
		Timeout:      30 * time.Second,
		IncludeFiles: []string{filepath.Join("config", "ascii", "terminal-overrides.yaml")},
		SchemaMapping: SchemaMappingConfig{
			Enabled: true,
		},
	}

	res, err := r.Assess(context.Background(), ".", cfg)
	if err != nil {
		t.Fatalf("assess returned error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success flag")
	}
	if len(res.Issues) != 0 {
		t.Fatalf("expected no issues for valid config mapping, got %d", len(res.Issues))
	}

	if res.Metrics == nil {
		t.Fatalf("expected metrics to be populated")
	}
	if v, ok := res.Metrics["schema_mapping_validation_success"].(int); !ok || v == 0 {
		t.Fatalf("expected schema_mapping_validation_success metric, got %+v", res.Metrics)
	}
	if rate, ok := res.Metrics["schema_mapping_detection_rate"].(float64); !ok || rate < 1.0 {
		t.Fatalf("expected detection rate metric, got %+v", res.Metrics)
	}
}

func TestSchemaRunner_ConfigMappingFailure(t *testing.T) {
	r := NewSchemaAssessmentRunner()
	badDir := filepath.Join("test_temp", "schema_mapping")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatalf("mkdir failure: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(badDir) })

	badConfigPath := filepath.Join(badDir, "bad-terminal-overrides.yaml")
	if err := os.WriteFile(badConfigPath, []byte("version: \"1.0.0\"\n"), 0o644); err != nil {
		t.Fatalf("write bad config: %v", err)
	}

	manifestContent := "version: \"1.0.0\"\n" +
		"mappings:\n" +
		"  - pattern: \"test_temp/schema_mapping/*.yaml\"\n" +
		"    schema_id: \"terminal-overrides-v1.0.0\"\n"

	if err := os.MkdirAll(".goneat", 0o755); err != nil {
		t.Fatalf("ensure .goneat dir: %v", err)
	}

	manifestPath := filepath.Join(".goneat", "test-schema-mappings.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(manifestPath) })

	cfg := AssessmentConfig{
		Mode:         AssessmentModeCheck,
		Timeout:      30 * time.Second,
		IncludeFiles: []string{badConfigPath},
		SchemaMapping: SchemaMappingConfig{
			Enabled:      true,
			ManifestPath: manifestPath,
			Strict:       true,
		},
	}

	res, err := r.Assess(context.Background(), badDir, cfg)
	if err != nil {
		t.Fatalf("assess returned error: %v", err)
	}
	if len(res.Issues) == 0 {
		t.Fatalf("expected issues for invalid config mapping")
	}

	foundMappingIssue := false
	for _, issue := range res.Issues {
		if issue.SubCategory == "schema_mapping_validation" || issue.SubCategory == "schema_mapping_missing" {
			foundMappingIssue = true
			break
		}
	}
	if !foundMappingIssue {
		t.Fatalf("expected schema mapping issue, got %+v", res.Issues)
	}
}
