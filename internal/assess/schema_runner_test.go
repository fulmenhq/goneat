package assess

import (
    "context"
    "path/filepath"
    "testing"
    "time"
)

func TestSchemaRunner_GoodFixtures(t *testing.T) {
    r := NewSchemaAssessmentRunner()
    cfg := AssessmentConfig{Mode: AssessmentModeCheck, Timeout: 30 * time.Second}
    base := filepath.Join("..", "..", "tests", "fixtures", "schemas", "good")

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
    base := filepath.Join("..", "..", "tests", "fixtures", "schemas", "bad")

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
        // Ensure subcategory and category are set
        found := false
        for _, is := range res.Issues {
            if is.Category == CategorySchema && is.SubCategory == "jsonschema" {
                found = true
                break
            }
        }
        if !found {
            t.Fatalf("expected at least one jsonschema issue for %s", f)
        }
    }
}

