package pathfinder

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/fulmenhq/goneat/pkg/schema/signature"
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

func TestFinderFacade_SchemaMode(t *testing.T) {
	t.Setenv("GONEAT_HOME", t.TempDir())

	testDir := filepath.Join("testdata", "schemasearch")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(testDir) }()

	schemaPath := filepath.Join(testDir, "draft7.json")
	schemaContent := []byte("{\"$schema\":\"https://json-schema.org/draft-07/schema#\",\"title\":\"Example\"}")
	if err := os.WriteFile(schemaPath, schemaContent, 0o644); err != nil {
		t.Fatalf("failed writing schema file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testDir, "notes.txt"), []byte("not a schema"), 0o644); err != nil {
		t.Fatalf("failed writing non-schema file: %v", err)
	}
	if content, err := os.ReadFile(schemaPath); err != nil {
		t.Fatalf("failed reading schema file: %v", err)
	} else if !strings.Contains(string(content), "\"$schema\"") {
		t.Fatalf("unexpected schema file content: %s", string(content))
	}
	manifest, err := signature.LoadDefaultManifest()
	if err != nil {
		t.Fatalf("LoadDefaultManifest error: %v", err)
	}
	var target signature.Signature
	for _, sig := range manifest.Signatures {
		if strings.EqualFold(sig.ID, "json-schema-draft-07") {
			target = sig
			break
		}
	}
	if len(target.Matchers) == 0 {
		t.Fatalf("draft-07 signature missing matchers: %+v", target)
	}
	detector, err := signature.NewDetector(manifest)
	if err != nil {
		t.Fatalf("NewDetector error: %v", err)
	}
	matches := detector.DetectAll(schemaPath, schemaContent, signature.DetectOptions{})
	if len(matches) == 0 {
		t.Fatalf("expected manifest to match schema file (signature: %+v)", target)
	}
	match := matches[0]
	if match.Signature.ID != "json-schema-draft-07" {
		t.Fatalf("expected draft-07 signature, got %s", match.Signature.ID)
	}

	facade := NewFinderFacade(NewPathFinder(), FinderConfig{})
	results, err := facade.Find(FindQuery{
		Root:                  testDir,
		SchemaMode:            true,
		IncludeSchemaMetadata: true,
		Context:               context.Background(),
	})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 schema result, got %d", len(results))
	}
	if results[0].RelativePath != "draft7.json" {
		t.Fatalf("unexpected schema result path: %s", results[0].RelativePath)
	}
	meta, ok := results[0].Metadata["schema"].(map[string]any)
	if !ok {
		t.Fatalf("expected schema metadata, got %+v", results[0].Metadata)
	}
	if meta["id"] != "json-schema-draft-07" {
		t.Fatalf("unexpected schema id: %v", meta["id"])
	}

	absRoot, err := filepath.Abs(testDir)
	if err != nil {
		t.Fatalf("Abs path error: %v", err)
	}
	absResults, err := facade.Find(FindQuery{
		Root:                  absRoot,
		SchemaMode:            true,
		IncludeSchemaMetadata: true,
		Context:               context.Background(),
	})
	if err != nil {
		t.Fatalf("Find (abs) returned error: %v", err)
	}
	if len(absResults) != 1 {
		t.Fatalf("expected 1 schema result with abs path, got %d", len(absResults))
	}
}

func TestFinderFacade_SchemaModeDefaultIncludes(t *testing.T) {
	t.Setenv("GONEAT_HOME", t.TempDir())

	testDir := filepath.Join("testdata", "schema-default-includes")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(testDir) }()

	jsonPath := filepath.Join(testDir, "draft7.json")
	if err := os.WriteFile(jsonPath, []byte("{\"$schema\":\"https://json-schema.org/draft-07/schema#\"}"), 0o644); err != nil {
		t.Fatalf("failed writing json schema: %v", err)
	}
	protoPath := filepath.Join(testDir, "helloworld.proto")
	protoContent := "syntax = \"proto3\";\n\npackage example.schema;\n\nmessage Ping { string name = 1; }\n"
	if err := os.WriteFile(protoPath, []byte(protoContent), 0o644); err != nil {
		t.Fatalf("failed writing proto schema: %v", err)
	}

	facade := NewFinderFacade(NewPathFinder(), FinderConfig{})
	results, err := facade.Find(FindQuery{
		Root:                  testDir,
		SchemaMode:            true,
		IncludeSchemaMetadata: true,
		Context:               context.Background(),
	})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 schema results, got %d", len(results))
	}

	ids := make(map[string]string, len(results))
	for _, res := range results {
		if meta, ok := res.Metadata["schema"].(map[string]any); ok {
			if id, _ := meta["id"].(string); id != "" {
				ids[res.RelativePath] = id
			}
		}
	}
	if ids["draft7.json"] != "json-schema-draft-07" {
		t.Fatalf("expected draft7.json id json-schema-draft-07, got %+v", ids)
	}
	if ids["helloworld.proto"] != "protobuf-schema" {
		t.Fatalf("expected helloworld.proto id protobuf-schema, got %+v", ids)
	}
}

func TestBuildSchemaIncludePatterns(t *testing.T) {
	manifest, err := signature.LoadDefaultManifest()
	if err != nil {
		t.Fatalf("LoadDefaultManifest error: %v", err)
	}
	patterns := buildSchemaIncludePatterns(manifest)
	if len(patterns) == 0 {
		t.Fatal("expected default schema include patterns")
	}
	if !containsPattern(patterns, "**/*.json") {
		t.Fatalf("expected pattern list to include json: %v", patterns)
	}
	if !containsPattern(patterns, "**/*.proto") {
		t.Fatalf("expected pattern list to include proto: %v", patterns)
	}
}

func containsPattern(patterns []string, target string) bool {
	for _, pattern := range patterns {
		if pattern == target {
			return true
		}
	}
	return false
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
