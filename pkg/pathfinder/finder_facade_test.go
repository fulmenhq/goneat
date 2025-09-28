package pathfinder

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestFinderFacade_FindBasic(t *testing.T) {
	// Use current directory for testing to avoid temp dir symlink issues
	testDir := "testdata"
	if err := os.MkdirAll(filepath.Join(testDir, "data"), 0o755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(filepath.Join(testDir, "data")) }()   // cleanup
	defer func() { _ = os.Remove(filepath.Join(testDir, "other.txt")) }() // cleanup

	mustCreateFile(t, filepath.Join(testDir, "data", "one.xml"))
	mustCreateFile(t, filepath.Join(testDir, "data", "nested", "two.xml"))
	mustCreateFile(t, filepath.Join(testDir, "other.txt"))

	facade := NewFinderFacade(NewPathFinder(), FinderConfig{})
	results, err := facade.Find(FindQuery{
		Root:    testDir,
		Include: []string{"**/*.xml"},
		Context: context.Background(),
	})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	sort.Slice(results, func(i, j int) bool { return results[i].RelativePath < results[j].RelativePath })
	if results[0].RelativePath != "data/nested/two.xml" && results[0].RelativePath != "data/one.xml" {
		t.Fatalf("unexpected relative path: %#v", results[0].RelativePath)
	}
	for _, res := range results {
		if res.LogicalPath != res.RelativePath {
			t.Fatalf("expected logical path to match relative path, got %q vs %q", res.LogicalPath, res.RelativePath)
		}
		if res.LoaderType == "" {
			t.Fatal("expected loader type to be set")
		}
	}
}

func TestFinderFacade_Transform(t *testing.T) {
	testDir := "testdata"
	testSubDir := filepath.Join(testDir, "transform")
	if err := os.MkdirAll(testSubDir, 0o755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(testSubDir) }() // cleanup

	mustCreateFile(t, filepath.Join(testSubDir, "stage", "alpha", "report.csv"))

	facade := NewFinderFacade(NewPathFinder(), FinderConfig{})
	results, err := facade.Find(FindQuery{
		Root:    testDir,
		Include: []string{"**/*.csv"},
		Context: context.Background(),
		Transform: func(result PathResult) PathResult {
			result.LogicalPath = filepath.Base(result.RelativePath)
			return result
		},
	})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].LogicalPath != "report.csv" {
		t.Fatalf("expected logical path to be flattened, got %q", results[0].LogicalPath)
	}
}

func TestFinderFacade_FindStreamRespectsContext(t *testing.T) {
	testDir := "testdata"
	testSubDir := filepath.Join(testDir, "stream")
	if err := os.MkdirAll(filepath.Join(testSubDir, "files"), 0o755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(testSubDir) }() // cleanup

	for i := 0; i < 3; i++ {
		mustCreateFile(t, filepath.Join(testSubDir, "files", fileNameForIndex(i)))
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately to test propagation

	facade := NewFinderFacade(NewPathFinder(), FinderConfig{})
	_, errCh := facade.FindStream(FindQuery{
		Root:    testDir,
		Context: ctx,
	})

	if err := <-errCh; err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestFinderFacade_MaxDepth(t *testing.T) {
	testDir := "testdata"
	testSubDir := filepath.Join(testDir, "maxdepth")
	if err := os.MkdirAll(testSubDir, 0o755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(testSubDir) }() // cleanup

	mustCreateFile(t, filepath.Join(testDir, "one.txt"))    // depth 1
	mustCreateFile(t, filepath.Join(testSubDir, "two.txt")) // depth 2

	facade := NewFinderFacade(NewPathFinder(), FinderConfig{})
	results, err := facade.Find(FindQuery{
		Root:     testDir,
		Include:  []string{"**/*.txt"},
		MaxDepth: 1,
		Context:  context.Background(),
	})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result at depth <= 1, got %d", len(results))
	}
	if results[0].RelativePath != "one.txt" {
		t.Fatalf("unexpected result: %#v", results[0].RelativePath)
	}
}

func mustCreateFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed creating directory for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatalf("failed creating file %s: %v", path, err)
	}
}

func fileNameForIndex(i int) string {
	if i == 0 {
		return "a.txt"
	}
	if i == 1 {
		return "b.txt"
	}
	return "c.txt"
}
