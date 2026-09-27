// Package gitrepo opens Git repositories with go-git in a way that works for
// linked worktrees (`git worktree add`), whose `.git` is a file pointing at
// per-worktree metadata while objects and refs live in the common git dir.
package gitrepo

import (
	"fmt"
	"sort"

	git "github.com/go-git/go-git/v5"
)

// Stage names the step of reading repository state that failed.
type Stage string

const (
	StageOpen     Stage = "open"
	StageWorktree Stage = "worktree"
	StageStatus   Stage = "status"
)

// Error reports a failure to read repository state. It is never a statement
// about whether the working tree is dirty.
type Error struct {
	Stage Stage
	Path  string
	Err   error
}

func (e *Error) Error() string {
	return fmt.Sprintf("git %s failed for %s: %v", e.Stage, e.Path, e.Err)
}

func (e *Error) Unwrap() error { return e.Err }

// Open opens the repository containing path. It walks up to the enclosing
// .git and resolves the common git dir of linked worktrees; without that,
// go-git sees only the per-worktree metadata and reports every tracked file
// as changed (attached worktree) or cannot find HEAD's objects (detached).
func Open(path string) (*git.Repository, error) {
	repo, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{
		DetectDotGit:          true,
		EnableDotGitCommonDir: true,
	})
	if err != nil {
		return nil, &Error{Stage: StageOpen, Path: path, Err: err}
	}
	return repo, nil
}

// OpenAt opens the repository whose .git is exactly at path (no walk-up to
// a parent directory), resolving the common git dir of linked worktrees.
// Use it where callers rely on paths being relative to path itself.
func OpenAt(path string) (*git.Repository, error) {
	repo, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{EnableDotGitCommonDir: true})
	if err != nil {
		return nil, &Error{Stage: StageOpen, Path: path, Err: err}
	}
	return repo, nil
}

// WorktreeStatus returns the working-tree status of the repository
// containing path. Failures are returned as *Error with the failing stage.
func WorktreeStatus(path string) (git.Status, error) {
	repo, err := Open(path)
	if err != nil {
		return nil, err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return nil, &Error{Stage: StageWorktree, Path: path, Err: err}
	}
	st, err := wt.Status()
	if err != nil {
		return nil, &Error{Stage: StageStatus, Path: path, Err: err}
	}
	return st, nil
}

// TrackedChanges lists paths with staged or unstaged changes to tracked
// files. Untracked files are excluded: they do not block releases.
func TrackedChanges(st git.Status) []string {
	var paths []string
	for path, fs := range st {
		if fs.Staging == git.Untracked {
			continue
		}
		if fs.Worktree != git.Unmodified || fs.Staging != git.Unmodified {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths
}
