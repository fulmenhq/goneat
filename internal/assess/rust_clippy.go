package assess

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fulmenhq/goneat/pkg/logger"
)

func runCargoClippyLint(target string, config AssessmentConfig) ([]Issue, error) {
	if !IsCargoAvailable() {
		return nil, nil
	}
	project := DetectRustProject(target)
	if project == nil || project.CargoTomlPath == "" {
		return nil, nil
	}

	presence := CheckRustToolPresence("cargo-clippy", "")
	if !presence.Present {
		logger.Info("cargo-clippy not found; skipping Rust lint")
		return nil, nil
	}

	root := project.EffectiveRoot()
	if root == "" {
		root = target
	}

	if config.Mode == AssessmentModeNoOp {
		logger.Info("[NO-OP] Would run cargo-clippy")
		return nil, nil
	}

	args := []string{"clippy", "--message-format=json"}
	if project.IsWorkspace || project.IsWorkspaceMember {
		args = append(args, "--workspace")
	}

	out, err := runToolStdoutOnly(root, "cargo", args, config.Timeout)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(out)) == 0 {
		return nil, nil
	}

	issues, err := parseCargoClippyOutput(out)
	if err != nil {
		return nil, err
	}

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

	return issues, nil
}

type cargoClippyMessage struct {
	Reason  string                      `json:"reason"`
	Message *cargoClippyCompilerMessage `json:"message"`
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

func parseCargoClippyOutput(out []byte) ([]Issue, error) {
	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	issues := []Issue{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var msg cargoClippyMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.Reason != "compiler-message" || msg.Message == nil {
			continue
		}
		issue, ok := clippyIssueFromMessage(msg.Message)
		if ok {
			issues = append(issues, issue)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse cargo-clippy output: %w", err)
	}
	return issues, nil
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
