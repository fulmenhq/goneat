package assess

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/pkg/logger"
)

func runCargoClippyLint(target string, config AssessmentConfig) ([]Issue, error) {
	settings := clippySettingsFrom(loadAssessOverrides(target))
	if !settings.enabled() {
		logger.Info("cargo-clippy disabled by lint.rust.clippy.enabled; skipping Rust lint")
		return nil, nil
	}
	if !IsCargoAvailable() {
		return nil, nil
	}
	project := DetectRustProject(target)
	if project == nil || project.CargoTomlPath == "" {
		return nil, nil
	}

	invocations, err := settings.invocations(project)
	if err != nil {
		return nil, err
	}

	if settings.Toolchain != "" {
		// An explicitly configured toolchain must exist; never skip or install.
		if !checkClippyWithToolchain(project.EffectiveRootOr(target), settings.Toolchain, config.Timeout) {
			return nil, fmt.Errorf("cargo-clippy is not available for configured toolchain %q (install it with rustup; goneat does not install toolchains)", settings.Toolchain)
		}
	} else if presence := CheckRustToolPresence("cargo-clippy", ""); !presence.Present {
		logger.Info("cargo-clippy not found; skipping Rust lint")
		return nil, nil
	}

	root := project.EffectiveRootOr(target)

	if config.Mode == AssessmentModeNoOp {
		logger.Info("[NO-OP] Would run cargo-clippy")
		return nil, nil
	}

	var issues []Issue
	var runErrs []error
	for _, args := range invocations {
		runIssues, runErr := runClippyInvocation(root, args, config.Timeout)
		issues = append(issues, runIssues...)
		if runErr != nil {
			runErrs = append(runErrs, runErr)
		}
	}
	issues = dedupeClippyIssues(issues)

	if len(config.IncludeFiles) > 0 && hasActualFiles(config.IncludeFiles) {
		issues = filterIssuesToFiles(issues, target, config.IncludeFiles, []string{".rs"})
	}

	if config.NewIssuesOnly {
		base := config.NewIssuesBase
		if base == "" {
			base = "HEAD~"
		}
		issues = filterIssuesByGitBase(issues, root, base)
	}

	return issues, errors.Join(runErrs...)
}

// rustupNoAutoInstallEnv stops rustup from installing a missing toolchain
// named by a "+toolchain" override. Toolchain and target installation is
// owned by bootstrap, never by assess.
var rustupNoAutoInstallEnv = []string{"RUSTUP_AUTO_INSTALL=0"}

func checkClippyWithToolchain(dir, toolchain string, timeout time.Duration) bool {
	run, err := runToolSplitEnv(dir, "cargo", []string{"+" + toolchain, "clippy", "--version"}, timeout, rustupNoAutoInstallEnv)
	return err == nil && run.ExitCode == 0
}

// runClippyInvocation runs one cargo clippy invocation and classifies the
// result. Diagnostics parsed from stdout are always returned. A non-zero
// Cargo exit is always an error: either compilation failed (the error-level
// diagnostics are already in the returned issues) or Cargo failed before
// completing a lint run (manifest, resolution, toolchain, build script), in
// which case the stderr tail is included.
func runClippyInvocation(root string, args []string, timeout time.Duration) ([]Issue, error) {
	command := "cargo " + strings.Join(args, " ")
	run, err := runToolSplitEnv(root, "cargo", args, timeout, rustupNoAutoInstallEnv)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", command, err)
	}

	parsed, perr := parseCargoClippyRun(run.Stdout)
	if perr != nil {
		return nil, fmt.Errorf("%s: %w", command, perr)
	}
	if run.ExitCode == 0 {
		return parsed.issues, nil
	}

	if parsed.buildFinished && parsed.errorCount > 0 {
		msg := fmt.Sprintf("%s exited %d: compilation failed with %d error diagnostic(s)", command, run.ExitCode, parsed.errorCount)
		// A missing target std surfaces as an E0463 diagnostic, not only on stderr.
		if hint := rustTargetHint(string(run.Stderr) + string(run.Stdout)); hint != "" {
			msg += "\n" + hint
		}
		return parsed.issues, errors.New(msg)
	}

	msg := fmt.Sprintf("%s exited %d before completing a lint run", command, run.ExitCode)
	if tail := run.stderrTail(10); tail != "" {
		msg += ":\n" + tail
	}
	if hint := rustTargetHint(string(run.Stderr)); hint != "" {
		msg += "\n" + hint
	}
	return parsed.issues, errors.New(msg)
}

func rustTargetHint(stderr string) string {
	if strings.Contains(stderr, "can't find crate for `core`") || strings.Contains(stderr, "can't find crate for `std`") ||
		strings.Contains(stderr, "target may not be installed") {
		return "hint: the target's standard library is not installed; run `rustup target add <triple>` in bootstrap (goneat does not install targets)"
	}
	return ""
}

func dedupeClippyIssues(issues []Issue) []Issue {
	if len(issues) < 2 {
		return issues
	}
	seen := make(map[string]struct{}, len(issues))
	out := make([]Issue, 0, len(issues))
	for _, issue := range issues {
		key := fmt.Sprintf("%s\x00%d\x00%d\x00%s", issue.File, issue.Line, issue.Column, issue.Message)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, issue)
	}
	return out
}

type cargoClippyMessage struct {
	Reason  string                      `json:"reason"`
	Message *cargoClippyCompilerMessage `json:"message"`
	Success *bool                       `json:"success,omitempty"`
}

type cargoClippyCompilerMessage struct {
	Message string            `json:"message"`
	Level   string            `json:"level"`
	Code    *cargoClippyCode  `json:"code,omitempty"`
	Spans   []cargoClippySpan `json:"spans,omitempty"`
}

type cargoClippyCode struct {
	Code string `json:"code"`
}

type cargoClippySpan struct {
	FileName    string `json:"file_name"`
	LineStart   int    `json:"line_start"`
	ColumnStart int    `json:"column_start"`
	IsPrimary   bool   `json:"is_primary"`
}

type cargoClippyRun struct {
	issues        []Issue
	errorCount    int
	buildFinished bool
	buildSuccess  bool
}

func parseCargoClippyOutput(out []byte) ([]Issue, error) {
	run, err := parseCargoClippyRun(out)
	if err != nil {
		return nil, err
	}
	return run.issues, nil
}

func parseCargoClippyRun(out []byte) (cargoClippyRun, error) {
	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	run := cargoClippyRun{issues: []Issue{}}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var msg cargoClippyMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.Reason == "build-finished" {
			run.buildFinished = true
			run.buildSuccess = msg.Success != nil && *msg.Success
			continue
		}
		if msg.Reason != "compiler-message" || msg.Message == nil {
			continue
		}
		issue, ok := clippyIssueFromMessage(msg.Message)
		if ok {
			if issue.Severity == SeverityHigh {
				run.errorCount++
			}
			run.issues = append(run.issues, issue)
		}
	}
	if err := scanner.Err(); err != nil {
		return cargoClippyRun{}, fmt.Errorf("failed to parse cargo-clippy output: %w", err)
	}
	return run, nil
}

func clippyIssueFromMessage(msg *cargoClippyCompilerMessage) (Issue, bool) {
	if msg == nil {
		return Issue{}, false
	}

	sev, ok := mapClippySeverity(msg.Level)
	if !ok {
		return Issue{}, false
	}

	span, hasSpan := pickClippySpan(msg.Spans)
	file := ""
	line := 0
	col := 0
	if hasSpan {
		file = filepath.ToSlash(span.FileName)
		line = span.LineStart
		col = span.ColumnStart
	}

	text := strings.TrimSpace(msg.Message)
	if msg.Code != nil {
		code := strings.TrimSpace(msg.Code.Code)
		if code != "" {
			text = fmt.Sprintf("%s: %s", code, text)
		}
	}
	if text == "" {
		text = "clippy finding"
	}

	return Issue{
		File:        file,
		Line:        line,
		Column:      col,
		Severity:    sev,
		Message:     text,
		Category:    CategoryLint,
		SubCategory: "rust:clippy",
		AutoFixable: false,
	}, true
}

func mapClippySeverity(level string) (IssueSeverity, bool) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "warning":
		return SeverityMedium, true
	case "error":
		return SeverityHigh, true
	default:
		return "", false
	}
}

func pickClippySpan(spans []cargoClippySpan) (cargoClippySpan, bool) {
	for _, span := range spans {
		if span.IsPrimary {
			return span, true
		}
	}
	if len(spans) > 0 {
		return spans[0], true
	}
	return cargoClippySpan{}, false
}

func filterIssuesToFiles(issues []Issue, baseDir string, files []string, exts []string) []Issue {
	filtered := filterByExtensions(files, exts)
	if len(filtered) == 0 {
		return nil
	}

	fileSet := make(map[string]struct{}, len(filtered))
	for _, f := range filtered {
		path := f
		if !filepath.IsAbs(path) {
			path = filepath.Join(baseDir, path)
		}
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
		fileSet[path] = struct{}{}
	}

	out := make([]Issue, 0, len(issues))
	for _, issue := range issues {
		if issue.File == "" {
			continue
		}
		path := issue.File
		if !filepath.IsAbs(path) {
			path = filepath.Join(baseDir, path)
		}
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
		if _, ok := fileSet[path]; ok {
			out = append(out, issue)
		}
	}
	return out
}

func filterIssuesByGitBase(issues []Issue, workDir, base string) []Issue {
	if len(issues) == 0 {
		return issues
	}
	changed, err := gitChangedFilesSince(workDir, base)
	if err != nil {
		logger.Warn(fmt.Sprintf("cargo-clippy incremental filtering disabled: %v", err))
		return issues
	}
	if len(changed) == 0 {
		return nil
	}

	out := make([]Issue, 0, len(issues))
	for _, issue := range issues {
		if issue.File == "" {
			continue
		}
		path := issue.File
		if !filepath.IsAbs(path) {
			path = filepath.Join(workDir, path)
		}
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
		if _, ok := changed[path]; ok {
			out = append(out, issue)
		}
	}
	return out
}

func gitChangedFilesSince(dir, base string) (map[string]struct{}, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, fmt.Errorf("git not available")
	}

	rootOut, err := gitCommandOutput(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("git toplevel lookup failed: %w", err)
	}
	root := strings.TrimSpace(string(rootOut))
	if root == "" {
		return nil, fmt.Errorf("git toplevel not found")
	}

	diffOut, err := gitCommandOutput(dir, "diff", "--name-only", base, "--")
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w", err)
	}

	files := make(map[string]struct{})
	scanner := bufio.NewScanner(bytes.NewReader(diffOut))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(line))
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
		files[path] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("git diff parse failed: %w", err)
	}
	return files, nil
}

func gitCommandOutput(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...) // #nosec G204 - internal git wrapper; args are git subcommands constructed by the clippy assessment runner, not user-controlled input
	cmd.Dir = dir
	return cmd.Output()
}
