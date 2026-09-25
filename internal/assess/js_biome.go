package assess

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/fulmenhq/goneat/pkg/work"
)

// biomeV2Report matches biome v2.x JSON output format
type biomeV2Report struct {
	Summary struct {
		Errors   int `json:"errors"`
		Warnings int `json:"warnings"`
	} `json:"summary"`
	Diagnostics []biomeV2Diagnostic `json:"diagnostics"`
}

type biomeV2Diagnostic struct {
	Category string `json:"category"`
	Severity string `json:"severity"`
	// biome v2.4+ uses "message"; older versions used "description".
	Message     string `json:"message"`
	Description string `json:"description"`
	Location    struct {
		Path biomePath `json:"path"`
		Span []int     `json:"span"` // older biome used [start,end] byte offsets
	} `json:"location"`
}

// biomePath supports both biome JSON shapes:
// - biome <= 2.3: { "path": { "file": "src/file.ts" } }
// - biome >= 2.4: { "path": "src/file.ts" }
type biomePath struct {
	File string
}

func (p *biomePath) UnmarshalJSON(b []byte) error {
	// String form
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		p.File = s
		return nil
	}
	// Object form
	var o struct {
		File string `json:"file"`
	}
	if err := json.Unmarshal(b, &o); err != nil {
		return err
	}
	p.File = o.File
	return nil
}

func parseBiomeReport(out []byte) (biomeV2Report, error) {
	var report biomeV2Report
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		return report, nil
	}
	start := bytes.IndexByte(trimmed, '{')
	end := bytes.LastIndexByte(trimmed, '}')
	if start == -1 || end == -1 || end < start {
		return report, fmt.Errorf("no json output from biome\nbiome output:\n%s", string(trimmed))
	}
	trimmed = trimmed[start : end+1]
	if uerr := json.Unmarshal(trimmed, &report); uerr != nil {
		return report, uerr
	}
	return report, nil
}

// biomeReportFromRun parses a biome --reporter json run and fails closed when
// the run did not check what it was given:
//   - any internalError diagnostic (unreadable or missing file, I/O) fails,
//     whatever the exit code: biome can exit 0 while skipping such a file;
//   - a non-zero exit is accepted only when ordinary diagnostics explain it,
//     or when biome reports that every selected path is ignored by its own
//     configuration (exit 1, no diagnostics, "provided but ignored"), which
//     is a deliberate no-op like .goneatignore.
func biomeReportFromRun(run toolRun, what string) (biomeV2Report, error) {
	if run.failedWithoutOutput() {
		return biomeV2Report{}, run.runFailure(what)
	}
	report, err := parseBiomeReport(run.Stdout)
	if err != nil {
		return biomeV2Report{}, fmt.Errorf("failed to parse biome json: %w", err)
	}
	var internal []string
	ordinary := 0
	for _, d := range report.Diagnostics {
		if strings.HasPrefix(d.Category, "internalError") {
			msg := strings.TrimSpace(d.Message)
			if msg == "" {
				msg = strings.TrimSpace(d.Description)
			}
			path := d.Location.Path.File
			if path != "" {
				msg = path + ": " + msg
			}
			internal = append(internal, fmt.Sprintf("%s: %s", d.Category, msg))
			continue
		}
		ordinary++
	}
	switch {
	case len(internal) > 0:
		if len(internal) > 5 {
			internal = internal[:5]
		}
		return biomeV2Report{}, fmt.Errorf("%s could not check every file (exit %d): %s", what, run.ExitCode, strings.Join(internal, "; "))
	case run.ExitCode == 0 || ordinary > 0:
		return report, nil
	case biomeAllPathsIgnored(run):
		logger.Debug(fmt.Sprintf("%s: all selected paths are ignored by biome configuration", what))
		return report, nil
	}
	msg := fmt.Sprintf("%s exited %d without completing", what, run.ExitCode)
	if tail := run.stderrTail(10); tail != "" {
		msg += "\n" + tail
	}
	return biomeV2Report{}, errors.New(msg)
}

// biomeAllPathsIgnored recognizes biome's "No files were processed ... These
// paths were provided but ignored" result (observed with biome 2.4).
func biomeAllPathsIgnored(run toolRun) bool {
	stderr := string(run.Stderr)
	return run.ExitCode == 1 && strings.Contains(stderr, "No files were processed") &&
		strings.Contains(stderr, "provided but ignored")
}

func groupBiomeFiles(target string, files []string) (map[string][]string, error) {
	groups := make(map[string][]string)
	for _, f := range files {
		absPath := f
		if !filepath.IsAbs(f) {
			absPath = filepath.Join(target, f)
		}
		cmdDir, relFile, err := work.ResolveBiomeContext(absPath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve context for %s: %w", f, err)
		}
		groups[cmdDir] = append(groups[cmdDir], relFile)
	}
	return groups, nil
}

func getRelativeIssueFile(cmdDir string, target string, reportedFile string) string {
	absFile := filepath.Join(cmdDir, reportedFile)
	if rel, err := filepath.Rel(target, absFile); err == nil {
		return rel
	}
	return reportedFile
}

func runBiomeLint(target string, config AssessmentConfig, files []string) ([]Issue, error) {
	if len(files) == 0 {
		return nil, nil
	}
	if _, err := exec.LookPath("biome"); err != nil {
		logger.Info("biome not found; skipping JS/TS lint")
		return nil, nil
	}

	var allIssues []Issue

	groups, err := groupBiomeFiles(target, files)
	if err != nil {
		return nil, fmt.Errorf("failed to group biome files: %w", err)
	}

	// Fix mode: apply safe fixes (never unsafe) then re-check.
	if config.Mode == AssessmentModeFix {
		for cmdDir, groupFiles := range groups {
			args := append([]string{"lint", "--write"}, groupFiles...)
			if err := runTool(cmdDir, "biome", args, config.Timeout); err != nil {
				return nil, err
			}
		}
	}

	// Add incremental checking flags when NewIssuesOnly is enabled
	// biome requires both --changed and --since=REF for incremental lint
	// When using --changed, biome determines file list from git; skip explicit files
	if config.NewIssuesOnly {
		args := []string{"lint", "--reporter", "json"}
		base := config.NewIssuesBase
		if base == "" {
			base = "HEAD~" // Default base reference
		}
		args = append(args, "--changed", "--since="+base)
		logger.Debug(fmt.Sprintf("biome lint: incremental mode enabled (since=%s)", base))

		run, err := runToolSplit(target, "biome", args, config.Timeout)
		if err != nil {
			return nil, err
		}
		report, rerr := biomeReportFromRun(run, "biome lint")
		if rerr != nil {
			return nil, rerr
		}

		for _, d := range report.Diagnostics {
			if strings.HasPrefix(d.Category, "internalError") {
				continue
			}

			desc := strings.TrimSpace(d.Message)
			if desc == "" {
				desc = strings.TrimSpace(d.Description)
			}

			sev := SeverityMedium
			switch strings.ToLower(strings.TrimSpace(d.Severity)) {
			case "error":
				sev = SeverityHigh
			case "warning":
				sev = SeverityMedium
			case "information", "hint":
				sev = SeverityLow
			}

			allIssues = append(allIssues, Issue{
				File:        filepath.ToSlash(getRelativeIssueFile(target, target, d.Location.Path.File)),
				Severity:    sev,
				Message:     fmt.Sprintf("[%s] %s", d.Category, desc),
				Category:    CategoryLint,
				SubCategory: "js:biome",
			})
		}
		return allIssues, nil
	}

	for cmdDir, groupFiles := range groups {
		args := append([]string{"lint", "--reporter", "json"}, groupFiles...)
		run, err := runToolSplit(cmdDir, "biome", args, config.Timeout)
		if err != nil {
			return nil, err
		}
		report, rerr := biomeReportFromRun(run, "biome lint")
		if rerr != nil {
			return nil, rerr
		}

		for _, d := range report.Diagnostics {
			if strings.HasPrefix(d.Category, "internalError") {
				continue
			}

			desc := strings.TrimSpace(d.Message)
			if desc == "" {
				desc = strings.TrimSpace(d.Description)
			}

			sev := SeverityMedium
			switch strings.ToLower(strings.TrimSpace(d.Severity)) {
			case "error":
				sev = SeverityHigh
			case "warning":
				sev = SeverityMedium
			case "information", "hint":
				sev = SeverityLow
			}

			allIssues = append(allIssues, Issue{
				File:        filepath.ToSlash(getRelativeIssueFile(cmdDir, target, d.Location.Path.File)),
				Severity:    sev,
				Message:     fmt.Sprintf("[%s] %s", d.Category, desc),
				Category:    CategoryLint,
				SubCategory: "js:biome",
			})
		}
	}

	return allIssues, nil
}

func runBiomeConfigCheck(target string, config AssessmentConfig) ([]Issue, error) {
	if _, err := exec.LookPath("biome"); err != nil {
		logger.Info("biome not found; skipping config check")
		return nil, nil
	}

	configPath := filepath.Join(target, "biome.json")
	if _, err := os.Stat(configPath); err != nil {
		return nil, nil
	}

	args := []string{"check", "--reporter", "json", "--formatter-enabled=false", "--linter-enabled=false"}
	run, err := runToolSplit(target, "biome", args, config.Timeout)
	if err != nil {
		return nil, err
	}

	report, rerr := biomeReportFromRun(run, "biome config check")
	if rerr != nil {
		return nil, rerr
	}
	if len(report.Diagnostics) == 0 {
		return nil, nil
	}

	issues := make([]Issue, 0, len(report.Diagnostics))
	for _, d := range report.Diagnostics {
		file := filepath.Base(d.Location.Path.File)
		if file != "biome.json" {
			continue
		}
		desc := strings.TrimSpace(d.Message)
		if desc == "" {
			desc = strings.TrimSpace(d.Description)
		}
		sev := SeverityMedium
		switch strings.ToLower(strings.TrimSpace(d.Severity)) {
		case "error":
			sev = SeverityHigh
		case "warning":
			sev = SeverityMedium
		case "information", "hint":
			sev = SeverityLow
		}
		issues = append(issues, Issue{
			File:        filepath.ToSlash(getRelativeIssueFile(target, target, d.Location.Path.File)),
			Severity:    sev,
			Message:     fmt.Sprintf("[%s] %s", d.Category, desc),
			Category:    CategoryLint,
			SubCategory: "js:biome-config",
		})
	}

	return issues, nil
}

func runBiomeFormat(target string, config AssessmentConfig, files []string) ([]Issue, error) {
	if len(files) == 0 {
		return nil, nil
	}
	if _, err := exec.LookPath("biome"); err != nil {
		logger.Info("biome not found; skipping JS/TS format")
		return nil, nil
	}

	groups, err := groupBiomeFiles(target, files)
	if err != nil {
		return nil, fmt.Errorf("failed to group biome files: %w", err)
	}

	var allIssues []Issue
	seen := make(map[string]struct{})

	for cmdDir, groupFiles := range groups {
		if config.Mode == AssessmentModeFix {
			args := append([]string{"format", "--write"}, groupFiles...)
			run, err := runToolSplit(cmdDir, "biome", args, config.Timeout)
			if err != nil {
				return nil, err
			}
			// format --write exits non-zero only when it could not format (parse or config error).
			if run.ExitCode != 0 {
				return nil, fmt.Errorf("biome format --write exited %d:\n%s", run.ExitCode, run.stderrTail(10))
			}
			continue
		}

		args := append([]string{"check", "--formatter-enabled=true", "--linter-enabled=false", "--reporter", "json"}, groupFiles...)
		run, err := runToolSplit(cmdDir, "biome", args, config.Timeout)
		if err != nil {
			return nil, err
		}

		report, rerr := biomeReportFromRun(run, "biome format check")
		if rerr != nil {
			return nil, rerr
		}

		for _, d := range report.Diagnostics {
			if strings.HasPrefix(d.Category, "internalError") {
				continue
			}
			if strings.ToLower(strings.TrimSpace(d.Category)) != "format" {
				continue
			}
			file := filepath.ToSlash(getRelativeIssueFile(cmdDir, target, d.Location.Path.File))
			if file == "" {
				continue
			}
			if _, ok := seen[file]; ok {
				continue
			}
			seen[file] = struct{}{}
			allIssues = append(allIssues, Issue{
				File:          file,
				Severity:      SeverityLow,
				Message:       "File not formatted (biome format)",
				Category:      CategoryFormat,
				SubCategory:   "js:biome-format",
				AutoFixable:   true,
				EstimatedTime: HumanReadableDuration(30 * time.Second),
			})
		}
	}

	return allIssues, nil
}
