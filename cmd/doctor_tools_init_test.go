package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulmenhq/goneat/internal/doctor"
	pkgtools "github.com/fulmenhq/goneat/pkg/tools"
	"github.com/spf13/cobra"
)

func TestDoctorToolsInitCreatesFoundationConfig(t *testing.T) {
	tmpDir := t.TempDir()

	output, err := runDoctorToolsInitInDir(t, tmpDir, toolsInitOptions{
		language: "go",
		scope:    "foundation",
		force:    true,
	})
	if err != nil {
		t.Fatalf("doctor tools init failed: %v\noutput: %s", err, output)
	}

	config := loadGeneratedToolsConfig(t, tmpDir)
	if len(config.Tools) == 0 {
		t.Fatalf("expected generated config to contain tools, output: %s", output)
	}

	scope, ok := config.Scopes["foundation"]
	if !ok {
		t.Fatalf("expected foundation scope in generated config, output: %s", output)
	}
	if len(scope.Tools) == 0 {
		t.Fatalf("expected foundation scope to contain tools, output: %s", output)
	}

	if !strings.Contains(output, "Successfully created .goneat/tools.yaml") {
		t.Fatalf("expected success message in output, got: %s", output)
	}
}

func TestDoctorToolsInitMinimalMatchesDefaults(t *testing.T) {
	tmpDir := t.TempDir()

	// In minimal mode, we generate all scopes but filter each to minimal tools
	// This test verifies that the generated config matches what
	// ConvertToToolsConfigWithAllScopes produces with minimal=true
	defaults, err := doctor.LoadToolsDefaultsConfig()
	if err != nil {
		t.Fatalf("failed to load defaults: %v", err)
	}

	// Calculate expected tools across ALL scopes in minimal mode
	expectedConfig := doctor.ConvertToToolsConfigWithAllScopes(defaults, "go", true)
	if len(expectedConfig.Tools) == 0 {
		t.Fatal("expected minimal tool list for go to be non-empty")
	}

	if _, err := runDoctorToolsInitInDir(t, tmpDir, toolsInitOptions{
		language: "go",
		scope:    "foundation",
		minimal:  true,
		force:    true,
	}); err != nil {
		t.Fatalf("doctor tools init minimal failed: %v", err)
	}

	config := loadGeneratedToolsConfig(t, tmpDir)
	if len(config.Tools) != len(expectedConfig.Tools) {
		t.Fatalf("expected %d tools, got %d", len(expectedConfig.Tools), len(config.Tools))
	}

	// Verify all expected tools are present
	for name := range expectedConfig.Tools {
		if _, exists := config.Tools[name]; !exists {
			t.Fatalf("expected tool %s not in generated config", name)
		}
	}
}

func TestDoctorToolsInitRequiresForceWhenConfigExists(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".goneat", "tools.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("existing"), 0o644); err != nil {
		t.Fatalf("failed to seed config: %v", err)
	}

	_, err := runDoctorToolsInitInDir(t, tmpDir, toolsInitOptions{
		language: "go",
	})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected error about existing config, got: %v", err)
	}
}

func TestDoctorToolsInitGeneratesAllStandardScopes(t *testing.T) {
	// Since v0.3.11, init always generates all 5 standard scopes (foundation, security, format, sbom, all)
	// regardless of which scope is specified via --scope flag
	tmpDir := t.TempDir()

	_, err := runDoctorToolsInitInDir(t, tmpDir, toolsInitOptions{
		language: "go",
		scope:    "foundation", // scope flag is now ignored
		force:    true,
	})
	if err != nil {
		t.Fatalf("doctor tools init failed: %v", err)
	}

	config := loadGeneratedToolsConfig(t, tmpDir)

	// Verify all 5 standard scopes are present
	expectedScopes := []string{"foundation", "security", "format", "sbom", "all"}
	for _, scope := range expectedScopes {
		if _, exists := config.Scopes[scope]; !exists {
			t.Fatalf("expected scope %s in generated config", scope)
		}
	}

	if len(config.Scopes) != 5 {
		t.Fatalf("expected 5 scopes, got %d", len(config.Scopes))
	}
}

func TestDoctorToolsInitGeneratesYamlfmtCompatibleFile(t *testing.T) {
	// Regression test for: generated .goneat/tools.yaml must be yamlfmt-compatible
	// Bug: yaml.Marshal with default settings produced non-yamlfmt-compatible output
	// Fix: Use yaml.NewEncoder with SetIndent(2) to match .yamlfmt configuration
	tmpDir := t.TempDir()

	_, err := runDoctorToolsInitInDir(t, tmpDir, toolsInitOptions{
		language: "go",
		force:    true,
	})
	if err != nil {
		t.Fatalf("doctor tools init failed: %v", err)
	}

	// Verify the generated file is valid YAML and can be parsed
	config := loadGeneratedToolsConfig(t, tmpDir)
	if len(config.Tools) == 0 {
		t.Fatal("expected generated config to contain tools")
	}

	// Verify indentation is correct (2 spaces as per .yamlfmt)
	configPath := filepath.Join(tmpDir, ".goneat", "tools.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read generated config: %v", err)
	}

	content := string(data)

	// Check that indentation uses 2 spaces (not tabs or 4 spaces)
	// Look for a line that should be indented (e.g., inside scopes)
	lines := strings.Split(content, "\n")
	foundProperIndent := false
	for _, line := range lines {
		// Look for lines that start with 2 spaces (first level indent)
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "  #") {
			foundProperIndent = true
			// Verify it's using spaces, not tabs
			if strings.HasPrefix(line, "\t") {
				t.Fatal("generated file uses tabs instead of spaces for indentation")
			}
			break
		}
	}

	if !foundProperIndent {
		t.Fatal("could not verify proper 2-space indentation in generated file")
	}
}

type toolsInitOptions struct {
	language string
	scope    string
	minimal  bool
	force    bool
}

func runDoctorToolsInitInDir(t *testing.T, dir string, opts toolsInitOptions) (string, error) {
	t.Helper()
	resetToolsInitFlags()

	if opts.language != "" {
		toolsInitLanguage = opts.language
	}
	if opts.scope != "" {
		toolsInitScope = opts.scope
	}
	toolsInitMinimal = opts.minimal
	toolsInitForce = opts.force

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir to %s: %v", dir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	err = runToolsInit(cmd, nil)
	return buf.String(), err
}

func loadGeneratedToolsConfig(t *testing.T, dir string) *pkgtools.Config {
	t.Helper()
	configPath := filepath.Join(dir, ".goneat", "tools.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read generated config: %v", err)
	}

	config, err := pkgtools.ParseConfig(data)
	if err != nil {
		t.Fatalf("failed to parse generated config: %v", err)
	}

	return config
}

func resetToolsInitFlags() {
	toolsInitMinimal = false
	toolsInitLanguage = ""
	toolsInitScope = "foundation"
	toolsInitForce = false
}

func TestDetectYamlfmtIndent(t *testing.T) {
	tests := []struct {
		name           string
		yamlfmtContent string
		expectedIndent int
	}{
		{
			name: "detects indent from valid yamlfmt",
			yamlfmtContent: `formatter:
  type: basic
  indent: 4
`,
			expectedIndent: 4,
		},
		{
			name: "uses default for missing indent",
			yamlfmtContent: `formatter:
  type: basic
`,
			expectedIndent: 2,
		},
		{
			name: "uses default for zero indent",
			yamlfmtContent: `formatter:
  indent: 0
`,
			expectedIndent: 2,
		},
		{
			name: "uses default for negative indent",
			yamlfmtContent: `formatter:
  indent: -1
`,
			expectedIndent: 2,
		},
		{
			name:           "uses default for invalid yaml",
			yamlfmtContent: `not: valid: yaml:`,
			expectedIndent: 2,
		},
		// Diabolical data protection tests - malicious/corrupt .yamlfmt values
		{
			name: "rejects absurdly large indent (100)",
			yamlfmtContent: `formatter:
  indent: 100
`,
			expectedIndent: 2,
		},
		{
			name: "rejects max int overflow attempt",
			yamlfmtContent: `formatter:
  indent: 2147483647
`,
			expectedIndent: 2,
		},
		{
			name: "accepts max valid indent (8)",
			yamlfmtContent: `formatter:
  indent: 8
`,
			expectedIndent: 8,
		},
		{
			name: "rejects indent above max (9)",
			yamlfmtContent: `formatter:
  indent: 9
`,
			expectedIndent: 2,
		},
		{
			name: "accepts min valid indent (1)",
			yamlfmtContent: `formatter:
  indent: 1
`,
			expectedIndent: 1,
		},
		{
			name: "rejects extremely negative indent",
			yamlfmtContent: `formatter:
  indent: -999999
`,
			expectedIndent: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Write .yamlfmt file
			yamlfmtPath := filepath.Join(tmpDir, ".yamlfmt")
			if err := os.WriteFile(yamlfmtPath, []byte(tt.yamlfmtContent), 0o644); err != nil {
				t.Fatalf("failed to write .yamlfmt: %v", err)
			}

			// Change to temp dir and test
			cwd, err := os.Getwd()
			if err != nil {
				t.Fatalf("failed to get cwd: %v", err)
			}
			if err := os.Chdir(tmpDir); err != nil {
				t.Fatalf("failed to chdir: %v", err)
			}
			t.Cleanup(func() { _ = os.Chdir(cwd) })

			indent := detectYamlfmtIndent()
			if indent != tt.expectedIndent {
				t.Errorf("expected indent %d, got %d", tt.expectedIndent, indent)
			}
		})
	}
}

func TestDetectYamlfmtIndentNoFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Change to temp dir (no .yamlfmt file)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	indent := detectYamlfmtIndent()
	if indent != 2 {
		t.Errorf("expected default indent 2, got %d", indent)
	}
}

func TestDetectYamlfmtIndentWalksUpTree(t *testing.T) {
	// Create nested directory structure: tmpDir/.yamlfmt and tmpDir/subdir/
	// Run from subdir, should find .yamlfmt in parent
	tmpDir := t.TempDir()

	yamlfmtPath := filepath.Join(tmpDir, ".yamlfmt")
	yamlfmtContent := `formatter:
  indent: 3
`
	if err := os.WriteFile(yamlfmtPath, []byte(yamlfmtContent), 0o644); err != nil {
		t.Fatalf("failed to write .yamlfmt: %v", err)
	}

	subdir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	// Change to subdir
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	if err := os.Chdir(subdir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	indent := detectYamlfmtIndent()
	if indent != 3 {
		t.Errorf("expected indent 3 from parent dir, got %d", indent)
	}
}

func TestDoctorToolsInitUsesYamlfmtIndent(t *testing.T) {
	// Test that generated tools.yaml uses indent from .yamlfmt
	tmpDir := t.TempDir()

	// Create .yamlfmt with indent 4
	yamlfmtPath := filepath.Join(tmpDir, ".yamlfmt")
	yamlfmtContent := `formatter:
  type: basic
  indent: 4
`
	if err := os.WriteFile(yamlfmtPath, []byte(yamlfmtContent), 0o644); err != nil {
		t.Fatalf("failed to write .yamlfmt: %v", err)
	}

	_, err := runDoctorToolsInitInDir(t, tmpDir, toolsInitOptions{
		language: "go",
		force:    true,
	})
	if err != nil {
		t.Fatalf("doctor tools init failed: %v", err)
	}

	// Read generated file and check for 4-space indentation
	configPath := filepath.Join(tmpDir, ".goneat", "tools.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read generated config: %v", err)
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	// Find a line with 4-space indent (first level under a key)
	found4SpaceIndent := false
	for _, line := range lines {
		// Look for lines starting with exactly 4 spaces
		if strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "        ") {
			found4SpaceIndent = true
			break
		}
	}

	if !found4SpaceIndent {
		t.Errorf("expected 4-space indentation from .yamlfmt config, file content:\n%s", content[:min(500, len(content))])
	}
}
