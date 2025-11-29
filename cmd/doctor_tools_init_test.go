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

	defaults, err := doctor.LoadToolsDefaultsConfig()
	if err != nil {
		t.Fatalf("failed to load defaults: %v", err)
	}

	scopeTools, err := defaults.GetToolsForScope("foundation")
	if err != nil {
		t.Fatalf("failed to get foundation scope tools: %v", err)
	}

	expectedTools := doctor.GetMinimalToolsForLanguage(scopeTools, "go")
	if len(expectedTools) == 0 {
		t.Fatal("expected minimal tool list for go to be non-empty")
	}
	expectedSet := make(map[string]struct{}, len(expectedTools))
	for _, tool := range expectedTools {
		expectedSet[tool.Name] = struct{}{}
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
	if len(config.Tools) != len(expectedSet) {
		t.Fatalf("expected %d tools, got %d", len(expectedSet), len(config.Tools))
	}

	for name := range config.Tools {
		if _, exists := expectedSet[name]; !exists {
			t.Fatalf("unexpected tool %s in minimal config", name)
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

func TestDoctorToolsInitInvalidScope(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := runDoctorToolsInitInDir(t, tmpDir, toolsInitOptions{
		language: "go",
		scope:    "unknown",
		force:    true,
	})
	if err == nil || !strings.Contains(err.Error(), "failed to get tools for scope") {
		t.Fatalf("expected scope error, got: %v", err)
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
