package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type pathfinderResult struct {
	RelativePath string `json:"relative_path"`
	LogicalPath  string `json:"logical_path"`
}

func TestPathfinderFindJSON(t *testing.T) {
	// Use .scratchpad directory to avoid symlink issues
	// Get repo root to create test directories in the right place
	cwd, _ := os.Getwd()
	repoRoot := cwd
	if root := findRepoRootFS(cwd); root != "" {
		repoRoot = root
	}
	cmdTestDir := filepath.Join(repoRoot, "test_temp", "test_pathfinder")
	if err := os.MkdirAll(filepath.Join(cmdTestDir, "input"), 0o755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(cmdTestDir) // cleanup

	mustWrite(t, filepath.Join(cmdTestDir, "input", "item.txt"))
	mustWrite(t, filepath.Join(cmdTestDir, "input", "nested", "skip.log"))

	out, err := execRoot(t, []string{"pathfinder", "find", "--path", cmdTestDir, "--include", "**/*.txt", "--output", "json"})
	if err != nil {
		t.Fatalf("pathfinder find failed: %v\n%s", err, out)
	}

	var results []pathfinderResult
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 JSON result, got %d (%s)", len(results), out)
	}
	if results[0].RelativePath != "input/item.txt" {
		t.Fatalf("unexpected relative path: %q", results[0].RelativePath)
	}
	if results[0].LogicalPath != "input/item.txt" {
		t.Fatalf("logical path should match relative path, got %q", results[0].LogicalPath)
	}
}

func TestPathfinderFindTextFlatten(t *testing.T) {
	// Get repo root to create test directories in the right place
	cwd, _ := os.Getwd()
	repoRoot := cwd
	if root := findRepoRootFS(cwd); root != "" {
		repoRoot = root
	}
	cmdTestDir := filepath.Join(repoRoot, "test_temp", "test_pathfinder2")
	if err := os.MkdirAll(filepath.Join(cmdTestDir, "stage", "alpha"), 0o755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(cmdTestDir) // cleanup

	mustWrite(t, filepath.Join(cmdTestDir, "stage", "alpha", "delta.csv"))

	out, err := execRoot(t, []string{"pathfinder", "find", "--path", cmdTestDir, "--include", "**/*.csv", "--output", "text", "--flatten"})
	if err != nil {
		t.Fatalf("pathfinder find (text) failed: %v\n%s", err, out)
	}

	lines := strings.FieldsFunc(strings.TrimSpace(out), func(r rune) bool { return r == '\n' || r == '\r' })
	if len(lines) != 1 {
		t.Fatalf("expected single line output, got %d (%s)", len(lines), out)
	}
	if lines[0] != "delta.csv" {
		t.Fatalf("unexpected logical path: %q", lines[0])
	}
}

func TestPathfinderFindMaxDepth(t *testing.T) {
	// Create temp directory in current working directory to avoid symlink issues
	cwd, _ := os.Getwd()
	tmp := filepath.Join(cwd, "test_maxdepth_tmp")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	mustWrite(t, filepath.Join(tmp, "top.txt"))
	mustWrite(t, filepath.Join(tmp, "nested", "deep.txt"))

	out, err := execRoot(t, []string{"pathfinder", "find", "--path", tmp, "--include", "**/*.txt", "--max-depth", "1", "--output", "json"})
	if err != nil {
		t.Fatalf("pathfinder find with max depth failed: %v\n%s", err, out)
	}

	var results []pathfinderResult
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result honoring max depth, got %d", len(results))
	}
	if results[0].RelativePath != "top.txt" {
		t.Fatalf("unexpected relative path with max depth: %q", results[0].RelativePath)
	}
}

func mustWrite(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed creating directory %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed writing file %s: %v", path, err)
	}
}
