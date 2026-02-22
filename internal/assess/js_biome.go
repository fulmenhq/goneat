package assess

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
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
		return report, fmt.Errorf("no json output from biome")
	}
	trimmed = trimmed[start : end+1]
	if uerr := json.Unmarshal(trimmed, &report); uerr != nil {
		return report, uerr
	}
	return report, nil
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

	report, uerr := parseBiomeReport(out)
	if uerr != nil {
		return nil, fmt.Errorf("failed to parse biome json: %w", uerr)
	}

	issues := make([]Issue, 0, len(report.Diagnostics))
	for _, d := range report.Diagnostics {
		// Skip internal errors (like "file not found")
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

		issues = append(issues, Issue{
			File:        filepath.ToSlash(d.Location.Path.File),
			Severity:    sev,
			Message:     fmt.Sprintf("[%s] %s", d.Category, desc),
			Category:    CategoryLint,
			SubCategory: "js:biome",
		})
	}

	return issues, nil
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
	out, _, err := runToolCapture(target, "biome", args, config.Timeout)
	if err != nil {
		return nil, err
	}

	report, uerr := parseBiomeReport(out)
	if uerr != nil {
		return nil, fmt.Errorf("failed to parse biome json: %w", uerr)
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
			File:        filepath.ToSlash(d.Location.Path.File),
			Severity:    sev,
			Message:     fmt.Sprintf("[%s] %s", d.Category, desc),
			Category:    CategoryLint,
			SubCategory: "js:biome-config",
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

	// Biome 2.x: use `check` with formatter enabled to get JSON diagnostics
	// This avoids parsing human-readable output and respects biome ignores.
	// TODO(v0.4.6): Refactor biome integration to share code between assess and format commands.
	// Currently duplicated in: internal/assess/js_biome.go, pkg/work/format_processor.go, cmd/format.go
	args := append([]string{"check", "--formatter-enabled=true", "--linter-enabled=false", "--reporter", "json"}, files...)
	out, _, err := runToolCapture(target, "biome", args, config.Timeout)
	if err != nil {
		return nil, err
	}

	report, uerr := parseBiomeReport(out)
	if uerr != nil {
		return nil, fmt.Errorf("failed to parse biome json: %w", uerr)
	}
	if len(report.Diagnostics) == 0 {
		return nil, nil
	}

	issues := make([]Issue, 0, len(report.Diagnostics))
	seen := make(map[string]struct{})
	for _, d := range report.Diagnostics {
		if strings.HasPrefix(d.Category, "internalError") {
			continue
		}
		if strings.ToLower(strings.TrimSpace(d.Category)) != "format" {
			continue
		}
		file := filepath.ToSlash(d.Location.Path.File)
		if file == "" {
			continue
		}
		if _, ok := seen[file]; ok {
			continue
		}
		seen[file] = struct{}{}
		issues = append(issues, Issue{
			File:          file,
			Severity:      SeverityLow,
			Message:       "File not formatted (biome format)",
			Category:      CategoryFormat,
			SubCategory:   "js:biome-format",
			AutoFixable:   true,
			EstimatedTime: HumanReadableDuration(30 * time.Second),
		})
	}
	return issues, nil
}
