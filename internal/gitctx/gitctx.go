package gitctx

import (
	"bufio"
	"bytes"
	git "github.com/go-git/go-git/v5"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ChangeContext captures a minimal view of the current git change-set.
type ChangeContext struct {
	ModifiedFiles []string `json:"modified_files"`
	TotalChanges  int      `json:"total_changes"`
	ChangeScope   string   `json:"change_scope"` // small | medium | large
	GitSHA        string   `json:"git_sha,omitempty"`
	Branch        string   `json:"branch,omitempty"`
}

// Collect gathers change context for the repo at target path. Returns nil if git is unavailable or not a repo.
func Collect(target string) (*ChangeContext, map[string]struct{}, error) {
	// Prefer go-git for repo info and file lists
	if ctx, files, err := collectGoGit(target); err == nil && ctx != nil {
		// If CLI is available, enrich with total changes for finer scope classification
		if _, err := exec.LookPath("git"); err == nil {
			total := 0
			a1, _ := parseNumstat(runGitBytes(target, "diff", "--numstat"))
			a2, _ := parseNumstat(runGitBytes(target, "diff", "--cached", "--numstat"))
			total = a1 + a2
			ctx.TotalChanges = total
			if total > 0 {
				ctx.ChangeScope = classifyScope(total)
			} else {
				// fallback scope by file count
				ctx.ChangeScope = classifyByFileCount(len(files))
			}
		}
		return ctx, files, nil
	}

	// CLI fallback
	if _, err := exec.LookPath("git"); err != nil {
		return nil, nil, nil
	}
	if !isRepoCLI(target) {
		return nil, nil, nil
	}
	branch := runGit(target, "rev-parse", "--abbrev-ref", "HEAD")
	sha := runGit(target, "rev-parse", "HEAD")
	filesSet := make(map[string]struct{})
	total := 0
	a1, files1 := parseNumstat(runGitBytes(target, "diff", "--numstat"))
	total += a1
	for f := range files1 {
		filesSet[f] = struct{}{}
	}
	a2, files2 := parseNumstat(runGitBytes(target, "diff", "--cached", "--numstat"))
	total += a2
	for f := range files2 {
		filesSet[f] = struct{}{}
	}
	var modified []string
	for f := range filesSet {
		modified = append(modified, filepath.ToSlash(f))
	}
	ctx := &ChangeContext{
		ModifiedFiles: modified,
		TotalChanges:  total,
		ChangeScope:   classifyScope(total),
		GitSHA:        sha,
		Branch:        branch,
	}
	return ctx, filesSet, nil
}

func collectGoGit(target string) (*ChangeContext, map[string]struct{}, error) {
	repo, err := git.PlainOpenWithOptions(target, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, nil, nil
	}
	head, err := repo.Head()
	if err != nil {
		return nil, nil, nil
	}
	branch := head.Name().Short()
	sha := head.Hash().String()

	wt, err := repo.Worktree()
	if err != nil {
		return nil, nil, nil
	}
	st, err := wt.Status()
	if err != nil {
		return nil, nil, nil
	}
	files := make(map[string]struct{})
	for path, s := range st {
		// Consider both staged and unstaged changes
		if s.Staging != git.Unmodified || s.Worktree != git.Unmodified {
			files[filepath.ToSlash(path)] = struct{}{}
		}
	}
	var modified []string
	for f := range files {
		modified = append(modified, f)
	}
	ctx := &ChangeContext{
		ModifiedFiles: modified,
		TotalChanges:  0, // enriched via CLI if available
		ChangeScope:   classifyByFileCount(len(files)),
		GitSHA:        sha,
		Branch:        branch,
	}
	return ctx, files, nil
}

// CollectWithLines returns change context along with a map of file -> added line numbers (best-effort),
// combining staged and unstaged diffs. Returns nil ctx if git is unavailable or not a repo.
func CollectWithLines(target string) (*ChangeContext, map[string]struct{}, map[string][]int, error) {
	ctx, files, err := Collect(target)
	if ctx == nil || err != nil {
		return ctx, files, nil, err
	}
	lines := make(map[string][]int)
	// Unstaged added lines
	parseUnifiedInto(lines, runGitBytes(target, "diff", "--unified=0"))
	// Staged added lines
	parseUnifiedInto(lines, runGitBytes(target, "diff", "--cached", "--unified=0"))
	return ctx, files, lines, nil
}

// parseUnifiedInto parses unified diff output and appends added line numbers into dst keyed by file path.
func parseUnifiedInto(dst map[string][]int, data []byte) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var file string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "+++ ") {
			// Format: +++ b/<path> or +++ /dev/null
			if strings.HasPrefix(line, "+++ b/") {
				file = strings.TrimPrefix(line, "+++ b/")
			} else {
				file = ""
			}
			continue
		}
		if !strings.HasPrefix(line, "@@") || file == "" {
			continue
		}
		// Hunk header: @@ -a,b +c,d @@
		// Extract +c,d part
		start := strings.Index(line, "+")
		if start == -1 {
			continue
		}
		// Take substring after + up to space or @@ end
		rest := line[start+1:]
		// rest like: c,d @@ ... or c @@ ...
		// Split by space first
		spaceIdx := strings.IndexByte(rest, ' ')
		if spaceIdx != -1 {
			rest = rest[:spaceIdx]
		}
		// Now rest is c or c,d
		var c, d int
		if strings.Contains(rest, ",") {
			parts := strings.SplitN(rest, ",", 2)
			c = atoiSafe(parts[0])
			d = atoiSafe(parts[1])
		} else {
			c = atoiSafe(rest)
			d = 1
		}
		if d <= 0 {
			continue
		}
		// Append line numbers c..c+d-1
		for ln := c; ln < c+d; ln++ {
			dst[file] = append(dst[file], ln)
		}
	}
}

func isRepoCLI(target string) bool {
	out := runGit(target, "rev-parse", "--is-inside-work-tree")
	return strings.TrimSpace(out) == "true"
}

func runGit(dir string, args ...string) string {
	b := runGitBytes(dir, args...)
	return strings.TrimSpace(string(b))
}

func runGitBytes(dir string, args ...string) []byte {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, _ := cmd.Output()
	return out
}

// parseNumstat parses `git diff --numstat`-style output, returning total changes and a set of files.
func parseNumstat(data []byte) (int, map[string]struct{}) {
	total := 0
	files := make(map[string]struct{})
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		// Format: <added>\t<deleted>\t<file>
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		a := atoiSafe(parts[0])
		d := atoiSafe(parts[1])
		total += a + d
		f := strings.TrimSpace(parts[2])
		if f != "" {
			files[f] = struct{}{}
		}
	}
	return total, files
}

func atoiSafe(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func classifyScope(total int) string {
	switch {
	case total <= 50:
		return "small"
	case total <= 200:
		return "medium"
	default:
		return "large"
	}
}

func classifyByFileCount(n int) string {
	switch {
	case n <= 5:
		return "small"
	case n <= 20:
		return "medium"
	default:
		return "large"
	}
}
