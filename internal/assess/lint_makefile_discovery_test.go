package assess

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectFilesWithScope_MatchesRootMakefileWithDoubleStar(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte("all:\n\techo ok\n"), 0o644); err != nil {
		t.Fatalf("write Makefile: %v", err)
	}

	cfg := DefaultAssessmentConfig()
	cfg.NoIgnore = true

	files, err := collectFilesWithScope(dir, []string{"**/Makefile"}, nil, cfg)
	if err != nil {
		t.Fatalf("collectFilesWithScope error: %v", err)
	}
	if len(files) != 1 || files[0] != "Makefile" {
		t.Fatalf("expected [Makefile], got %v", files)
	}
}
