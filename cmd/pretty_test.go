package cmd

import (
    "os"
    "path/filepath"
    "strings"
    "testing"
)

// minimalReportJSON returns a minimal valid AssessmentReport JSON
func minimalReportJSON() string {
    return `{
      "metadata": {"generated_at": "2025-01-01T00:00:00Z","tool":"goneat","version":"1.0.0","target":".","execution_time":0,"commands_run":[]},
      "summary": {"overall_health":1,"critical_issues":0,"total_issues":0,"estimated_time":0,"parallel_groups":0,"categories_with_issues":0},
      "categories": {},
      "workflow": {"phases": null, "parallel_groups": null, "total_time": 0}
    }`
}

func TestParseReportJSON_Valid(t *testing.T) {
    data := []byte(minimalReportJSON())
    r, err := parseReportJSON(data)
    if err != nil {
        t.Fatalf("parseReportJSON failed: %v", err)
    }
    if r == nil || r.Metadata.Tool != "goneat" {
        t.Fatalf("unexpected report parsed: %+v", r)
    }
}

func TestParseReportJSON_WithLogPreamble(t *testing.T) {
    payload := "[INFO] something before...\n" + minimalReportJSON() + "\n[INFO] after"
    r, err := parseReportJSON([]byte(payload))
    if err != nil {
        t.Fatalf("parseReportJSON with preamble failed: %v", err)
    }
    if r.Summary.TotalIssues != 0 {
        t.Errorf("expected 0 issues, got %d", r.Summary.TotalIssues)
    }
}

func TestPrettyConsole_FromFile(t *testing.T) {
    // Write minimal report to a temp file
    dir := t.TempDir()
    path := filepath.Join(dir, "report.json")
    if err := os.WriteFile(path, []byte(minimalReportJSON()), 0o600); err != nil {
        t.Fatalf("write temp report: %v", err)
    }
    out, err := execRoot(t, []string{"pretty", "--from", "json", "--to", "console", "--input", path})
    if err != nil {
        t.Fatalf("pretty console failed: %v\n%s", err, out)
    }
    if !strings.Contains(out, "Assessment") || !strings.Contains(out, "total issues") {
        t.Errorf("unexpected console output: %s", out)
    }
}

