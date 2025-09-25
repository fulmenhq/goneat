package guardian

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTempHomeGrant(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("GONEAT_HOME", dir)
	return dir
}

func TestIssueAndConsumeGrant(t *testing.T) {
	setupTempHomeGrant(t)

	policy := &ResolvedPolicy{
		Scope:     "git",
		Operation: "commit",
		Method:    MethodBrowser,
		Expires:   5 * time.Minute,
	}

	ctx := OperationContext{Branch: "main"}

	grant, err := IssueGrant("git", "commit", policy, ctx)
	if err != nil {
		t.Fatalf("IssueGrant failed: %v", err)
	}
	if grant.Branch != "main" {
		t.Fatalf("expected branch recorded, got %q", grant.Branch)
	}

	// Grant file should exist
	dir, err := GrantsDir()
	if err != nil {
		t.Fatalf("GrantsDir failed: %v", err)
	}
	path := filepath.Join(dir, grant.ID+".json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected grant file to exist: %v", err)
	}

	used, err := consumeGrant("git", "commit", ctx)
	if err != nil {
		t.Fatalf("consumeGrant failed: %v", err)
	}
	if !used {
		t.Fatal("expected grant to be consumed")
	}

	// Grant file should be removed
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected grant file removed, got err=%v", err)
	}
}

func TestIssueGrantRespectsExpiration(t *testing.T) {
	setupTempHomeGrant(t)

	policy := &ResolvedPolicy{
		Scope:     "git",
		Operation: "push",
		Method:    MethodBrowser,
		Expires:   1 * time.Second,
	}

	ctx := OperationContext{Remote: "origin"}
	_, err := IssueGrant("git", "push", policy, ctx)
	if err != nil {
		t.Fatalf("IssueGrant failed: %v", err)
	}

	// Wait for grant to expire
	time.Sleep(1100 * time.Millisecond)

	used, _ := consumeGrant("git", "push", ctx)
	if used {
		t.Fatal("expected expired grant to be ignored")
	}

	// Ensure expired grant cleaned on next issue
	_, err = IssueGrant("git", "push", policy, ctx)
	if err != nil {
		t.Fatalf("IssueGrant second run failed: %v", err)
	}
}
