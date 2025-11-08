package ssot

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestCloneRepository_FileURL(t *testing.T) {
	t.Setenv("GONEAT_HOME", t.TempDir())

	repoPath := initTestRepo(t)
	target := fmt.Sprintf("file://%s", repoPath)

	cloned, err := CloneRepository(target, "master")
	if err != nil {
		t.Fatalf("CloneRepository failed: %v", err)
	}
	if cloned == nil {
		t.Fatal("expected cloned repo, got nil")
	}
	if _, err := os.Stat(filepath.Join(cloned.Path, ".git")); err != nil {
		t.Fatalf("expected .git directory: %v", err)
	}
	if cloned.Cached {
		t.Fatal("first clone should not be cached")
	}

	// Second call should reuse cache
	clonedAgain, err := CloneRepository(target, "master")
	if err != nil {
		t.Fatalf("CloneRepository (cached) failed: %v", err)
	}
	if !clonedAgain.Cached {
		t.Fatal("expected cached clone on second invocation")
	}
}

func TestCloneRepository_InvalidRef(t *testing.T) {
	t.Setenv("GONEAT_HOME", t.TempDir())
	repoPath := initTestRepo(t)
	target := fmt.Sprintf("file://%s", repoPath)

	if _, err := CloneRepository(target, "does-not-exist"); err == nil {
		t.Fatal("expected error for invalid ref, got nil")
	}
}

func initTestRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}

	// Create subdirectories to simulate sync_path_base
	subdir := filepath.Join(dir, "lang", "go")
	if err := os.MkdirAll(subdir, 0o750); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "README.md"), []byte("hello"), 0o640); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	if _, err := worktree.Add("."); err != nil {
		t.Fatalf("failed to add files: %v", err)
	}

	_, err = worktree.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "goneat",
			Email: "ci@goneat.dev",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	return dir
}
