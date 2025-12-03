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
