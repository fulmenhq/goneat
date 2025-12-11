package assess

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveYamllintTargets_DefaultPatterns(t *testing.T) {
	tdir := t.TempDir()
	paths := []string{
		".github/workflows/build.yml",
		".github/workflows/deploy.yaml",
		"docs/workflows/ignored.yaml",
	}
	for _, p := range paths {
		full := filepath.Join(tdir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
		if err := os.WriteFile(full, []byte("name: test"), 0o644); err != nil {
			t.Fatalf("write failed: %v", err)
		}
	}

	files, err := resolveYamllintTargets(tdir, nil)
	if err != nil {
		t.Fatalf("resolveYamllintTargets error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d (%v)", len(files), files)
	}
}

func TestResolveYamllintTargets_WithOverrides(t *testing.T) {
	tdir := t.TempDir()
	files := []string{"workflows/root.yaml", "workflows/skip.yaml"}
	for _, p := range files {
		full := filepath.Join(tdir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
		if err := os.WriteFile(full, []byte("name: test"), 0o644); err != nil {
			t.Fatalf("write failed: %v", err)
		}
	}
	cfg := &yamllintOverrides{
		Paths:  []string{"workflows/*.yaml"},
		Ignore: []string{"**/skip.yaml"},
	}
	result, err := resolveYamllintTargets(tdir, cfg)
	if err != nil {
		t.Fatalf("resolve error: %v", err)
	}
	if len(result) != 1 || result[0] != "workflows/root.yaml" {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestParseYamllintOutput(t *testing.T) {
	output := "env.yaml:4:1: [warning] missing document start \"---\" (document-start)\n" +
		"env.yaml:10:5: [error] wrong indentation (indentation)"
	issues := parseYamllintOutput(output, ".")
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}
	if issues[0].Severity != SeverityMedium {
		t.Fatalf("expected warning severity, got %v", issues[0].Severity)
	}
	if issues[1].Severity != SeverityHigh {
		t.Fatalf("expected error severity, got %v", issues[1].Severity)
	}
}
