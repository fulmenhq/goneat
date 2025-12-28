package assess

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLintAssessmentRunner_PassesStagedOnlyFilesToRuff(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "pyproject.toml"), []byte("[tool.ruff]\n"), 0o644); err != nil {
		t.Fatalf("write pyproject: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "a.py"), []byte("print('a')\n"), 0o644); err != nil {
		t.Fatalf("write python file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "b.py"), []byte("print('b')\n"), 0o644); err != nil {
		t.Fatalf("write python file: %v", err)
	}

	binDir := filepath.Join(repo, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}

	outPath := filepath.Join(repo, "args.json")

	ruffJSON, _ := json.Marshal([]map[string]any{{
		"code":     "F401",
		"message":  "unused import",
		"filename": "a.py",
		"location": map[string]any{"row": 1, "column": 1},
	}})

	script := "#!/usr/bin/env bash\n" +
		"set -euo pipefail\n" +
		"if [[ -n \"${RUFF_ARGS_OUT:-}\" ]]; then printf '%s\n' \"$@\" > \"$RUFF_ARGS_OUT\"; fi\n" +
		"if [[ \"$1\" == \"--version\" ]]; then echo 'ruff 0.6.0'; exit 0; fi\n" +
		"if [[ \"$1\" == \"check\" ]]; then echo '" + string(ruffJSON) + "'; exit 1; fi\n" +
		"exit 0\n"

	ruffPath := filepath.Join(binDir, "ruff")
	if err := os.WriteFile(ruffPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write ruff script: %v", err)
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+oldPath)
	t.Setenv("RUFF_ARGS_OUT", outPath)

	cfg := DefaultAssessmentConfig()
	cfg.Mode = AssessmentModeCheck
	cfg.NoIgnore = true
	cfg.IncludeFiles = []string{"a.py"} // staged-only equivalent

	r := NewLintAssessmentRunner()
	res, err := r.Assess(context.Background(), repo, cfg)
	if err != nil {
		t.Fatalf("Assess error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success")
	}

	argsBytes, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read args out: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(argsBytes)), "\n")

	// Expect: ruff check --output-format json a.py
	foundA := false
	foundB := false
	for _, a := range lines {
		if a == "a.py" {
			foundA = true
		}
		if a == "b.py" {
			foundB = true
		}
	}
	if !foundA || foundB {
		t.Fatalf("expected args to include only a.py, got %v", lines)
	}
}
