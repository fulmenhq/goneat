package cargo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseLock(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "Cargo.lock"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	pkgs, err := ParseLock(data)
	if err != nil {
		t.Fatalf("ParseLock: %v", err)
	}
	if len(pkgs) != 6 {
		t.Fatalf("expected 6 packages, got %d", len(pkgs))
	}

	byName := map[string]Package{}
	for _, p := range pkgs {
		byName[p.Name] = p
	}

	if byName["cooling-fixture"].Source != SourcePath {
		t.Errorf("workspace package source = %q, want path", byName["cooling-fixture"].Source)
	}
	if byName["young-crate"].Source != SourceRegistry || !IsCratesIO(byName["young-crate"].RawSource) {
		t.Errorf("young-crate should be crates.io registry, got %+v", byName["young-crate"])
	}
	if byName["git-only-dep"].Source != SourceGit {
		t.Errorf("git-only-dep source = %q, want git", byName["git-only-dep"].Source)
	}
}

func TestParseMetadataJSON(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "metadata.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	pkgs, err := ParseMetadataJSON(data)
	if err != nil {
		t.Fatalf("ParseMetadataJSON: %v", err)
	}
	if len(pkgs) != 3 {
		t.Fatalf("expected 3 packages, got %d", len(pkgs))
	}
	if pkgs[0].Source != SourcePath {
		t.Errorf("null source should be path, got %q", pkgs[0].Source)
	}
	if pkgs[1].Source != SourceRegistry {
		t.Errorf("registry source = %q", pkgs[1].Source)
	}
	if pkgs[2].Source != SourceGit {
		t.Errorf("git source = %q", pkgs[2].Source)
	}
}

func TestClassifySource(t *testing.T) {
	tests := []struct {
		raw  string
		want SourceKind
	}{
		{"", SourcePath},
		{"registry+https://github.com/rust-lang/crates.io-index", SourceRegistry},
		{"git+https://github.com/example/dep#abc", SourceGit},
		{"path+file:///tmp/foo", SourcePath},
	}
	for _, tt := range tests {
		if got := ClassifySource(tt.raw); got != tt.want {
			t.Errorf("ClassifySource(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}

func TestFindLock(t *testing.T) {
	dir := filepath.Join("testdata")
	if FindLock(dir) == "" {
		t.Fatal("expected to find testdata/Cargo.lock")
	}
	if FindLock(t.TempDir()) != "" {
		t.Fatal("empty dir should not have a lockfile")
	}
}
