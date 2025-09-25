package guardian

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fulmenhq/goneat/internal/assets"
	"gopkg.in/yaml.v3"
)

func TestEmbeddedTemplateExists(t *testing.T) {
	// List all files in the embedded FS
	t.Log("Listing embedded templates FS:")
	fs.WalkDir(assets.Templates, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			t.Errorf("Error walking FS: %v", err)
			return err
		}
		t.Logf("  %s", path)
		return nil
	})

	// Test that the embedded template file exists and contains expected content
	data, err := fs.ReadFile(assets.Templates, "embedded_templates/templates/guardian/approval.html")
	if err != nil {
		t.Fatalf("Failed to read embedded template: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "<!DOCTYPE html>") {
		t.Error("Embedded template does not contain expected HTML DOCTYPE")
	}

	if !strings.Contains(content, "{{.ProjectName}}") {
		t.Error("Embedded template does not render project name placeholder")
	}

	if !strings.Contains(content, "Approve") || !strings.Contains(content, "Deny") {
		t.Error("Embedded template does not contain expected buttons")
	}

	t.Logf("Embedded template size: %d bytes", len(data))
}

func TestApprovalPageTemplateLoading(t *testing.T) {
	// Test that the template() method loads the embedded template successfully
	page := &approvalPage{
		ApprovalSession: ApprovalSession{
			Scope:       "git",
			Operation:   "push",
			Reason:      "test",
			RequestedAt: time.Now(),
		},
		ProjectName:   "Example Project",
		CustomMessage: "Please confirm before deployment.",
		Nonce:         "test-nonce",
		MachineName:   "test-machine",
		ProjectFolder: "test-project",
		RiskLevel:     "critical",
		ExpiresIn:     "14:59",
		Timestamp:     "2023-01-01T12:00:00Z",
	}

	tmpl, err := page.template()
	if err != nil {
		t.Fatalf("Failed to load template: %v", err)
	}

	// Execute template with test data
	var buf strings.Builder
	err = tmpl.Execute(&buf, page)
	if err != nil {
		t.Fatalf("Failed to execute template: %v", err)
	}

	result := buf.String()

	// Verify it contains full template content, not fallback
	if !strings.Contains(result, "Example Project") {
		t.Error("Template does not include project name")
	}

	if !strings.Contains(result, "test-machine") || !strings.Contains(result, "test-project") {
		t.Error("Template does not contain expected machine/project data")
	}

	if !strings.Contains(result, "critical") {
		t.Error("Template does not contain expected risk level")
	}

	if !strings.Contains(result, "Please confirm before deployment.") {
		t.Error("Template missing custom message content")
	}

	t.Logf("Template execution successful, output length: %d", len(result))
}

func TestBrowserServerExpiry(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GONEAT_HOME", home)
	t.Setenv("GONEAT_GUARDIAN_AUTO_OPEN", "0")

	cfgPath, err := EnsureConfig()
	if err != nil {
		t.Fatalf("EnsureConfig failed: %v", err)
	}

	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config failed: %v", err)
	}

	var cfg ConfigRoot
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("unmarshal config failed: %v", err)
	}

	gitScope := cfg.Guardian.Scopes["git"]
	pushPolicy := gitScope.Operations["push"]
	pushPolicy.Expires = "1s"
	gitScope.Operations["push"] = pushPolicy
	cfg.Guardian.Scopes["git"] = gitScope
	cfg.Guardian.Security.Browser.TimeoutSeconds = 1
	cfg.Guardian.Security.Browser.AutoOpen = false
	cfg.Guardian.Security.Branding.ProjectName = "Expiry Test"

	bytes, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config failed: %v", err)
	}
	if err := os.WriteFile(cfgPath, bytes, 0o600); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	policy, err := engine.Check("git", "push", OperationContext{Branch: "main", Remote: "origin"})
	if err == nil {
		t.Fatalf("expected approval requirement")
	}
	if !IsApprovalRequired(err) {
		t.Fatalf("expected ErrApprovalRequired, got %v", err)
	}
	if policy == nil {
		t.Fatalf("expected resolved policy")
	}

	session := ApprovalSession{
		Scope:       "git",
		Operation:   "push",
		Policy:      policy,
		Reason:      "test",
		RequestedAt: time.Now().UTC(),
	}

	server, err := StartBrowserApproval(context.Background(), session)
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			t.Skipf("sandbox prevented listener allocation: %v", err)
		}
		t.Fatalf("StartBrowserApproval failed: %v", err)
	}

	if exp := server.EffectiveExpiry(); exp > 2*time.Second {
		t.Fatalf("expected short expiry, got %v", exp)
	}

	if err := server.Wait(); !errors.Is(err, ErrApprovalExpired) {
		t.Fatalf("expected expiry error, got %v", err)
	}

	infoPath := filepath.Join(home, "servers", "guardian.json")
	if _, err := os.Stat(infoPath); err == nil {
		t.Fatalf("server metadata should be cleaned up")
	}
}
