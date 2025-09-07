package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	assess "github.com/fulmenhq/goneat/internal/assess"
)

type validateJSON struct {
	Summary struct {
		TotalIssues int `json:"total_issues"`
	} `json:"summary"`
}

func TestValidate_Good_Fixtures_JSON(t *testing.T) {
	// Ensure schema runner is real (other tests may replace it)
	assess.RegisterAssessmentRunner(assess.CategorySchema, assess.NewSchemaAssessmentRunner())
	// Write to a temp file to avoid log noise in stdout
	tmp := filepath.Join(t.TempDir(), "out.json")
	out, err := execRoot(t, []string{"validate", "--include", "tests/fixtures/schemas/good", "--format", "json", "--output", tmp})
	if err != nil {
		t.Fatalf("validate good failed: %v\n%s", err, out)
	}
	data, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	// Output may contain logs in file? should be clean JSON
	rpt, err := parseReportJSON(data)
	if err != nil {
		t.Fatalf("validate output not parseable JSON: %v\n%s", err, string(data))
	}
	var v validateJSON
	b, _ := json.Marshal(rpt)
	_ = json.Unmarshal(b, &v)
	if v.Summary.TotalIssues != 0 {
		t.Errorf("expected 0 issues for good fixtures, got %d", v.Summary.TotalIssues)
	}
}

func TestValidate_Bad_Single_JSON(t *testing.T) {
	// Ensure schema runner is real (other tests may replace it)
	assess.RegisterAssessmentRunner(assess.CategorySchema, assess.NewSchemaAssessmentRunner())
	// Ensure we run from repo root so relative paths resolve
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("chdir back failed: %v", err)
		}
	}()
	// Walk up to find go.mod
	cur := wd
	for i := 0; i < 5; i++ {
		if _, err := os.Stat(filepath.Join(cur, "go.mod")); err == nil {
			if err := os.Chdir(cur); err != nil {
				t.Fatalf("chdir to module root failed: %v", err)
			}
			break
		}
		cur = filepath.Dir(cur)
	}
	path := filepath.Join("tests", "fixtures", "schemas", "bad", "bad-required-wrong.yaml")
	tmp := filepath.Join(t.TempDir(), "out.json")
	out, _ := execRoot(t, []string{"validate", "--include", path, "--format", "json", "--fail-on", "info", "--output", tmp})
	// Command returns non-zero on failure gate; still parse JSON
	data, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	rpt, err := parseReportJSON(data)
	if err != nil {
		t.Fatalf("validate output not parseable JSON: %v\n%s", err, string(data))
	}
	var v validateJSON
	b, _ := json.Marshal(rpt)
	_ = json.Unmarshal(b, &v)
	if v.Summary.TotalIssues == 0 {
		t.Errorf("expected issues for bad fixture, got 0\n%s", out)
	}
}
