package dates

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulmenhq/goneat/internal/gitrepo/gitrepotest"
)

// A depth-1 clone still has a commit, but that commit is not repository
// creation. The chronology check must say the history is incomplete.
func TestDates_ShallowCloneDoesNotUseTipAsCreation(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git CLI required")
	}
	root := t.TempDir()
	origin := filepath.Join(root, "origin")
	if err := os.MkdirAll(origin, 0o755); err != nil {
		t.Fatal(err)
	}
	gitrepotest.Git(t, origin, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(origin, "CHANGELOG.md"), []byte("# Changelog\n\n## [1.0.0] - 2000-01-01\n\n- first\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitrepotest.Git(t, origin, "add", "-A")
	gitrepotest.Git(t, origin, "commit", "-q", "-m", "init")
	if err := os.WriteFile(filepath.Join(origin, "README.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitrepotest.Git(t, origin, "add", "-A")
	gitrepotest.Git(t, origin, "commit", "-q", "-m", "second")

	shallow := filepath.Join(root, "shallow")
	gitrepotest.Git(t, root, "clone", "--depth", "1", "file://"+origin, shallow)
	if gitrepotest.Git(t, shallow, "rev-parse", "--is-shallow-repository") != "true" {
		t.Fatal("expected a shallow clone")
	}
	assertShallowHistory(t, shallow)

	wt := filepath.Join(root, "wt")
	gitrepotest.Git(t, shallow, "worktree", "add", "-q", "--detach", wt, "HEAD")
	assertShallowHistory(t, wt)
}

func assertShallowHistory(t *testing.T, dir string) {
	t.Helper()
	res, err := NewDatesRunner().Assess(t.Context(), dir, nil)
	if err != nil {
		t.Fatalf("%s: %v", filepath.Base(dir), err)
	}
	found := false
	for _, issue := range res.Issues {
		if strings.Contains(issue.Message, "predates repository creation") {
			t.Errorf("%s: tip commit used as repository creation: %s", filepath.Base(dir), issue.Message)
		}
		if issue.Severity == "high" && strings.Contains(issue.Message, "repository is shallow") {
			found = true
		}
	}
	if !found {
		t.Errorf("%s: expected a high shallow-history issue, got %+v", filepath.Base(dir), res.Issues)
	}
}
