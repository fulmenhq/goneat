package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

type validateReport struct {
	Summary struct {
		TotalIssues int `json:"total_issues"`
	} `json:"summary"`
}

func TestValidate_GoodSchemas(t *testing.T) {
	env := NewTestEnv(t)
	goneatPath := env.findGoneatBinary()
	if goneatPath == "" {
		t.Skip("goneat binary not found")
	}
	outFile := filepath.Join(env.Dir, "validate-good.json")
	cmd := exec.Command(goneatPath, "validate", "--include", "tests/fixtures/schemas/good", "--format", "json", "--output", outFile)
	cmd.Dir = repoRootFromIntegration()
	// Good schemas should succeed (exit code 0)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("validate good failed: %v\n%s", err, string(out))
	}
	// Parse report
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var rpt validateReport
	if err := json.Unmarshal(data, &rpt); err != nil {
		t.Fatalf("parse report: %v", err)
	}
	if rpt.Summary.TotalIssues != 0 {
		t.Fatalf("expected 0 issues, got %d", rpt.Summary.TotalIssues)
	}
}

func TestValidate_BadSchemas(t *testing.T) {
	env := NewTestEnv(t)
	goneatPath := env.findGoneatBinary()
	if goneatPath == "" {
		t.Skip("goneat binary not found")
	}
	outFile := filepath.Join(env.Dir, "validate-bad.json")
	// Pass explicit files to bypass .goneatignore for this test
	bad1 := filepath.Join("tests", "fixtures", "schemas", "bad", "bad-required-wrong.yaml")
	bad2 := filepath.Join("tests", "fixtures", "schemas", "bad", "bad-additionalprops-wrong.json")
	cmd := exec.Command(goneatPath, "validate", "--include", bad1, "--include", bad2, "--format", "json", "--output", outFile)
	cmd.Dir = repoRootFromIntegration()
	// Bad schemas should fail (non-zero), but still write report
	_, _ = cmd.CombinedOutput()
	if _, err := os.Stat(outFile); err != nil {
		t.Fatalf("expected output file, got error: %v", err)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var rpt validateReport
	if err := json.Unmarshal(data, &rpt); err != nil {
		t.Fatalf("parse report: %v", err)
	}
	if rpt.Summary.TotalIssues == 0 {
		t.Fatalf("expected issues > 0, got 0")
	}
}

// repoRootFromIntegration attempts to resolve repository root from the tests/integration directory
func repoRootFromIntegration() string {
	wd, _ := os.Getwd()
	// tests/integration -> repo root two levels up
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	// On Windows, ensure .exe naming is handled consistently elsewhere
	_ = runtime.GOOS
	return root
}
