package assess

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/pkg/logger"
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
	Category    string `json:"category"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Location    struct {
		Path struct {
			File string `json:"file"`
		} `json:"path"`
		Span []int `json:"span"` // [start, end] byte offsets
	} `json:"location"`
}

func runBiomeLint(target string, config AssessmentConfig, files []string) ([]Issue, error) {
	if len(files) == 0 {
		return nil, nil
	}
	if _, err := exec.LookPath("biome"); err != nil {
		logger.Info("biome not found; skipping JS/TS lint")
		return nil, nil
	}

	// Fix mode: apply safe fixes (never unsafe) then re-check.
	if config.Mode == AssessmentModeFix {
		args := append([]string{"lint", "--write"}, files...)
		if err := runTool(target, "biome", args, config.Timeout); err != nil {
			return nil, err
		}
	}

	args := []string{"lint", "--reporter", "json"}

	// Add incremental checking flags when NewIssuesOnly is enabled
	// biome requires both --changed and --since=REF for incremental lint
	// When using --changed, biome determines file list from git; skip explicit files
	if config.NewIssuesOnly {
		base := config.NewIssuesBase
		if base == "" {
			base = "HEAD~" // Default base reference
		}
		args = append(args, "--changed", "--since="+base)
		logger.Debug(fmt.Sprintf("biome lint: incremental mode enabled (since=%s)", base))
		// Let biome determine changed files; don't pass explicit file list
	} else {
		args = append(args, files...)
	}

	out, err := runToolStdoutOnly(target, "biome", args, config.Timeout)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(out)) == 0 {
		return nil, nil
	}

	var report biomeV2Report
	if uerr := json.Unmarshal(out, &report); uerr != nil {
		return nil, fmt.Errorf("failed to parse biome json: %w", uerr)
	}

	issues := make([]Issue, 0, len(report.Diagnostics))
	for _, d := range report.Diagnostics {
		// Skip internal errors (like "file not found")
		if strings.HasPrefix(d.Category, "internalError") {
			continue
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
			File:        filepath.ToSlash(d.Location.Path.File),
			Severity:    sev,
			Message:     fmt.Sprintf("[%s] %s", d.Category, strings.TrimSpace(d.Description)),
			Category:    CategoryLint,
			SubCategory: "js:biome",
		})
	}

	return issues, nil
}

// runToolStdoutOnly is now in tool_runner.go

func runBiomeFormat(target string, config AssessmentConfig, files []string) ([]Issue, error) {
	if len(files) == 0 {
		return nil, nil
	}
	if _, err := exec.LookPath("biome"); err != nil {
		logger.Info("biome not found; skipping JS/TS format")
		return nil, nil
	}

	if config.Mode == AssessmentModeFix {
		args := append([]string{"format", "--write"}, files...)
		if err := runTool(target, "biome", args, config.Timeout); err != nil {
			return nil, err
		}
		return nil, nil
	}

	// Biome 2.x: running without --write performs a dry-run check
	// Exit code 0 = all files formatted, exit code 1 = some files need formatting
	// The output contains the file paths that need formatting
	// TODO(v0.4.6): Refactor biome integration to share code between assess and format commands.
	// Currently duplicated in: internal/assess/js_biome.go, pkg/work/format_processor.go, cmd/format.go
	args := append([]string{"format"}, files...)
	out, exitCode, err := runToolCapture(target, "biome", args, config.Timeout)
	if err == nil && exitCode == 0 {
		return nil, nil
	}

	// Parse biome output to find which files need formatting
	// Biome 2.x outputs lines like "path/to/file.ts format ━━━" for each unformatted file
	issues := make([]Issue, 0)
	outStr := string(out)
	for _, f := range files {
		// Check if file appears in output as needing formatting
		if strings.Contains(outStr, f+" format") || strings.Contains(outStr, filepath.ToSlash(f)+" format") {
			issues = append(issues, Issue{
				File:          filepath.ToSlash(f),
				Severity:      SeverityLow,
				Message:       "File not formatted (biome format)",
				Category:      CategoryFormat,
				SubCategory:   "js:biome-format",
				AutoFixable:   true,
				EstimatedTime: HumanReadableDuration(30 * time.Second),
			})
		}
	}
	return issues, nil
}
