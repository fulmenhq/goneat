package dates

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/fulmenhq/goneat/internal/gitrepo"
	"github.com/fulmenhq/goneat/internal/gitrepo/gitrepotest"
)

// A changelog date before the first commit is impossible chronology. The
// check needs the repository creation date, which must be readable in
// linked worktrees too.
func TestDates_ImpossibleChronologyInLinkedWorktrees(t *testing.T) {
	f := gitrepotest.New(t, map[string]string{
		"CHANGELOG.md": "# Changelog\n\n## [1.0.0] - 2000-01-01\n\n- first\n",
	})
	for _, dir := range f.All() {
		res, err := NewDatesRunner().Assess(t.Context(), dir, nil)
		if err != nil {
			t.Fatalf("%s: %v", filepath.Base(dir), err)
		}
		found := false
		for _, issue := range res.Issues {
			if issue.Severity == "high" && strings.Contains(issue.Message, "Impossible chronology") {
				found = true
			}
			if strings.Contains(issue.Message, "check skipped") {
				t.Errorf("%s: creation date must be readable: %s", filepath.Base(dir), issue.Message)
			}
		}
		if !found {
			t.Errorf("%s: expected a high impossible-chronology issue, got %+v", filepath.Base(dir), res.Issues)
		}
	}
}

// Disabling incremental scanning must not disable the chronology check.
func TestDates_NoIncrementalKeepsChronologyCheck(t *testing.T) {
	t.Setenv("GONEAT_DATES_NO_INC", "1")
	TestDates_ImpossibleChronologyInLinkedWorktrees(t)
}

// Incremental scanning must select what git reports as changed: nothing on a
// clean worktree (no phantom every-file list), and the real changes when dirty.
func TestDates_ChangedFilesMatchGitInLinkedWorktrees(t *testing.T) {
	f := gitrepotest.New(t, map[string]string{"a.md": "a\n", "b.md": "b\n", "c.md": "c\n"})
	for _, dir := range f.All() {
		repo, err := gitrepo.OpenAt(dir)
		if err != nil {
			t.Fatalf("%s: %v", filepath.Base(dir), err)
		}
		changed, err := changedFiles(repo)
		if err != nil || len(changed) != 0 {
			t.Fatalf("%s: clean tree must have no changed files, got %v err=%v", filepath.Base(dir), changed, err)
		}
		gitrepotest.Write(t, dir, "a.md", "changed\n")
		gitrepotest.Write(t, dir, "b.md", "staged\n")
		gitrepotest.Git(t, dir, "add", "b.md")
		gitrepotest.Write(t, dir, "new.md", "new\n")
		changed, err = changedFiles(repo)
		if err != nil {
			t.Fatalf("%s: %v", filepath.Base(dir), err)
		}
		want := append(gitrepotest.PorcelainTracked(t, dir), "new.md")
		sort.Strings(want)
		if strings.Join(changed, ",") != strings.Join(want, ",") {
			t.Errorf("%s: changed files %v, git reports %v", filepath.Base(dir), changed, want)
		}
	}
}

// If the creation date cannot be read, the skipped check must be reported.
func TestDates_UnreadableHistoryReportsSkippedCheck(t *testing.T) {
	f := gitrepotest.New(t, map[string]string{
		"CHANGELOG.md": "# Changelog\n\n## [1.0.0] - 2000-01-01\n",
	})
	commit := gitrepotest.Git(t, f.Main, "rev-parse", "HEAD")
	if err := os.Remove(filepath.Join(f.Main, ".git", "objects", commit[:2], commit[2:])); err != nil {
		t.Fatalf("remove commit object: %v", err)
	}
	for _, dir := range f.All() {
		res, err := NewDatesRunner().Assess(t.Context(), dir, nil)
		if err != nil {
			t.Fatalf("%s: %v", filepath.Base(dir), err)
		}
		found := false
		for _, issue := range res.Issues {
			if strings.Contains(issue.Message, "Impossible-chronology check skipped") {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: expected a skipped-check diagnostic, got %+v", filepath.Base(dir), res.Issues)
		}
	}
}

// A present but unreadable .git (broken linked-worktree pointer) must not
// silently drop the chronology check; a directory with no .git is simply
// not a repository and gets no diagnostic.
func TestDates_BrokenGitPointerReportsSkippedCheck(t *testing.T) {
	f := gitrepotest.New(t, map[string]string{
		"CHANGELOG.md": "# Changelog\n\n## [1.0.0] - 2000-01-01\n",
	})
	gitrepotest.Write(t, f.Attached, ".git", "gitdir: "+filepath.Join(t.TempDir(), "missing")+"\n")
	res, err := NewDatesRunner().Assess(t.Context(), f.Attached, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !hasSkippedDiagnostic(res.Issues) {
		t.Fatalf("broken .git pointer must report the skipped check, got %+v", res.Issues)
	}

	plain := t.TempDir()
	gitrepotest.Write(t, plain, "CHANGELOG.md", "# Changelog\n\n## [1.0.0] - 2000-01-01\n")
	res, err = NewDatesRunner().Assess(t.Context(), plain, nil)
	if err != nil {
		t.Fatal(err)
	}
	if hasSkippedDiagnostic(res.Issues) {
		t.Fatalf("a non-repository must not report a skipped git check, got %+v", res.Issues)
	}
}

func hasSkippedDiagnostic(issues []DatesIssue) bool {
	for _, issue := range issues {
		if strings.Contains(issue.Message, "Impossible-chronology check skipped") {
			return true
		}
	}
	return false
}
