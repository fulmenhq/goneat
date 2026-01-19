package cmd

import (
	"strings"
	"testing"

	"github.com/fulmenhq/goneat/pkg/dependencies"
)

func TestRenderDependenciesTextIncludesVulnInfo(t *testing.T) {
	result := &dependencies.AnalysisResult{
		Dependencies:    []dependencies.Dependency{},
		PackagesScanned: 10,
		Issues: []dependencies.Issue{
			{Type: "vulnerability", Severity: "info", Message: "Vulnerability report generated: sbom/vuln.json (packages=10 findings=5 suppressed=1 violations=0 fail_on=none; critical=1 high=2 medium=3 low=4 unknown=5)"},
		},
		Passed: true,
	}

	out := renderDependenciesText(result)
	if !strings.Contains(out, "Vulnerability report generated") {
		t.Fatalf("expected vuln info line, got: %s", out)
	}
	if !strings.Contains(out, "Issues:") {
		t.Fatalf("expected issues block, got: %s", out)
	}
	if !strings.Contains(out, "info: 1") {
		t.Fatalf("expected info count, got: %s", out)
	}
}
