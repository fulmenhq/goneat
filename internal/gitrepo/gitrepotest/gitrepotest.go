// Package gitrepotest builds real Git repositories with linked worktrees
// (`git worktree add`) for tests. It requires the git CLI.
package gitrepotest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Fixture is a repository with a main clone plus an attached ("feat") and a
// detached linked worktree, all at the same clean commit.
type Fixture struct {
	Main, Attached, Detached string
}

// All returns the three working trees.
func (f Fixture) All() []string { return []string{f.Main, f.Attached, f.Detached} }

// Linked returns the two linked worktrees.
func (f Fixture) Linked() []string { return []string{f.Attached, f.Detached} }

// Git runs git in dir with a fixed identity and no user or system config,
// returning trimmed output.
func Git(t testing.TB, dir string, args ...string) string {
	t.Helper()
	return strings.TrimSpace(gitRaw(t, dir, args...))
}

func gitRaw(t testing.TB, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return string(out)
}

// New commits files (path -> content) in a fresh repository and adds the
// linked worktrees. Tests are skipped when git is not installed.
func New(t testing.TB, files map[string]string) Fixture {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git CLI required for linked-worktree fixtures")
	}
	root := t.TempDir()
	f := Fixture{
		Main:     filepath.Join(root, "main"),
		Attached: filepath.Join(root, "attached"),
		Detached: filepath.Join(root, "detached"),
	}
	if err := os.MkdirAll(f.Main, 0o755); err != nil {
		t.Fatal(err)
	}
	Git(t, f.Main, "init", "-q", "-b", "main")
	for rel, body := range files {
		path := filepath.Join(f.Main, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	Git(t, f.Main, "add", "-A")
	Git(t, f.Main, "commit", "-q", "-m", "init")
	Git(t, f.Main, "worktree", "add", "-q", f.Attached, "-b", "feat")
	Git(t, f.Main, "worktree", "add", "-q", "--detach", f.Detached, "HEAD")
	for _, wt := range f.Linked() {
		if info, err := os.Stat(filepath.Join(wt, ".git")); err != nil || info.IsDir() {
			t.Fatalf("%s: expected a .git file (linked worktree layout)", wt)
		}
	}
	return f
}

// Write writes a file relative to dir.
func Write(t testing.TB, dir, rel, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, rel), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// PorcelainTracked returns the paths git reports as changed, excluding untracked.
func PorcelainTracked(t testing.TB, dir string) []string {
	t.Helper()
	var paths []string
	for _, entry := range strings.Split(gitRaw(t, dir, "status", "--porcelain=v1", "-z", "--untracked-files=all"), "\x00") {
		if len(entry) > 3 && !strings.HasPrefix(entry, "??") {
			paths = append(paths, entry[3:])
		}
	}
	return paths
}

// RemoveHeadTree deletes the loose object for HEAD's tree from the common
// object store, so reading status fails with "object not found".
func RemoveHeadTree(t testing.TB, f Fixture) {
	t.Helper()
	tree := Git(t, f.Main, "rev-parse", "HEAD^{tree}")
	if err := os.Remove(filepath.Join(f.Main, ".git", "objects", tree[:2], tree[2:])); err != nil {
		t.Fatalf("remove tree object: %v", err)
	}
}
