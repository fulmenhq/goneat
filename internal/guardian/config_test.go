package guardian_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fulmenhq/goneat/internal/guardian"
)

func setupTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("GONEAT_HOME", home)
	return home
}

func TestEnsureConfigCreatesDefault(t *testing.T) {
	home := setupTempHome(t)

	path, err := guardian.EnsureConfig()
	if err != nil {
		t.Fatalf("EnsureConfig() error = %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file to exist, got error %v", err)
	}

	expectedDir := filepath.Join(home, "guardian")
	if filepath.Dir(path) != expectedDir {
		t.Fatalf("expected config dir %s, got %s", expectedDir, filepath.Dir(path))
	}
}

func TestLoadConfigAndResolvePolicy(t *testing.T) {
	setupTempHome(t)

	cfg, err := guardian.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	policy, enforced, err := cfg.ResolvePolicy("git", "push")
	if err != nil {
		t.Fatalf("ResolvePolicy() error = %v", err)
	}
	if !enforced || policy == nil {
		t.Fatalf("expected git.push policy to be enforced")
	}

	if policy.Method != guardian.MethodBrowser {
		t.Fatalf("expected method browser, got %s", policy.Method)
	}

	if !policy.RequireReason {
		t.Fatalf("expected push to require reason")
	}

	if policy.Expires <= 0 {
		t.Fatalf("expected positive expiry, got %s", policy.Expires)
	}
}

func TestEngineCheckHonorsConditions(t *testing.T) {
	setupTempHome(t)

	engine, err := guardian.NewEngine()
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	ctx := guardian.OperationContext{Branch: "main", Remote: "origin"}
	policy, err := engine.Check("git", "push", ctx)
	if err == nil {
		t.Fatalf("expected approval requirement error, got nil")
	}
	if !guardian.IsApprovalRequired(err) {
		t.Fatalf("expected ErrApprovalRequired, got %v", err)
	}
	if policy == nil {
		t.Fatalf("expected policy result")
	}

	// Branch mismatch should skip enforcement
	ctx = guardian.OperationContext{Branch: "feature/foo", Remote: "origin"}
	policy, err = engine.Check("git", "push", ctx)
	if err != nil {
		t.Fatalf("unexpected error for branch mismatch: %v", err)
	}
	if policy != nil {
		t.Fatalf("expected no policy enforcement for feature branch")
	}
}

func TestEngineCheckUsesGrant(t *testing.T) {
	setupTempHome(t)

	cfg, err := guardian.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	policy, enforced, err := cfg.ResolvePolicy("git", "commit")
	if err != nil {
		t.Fatalf("ResolvePolicy() error = %v", err)
	}
	if !enforced || policy == nil {
		t.Fatalf("expected git.commit policy to be enforced")
	}

	engine, err := guardian.NewEngine()
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	ctx := guardian.OperationContext{Branch: "main"}
	_, err = engine.Check("git", "commit", ctx)
	if err == nil || !guardian.IsApprovalRequired(err) {
		t.Fatalf("expected approval required before grant, got %v", err)
	}

	grant, err := guardian.IssueGrant("git", "commit", policy, ctx)
	if err != nil {
		t.Fatalf("IssueGrant() error = %v", err)
	}
	if grant == nil {
		t.Fatal("expected grant")
	}

	result, err := engine.Check("git", "commit", ctx)
	if err != nil {
		t.Fatalf("expected grant to satisfy check, got error %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil policy when grant consumed")
	}
}
