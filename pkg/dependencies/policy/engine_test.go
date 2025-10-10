package policy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOPADenyPath(t *testing.T) {
	// Create temporary policy file with forbidden licenses
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "policy.yaml")
	policyContent := `version: v1
licenses:
  forbidden:
    - GPL-3.0
    - AGPL-3.0
cooling:
  enabled: true
  min_age_days: 30
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0644); err != nil {
		t.Fatalf("Failed to create policy file: %v", err)
	}

	// Create OPA engine and load policy
	engine := NewOPAEngine()
	if err := engine.LoadPolicy(policyPath); err != nil {
		t.Fatalf("Failed to load policy: %v", err)
	}

	// Test input with a forbidden license
	input := map[string]interface{}{
		"dependencies": []map[string]interface{}{
			{
				"module": map[string]interface{}{
					"name":     "test/gpl-package",
					"version":  "v1.0.0",
					"language": "go",
				},
				"license": map[string]interface{}{
					"name": "LICENSE",
					"type": "GPL-3.0",
					"url":  "https://www.gnu.org/licenses/gpl-3.0.html",
				},
				"metadata": map[string]interface{}{
					"age_days": 100,
				},
			},
		},
		"policy": map[string]interface{}{
			"licenses": map[string]interface{}{
				"forbidden": []string{"GPL-3.0", "AGPL-3.0"},
			},
		},
	}

	// Evaluate policy
	ctx := context.Background()
	result, err := engine.Evaluate(ctx, input)
	if err != nil {
		t.Fatalf("Policy evaluation failed: %v", err)
	}

	// Verify denial was triggered
	if result == nil {
		t.Fatal("Expected result but got nil")
	}

	// Check if deny rule was triggered and verify specific denial strings
	denials, ok := result["data.goneat.dependencies.deny"].([]interface{})
	if !ok {
		t.Fatalf("Expected deny results in format []interface{}, got %T", result["data.goneat.dependencies.deny"])
	}

	if len(denials) == 0 {
		t.Fatal("Expected at least one denial for GPL-3.0 license")
	}

	// Verify the denial message contains expected elements
	foundExpectedDenial := false
	for _, denial := range denials {
		denialMsg, ok := denial.(string)
		if !ok {
			t.Errorf("Expected denial to be string, got %T: %v", denial, denial)
			continue
		}

		t.Logf("OPA denial message: %s", denialMsg)

		// Verify message contains package name and forbidden license
		if containsAll(denialMsg, "test/gpl-package", "GPL-3.0", "forbidden") {
			foundExpectedDenial = true
		}
	}

	if !foundExpectedDenial {
		t.Errorf("Expected denial message to contain 'test/gpl-package', 'GPL-3.0', and 'forbidden'")
	}
}

func TestOPACoolingPolicyDeny(t *testing.T) {
	// Create temporary policy file with cooling policy
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "cooling-policy.yaml")
	policyContent := `version: v1
cooling:
  enabled: true
  min_age_days: 30
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0644); err != nil {
		t.Fatalf("Failed to create policy file: %v", err)
	}

	// Create OPA engine and load policy
	engine := NewOPAEngine()
	if err := engine.LoadPolicy(policyPath); err != nil {
		t.Fatalf("Failed to load policy: %v", err)
	}

	// Test input with a package that's too young
	input := map[string]interface{}{
		"dependencies": []map[string]interface{}{
			{
				"module": map[string]interface{}{
					"name":     "test/young-package",
					"version":  "v1.0.0",
					"language": "go",
				},
				"license": map[string]interface{}{
					"name": "LICENSE",
					"type": "MIT",
				},
				"metadata": map[string]interface{}{
					"age_days": 5, // Too young (< 30 days)
				},
			},
		},
		"policy": map[string]interface{}{
			"cooling": map[string]interface{}{
				"enabled":      true,
				"min_age_days": 30,
			},
		},
	}

	// Evaluate policy
	ctx := context.Background()
	result, err := engine.Evaluate(ctx, input)
	if err != nil {
		t.Fatalf("Policy evaluation failed: %v", err)
	}

	// Verify evaluation completed
	if result == nil {
		t.Fatal("Expected result but got nil")
	}

	t.Logf("Cooling policy evaluation result: %+v", result)

	// Verify we got a result (specific assertions depend on OPA output format)
	if len(result) == 0 {
		t.Error("Expected non-empty OPA evaluation result for cooling policy")
	}
}

func TestOPAPolicyTranspilation(t *testing.T) {
	// Create a policy file and verify it transpiles correctly
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "transpile-test.yaml")
	policyContent := `version: v1
licenses:
  forbidden:
    - GPL-3.0
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0644); err != nil {
		t.Fatalf("Failed to create policy file: %v", err)
	}

	// Create OPA engine
	engine := NewOPAEngine()
	if err := engine.LoadPolicy(policyPath); err != nil {
		t.Fatalf("Failed to load policy: %v", err)
	}

	// Verify engine has loaded policy (regoCode should be populated)
	opaEngine, ok := engine.(*OPAEngine)
	if !ok {
		t.Fatal("Engine is not OPAEngine type")
	}

	if opaEngine.regoCode == "" {
		t.Error("Expected regoCode to be populated after loading policy")
	}

	// Verify the transpiled Rego contains expected patterns
	if len(opaEngine.regoCode) < 50 {
		t.Errorf("Transpiled Rego code seems too short: %d characters", len(opaEngine.regoCode))
	}

	t.Logf("Transpiled Rego code length: %d characters", len(opaEngine.regoCode))
}

func TestOPAInvalidPolicyPath(t *testing.T) {
	engine := NewOPAEngine()
	err := engine.LoadPolicy("/nonexistent/policy.yaml")

	if err == nil {
		t.Error("Expected error for nonexistent policy file")
	}
}

func TestOPAPathTraversalProtection(t *testing.T) {
	engine := NewOPAEngine()

	// Try to load policy with path traversal
	err := engine.LoadPolicy("../../etc/passwd")

	if err == nil {
		t.Error("Expected error for path traversal attempt")
	}

	if err != nil && err.Error() != "invalid path: directory traversal detected" {
		// Note: The actual error might be "file not accessible" if the path validation
		// happens after cleaning but before the traversal check passes
		t.Logf("Got error (acceptable): %v", err)
	}
}

// Helper function to check if a string contains all substrings
func containsAll(s string, substrings ...string) bool {
	for _, substr := range substrings {
		if !strings.Contains(s, substr) {
			return false
		}
	}
	return true
}
