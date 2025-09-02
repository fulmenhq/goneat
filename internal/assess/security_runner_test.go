package assess

import (
	"encoding/json"
	"testing"
)

func TestParseGosecOutput_WellFormed(t *testing.T) {
	runner := NewSecurityAssessmentRunner()
	sample := map[string]interface{}{
		"Issues": []map[string]interface{}{
			{
				"severity": "HIGH",
				"details":  "hardcoded credentials",
				"file":     "internal/app/a.go",
				"line":     42,
				"rule_id":  "G101",
			},
			{
				"severity": "low",
				"details":  "minor issue",
				"file":     "pkg/b.go",
				"line":     "13",
				"rule_id":  "G999",
			},
		},
	}
	data, _ := json.Marshal(sample)
	issues, err := runner.parseGosecOutput(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}
	if issues[0].Severity != SeverityHigh {
		t.Fatalf("expected first severity high, got %v", issues[0].Severity)
	}
	if issues[0].Line != 42 {
		t.Fatalf("expected first line 42, got %d", issues[0].Line)
	}
	if issues[1].Severity != SeverityLow {
		t.Fatalf("expected second severity low, got %v", issues[1].Severity)
	}
	if issues[1].Line != 13 {
		t.Fatalf("expected second line 13 parsed from string, got %d", issues[1].Line)
	}
}

func TestParseGosecOutput_Noisy(t *testing.T) {
	runner := NewSecurityAssessmentRunner()
	noisy := []byte("gosec starting...\nWARNING something\n{\n  \"Issues\": [ { \"severity\": \"medium\", \"details\": \"x\", \"file\": \"x.go\", \"line\": 7, \"rule_id\": \"G000\" } ]\n}\ntrailer text\n")
	issues, err := runner.parseGosecOutput(noisy)
	if err != nil {
		t.Fatalf("unexpected error parsing noisy output: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Severity != SeverityMedium {
		t.Fatalf("expected severity medium, got %v", issues[0].Severity)
	}
}

func TestUniqueDirs(t *testing.T) {
	runner := NewSecurityAssessmentRunner()
	files := []string{
		"a/b/c.go",
		"a/b/d.go",
		"x/y/z.go",
	}
	dirs := runner.uniqueDirs(files)
	if len(dirs) != 2 {
		t.Fatalf("expected 2 unique dirs, got %d (%v)", len(dirs), dirs)
	}

	// Empty input falls back to ./...
	dirs2 := runner.uniqueDirs(nil)
	if len(dirs2) != 1 || dirs2[0] != "./..." {
		t.Fatalf("expected fallback ./..., got %v", dirs2)
	}
}
