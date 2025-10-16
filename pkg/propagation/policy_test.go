package propagation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewPolicyLoader(t *testing.T) {
	loader := NewPolicyLoader()
	if loader == nil {
		t.Fatal("NewPolicyLoader() returned nil")
	}
}

func TestPolicyLoader_LoadPolicy_Defaults(t *testing.T) {
	loader := NewPolicyLoader()

	// Test loading when no policy file exists
	policy, err := loader.LoadPolicy("")
	if err != nil {
		t.Fatalf("LoadPolicy() failed: %v", err)
	}

	// Check defaults
	if policy.Version.Scheme != "semver" {
		t.Errorf("expected default scheme 'semver', got %s", policy.Version.Scheme)
	}
	if !policy.Version.AllowExtended {
		t.Error("expected AllowExtended to be true by default")
	}
	if len(policy.Propagation.Defaults.Include) != 2 {
		t.Errorf("expected 2 default includes, got %d", len(policy.Propagation.Defaults.Include))
	}
	if policy.Propagation.Defaults.Include[0] != "package.json" {
		t.Errorf("expected first include to be 'package.json', got %s", policy.Propagation.Defaults.Include[0])
	}
}

func TestPolicyLoader_LoadPolicy_FromFile(t *testing.T) {
	loader := NewPolicyLoader()

	// Create a temporary policy file
	tempDir := t.TempDir()
	policyPath := filepath.Join(tempDir, "test-policy.yaml")

	policyContent := `# Test policy
version:
  scheme: calver
  allow_extended: false

propagation:
  defaults:
    include: ["test.json"]
    exclude: ["test/**"]

  targets:
    test:
      include: ["custom.json"]
`

	err := os.WriteFile(policyPath, []byte(policyContent), 0644)
	if err != nil {
		t.Fatalf("failed to create test policy file: %v", err)
	}

	// Load the policy
	policy, err := loader.LoadPolicy(policyPath)
	if err != nil {
		t.Fatalf("LoadPolicy() failed: %v", err)
	}

	// Verify loaded values
	if policy.Version.Scheme != "calver" {
		t.Errorf("expected scheme 'calver', got %s", policy.Version.Scheme)
	}
	if policy.Version.AllowExtended {
		t.Error("expected AllowExtended to be false")
	}
	if len(policy.Propagation.Defaults.Include) != 1 || policy.Propagation.Defaults.Include[0] != "test.json" {
		t.Errorf("expected include ['test.json'], got %v", policy.Propagation.Defaults.Include)
	}
	if len(policy.Propagation.Targets) != 1 {
		t.Errorf("expected 1 target, got %d", len(policy.Propagation.Targets))
	}
}

func TestPolicyLoader_ValidatePolicy(t *testing.T) {
	loader := NewPolicyLoader()

	tests := []struct {
		name    string
		policy  *VersionPolicy
		wantErr bool
	}{
		{
			name: "valid semver policy",
			policy: &VersionPolicy{
				Version: VersionConfig{Scheme: "semver"},
			},
			wantErr: false,
		},
		{
			name: "valid calver policy",
			policy: &VersionPolicy{
				Version: VersionConfig{Scheme: "calver"},
			},
			wantErr: false,
		},
		{
			name: "invalid scheme",
			policy: &VersionPolicy{
				Version: VersionConfig{Scheme: "invalid"},
			},
			wantErr: true,
		},
		{
			name: "invalid target mode",
			policy: &VersionPolicy{
				Propagation: PropagationConfig{
					Targets: map[string]PropagationTarget{
						"test": {Mode: "invalid"},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := loader.validatePolicy(tt.policy)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePolicy() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPolicyLoader_GeneratePolicyFile(t *testing.T) {
	loader := NewPolicyLoader()

	tempDir := t.TempDir()
	policyPath := filepath.Join(tempDir, ".goneat", "version-policy.yaml")

	err := loader.GeneratePolicyFile(policyPath)
	if err != nil {
		t.Fatalf("GeneratePolicyFile() failed: %v", err)
	}

	// Check that file was created
	if _, err := os.Stat(policyPath); os.IsNotExist(err) {
		t.Fatal("policy file was not created")
	}

	// Check content
	content, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}

	contentStr := string(content)
	if len(contentStr) == 0 {
		t.Error("generated file is empty")
	}

	// Check for expected content
	expectedStrings := []string{
		"Version SSOT Propagation Policy",
		"scheme: semver",
		"allow_extended: true",
		"include: [\"package.json\", \"pyproject.toml\"]",
		"exclude: [\"**/node_modules/**\", \"docs/**\"]",
	}

	for _, expected := range expectedStrings {
		if !contains(contentStr, expected) {
			t.Errorf("generated file does not contain expected string: %s", expected)
		}
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsAt(s, substr)))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
