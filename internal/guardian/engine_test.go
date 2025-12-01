package guardian

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	// Test creating a new engine
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
	if engine != nil && engine.config == nil {
		t.Fatal("expected engine to have config")
	}
}

func TestEngine_Check_NoPolicy(t *testing.T) {
	// Test checking a scope/operation with no policy
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	ctx := OperationContext{}
	policy, err := engine.Check("nonexistent", "scope", ctx)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if policy != nil {
		t.Errorf("expected nil policy for non-existent scope, got: %v", policy)
	}
}

func TestEngine_Check_DisabledOperation(t *testing.T) {
	// Test checking a disabled operation
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	// git commit is enabled: false in default config
	ctx := OperationContext{}
	policy, err := engine.Check("git", "commit", ctx)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if policy != nil {
		t.Errorf("expected nil policy for disabled operation, got: %v", policy)
	}
}

func TestEngine_Check_ApprovalRequired(t *testing.T) {
	// Test checking an operation that requires approval
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	// git push should require approval with proper context
	ctx := OperationContext{
		Branch: "main",
		Remote: "origin",
	}
	policy, err := engine.Check("git", "push", ctx)
	if err == nil {
		t.Fatal("expected approval required error, got nil")
	}
	if !IsApprovalRequired(err) {
		t.Errorf("expected approval required error, got: %v", err)
	}

	var approvalErr *ApprovalRequiredError
	if !errors.As(err, &approvalErr) {
		t.Errorf("expected ApprovalRequiredError, got: %T", err)
	}

	if policy == nil {
		t.Error("expected policy to be returned with approval error")
	} else {
		if policy.Method != MethodBrowser {
			t.Errorf("expected browser method, got: %s", policy.Method)
		}
		if policy.Scope != "git" {
			t.Errorf("expected git scope, got: %s", policy.Scope)
		}
		if policy.Operation != "push" {
			t.Errorf("expected push operation, got: %s", policy.Operation)
		}
	}
}

func TestEngine_Check_ConditionsNotMet(t *testing.T) {
	// Test checking an operation where conditions are not met
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	// git push requires main/master branches and origin/upstream remotes
	ctx := OperationContext{
		Branch: "feature/foo", // doesn't match conditions
		Remote: "origin",
	}
	policy, err := engine.Check("git", "push", ctx)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if policy != nil {
		t.Errorf("expected nil policy when conditions not met, got: %v", policy)
	}
}

func TestEngine_Check_WithDefaults(t *testing.T) {
	// Test that defaults are applied correctly
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	ctx := OperationContext{
		Branch: "main",
		Remote: "origin",
	}
	policy, err := engine.Check("git", "push", ctx)
	if err == nil || !IsApprovalRequired(err) {
		t.Fatal("expected approval required error")
	}

	if policy == nil {
		t.Fatal("expected policy to be returned")
	}

	// Check that defaults are applied
	if policy != nil && policy.Method != MethodBrowser {
		t.Errorf("expected default method browser, got: %s", policy.Method)
	}
	expectedDuration := 15 * time.Minute // from default config
	if policy.Expires != expectedDuration {
		t.Errorf("expected default expires 15m, got: %v", policy.Expires)
	}
}

func TestCheckAndExplain_Success(t *testing.T) {
	// Test CheckAndExplain with successful check
	ctx := OperationContext{}
	policy, err := CheckAndExplain("nonexistent", "scope", ctx)
	if err != nil {
		t.Fatalf("CheckAndExplain failed: %v", err)
	}
	if policy != nil {
		t.Errorf("expected nil policy, got: %v", policy)
	}
}

func TestCheckAndExplain_ApprovalRequired(t *testing.T) {
	// Test CheckAndExplain with approval required
	ctx := OperationContext{
		Branch: "main",
		Remote: "origin",
	}
	policy, err := CheckAndExplain("git", "push", ctx)
	if err == nil {
		t.Fatal("expected approval required error")
	}
	if !IsApprovalRequired(err) {
		t.Errorf("expected approval required error, got: %v", err)
	}
	if policy == nil {
		t.Error("expected policy to be returned")
	}
}

func TestCheckAndExplain_Error(t *testing.T) {
	// Test CheckAndExplain with engine creation error
	// This is hard to test directly, but we can verify the error wrapping
	ctx := OperationContext{
		Branch: "main",
		Remote: "origin",
	}
	_, err := CheckAndExplain("git", "push", ctx)
	if err == nil {
		t.Fatal("expected error")
	}
	if IsApprovalRequired(err) {
		// This is expected - the approval error should be preserved
		return
	}
	// If it's not an approval error, it should be wrapped
	if !errors.Is(err, ErrApprovalRequired) && !IsApprovalRequired(err) {
		// Check if it's wrapped properly
		if !strings.Contains(err.Error(), "guardian check failed") {
			t.Errorf("expected wrapped error, got: %v", err)
		}
	}
}

func TestIsApprovalRequired(t *testing.T) {
	// Test IsApprovalRequired function
	if IsApprovalRequired(nil) {
		t.Error("nil error should not be approval required")
	}

	regularErr := errors.New("regular error")
	if IsApprovalRequired(regularErr) {
		t.Error("regular error should not be approval required")
	}

	approvalErr := &ApprovalRequiredError{Scope: "test", Operation: "test"}
	if !IsApprovalRequired(approvalErr) {
		t.Error("ApprovalRequiredError should be approval required")
	}

	wrappedErr := fmt.Errorf("wrapped: %w", approvalErr)
	if !IsApprovalRequired(wrappedErr) {
		t.Error("wrapped ApprovalRequiredError should be approval required")
	}
}

func TestApprovalRequiredError(t *testing.T) {
	// Test ApprovalRequiredError methods
	err := &ApprovalRequiredError{
		Scope:     "git",
		Operation: "push",
		Policy:    &ResolvedPolicy{Scope: "git", Operation: "push"},
	}

	expectedMsg := "guardian approval required for git.push"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}

	if err.Unwrap() != ErrApprovalRequired {
		t.Errorf("expected Unwrap to return ErrApprovalRequired, got: %v", err.Unwrap())
	}
}

func TestLooksLikeURL(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		// URL patterns - should return true
		{"https URL", "https://github.com/user/repo.git", true},
		{"http URL", "http://gitlab.example.com/repo", true},
		{"ssh URL with protocol", "ssh://git@github.com/user/repo.git", true},
		{"git protocol", "git://github.com/user/repo.git", true},
		{"SSH style git@", "git@github.com:user/repo.git", true},
		{"SSH style gitlab", "git@gitlab.com:user/repo.git", true},
		{"github.com without protocol", "github.com/user/repo", true},
		{"gitlab.com in string", "my-gitlab.com/foo", true}, // Contains gitlab.com - fail-closed for security
		{"bitbucket.org", "bitbucket.org/user/repo", true},
		{"azure devops", "dev.azure.com/org/project/_git/repo", true},

		// Remote names - should return false
		{"origin", "origin", false},
		{"upstream", "upstream", false},
		{"fork", "my-fork", false},
		{"custom remote", "company-backup", false},
		{"remote with hyphen", "origin-backup", false},
		{"remote with underscore", "my_remote", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := looksLikeURL(tt.value)
			if result != tt.expected {
				t.Errorf("looksLikeURL(%q) = %v, expected %v", tt.value, result, tt.expected)
			}
		})
	}
}

func TestEngine_Check_URLRemoteRequiresApproval(t *testing.T) {
	// Security test: URL-based remotes should require approval (fail-closed)
	// This tests the fix for the auto-approval bypass bug
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	tests := []struct {
		name           string
		remote         string
		expectApproval bool
	}{
		// Named remotes matching patterns should require approval
		{"origin remote", "origin", true},
		{"upstream remote", "upstream", true},

		// Named remotes NOT matching patterns should NOT require approval
		{"fork remote", "my-fork", false},
		{"custom remote", "backup", false},

		// URL-based remotes should ALWAYS require approval (security fix)
		{"https github URL", "https://github.com/user/repo.git", true},
		{"ssh github URL", "git@github.com:user/repo.git", true},
		{"https gitlab URL", "https://gitlab.com/user/repo.git", true},
		{"ssh gitlab URL", "git@gitlab.com:user/repo.git", true},
		{"https bitbucket URL", "https://bitbucket.org/user/repo.git", true},
		{"azure devops URL", "https://dev.azure.com/org/project/_git/repo", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := OperationContext{
				Branch: "main", // Protected branch
				Remote: tt.remote,
			}
			policy, err := engine.Check("git", "push", ctx)

			if tt.expectApproval {
				if err == nil {
					t.Errorf("expected approval required for remote %q, got no error", tt.remote)
				}
				if !IsApprovalRequired(err) {
					t.Errorf("expected approval required error for remote %q, got: %v", tt.remote, err)
				}
				if policy == nil {
					t.Errorf("expected policy to be returned for remote %q", tt.remote)
				}
			} else {
				if err != nil {
					t.Errorf("expected no approval required for remote %q, got: %v", tt.remote, err)
				}
				if policy != nil {
					t.Errorf("expected nil policy for remote %q, got: %v", tt.remote, policy)
				}
			}
		})
	}
}
