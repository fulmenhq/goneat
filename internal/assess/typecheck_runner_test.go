package assess

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseTscOutput(t *testing.T) {
	output := `src/api/client.ts(42,15): error TS2345: Argument of type 'string' is not assignable to parameter of type 'number'.
src/utils/helpers.ts(18,3): warning TS2322: Type 'undefined' is not assignable to type 'string'.`
	issues := parseTscOutput(output)
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}

	if issues[0].Severity != SeverityHigh {
		t.Fatalf("expected error severity high, got %s", issues[0].Severity)
	}
	if issues[1].Severity != SeverityMedium {
		t.Fatalf("expected warning severity medium, got %s", issues[1].Severity)
	}
	if issues[0].Message == "" || issues[1].Message == "" {
		t.Fatalf("expected messages to be populated")
	}
}

func TestResolveTsconfigPath(t *testing.T) {
	workingDir := t.TempDir()
	configPath := filepath.Join(workingDir, "tsconfig.build.json")
	if err := os.WriteFile(configPath, []byte("{}"), 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	resolved, err := resolveTsconfigPath(workingDir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != configPath {
		t.Fatalf("expected %s, got %s", configPath, resolved)
	}
}

func TestResolveTsconfigPathOverride(t *testing.T) {
	workingDir := t.TempDir()
	configPath := filepath.Join(workingDir, "custom-tsconfig.json")
	if err := os.WriteFile(configPath, []byte("{}"), 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	overrides := &typescriptTypecheckConfig{Config: "custom-tsconfig.json"}

	resolved, err := resolveTsconfigPath(workingDir, overrides)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != configPath {
		t.Fatalf("expected %s, got %s", configPath, resolved)
	}
}

func TestWriteTempTsconfig(t *testing.T) {
	workingDir := t.TempDir()
	baseConfig := filepath.Join(workingDir, "tsconfig.json")
	if err := os.WriteFile(baseConfig, []byte("{}"), 0600); err != nil {
		t.Fatalf("failed to write base config: %v", err)
	}
	targetFile := filepath.Join(workingDir, "src", "index.ts")
	if err := os.MkdirAll(filepath.Dir(targetFile), 0750); err != nil {
		t.Fatalf("failed to create target dir: %v", err)
	}
	if err := os.WriteFile(targetFile, []byte("export const x = 1;"), 0600); err != nil {
		t.Fatalf("failed to write target file: %v", err)
	}

	tempPath, cleanup, err := writeTempTsconfig(workingDir, baseConfig, targetFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	data, err := os.ReadFile(tempPath)
	if err != nil {
		t.Fatalf("failed to read temp config: %v", err)
	}
	var parsed tsconfigTemp
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse temp config: %v", err)
	}
	expected := filepath.ToSlash(baseConfig)
	if parsed.Extends != expected {
		t.Fatalf("expected extends %s, got %s", expected, parsed.Extends)
	}
	if len(parsed.Include) != 1 || parsed.Include[0] != "src/index.ts" {
		t.Fatalf("unexpected include list: %v", parsed.Include)
	}
}
