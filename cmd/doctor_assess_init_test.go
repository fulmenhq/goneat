package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func TestDoctorAssessInitCreatesConfigForForcedLanguage(t *testing.T) {
	tmpDir := t.TempDir()

	output, err := runDoctorAssessInitInDir(t, tmpDir, assessInitOptions{language: "python", force: true})
	if err != nil {
		t.Fatalf("doctor assess init failed: %v\noutput: %s", err, output)
	}

	configPath := filepath.Join(tmpDir, ".goneat", "assess.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read generated config: %v", err)
	}

	if !strings.Contains(string(data), "# Repo type: python") {
		t.Fatalf("expected python template marker, got:\n%s", string(data))
	}

	// Sanity check YAML parses
	var decoded map[string]any
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("generated config is invalid YAML: %v", err)
	}

	if decoded["version"] != 1 {
		t.Fatalf("expected version=1, got %v", decoded["version"])
	}
}

func TestDoctorAssessInitAutoDetectsRepoType(t *testing.T) {
	tmpDir := t.TempDir()

	// Minimal marker for repo type detection
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module example.com/test\n"), 0o644); err != nil {
		t.Fatalf("failed to write marker: %v", err)
	}

	output, err := runDoctorAssessInitInDir(t, tmpDir, assessInitOptions{force: true})
	if err != nil {
		t.Fatalf("doctor assess init failed: %v\noutput: %s", err, output)
	}

	configPath := filepath.Join(tmpDir, ".goneat", "assess.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read generated config: %v", err)
	}

	if !strings.Contains(string(data), "# Repo type: go") {
		t.Fatalf("expected go template marker, got:\n%s", string(data))
	}

	if !strings.Contains(output, "Detected repository type: go") {
		t.Fatalf("expected detect output, got: %s", output)
	}
}

func TestDoctorAssessInitUsesUnknownTemplateWhenNoMarkers(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := runDoctorAssessInitInDir(t, tmpDir, assessInitOptions{force: true})
	if err != nil {
		t.Fatalf("doctor assess init failed: %v", err)
	}

	configPath := filepath.Join(tmpDir, ".goneat", "assess.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read generated config: %v", err)
	}

	if !strings.Contains(string(data), "# Repo type: unknown") {
		t.Fatalf("expected unknown template marker, got:\n%s", string(data))
	}
}

func TestDoctorAssessInitRequiresForceWhenConfigExists(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".goneat", "assess.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("existing"), 0o644); err != nil {
		t.Fatalf("failed to seed config: %v", err)
	}

	_, err := runDoctorAssessInitInDir(t, tmpDir, assessInitOptions{language: "go"})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected error about existing config, got: %v", err)
	}
}

type assessInitOptions struct {
	language string
	force    bool
}

func runDoctorAssessInitInDir(t *testing.T, dir string, opts assessInitOptions) (string, error) {
	t.Helper()
	resetDoctorAssessInitFlags()

	assessInitLanguage = opts.language
	assessInitForce = opts.force

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

	err = runDoctorAssessInit(cmd, nil)
	return buf.String(), err
}

func resetDoctorAssessInitFlags() {
	assessInitLanguage = ""
	assessInitForce = false
}
