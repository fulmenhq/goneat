package doctor

import (
	"strings"
	"testing"
)

func TestGetToolByName_Known(t *testing.T) {
	tool, ok := GetToolByName("GoSeC")
	if !ok {
		t.Fatalf("expected to find known tool 'gosec'")
	}
	if tool.Name != "gosec" {
		t.Fatalf("expected tool name 'gosec', got %q", tool.Name)
	}
}

func TestGetToolByName_Gitleaks(t *testing.T) {
	tool, ok := GetToolByName("gitleaks")
	if !ok {
		t.Fatalf("expected to find known tool 'gitleaks'")
	}
	if tool.Name != "gitleaks" {
		t.Fatalf("expected tool name 'gitleaks', got %q", tool.Name)
	}
	if tool.Kind != "go" || tool.InstallPackage == "" {
		t.Fatalf("expected gitleaks to be go-installable with a package path")
	}
	// Ensure correct module path is used
	if tool.InstallPackage != "github.com/zricethezav/gitleaks/v8@latest" {
		t.Fatalf("unexpected gitleaks install path: %q", tool.InstallPackage)
	}
}

func TestGetToolByName_Unknown(t *testing.T) {
	_, ok := GetToolByName("not-a-real-tool")
	if ok {
		t.Fatalf("expected unknown tool to return ok=false")
	}
}

func TestGoInstallCommand(t *testing.T) {
	tool := Tool{
		Name:           "gosec",
		Kind:           "go",
		InstallPackage: "github.com/securego/gosec/v2/cmd/gosec@latest",
	}
	cmd := goInstallCommand(tool)
	if !strings.Contains(cmd, "go install") || !strings.Contains(cmd, tool.InstallPackage) {
		t.Fatalf("unexpected go install command: %q", cmd)
	}
}

func TestInstallInstruction_Go(t *testing.T) {
	tool := Tool{
		Name:           "govulncheck",
		Kind:           "go",
		InstallPackage: "golang.org/x/vuln/cmd/govulncheck@latest",
	}
	inst := installInstruction(tool)
	if !strings.HasPrefix(inst, "go install ") || !strings.Contains(inst, tool.InstallPackage) {
		t.Fatalf("unexpected install instruction for go tool: %q", inst)
	}
}

func TestSanitizeVersion_CommonPatterns(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"gosec 2.19.0", "gosec 2.19.0"},
		{"version v1.2.3", "v1.2.3"},
		{"Version 1.0.0", "1.0.0"},
		{"govulncheck: version v1.1.0", "v1.1.0"},
	}
	for _, c := range cases {
		got := sanitizeVersion(c.in)
		if got != c.want {
			t.Fatalf("sanitizeVersion(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestExtractFirstVersionToken(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"usage: something v0.9.0 build xyz", "v0.9.0"},
		{"tool 1.2.3 extra", "1.2.3"},
		{"no version tokens here", ""},
	}
	for _, c := range cases {
		got := extractFirstVersionToken(c.in)
		if got != c.want {
			t.Fatalf("extractFirstVersionToken(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestLooksLikeVersion(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"v1.2.3", true},
		{"1.2.3", true},
		{"1.2", true},
		{"v1", false},
		{"version", false},
	}
	for _, c := range cases {
		if got := looksLikeVersion(c.in); got != c.want {
			t.Fatalf("looksLikeVersion(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
