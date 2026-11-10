package assess

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeSecurityPolicy(t *testing.T, root, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".goneat"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".goneat", "dependencies.yaml"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestSecurityAllowlistExpiry(t *testing.T) {
	now := time.Date(2030, 1, 5, 0, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		name, entry     string
		active, invalid bool
	}{
		{"absent expiry", "", true, false},
		{"future", "until: 2030-01-06", true, false},
		{"quoted future", "until: '2030-01-06'", true, false},
		{"expired", "until: 2030-01-04", false, false},
		{"at expiry", "until: 2030-01-05", false, false},
		{"invalid", "until: never", false, true},
		{"short date", "until: 2030-1-6", false, true},
		{"timestamp", "until: 2030-01-06T00:00:00Z", false, true},
		{"null", "until: null", false, true},
		{"tilde", "until: ~", false, true},
		{"empty", "until: ''", false, true},
		{"bare", "until:", false, true},
		{"number", "until: 123", false, true},
		{"list", "until: []", false, true},
		{"mapping", "until: {}", false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeSecurityPolicy(t, root, "vulnerabilities:\n  enabled: false\n  allow:\n    - id: GO-2026-5932\n      "+tt.entry+"\n")
			allowed, err := loadSecurityAllowlist(root, now)
			if (err != nil) != tt.invalid || allowed[govulnInventoryAdvisory] != tt.active {
				t.Fatalf("allowed=%v err=%v", allowed, err)
			}
		})
	}
}

func TestSecurityAllowlistRejectsMalformedPolicy(t *testing.T) {
	for _, body := range []string{
		"vulnerabilities: [", "vulnerabilities: {allow: wrong}",
		"vulnerabilities: {allow: [{id: ''}]}",
		"vulnerabilities: {allow: [{id: GO-1}, {id: GO-1, until: '2030-01-01'}]}",
		"defaults: &defaults {until: never}\nvulnerabilities: {allow: [{<<: *defaults, id: GO-1}]}",
	} {
		root := t.TempDir()
		writeSecurityPolicy(t, root, body)
		if allowed, err := loadSecurityAllowlist(root, time.Now()); err == nil || len(allowed) != 0 {
			t.Fatalf("malformed policy allowed: %s %v %v", body, allowed, err)
		}
	}
	root := t.TempDir()
	if _, err := loadSecurityAllowlist(root, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".goneat", "dependencies.yaml"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := loadSecurityAllowlist(root, time.Now()); err == nil {
		t.Fatal("unreadable policy must fail")
	}
}

func TestGovulnAllowlistIsExplicitAndFailClosed(t *testing.T) {
	for _, tt := range []struct {
		name       string
		allowed    map[string]bool
		completion error
		want       IssueSeverity
	}{
		{"no policy", nil, nil, SeverityHigh},
		{"different id", map[string]bool{"GO-OTHER": true}, nil, SeverityHigh},
		{"expired", map[string]bool{govulnInventoryAdvisory: false}, nil, SeverityHigh},
		{"listed", map[string]bool{govulnInventoryAdvisory: true}, nil, SeverityInfo},
		{"execution failed", map[string]bool{govulnInventoryAdvisory: true}, errors.New("failed"), SeverityHigh},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r, parseErr := readGovulnStream("/fixture", strings.NewReader(govulnInventoryStream(t, govulnModuleInventoryFixture)))
			issues, _, err := r.finishWithAllowlist(errors.Join(parseErr, tt.completion), tt.allowed)
			if len(issues) != 1 || issues[0].Severity != tt.want || (err != nil) != (tt.completion != nil) {
				t.Fatalf("issues=%+v err=%v", issues, err)
			}
		})
	}
}

func TestGovulncheckProcessAppliesRepositoryAllowlist(t *testing.T) {
	installSecurityProcessFixture(t, "govulncheck")
	t.Setenv("GONEAT_SECURITY_PROCESS_FIXTURE", "govuln-inventory")
	root := t.TempDir()
	writeSecurityPolicy(t, root, "vulnerabilities: {allow: [{id: GO-2026-5932, until: '2999-01-01'}]}")
	issues, _, err := NewSecurityAssessmentRunner().runGovulncheckWithMetadata(t.Context(), root, AssessmentConfig{})
	if err != nil || len(issues) != 1 || issues[0].Severity != SeverityInfo {
		t.Fatalf("issues=%+v err=%v", issues, err)
	}
}
