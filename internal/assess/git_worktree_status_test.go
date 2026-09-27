package assess

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulmenhq/goneat/internal/gitctx"
	"github.com/fulmenhq/goneat/internal/gitrepo/gitrepotest"
)

func newGitFixture(t *testing.T) gitrepotest.Fixture {
	t.Helper()
	return gitrepotest.New(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n", "c.txt": "c\n"})
}

func porcelainTracked(t *testing.T, dir string) int {
	return len(gitrepotest.PorcelainTracked(t, dir))
}

func runGit(t *testing.T, dir string, args ...string) string {
	return gitrepotest.Git(t, dir, args...)
}

func makeDirty(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "b.txt")
	if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assessRepoStatus(t *testing.T, dir string) []Issue {
	t.Helper()
	res, err := NewRepoStatusRunner().Assess(t.Context(), dir, DefaultAssessmentConfig())
	if err != nil {
		t.Fatalf("repo-status: %v", err)
	}
	return res.Issues
}

func assessReleaseMaturity(t *testing.T, dir string) []Issue {
	t.Helper()
	for name, body := range map[string]string{"RELEASE_PHASE": "release\n", "LIFECYCLE_PHASE": "ga\n", "VERSION": "1.0.0\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	res, err := NewMaturityRunner().Assess(t.Context(), dir, DefaultAssessmentConfig())
	if err != nil {
		t.Fatalf("maturity: %v", err)
	}
	var gitIssues []Issue
	for _, issue := range res.Issues {
		if strings.HasPrefix(issue.SubCategory, "git-") {
			gitIssues = append(gitIssues, issue)
		}
	}
	return gitIssues
}

type gitAssess func(t *testing.T, dir string) []Issue

var gitStateCategories = map[string]gitAssess{
	"repo-status":      assessRepoStatus,
	"release-maturity": assessReleaseMaturity,
}

func TestGitState_LinkedWorktreesMatchGit(t *testing.T) {
	for category, assess := range gitStateCategories {
		t.Run(category+"/clean", func(t *testing.T) {
			f := newGitFixture(t)
			for _, dir := range []string{f.Main, f.Attached, f.Detached} {
				if issues := assess(t, dir); len(issues) != 0 {
					t.Errorf("%s: clean tree must have no git issues, got %+v", filepath.Base(dir), issues)
				}
			}
		})
		t.Run(category+"/tracked dirty", func(t *testing.T) {
			f := newGitFixture(t)
			for _, dir := range []string{f.Main, f.Attached, f.Detached} {
				makeDirty(t, dir)
				issues := assess(t, dir)
				want := porcelainTracked(t, dir) // modified + staged = 2
				if want != 2 || len(issues) != 1 || issues[0].Severity != SeverityHigh || issues[0].SubCategory != "git-state" {
					t.Fatalf("%s: expected one high git-state issue for %d tracked changes, got %+v", filepath.Base(dir), want, issues)
				}
				if !strings.Contains(issues[0].Message, "a.txt") || !strings.Contains(issues[0].Message, "b.txt") || strings.Contains(issues[0].Message, "untracked.txt") {
					t.Errorf("%s: issue must list exactly the tracked changes, got %q", filepath.Base(dir), issues[0].Message)
				}
			}
		})
		t.Run(category+"/untracked only does not block", func(t *testing.T) {
			f := newGitFixture(t)
			for _, dir := range []string{f.Main, f.Attached, f.Detached} {
				if err := os.WriteFile(filepath.Join(dir, "scratch.txt"), []byte("x\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				if issues := assess(t, dir); len(issues) != 0 {
					t.Errorf("%s: untracked-only must not block, got %+v", filepath.Base(dir), issues)
				}
			}
		})
		t.Run(category+"/broken .git pointer is a distinct error", func(t *testing.T) {
			f := newGitFixture(t)
			if err := os.WriteFile(filepath.Join(f.Attached, ".git"), []byte("gitdir: "+filepath.Join(t.TempDir(), "missing")+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			assertStatusError(t, assess(t, f.Attached))
		})
		t.Run(category+"/missing object is a distinct error", func(t *testing.T) {
			f := newGitFixture(t)
			gitrepotest.RemoveHeadTree(t, f)
			for _, dir := range []string{f.Attached, f.Detached} {
				assertStatusError(t, assess(t, dir))
			}
		})
	}
}

func assertStatusError(t *testing.T, issues []Issue) {
	t.Helper()
	if len(issues) != 1 || issues[0].Severity != SeverityHigh || issues[0].SubCategory != "git-status-error" {
		t.Fatalf("expected one high git-status-error issue, got %+v", issues)
	}
	if strings.Contains(issues[0].Message, "uncommitted") {
		t.Fatalf("a status failure must not be reported as uncommitted changes: %q", issues[0].Message)
	}
}

// The change context attached to reports must agree with repo-status.
func TestGitState_ChangeContextAgreesWithRepoStatus(t *testing.T) {
	f := newGitFixture(t)
	for _, dir := range []string{f.Main, f.Attached, f.Detached} {
		ctx, _, err := gitctx.Collect(dir)
		if err != nil || ctx == nil {
			t.Fatalf("%s: change context: %v", filepath.Base(dir), err)
		}
		if len(ctx.ModifiedFiles) != 0 || len(assessRepoStatus(t, dir)) != 0 {
			t.Errorf("%s: clean: change context %v vs repo-status must both be empty", filepath.Base(dir), ctx.ModifiedFiles)
		}
		makeDirty(t, dir)
		ctx, _, err = gitctx.Collect(dir)
		if err != nil || ctx == nil {
			t.Fatalf("%s: change context: %v", filepath.Base(dir), err)
		}
		issues := assessRepoStatus(t, dir)
		// Agreement is on tracked changes. gitctx also lists untracked files
		// on its go-git path (ordinary clones) but not on its git CLI fallback;
		// that difference is gitctx's own and is outside repo-status.
		var tracked []string
		for _, f := range ctx.ModifiedFiles {
			if f != "untracked.txt" {
				tracked = append(tracked, f)
			}
		}
		if len(tracked) != 2 || len(issues) != 1 || !strings.Contains(issues[0].Message, "2 uncommitted") {
			t.Errorf("%s: dirty: change context %v and repo-status %+v must agree on 2 tracked changes", filepath.Base(dir), ctx.ModifiedFiles, issues)
		}
	}
}
