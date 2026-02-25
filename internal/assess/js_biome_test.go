package assess

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseBiomeReport(t *testing.T) {
	t.Run("valid json", func(t *testing.T) {
		out := []byte(`{"summary": {"errors": 0, "warnings": 0}, "diagnostics": []}`)
		report, err := parseBiomeReport(out)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if report.Summary.Errors != 0 {
			t.Errorf("expected 0 errors, got %d", report.Summary.Errors)
		}
	})

	t.Run("no json output", func(t *testing.T) {
		out := []byte(`biome format failed: configuration error`)
		_, err := parseBiomeReport(out)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		expectedErrStr := "no json output from biome\nbiome output:\nbiome format failed: configuration error"
		if err.Error() != expectedErrStr {
			t.Errorf("expected error %q, got %q", expectedErrStr, err.Error())
		}
	})
}

func TestGroupBiomeFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "biome-group-test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	// Create structure:
	// /tempDir/.git/ (to anchor the repo)
	// /tempDir/biome.json
	// /tempDir/src/app.ts
	// /tempDir/packages/nested/biome.json
	// /tempDir/packages/nested/src/index.ts

	_ = os.MkdirAll(filepath.Join(tempDir, ".git"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir, "biome.json"), []byte("{}"), 0644)
	_ = os.MkdirAll(filepath.Join(tempDir, "src"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir, "src", "app.ts"), []byte(""), 0644)

	nestedDir := filepath.Join(tempDir, "packages", "nested")
	_ = os.MkdirAll(nestedDir, 0755)
	_ = os.WriteFile(filepath.Join(nestedDir, "biome.json"), []byte("{}"), 0644)
	_ = os.MkdirAll(filepath.Join(nestedDir, "src"), 0755)
	_ = os.WriteFile(filepath.Join(nestedDir, "src", "index.ts"), []byte(""), 0644)

	files := []string{
		filepath.Join("src", "app.ts"),
		filepath.Join("packages", "nested", "src", "index.ts"),
	}

	groups, err := groupBiomeFiles(tempDir, files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	// Root group
	rootGroup := groups[tempDir]
	if len(rootGroup) != 1 || rootGroup[0] != filepath.Join("src", "app.ts") {
		t.Errorf("unexpected root group: %v", rootGroup)
	}

	// Nested group
	nestedGroup := groups[nestedDir]
	if len(nestedGroup) != 1 || nestedGroup[0] != filepath.Join("src", "index.ts") {
		t.Errorf("unexpected nested group: %v", nestedGroup)
	}
}
