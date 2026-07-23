package assess

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRuffAssessPathsSkipWhenToolUnavailable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	target := t.TempDir()
	pythonFile := filepath.Join(target, "sample.py")
	original := []byte("x=1\n")
	if err := os.WriteFile(pythonFile, original, 0o644); err != nil {
		t.Fatal(err)
	}

	files := []string{pythonFile}
	config := AssessmentConfig{Mode: AssessmentModeFix}

	if issues, err := runRuffCheck(target, config, files); err != nil {
		t.Fatalf("missing ruff should skip lint without error: %v", err)
	} else if len(issues) != 0 {
		t.Fatalf("missing ruff should not report lint issues: %+v", issues)
	}

	if issues, err := runRuffFormat(target, config, files); err != nil {
		t.Fatalf("missing ruff should skip format without error: %v", err)
	} else if len(issues) != 0 {
		t.Fatalf("missing ruff should not report format issues: %+v", issues)
	}

	after, err := os.ReadFile(pythonFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, original) {
		t.Fatalf("assess missing-tool skip mutated file: got %q want %q", after, original)
	}
}
