package assess

import (
    "testing"
    "time"
)

func minimalReport() *AssessmentReport {
    return &AssessmentReport{
        Metadata: ReportMetadata{
            GeneratedAt:   time.Now(),
            Tool:          "goneat",
            Version:       "test",
            Target:        ".",
            ExecutionTime: 0,
            CommandsRun:   []string{"schema"},
        },
        Summary: ReportSummary{
            OverallHealth: 1,
            TotalIssues:   0,
        },
        Categories: map[string]CategoryResult{},
    }
}

func TestFormatter_Concise_Minimal(t *testing.T) {
    f := NewFormatter(FormatConcise)
    out, err := f.FormatReport(minimalReport())
    if err != nil {
        t.Fatalf("format concise failed: %v", err)
    }
    if out == "" || !containsAll(out, []string{"Assessment", "total issues"}) {
        t.Errorf("unexpected concise output: %s", out)
    }
}

func containsAll(s string, parts []string) bool {
    for _, p := range parts {
        if !contains(s, p) { return false }
    }
    return true
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (len(sub) == 0 || (len(s) > 0 && (indexOf(s, sub) >= 0))) }

func indexOf(s, sub string) int {
    // simple substring search
    for i := 0; i+len(sub) <= len(s); i++ {
        if s[i:i+len(sub)] == sub { return i }
    }
    return -1
}

