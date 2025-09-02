package assess

import (
	"testing"
)

func TestParseGovulnEventLine(t *testing.T) {
	r := NewSecurityAssessmentRunner()
	line := `{"type":"finding","finding":{"osv":"GO-2023-0001","module":{"path":"example.com/mod"},"package":{"path":"example.com/mod/pkg"}}}`
	if iss, ok := r.parseGovulnEventLine("/repo", line); !ok {
		t.Fatalf("expected finding parsed")
	} else {
		if iss.Severity != SeverityHigh {
			t.Fatalf("expected SeverityHigh, got %v", iss.Severity)
		}
		if iss.Category != CategorySecurity {
			t.Fatalf("wrong category")
		}
		if iss.SubCategory != "vulnerability" {
			t.Fatalf("wrong subcategory")
		}
		if iss.File == "" {
			t.Fatalf("expected file path set")
		}
	}
	// Non-JSON
	if _, ok := r.parseGovulnEventLine("/repo", "progress..."); ok {
		t.Fatalf("expected non-json line ignored")
	}
}

func TestParseGitleaks_ArrayAndNDJSON(t *testing.T) {
	r := NewSecurityAssessmentRunner()
	// Array form
	arr := []byte(`[{"RuleID":"generic","Description":"aws key","File":"a.txt","StartLine":5}]`)
	iss, err := r.parseGitleaksOutput(arr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(iss) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(iss))
	}
	// NDJSON form (two findings)
	nd := []byte("{\"Description\":\"secret1\",\"File\":\"b.txt\",\"StartLine\":10}\n{\"description\":\"secret2\",\"file\":\"c.txt\",\"line\":2}\n")
	iss2, err := r.parseGitleaksOutput(nd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(iss2) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(iss2))
	}
}
