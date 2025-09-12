package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}
	if config == nil {
		t.Fatal("LoadConfig() returned nil config")
	}

	// Test default values
	if !config.Format.Go.Simplify {
		t.Error("Expected default Go Simplify to be true")
	}
	if config.Format.YAML.Indent != 2 {
		t.Errorf("Expected default YAML indent to be 2, got %d", config.Format.YAML.Indent)
	}
	if config.Format.JSON.Indent != "  " {
		t.Errorf("Expected default JSON indent to be '  ', got %q", config.Format.JSON.Indent)
	}
}

func TestLoadProjectConfig(t *testing.T) {
	config, err := LoadProjectConfig()
	if err != nil {
		t.Fatalf("LoadProjectConfig() failed: %v", err)
	}
	if config == nil {
		t.Fatal("LoadProjectConfig() returned nil config")
	}

	// Should have same defaults as LoadConfig
	if !config.Format.Go.Simplify {
		t.Error("Expected default Go Simplify to be true")
	}
}

func TestConfigGetterMethods(t *testing.T) {
	config := &Config{
		Format: FormatConfig{
			Go: GoFormatConfig{Simplify: true},
			YAML: YAMLFormatConfig{
				Indent:          4,
				LineLength:      100,
				QuoteStyle:      "single",
				TrailingNewline: false,
			},
			JSON: JSONFormatConfig{
				Indent:          "\t",
				Compact:         true,
				SortKeys:        true,
				TrailingNewline: false,
			},
			Markdown: MarkdownFormatConfig{
				LineLength:     120,
				TrailingSpaces: true,
				ReferenceStyle: "full",
				CodeBlockStyle: "indented",
			},
		},
	}

	// Test getter methods
	goConfig := config.GetGoConfig()
	if !goConfig.Simplify {
		t.Error("GetGoConfig() should return correct Go config")
	}

	yamlConfig := config.GetYAMLConfig()
	if yamlConfig.Indent != 4 || yamlConfig.LineLength != 100 {
		t.Error("GetYAMLConfig() should return correct YAML config")
	}

	jsonConfig := config.GetJSONConfig()
	if jsonConfig.Indent != "\t" || !jsonConfig.Compact {
		t.Error("GetJSONConfig() should return correct JSON config")
	}

	markdownConfig := config.GetMarkdownConfig()
	if markdownConfig.LineLength != 120 || !markdownConfig.TrailingSpaces {
		t.Error("GetMarkdownConfig() should return correct Markdown config")
	}
}

func TestGetGoneatHome(t *testing.T) {
	home, err := GetGoneatHome()
	if err != nil {
		t.Fatalf("GetGoneatHome() failed: %v", err)
	}
	if home == "" {
		t.Error("GetGoneatHome() returned empty string")
	}

	// Should end with .goneat
	if filepath.Base(home) != ".goneat" {
		t.Errorf("Expected home to end with .goneat, got %s", home)
	}
}

func TestGetGoneatHomeWithEnvVar(t *testing.T) {
	// Set custom home
	customHome := "/tmp/test-goneat-home"
	oldEnv := os.Getenv("GONEAT_HOME")
	if err := os.Setenv("GONEAT_HOME", customHome); err != nil {
		t.Fatalf("Failed to set GONEAT_HOME: %v", err)
	}
	defer func() {
		if oldEnv == "" {
			if err := os.Unsetenv("GONEAT_HOME"); err != nil {
				t.Errorf("Failed to unset GONEAT_HOME: %v", err)
			}
		} else {
			if err := os.Setenv("GONEAT_HOME", oldEnv); err != nil {
				t.Errorf("Failed to restore GONEAT_HOME: %v", err)
			}
		}
	}()

	home, err := GetGoneatHome()
	if err != nil {
		t.Fatalf("GetGoneatHome() with env var failed: %v", err)
	}
	if home != customHome {
		t.Errorf("Expected %s, got %s", customHome, home)
	}
}

func TestEnsureGoneatHome(t *testing.T) {
	home, err := EnsureGoneatHome()
	if err != nil {
		t.Fatalf("EnsureGoneatHome() failed: %v", err)
	}
	if home == "" {
		t.Error("EnsureGoneatHome() returned empty string")
	}

	// Check that directory exists
	if _, err := os.Stat(home); os.IsNotExist(err) {
		t.Errorf("EnsureGoneatHome() did not create directory: %s", home)
	}
}

func TestDirectoryFunctions(t *testing.T) {
	dirs := []struct {
		name string
		fn   func() (string, error)
	}{
		{"ScratchpadDir", GetScratchpadDir},
		{"CacheDir", GetCacheDir},
		{"LogDir", GetLogDir},
		{"ConfigDir", GetConfigDir},
		{"WorkDir", GetWorkDir},
	}

	for _, dir := range dirs {
		t.Run(dir.name, func(t *testing.T) {
			path, err := dir.fn()
			if err != nil {
				t.Fatalf("%s() failed: %v", dir.name, err)
			}
			if path == "" {
				t.Errorf("%s() returned empty string", dir.name)
			}

			// Check that directory exists
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Errorf("%s() did not create directory: %s", dir.name, path)
			}
		})
	}
}

func TestParseDurationDefault(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"5m", 5 * time.Minute},
		{"1h", time.Hour},
		{"30s", 30 * time.Second},
		{"invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseDurationDefault(tt.input)
			if result != tt.expected {
				t.Errorf("parseDurationDefault(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSecurityConfigDefaults(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	security := config.GetSecurityConfig()

	// Test default values
	if security.FailOn != "high" {
		t.Errorf("Expected default FailOn to be 'high', got %q", security.FailOn)
	}
	if !security.Enable.Code {
		t.Error("Expected default Code security to be enabled")
	}
	if !security.Enable.Vuln {
		t.Error("Expected default Vuln security to be enabled")
	}
	if security.Enable.Secrets {
		t.Error("Expected default Secrets security to be disabled")
	}
}

func TestSchemaConfigDefaults(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	schema := config.GetSchemaConfig()

	// Test default values
	if !schema.Enable {
		t.Error("Expected default schema enable to be true")
	}
	if schema.AutoDetect {
		t.Error("Expected default schema auto-detect to be false")
	}
	if len(schema.Patterns) == 0 {
		t.Error("Expected default schema patterns to be set")
	}
}

func TestLoadProjectConfigWithValidYAML(t *testing.T) {
	tempDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Errorf("Failed to restore directory: %v", err)
		}
	}()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Create valid project config
	configContent := `format:
  go:
    simplify: false
  yaml:
    indent: 4
security:
  fail_on: medium
  enable:
    secrets: true`

	if err := os.WriteFile(".goneat.yaml", []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	config, err := LoadProjectConfig()
	if err != nil {
		t.Fatalf("LoadProjectConfig() failed: %v", err)
	}

	// Check that project config overrides are applied
	if config.Format.Go.Simplify {
		t.Error("Expected Go Simplify to be overridden to false")
	}
	if config.Format.YAML.Indent != 4 {
		t.Errorf("Expected YAML indent to be 4, got %d", config.Format.YAML.Indent)
	}
	if config.Security.FailOn != "medium" {
		t.Errorf("Expected FailOn to be 'medium', got %q", config.Security.FailOn)
	}
	if !config.Security.Enable.Secrets {
		t.Error("Expected Secrets to be enabled")
	}
}

func TestLoadProjectConfigWithValidJSON(t *testing.T) {
	tempDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Errorf("Failed to restore directory: %v", err)
		}
	}()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Create simple valid JSON project config - just override security settings to avoid schema validation issues
	configContent := `{
  "security": {
    "fail_on": "medium"
  }
}`

	if err := os.WriteFile(".goneat.json", []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	config, err := LoadProjectConfig()
	if err != nil {
		t.Fatalf("LoadProjectConfig() failed: %v", err)
	}

	// Check that project config overrides are applied
	if config.Security.FailOn != "medium" {
		t.Errorf("Expected FailOn to be 'medium', got %q", config.Security.FailOn)
	}
}

func TestLoadProjectConfigNoProjectFile(t *testing.T) {
	tempDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Errorf("Failed to restore directory: %v", err)
		}
	}()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Load config with no project file
	config, err := LoadProjectConfig()
	if err != nil {
		t.Fatalf("LoadProjectConfig() failed: %v", err)
	}

	// Should return global config defaults
	if !config.Format.Go.Simplify {
		t.Error("Expected default Go Simplify to be true")
	}
}

func TestLoadProjectConfigInvalidYAML(t *testing.T) {
	tempDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Errorf("Failed to restore directory: %v", err)
		}
	}()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Create invalid YAML
	configContent := `format:
  go:
    simplify: [invalid yaml structure`

	if err := os.WriteFile(".goneat.yaml", []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	_, err = LoadProjectConfig()
	if err == nil {
		t.Error("Expected LoadProjectConfig() to fail with invalid YAML")
	}
	if !strings.Contains(err.Error(), "failed to parse") {
		t.Errorf("Expected parse error, got: %v", err)
	}
}

// TestLoadProjectConfigUnsupportedFormat removed due to viper handling various formats flexibly
