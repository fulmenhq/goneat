package assess

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/fulmenhq/goneat/pkg/logger"
)

func TestParseGosecOutput_WellFormed(t *testing.T) {
	runner := NewSecurityAssessmentRunner()
	sample := map[string]interface{}{
		"Issues": []map[string]interface{}{
			{
				"severity": "HIGH",
				"details":  "hardcoded credentials",
				"file":     "internal/app/a.go",
				"line":     42,
				"rule_id":  "G101",
			},
			{
				"severity": "low",
				"details":  "minor issue",
				"file":     "pkg/b.go",
				"line":     "13",
				"rule_id":  "G999",
			},
		},
	}
	data, _ := json.Marshal(sample)
	issues, err := runner.parseGosecOutput(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}
	if issues[0].Severity != SeverityHigh {
		t.Fatalf("expected first severity high, got %v", issues[0].Severity)
	}
	if issues[0].Line != 42 {
		t.Fatalf("expected first line 42, got %d", issues[0].Line)
	}
	if issues[1].Severity != SeverityLow {
		t.Fatalf("expected second severity low, got %v", issues[1].Severity)
	}
	if issues[1].Line != 13 {
		t.Fatalf("expected second line 13 parsed from string, got %d", issues[1].Line)
	}
}

func TestParseGosecOutput_Noisy(t *testing.T) {
	runner := NewSecurityAssessmentRunner()
	noisy := []byte("gosec starting...\nWARNING something\n{\n  \"Issues\": [ { \"severity\": \"medium\", \"details\": \"x\", \"file\": \"x.go\", \"line\": 7, \"rule_id\": \"G000\" } ]\n}\ntrailer text\n")
	issues, err := runner.parseGosecOutput(noisy)
	if err != nil {
		t.Fatalf("unexpected error parsing noisy output: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Severity != SeverityMedium {
		t.Fatalf("expected severity medium, got %v", issues[0].Severity)
	}
}

func TestParseGosecOutput_IgnoresBraceStatsNoise(t *testing.T) {
	runner := NewSecurityAssessmentRunner()
	noisy := []byte("stats: {0 packages, 0 issues}\n{\n  \"Issues\": [ { \"severity\": \"low\", \"details\": \"x\", \"file\": \"x.go\", \"line\": 1, \"rule_id\": \"G000\" } ]\n}\n")
	issues, err := runner.parseGosecOutput(noisy)
	if err != nil {
		t.Fatalf("unexpected error parsing brace-stats noise: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
}

func TestUniqueDirs(t *testing.T) {
	runner := NewSecurityAssessmentRunner()
	files := []string{
		"a/b/c.go",
		"a/b/d.go",
		"x/y/z.go",
	}
	dirs := runner.uniqueDirs(files)
	if len(dirs) != 2 {
		t.Fatalf("expected 2 unique dirs, got %d (%v)", len(dirs), dirs)
	}

	// Empty input falls back to ./...
	dirs2 := runner.uniqueDirs(nil)
	if len(dirs2) != 1 || dirs2[0] != "./..." {
		t.Fatalf("expected fallback ./..., got %v", dirs2)
	}
}

func TestParseIgnorePatternsForGosec_ConvertsAndLogsSkips(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "goneat-gosec-ignore-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	gitignore := strings.Join([]string{
		"*.egg-info/",
		"!vendor/",
		"**/dist/",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(tempDir, ".gitignore"), []byte(gitignore), 0600); err != nil {
		t.Fatalf("failed to write .gitignore: %v", err)
	}

	if err := logger.Initialize(logger.Config{Level: logger.DebugLevel, Component: "goneat"}); err != nil {
		t.Fatalf("failed to init logger: %v", err)
	}
	var logBuf bytes.Buffer
	logger.SetOutput(&logBuf)
	defer logger.SetOutput(io.Discard)

	excludes := parseIgnorePatternsForGosec(tempDir)

	if !sliceContainsString(excludes, `[^/]*\.egg-info`) {
		t.Fatalf("expected converted egg-info regex in excludes, got %v", excludes)
	}
	if !sliceContainsString(excludes, `(.*/)?dist`) {
		t.Fatalf("expected converted doublestar dist regex in excludes, got %v", excludes)
	}
	if !sliceContainsString(excludes, "vendor") {
		t.Fatalf("expected default vendor exclude present, got %v", excludes)
	}

	logs := logBuf.String()
	if !strings.Contains(logs, "gosec exclude pattern skipped:") {
		t.Fatalf("expected skip debug log, got logs: %s", logs)
	}
	if !strings.Contains(logs, "reason=negation_not_supported") {
		t.Fatalf("expected negation reason in logs, got logs: %s", logs)
	}
	if !strings.Contains(logs, "gosec exclude conversion completed with skips:") {
		t.Fatalf("expected summary warn log, got logs: %s", logs)
	}
}

func TestFindModuleDirsHonorsGitignoreAndNoIgnore(t *testing.T) {
	runner := NewSecurityAssessmentRunner()
	repo := t.TempDir()

	writeTestFile(t, filepath.Join(repo, "go.mod"), "module example.com/root\n\ngo 1.23\n")
	writeTestFile(t, filepath.Join(repo, ".gitignore"), ".cache/\n")
	cachedModule := filepath.Join(repo, ".cache", "go-mod", "pkg", "mod", "example.com", "dep")
	writeTestFile(t, filepath.Join(cachedModule, "go.mod"), "module example.com/dep\n\ngo 1.23\n")

	config := DefaultAssessmentConfig()
	dirs, err := runner.findModuleDirs(repo, config)
	if err != nil {
		t.Fatalf("findModuleDirs failed: %v", err)
	}
	if !containsCleanPath(dirs, repo) {
		t.Fatalf("expected root module in dirs, got %v", dirs)
	}
	if containsCleanPath(dirs, cachedModule) {
		t.Fatalf("expected gitignored cached module to be pruned, got %v", dirs)
	}

	config.NoIgnore = true
	dirs, err = runner.findModuleDirs(repo, config)
	if err != nil {
		t.Fatalf("findModuleDirs with NoIgnore failed: %v", err)
	}
	if !containsCleanPath(dirs, cachedModule) {
		t.Fatalf("expected --no-ignore to include cached module, got %v", dirs)
	}
}

func TestListGoPackageDirsFiltersIgnoredGeneratedPackages(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not available")
	}

	runner := NewSecurityAssessmentRunner()
	repo := t.TempDir()
	writeTestFile(t, filepath.Join(repo, "go.mod"), "module example.com/root\n\ngo 1.23\n")
	writeTestFile(t, filepath.Join(repo, ".gitignore"), "generated/\n")
	writeTestFile(t, filepath.Join(repo, "pkg", "pkg.go"), "package pkg\n")
	writeTestFile(t, filepath.Join(repo, "generated", "gen.go"), "package generated\n")

	config := DefaultAssessmentConfig()
	dirs, err := runner.listGoPackageDirs(repo, repo, config)
	if err != nil {
		t.Fatalf("listGoPackageDirs failed: %v", err)
	}
	if !containsCleanPath(dirs, filepath.Join(repo, "pkg")) {
		t.Fatalf("expected normal package dir in result, got %v", dirs)
	}
	if containsCleanPath(dirs, filepath.Join(repo, "generated")) {
		t.Fatalf("expected gitignored generated package to be filtered, got %v", dirs)
	}

	config.NoIgnore = true
	dirs, err = runner.listGoPackageDirs(repo, repo, config)
	if err != nil {
		t.Fatalf("listGoPackageDirs with NoIgnore failed: %v", err)
	}
	if !containsCleanPath(dirs, filepath.Join(repo, "generated")) {
		t.Fatalf("expected --no-ignore to include generated package, got %v", dirs)
	}
}

func TestRunGosecFallsBackWhenPackageDiscoveryFails(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not available")
	}
	if runtime.GOOS == "windows" {
		t.Skip("shell-based fake gosec is not supported on Windows")
	}

	runner := NewSecurityAssessmentRunner()
	repo := t.TempDir()
	writeTestFile(t, filepath.Join(repo, "go.mod"), "module example.com/root\n\ngo 1.23\nrequire (\n")

	binDir := filepath.Join(repo, "bin")
	capturePath := filepath.Join(repo, "gosec-args.txt")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + shellQuote(capturePath) + "\n" +
		"printf '{\"Issues\":[]}'\n"
	writeExecutableTestFile(t, filepath.Join(binDir, "gosec"), script)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	config := DefaultAssessmentConfig()
	config.Concurrency = 1
	_, _, err := runner.runGosec(context.Background(), repo, config)
	if err != nil {
		t.Fatalf("runGosec returned error: %v", err)
	}

	args, err := os.ReadFile(capturePath)
	if err != nil {
		t.Fatalf("expected fallback gosec invocation, failed to read captured args: %v", err)
	}
	if !strings.Contains(string(args), "./...") {
		t.Fatalf("expected fallback gosec invocation to include ./..., got args:\n%s", string(args))
	}
}

func TestForceIncludeDescendsFromDoesNotReopenAllIgnoredDirsForBroadGlobs(t *testing.T) {
	tests := []struct {
		name     string
		rel      string
		patterns []string
		want     bool
	}{
		{
			name:     "broad recursive go glob does not descend into cache",
			rel:      ".cache",
			patterns: []string{"**/*.go"},
			want:     false,
		},
		{
			name:     "specific ignored recursive glob descends into matching dir",
			rel:      "ignored",
			patterns: []string{"ignored/**/*.go"},
			want:     true,
		},
		{
			name:     "specific ignored glob does not descend into unrelated cache",
			rel:      ".cache",
			patterns: []string{"ignored/**/*.go"},
			want:     false,
		},
		{
			name:     "exact forced descendant descends into cache",
			rel:      ".cache",
			patterns: []string{".cache/go-mod/pkg/mod/example.com/dep/main.go"},
			want:     true,
		},
		{
			name:     "directory recursive force include descends into cache",
			rel:      ".cache",
			patterns: []string{".cache/**"},
			want:     true,
		},
		{
			name:     "file glob under concrete dir descends into that dir",
			rel:      "generated",
			patterns: []string{"generated/*.go"},
			want:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := forceIncludeDescendsFrom(tc.rel, tc.patterns); got != tc.want {
				t.Fatalf("forceIncludeDescendsFrom(%q, %v) = %v, want %v", tc.rel, tc.patterns, got, tc.want)
			}
		})
	}
}

func TestPathWithinAssessmentRoot(t *testing.T) {
	root := filepath.Join(string(os.PathSeparator), "repo")

	tests := []struct {
		name      string
		candidate string
		want      bool
	}{
		{name: "empty path", candidate: "", want: false},
		{name: "relative file in repo", candidate: filepath.Join("internal", "assess", "security_runner.go"), want: true},
		{name: "absolute file in repo", candidate: filepath.Join(root, "internal", "assess", "security_runner.go"), want: true},
		{name: "cache artifact absolute", candidate: filepath.Join(string(os.PathSeparator), "tmp", "sysprims-gocache-release", "foo.go"), want: false},
		{name: "default go cache absolute", candidate: filepath.Join(string(os.PathSeparator), "Users", "davethompson", "Library", "Caches", "go-build", "ab", "foo.go"), want: false},
		{name: "relative path escape", candidate: filepath.Join("..", "..", "tmp", "foo.go"), want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := pathWithinAssessmentRoot(root, tc.candidate); got != tc.want {
				t.Fatalf("pathWithinAssessmentRoot(%q, %q) = %v, want %v", root, tc.candidate, got, tc.want)
			}
		})
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeExecutableTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}

func shellQuote(path string) string {
	return "'" + strings.ReplaceAll(path, "'", "'\\''") + "'"
}

func containsCleanPath(paths []string, want string) bool {
	want = filepath.Clean(want)
	for _, path := range paths {
		if filepath.Clean(path) == want {
			return true
		}
	}
	return false
}

func TestFilterToAssessmentRoot_DropsExternalSecurityFindings(t *testing.T) {
	runner := NewSecurityAssessmentRunner()
	repoRoot := t.TempDir()
	repoFile := filepath.Join(repoRoot, "internal", "assess", "security_runner.go")
	cacheFile := filepath.Join(string(os.PathSeparator), "tmp", "sysprims-gocache-release", "123", "cache.go")

	issues := []Issue{
		{File: repoFile, Severity: SeverityHigh, Message: "gosec(G115): repo issue", Category: CategorySecurity, SubCategory: "code"},
		{File: cacheFile, Severity: SeverityHigh, Message: "gosec(G115): cache issue", Category: CategorySecurity, SubCategory: "code"},
	}
	suppressions := []Suppression{
		{Tool: "gosec", File: repoFile, RuleID: "G115"},
		{Tool: "gosec", File: cacheFile, RuleID: "G115"},
	}

	filteredIssues, filteredSuppressions := runner.filterToAssessmentRoot(repoRoot, issues, suppressions)

	if len(filteredIssues) != 1 {
		t.Fatalf("expected 1 in-scope issue, got %d: %+v", len(filteredIssues), filteredIssues)
	}
	if filteredIssues[0].File != repoFile {
		t.Fatalf("expected repo-local issue to remain, got %+v", filteredIssues[0])
	}
	if len(filteredSuppressions) != 1 {
		t.Fatalf("expected 1 in-scope suppression, got %d: %+v", len(filteredSuppressions), filteredSuppressions)
	}
	if filteredSuppressions[0].File != repoFile {
		t.Fatalf("expected repo-local suppression to remain, got %+v", filteredSuppressions[0])
	}
}

func TestFilterToAssessmentRoot_DropsUnlocatableFindings(t *testing.T) {
	runner := NewSecurityAssessmentRunner()
	repoRoot := t.TempDir()

	issues := []Issue{{File: "", Severity: SeverityHigh, Message: "gosec(G115): missing file", Category: CategorySecurity, SubCategory: "code"}}
	suppressions := []Suppression{{Tool: "gosec", File: "", RuleID: "G115"}}

	filteredIssues, filteredSuppressions := runner.filterToAssessmentRoot(repoRoot, issues, suppressions)

	if len(filteredIssues) != 0 {
		t.Fatalf("expected unlocatable issues to be dropped, got %+v", filteredIssues)
	}
	if len(filteredSuppressions) != 0 {
		t.Fatalf("expected unlocatable suppressions to be dropped, got %+v", filteredSuppressions)
	}
}
