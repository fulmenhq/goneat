package assess

import "testing"

func TestGenerateSummaryAggregations(t *testing.T) {
	supps := []Suppression{
		{Tool: "gosec", RuleID: "G404", File: "a.go"},
		{Tool: "gosec", RuleID: "G404", File: "a.go"},
		{Tool: "gosec", RuleID: "G304", File: "b.go"},
	}
	sum := GenerateSummary(supps)
	if sum.Total != 3 {
		t.Fatalf("total=%d", sum.Total)
	}
	if sum.ByRule["G404"] != 2 || sum.ByRule["G304"] != 1 {
		t.Fatalf("byrule=%v", sum.ByRule)
	}
	if sum.ByFile["a.go"] != 2 || sum.ByFile["b.go"] != 1 {
		t.Fatalf("byfile=%v", sum.ByFile)
	}
	files := sum.ByRuleFiles["G404"]
	if len(files) != 1 || files[0] != "a.go" {
		t.Fatalf("byrulefiles=%v", sum.ByRuleFiles)
	}
}
