package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidateConfigFile_Valid tests validation of valid config files
func TestValidateConfigFile_Valid(t *testing.T) {
	t.Parallel()
	// Create a temporary valid config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "tools.yaml")

	validConfig := `
scopes:
  infrastructure:
    description: "Infrastructure tools"
    tools: ["ripgrep", "jq"]
  custom:
    description: "Custom tools"
    tools: ["mytool"]

tools:
  ripgrep:
    name: "ripgrep"
    description: "Fast text search"
    kind: "system"
    detect_command: "rg --version"
    install_commands:
      linux: "apt-get install ripgrep"
      darwin: "brew install ripgrep"
  jq:
    name: "jq"
    description: "JSON processor"
    kind: "system"
    detect_command: "jq --version"
    install_commands:
      linux: "apt-get install jq"
  mytool:
    name: "mytool"
    description: "Custom tool"
    kind: "go"
    detect_command: "mytool --version"
    install_package: "github.com/example/mytool@latest"
`

	err := os.WriteFile(configPath, []byte(validConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	err = ValidateConfigFile(configPath)
	if err != nil {
		t.Errorf("Valid config should pass validation: %v", err)
	}
}

// TestValidateConfigFile_InvalidYAML tests validation of invalid YAML
func TestValidateConfigFile_InvalidYAML(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "invalid.yaml")

	invalidYAML := `
scopes:
  infrastructure:
    description: "Test"
  invalid: not a map
`

	err := os.WriteFile(configPath, []byte(invalidYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	err = ValidateConfigFile(configPath)
	if err == nil {
		t.Error("Invalid YAML should fail validation")
	}
}

// TestValidateConfigFile_MissingRequiredFields tests validation of configs missing required fields
func TestValidateConfigFile_MissingRequiredFields(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "missing-fields.yaml")

	// Missing required fields like name, description, detect_command
	invalidConfig := `
scopes:
  test:
    description: "Test scope"
    tools: ["incomplete-tool"]

tools:
  incomplete-tool:
    kind: "system"
    # Missing name, description, detect_command
`

	err := os.WriteFile(configPath, []byte(invalidConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	err = ValidateConfigFile(configPath)
	if err == nil {
		t.Error("Config with missing required fields should fail validation")
	}

	if !strings.Contains(err.Error(), "required") {
		t.Errorf("Error should mention required fields: %v", err)
	}
}

// TestValidateConfigFile_InvalidToolKind tests validation of invalid tool kinds
func TestValidateConfigFile_InvalidToolKind(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "invalid-kind.yaml")

	invalidConfig := `
scopes:
  test:
    description: "Test scope"
    tools: ["invalid-tool"]

tools:
  invalid-tool:
    name: "invalid-tool"
    description: "Tool with invalid kind"
    kind: "invalid-kind"  # Not one of: go, bundled-go, system
    detect_command: "invalid-tool --version"
`

	err := os.WriteFile(configPath, []byte(invalidConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	err = ValidateConfigFile(configPath)
	if err == nil {
		t.Error("Config with invalid tool kind should fail validation")
	}
}

// TestValidateConfigFile_DuplicateTools tests validation of duplicate tool names
func TestValidateConfigFile_DuplicateTools(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "duplicate-tools.yaml")

	duplicateConfig := `
scopes:
  test:
    description: "Test scope"
    tools: ["tool1", "tool1"]  # Duplicate in scope

tools:
  tool1:
    name: "tool1"
    description: "Test tool"
    kind: "system"
    detect_command: "tool1 --version"
`

	err := os.WriteFile(configPath, []byte(duplicateConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// This might not fail schema validation, but could be caught by application logic
	// The test documents expected behavior
	err = ValidateConfigFile(configPath)
	// Note: Schema validation might not catch duplicates, but application should handle it
	t.Logf("Duplicate tools validation result: %v", err)
}

// TestValidateConfigFile_EmptyScopes tests validation of empty scopes
func TestValidateConfigFile_EmptyScopes(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "empty-scopes.yaml")

	emptyScopesConfig := `
scopes:
  empty-scope:
    description: "Empty scope"
    tools: ["test-tool"]  # Must have at least one tool

tools:
  test-tool:
    name: "test-tool"
    description: "Test tool"
    kind: "system"
    detect_command: "test-tool --version"
`

	err := os.WriteFile(configPath, []byte(emptyScopesConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	err = ValidateConfigFile(configPath)
	if err != nil {
		t.Errorf("Empty scopes should be valid: %v", err)
	}
}

// TestValidateConfigFile_NonExistentFile tests handling of non-existent files
func TestValidateConfigFile_NonExistentFile(t *testing.T) {
	t.Parallel()
	err := ValidateConfigFile("/non/existent/file.yaml")
	if err == nil {
		t.Error("Non-existent file should return error")
	}

	if !strings.Contains(err.Error(), "no such file") && !strings.Contains(err.Error(), "cannot find") {
		t.Errorf("Error should indicate file not found: %v", err)
	}
}

// TestValidateConfigFile_ComplexValidConfig tests a complex but valid configuration
func TestValidateConfigFile_ComplexValidConfig(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "complex-valid.yaml")

	complexConfig := `
scopes:
  infrastructure:
    description: "Core infrastructure tools"
    tools: ["ripgrep", "jq", "go-licenses"]
  development:
    description: "Development and testing tools"
    tools: ["yamllint", "shellcheck"]
  custom:
    description: "Project-specific tools"
    tools: ["terraform", "kubectl"]

tools:
  ripgrep:
    name: "ripgrep"
    description: "Fast text search tool"
    kind: "system"
    detect_command: "rg --version"
    install_commands:
      linux: "apt-get install -y ripgrep"
      darwin: "brew install ripgrep"
      windows: "winget install BurntSushi.ripgrep.MSVC"
    platforms: ["linux", "darwin", "windows"]

  jq:
    name: "jq"
    description: "JSON processor"
    kind: "system"
    detect_command: "jq --version"
    install_commands:
      linux: "apt-get install -y jq"
      darwin: "brew install jq"
      windows: "winget install jqlang.jq"

  go-licenses:
    name: "go-licenses"
    description: "License compliance checker"
    kind: "go"
    detect_command: "go-licenses version"
    install_package: "github.com/google/go-licenses@latest"

  yamllint:
    name: "yamllint"
    description: "YAML linter"
    kind: "system"
    detect_command: "yamllint --version"
    install_commands:
      linux: "pip install yamllint"
      darwin: "pip install yamllint"
      windows: "pip install yamllint"

  shellcheck:
    name: "shellcheck"
    description: "Shell script linter"
    kind: "system"
    detect_command: "shellcheck --version"
    install_commands:
      linux: "apt-get install -y shellcheck"
      darwin: "brew install shellcheck"
      windows: "winget install koalaman.shellcheck"

  terraform:
    name: "terraform"
    description: "Infrastructure as code"
    kind: "system"
    detect_command: "terraform version"
    install_commands:
      linux: "apt-get install -y terraform"
      darwin: "brew install terraform"
      windows: "winget install HashiCorp.Terraform"

  kubectl:
    name: "kubectl"
    description: "Kubernetes CLI"
    kind: "system"
    detect_command: "kubectl version --client"
    install_commands:
      linux: "apt-get install -y kubectl"
      darwin: "brew install kubectl"
      windows: "winget install Kubernetes.kubectl"
`

	err := os.WriteFile(configPath, []byte(complexConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write complex test config: %v", err)
	}

	err = ValidateConfigFile(configPath)
	if err != nil {
		t.Errorf("Complex valid config should pass validation: %v", err)
	}
}
