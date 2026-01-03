package assess

import "testing"

func TestParseCargoClippyOutput(t *testing.T) {
	out := []byte(`{"reason":"compiler-message","message":{"message":"use of unwrap","level":"warning","code":{"code":"clippy::unwrap_used"},"spans":[{"file_name":"src/lib.rs","line_start":10,"column_start":5,"is_primary":true}]}}
{"reason":"compiler-message","message":{"message":"panic detected","level":"error","spans":[{"file_name":"src/main.rs","line_start":3,"column_start":1,"is_primary":true}]}}
{"reason":"build-finished","success":true}`)

	issues, err := parseCargoClippyOutput(out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}
	if issues[0].Severity != SeverityMedium {
		t.Fatalf("expected warning to map to medium, got %v", issues[0].Severity)
	}
	if issues[0].SubCategory != "rust:clippy" {
		t.Fatalf("expected subcategory rust:clippy, got %s", issues[0].SubCategory)
	}
	if issues[1].Severity != SeverityHigh {
		t.Fatalf("expected error to map to high, got %v", issues[1].Severity)
	}
}

func TestMapClippySeverity(t *testing.T) {
	if _, ok := mapClippySeverity("note"); ok {
		t.Fatalf("expected note to be ignored")
	}
	if sev, ok := mapClippySeverity("warning"); !ok || sev != SeverityMedium {
		t.Fatalf("expected warning to map to medium")
	}
	if sev, ok := mapClippySeverity("error"); !ok || sev != SeverityHigh {
		t.Fatalf("expected error to map to high")
	}
}
