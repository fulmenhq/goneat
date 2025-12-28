package assess

import (
	"bytes"
	"context"
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

	args := append([]string{"lint", "--reporter", "json"}, files...)
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

// runToolStdoutOnly captures only stdout (not stderr) for tools that mix formats
func runToolStdoutOnly(target, bin string, args []string, timeout time.Duration) ([]byte, error) {
	tctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		tctx, cancel = context.WithTimeout(context.Background(), timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(tctx, bin, args...) // #nosec G204
	cmd.Dir = target
	out, err := cmd.Output() // Only stdout, stderr is discarded
	if err != nil {
		// Non-zero exit is expected when issues are found
		if _, ok := err.(*exec.ExitError); ok {
			return out, nil
		}
		return nil, fmt.Errorf("%s execution failed: %w", bin, err)
	}
	return out, nil
}

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

	args := append([]string{"format", "--check"}, files...)
	out, exitCode, err := runToolCapture(target, "biome", args, config.Timeout)
	if err == nil && exitCode == 0 {
		return nil, nil
	}

	_ = out
	issues := make([]Issue, 0, len(files))
	for _, f := range files {
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
	return issues, nil
}
