package assess

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/fulmenhq/goneat/pkg/logger"
	"gopkg.in/yaml.v3"
)

type assessOverrides struct {
	Version int            `yaml:"version"`
	Lint    *lintOverrides `yaml:"lint"`
}

type lintOverrides struct {
	Yamllint      *yamllintOverrides   `yaml:"yamllint"`
	Shell         *shellOverrides      `yaml:"shell"`
	GitHubActions *githubActionsConfig `yaml:"github_actions"`
	Make          *makeOverrides       `yaml:"make"`
}

type yamllintOverrides struct {
	Enabled *bool    `yaml:"enabled"`
	Strict  *bool    `yaml:"strict"`
	Paths   []string `yaml:"paths"`
	Ignore  []string `yaml:"ignore"`
}

type shellOverrides struct {
	Paths      []string          `yaml:"paths"`
	Ignore     []string          `yaml:"ignore"`
	Shfmt      *shfmtOverrides   `yaml:"shfmt"`
	Shellcheck *shellcheckConfig `yaml:"shellcheck"`
}

type shfmtOverrides struct {
	Enabled *bool    `yaml:"enabled"`
	Fix     *bool    `yaml:"fix"`
	Paths   []string `yaml:"paths"`
	Ignore  []string `yaml:"ignore"`
}

type shellcheckConfig struct {
	Enabled *bool    `yaml:"enabled"`
	Path    string   `yaml:"path"`
	Paths   []string `yaml:"paths"`
	Ignore  []string `yaml:"ignore"`
}

type githubActionsConfig struct {
	Actionlint *actionlintOverrides `yaml:"actionlint"`
}

type actionlintOverrides struct {
	Enabled *bool    `yaml:"enabled"`
	Paths   []string `yaml:"paths"`
	Ignore  []string `yaml:"ignore"`
}

type makeOverrides struct {
	Checkmake *checkmakeOverrides `yaml:"checkmake"`
	Paths     []string            `yaml:"paths"`
	Ignore    []string            `yaml:"ignore"`
}

type checkmakeOverrides struct {
	Enabled *bool `yaml:"enabled"`
}

var assessConfigCache sync.Map

var defaultYamllintPatterns = []string{
	".github/workflows/**/*.yml",
	".github/workflows/**/*.yaml",
}

var yamllintLinePattern = regexp.MustCompile(`^([^:]+):(\d+):(\d+):\s+\[([^\]]+)\]\s+(.*)$`)

func loadAssessOverrides(target string) *assessOverrides {
	absTarget, err := filepath.Abs(target)
	if err != nil {
		absTarget = target
	}
	if cached, ok := assessConfigCache.Load(absTarget); ok {
		if cached == nil {
			return nil
		}
		return cached.(*assessOverrides)
	}

	configPath := filepath.Join(absTarget, ".goneat", "assess.yaml")
	// #nosec G304 -- configPath is repo-rooted (.goneat/assess.yaml) and cleaned above
	data, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn(fmt.Sprintf("Failed to read %s: %v", configPath, err))
		}
		assessConfigCache.Store(absTarget, nil)
		return nil
	}

	var overrides assessOverrides
	if err := yaml.Unmarshal(data, &overrides); err != nil {
		logger.Warn(fmt.Sprintf("Failed to parse %s: %v", configPath, err))
		assessConfigCache.Store(absTarget, nil)
		return nil
	}
	if overrides.Version == 0 {
		overrides.Version = 1
	}
	assessConfigCache.Store(absTarget, &overrides)
	return &overrides
}

func (o *lintOverrides) yamllintConfig() *yamllintOverrides {
	if o == nil {
		return nil
	}
	return o.Yamllint
}

func (o *yamllintOverrides) enabled() bool {
	if o == nil || o.Enabled == nil {
		return true
	}
	return *o.Enabled
}

func (o *yamllintOverrides) strictEnabled() bool {
	if o == nil || o.Strict == nil {
		return true
	}
	return *o.Strict
}

func (o *yamllintOverrides) ignorePatterns() []string {
	if o == nil {
		return nil
	}
	return o.Ignore
}

func boolWithDefault(val *bool, def bool) bool {
	if val == nil {
		return def
	}
	return *val
}

func (r *LintAssessmentRunner) runYamllintAssessment(target string, overrides *assessOverrides) ([]Issue, error) {
	yamllintBin := os.Getenv("GONEAT_YAMLLINT_BIN")
	if yamllintBin == "" {
		yamllintBin = "yamllint"
	}
	yamllintBin = filepath.Clean(yamllintBin)
	useYamlfmtFallback := false
	if _, err := exec.LookPath(yamllintBin); err != nil {
		logger.Info("yamllint not found in PATH; falling back to yamlfmt --lint")
		if _, err := exec.LookPath("yamlfmt"); err != nil {
			logger.Info("yamlfmt also not found; skipping YAML lint stage")
			return nil, nil
		}
		useYamlfmtFallback = true
	}
	var yamllintCfg *yamllintOverrides
	if overrides != nil && overrides.Lint != nil {
		yamllintCfg = overrides.Lint.yamllintConfig()
	}
	if yamllintCfg != nil && !yamllintCfg.enabled() {
		return nil, nil
	}

	files, err := resolveYamllintTargets(target, yamllintCfg)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		logger.Info("No YAML workflow files detected for yamllint")
		return nil, nil
	}

	if useYamlfmtFallback {
		return runYamlfmtFallback(target, files)
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.config.Timeout)
	defer cancel()

	args := []string{"--format", "parsable"}
	if yamllintCfg == nil || yamllintCfg.strictEnabled() {
		args = append(args, "--strict")
	}
	args = append(args, files...)

	// #nosec G204 -- yamllintBin is either the default "yamllint" or a cleaned,
	// repo-configured override validated via LookPath above.
	cmd := exec.CommandContext(ctx, yamllintBin, args...)
	cmd.Dir = target
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 1 {
				return nil, fmt.Errorf("yamllint failed: %v\n%s", err, string(output))
			}
		} else {
			return nil, fmt.Errorf("yamllint execution failed: %v", err)
		}
	}

	issues := parseYamllintOutput(string(output), target)
	if len(issues) > 0 {
		logger.Info(fmt.Sprintf("yamllint completed: %d issues", len(issues)))
	}
	return issues, nil
}

func resolveYamllintTargets(root string, cfg *yamllintOverrides) ([]string, error) {
	patterns := defaultYamllintPatterns
	if cfg != nil && len(cfg.Paths) > 0 {
		patterns = cfg.Paths
	}
	ignore := cfg.ignorePatterns()

	fileSet := make(map[string]struct{})
	for _, pattern := range patterns {
		absPattern := filepath.Join(root, filepath.FromSlash(pattern))
		matches, err := doublestar.FilepathGlob(absPattern)
		if err != nil {
			return nil, fmt.Errorf("invalid yamllint pattern %q: %w", pattern, err)
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil || info.IsDir() {
				continue
			}
			rel, err := filepath.Rel(root, match)
			if err != nil {
				continue
			}
			rel = filepath.ToSlash(rel)
			if shouldIgnoreYAML(rel, ignore) {
				continue
			}
			fileSet[rel] = struct{}{}
		}
	}

	var files []string
	for rel := range fileSet {
		files = append(files, rel)
	}
	sort.Strings(files)
	return files, nil
}

func shouldIgnoreYAML(rel string, ignore []string) bool {
	if len(ignore) == 0 {
		return false
	}
	for _, pattern := range ignore {
		if pattern == "" {
			continue
		}
		pattern = filepath.ToSlash(pattern)
		if matched, err := doublestar.Match(pattern, rel); err == nil && matched {
			return true
		}
	}
	return false
}

func parseYamllintOutput(output string, target string) []Issue {
	if strings.TrimSpace(output) == "" {
		return nil
	}
	lines := strings.Split(output, "\n")
	var issues []Issue
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		match := yamllintLinePattern.FindStringSubmatch(line)
		if len(match) != 6 {
			continue
		}
		file := filepath.Clean(match[1])
		lineNum := parseInt(match[2])
		colNum := parseInt(match[3])
		level := strings.ToLower(strings.TrimSpace(match[4]))
		message := strings.TrimSpace(match[5])
		relPath := normalizeRelativePath(file, target)
		issues = append(issues, Issue{
			File:          relPath,
			Line:          lineNum,
			Column:        colNum,
			Severity:      yamllintSeverity(level),
			Message:       fmt.Sprintf("yamllint: %s", message),
			Category:      CategoryLint,
			SubCategory:   "yaml-lint",
			AutoFixable:   false,
			EstimatedTime: HumanReadableDuration(2 * time.Minute),
		})
	}
	return issues
}

func normalizeRelativePath(path string, root string) string {
	if filepath.IsAbs(path) {
		if rel, err := filepath.Rel(root, path); err == nil {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(path)
}

func yamllintSeverity(level string) IssueSeverity {
	switch level {
	case "error":
		return SeverityHigh
	case "warning":
		return SeverityMedium
	default:
		return SeverityLow
	}
}

func runYamlfmtFallback(target string, files []string) ([]Issue, error) {
	args := []string{"--lint"}
	args = append(args, files...)
	cmd := exec.Command("yamlfmt", args...)
	cmd.Dir = target
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 3 {
		return parseYamlfmtLintOutput(string(output), target), nil
	}
	return nil, fmt.Errorf("yamlfmt lint failed: %v\n%s", err, string(output))
}

func parseYamlfmtLintOutput(output, root string) []Issue {
	if strings.TrimSpace(output) == "" {
		return nil
	}
	var issues []Issue
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		file := filepath.ToSlash(strings.TrimSpace(parts[0]))
		msg := strings.TrimSpace(parts[1])
		issues = append(issues, Issue{
			File:        normalizeRelativePath(file, root),
			Severity:    SeverityMedium,
			Message:     fmt.Sprintf("yamlfmt lint: %s", msg),
			Category:    CategoryLint,
			SubCategory: "yaml-lint",
		})
	}
	return issues
}

func parseInt(value string) int {
	v, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return v
}
