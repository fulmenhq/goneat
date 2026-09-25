package propagation

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulmenhq/goneat/internal/gitrepo/gitrepotest"
)

func guardPolicy(branches ...string) *VersionPolicy {
	return &VersionPolicy{Guards: GuardsConfig{RequiredBranches: branches, DisallowDirtyWorktree: true}}
}

// Version propagation guards must behave in linked worktrees exactly as in
// an ordinary clone: clean passes, tracked changes block, branch guard sees
// the worktree's branch.
func TestGuards_LinkedWorktrees(t *testing.T) {
	f := gitrepotest.New(t, map[string]string{"VERSION": "1.0.0\n"})
	p := &Propagator{}

	cases := []struct {
		dir     string
		branch  string
		wantErr string
	}{
		{f.Main, "main", ""},
		{f.Attached, "feat", ""},
		{f.Detached, "HEAD", ""},
	}
	for _, tc := range cases {
		t.Run(filepath.Base(tc.dir)+"/clean", func(t *testing.T) {
			t.Chdir(tc.dir)
			if got, err := p.getCurrentBranch(); err != nil || got != tc.branch {
				t.Fatalf("branch = %q, err=%v; want %q", got, err, tc.branch)
			}
			if err := p.checkGuards(guardPolicy()); err != nil {
				t.Fatalf("clean tree must pass the dirty-worktree guard: %v", err)
			}
			if err := p.checkGuards(guardPolicy(tc.branch)); err != nil {
				t.Fatalf("branch guard for %q must pass: %v", tc.branch, err)
			}
			err := p.checkGuards(guardPolicy("release/*"))
			if err == nil || !strings.Contains(err.Error(), "not in required branches") {
				t.Fatalf("branch guard must block other branches with a branch message, got %v", err)
			}
		})
		t.Run(filepath.Base(tc.dir)+"/tracked dirty", func(t *testing.T) {
			t.Chdir(tc.dir)
			gitrepotest.Write(t, tc.dir, "VERSION", "1.0.1\n")
			defer gitrepotest.Git(t, tc.dir, "checkout", "--", "VERSION")
			err := p.checkGuards(guardPolicy())
			if err == nil || !strings.Contains(err.Error(), "uncommitted changes") {
				t.Fatalf("tracked change must block with the dirty message, got %v", err)
			}
		})
	}
}
