package assess

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLintAssessmentRunner_RunsRuffWhenPythonFilesPresent(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "pyproject.toml"), []byte("[tool.ruff]\n"), 0o644); err != nil {
		t.Fatalf("write pyproject: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "a.py"), []byte("print('hi')\n"), 0o644); err != nil {
		t.Fatalf("write python file: %v", err)
	}

	binDir := filepath.Join(repo, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}

	ruffPath := filepath.Join(binDir, "ruff")
	ruffJSON, _ := json.Marshal([]map[string]any{{
		"code":     "F401",
		"message":  "unused import",
		"filename": "a.py",
		"location": map[string]any{"row": 1, "column": 1},
	}})

	script := "#!/usr/bin/env bash\n" +
		"set -euo pipefail\n" +
		"if [[ \"$1\" == \"--version\" ]]; then echo 'ruff 0.6.0'; exit 0; fi\n" +
		"if [[ \"$1\" == \"check\" ]]; then echo '" + string(ruffJSON) + "'; exit 1; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(ruffPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write ruff script: %v", err)
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+oldPath)

	cfg := DefaultAssessmentConfig()
	cfg.Mode = AssessmentModeCheck
	cfg.NoIgnore = true
	cfg.Concurrency = runtime.NumCPU()

	r := NewLintAssessmentRunner()
	res, err := r.Assess(context.Background(), repo, cfg)
	if err != nil {
		t.Fatalf("Assess error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success")
	}

	found := false
	for _, is := range res.Issues {
		if is.SubCategory == "python:ruff" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected python:ruff issues, got %+v", res.Issues)
	}
}
