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

type ruffJSONLocation struct {
	Row    int `json:"row"`
	Column int `json:"column"`
}

type ruffJSONMessage struct {
	Code     string           `json:"code"`
	Message  string           `json:"message"`
	Filename string           `json:"filename"`
	Location ruffJSONLocation `json:"location"`
}

func runRuffCheck(target string, config AssessmentConfig, files []string) ([]Issue, error) {
	if len(files) == 0 {
		return nil, nil
	}
	if _, err := exec.LookPath("ruff"); err != nil {
		logger.Info("ruff not found; skipping Python lint")
		return nil, nil
	}

	// Fix mode: apply safe fixes first, then re-check to report remaining issues.
	if config.Mode == AssessmentModeFix {
		args := append([]string{"check", "--fix"}, files...)
		if err := runTool(target, "ruff", args, config.Timeout); err != nil {
			return nil, err
		}
	}

	args := append([]string{"check", "--output-format", "json"}, files...)
	out, exitCode, err := runToolCapture(target, "ruff", args, config.Timeout)
	if err != nil && exitCode == 0 {
		return nil, err
	}
	if len(bytes.TrimSpace(out)) == 0 {
		return nil, nil
	}

	var msgs []ruffJSONMessage
	if uerr := json.Unmarshal(out, &msgs); uerr != nil {
		return nil, fmt.Errorf("failed to parse ruff json: %w", uerr)
	}

	issues := make([]Issue, 0, len(msgs))
	for _, m := range msgs {
		msg := strings.TrimSpace(m.Message)
		code := strings.TrimSpace(m.Code)
		if code != "" {
			msg = fmt.Sprintf("%s %s", code, msg)
		}
		issues = append(issues, Issue{
			File:        filepath.ToSlash(m.Filename),
			Line:        m.Location.Row,
			Column:      m.Location.Column,
			Severity:    SeverityMedium,
			Message:     msg,
			Category:    CategoryLint,
			SubCategory: "python:ruff",
		})
	}

	return issues, nil
}

func runRuffFormat(target string, config AssessmentConfig, files []string) ([]Issue, error) {
	if len(files) == 0 {
		return nil, nil
	}
	if _, err := exec.LookPath("ruff"); err != nil {
		logger.Info("ruff not found; skipping Python format")
		return nil, nil
	}

	// Fix mode: format in place then return no issues.
	if config.Mode == AssessmentModeFix {
		args := append([]string{"format"}, files...)
		if err := runTool(target, "ruff", args, config.Timeout); err != nil {
			return nil, err
		}
		return nil, nil
	}

	args := append([]string{"format", "--check"}, files...)
	out, exitCode, err := runToolCapture(target, "ruff", args, config.Timeout)
	if err == nil && exitCode == 0 {
		return nil, nil
	}

	// Ruff returns non-zero when formatting would change.
	if len(bytes.TrimSpace(out)) == 0 {
		return issuesFromFiles(files, "ruff format reported formatting differences"), nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var touched []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Typical: "Would reformat: path.py"
		if idx := strings.Index(line, ":"); idx != -1 {
			maybe := strings.TrimSpace(line[idx+1:])
			if strings.HasSuffix(strings.ToLower(maybe), ".py") {
				touched = append(touched, maybe)
				continue
			}
		}
	}
	if len(touched) == 0 {
		touched = files
	}

	issues := make([]Issue, 0, len(touched))
	for _, f := range touched {
		issues = append(issues, Issue{
			File:          filepath.ToSlash(f),
			Severity:      SeverityLow,
			Message:       "Python file not formatted (ruff format)",
			Category:      CategoryFormat,
			SubCategory:   "python:ruff-format",
			AutoFixable:   true,
			EstimatedTime: HumanReadableDuration(30 * time.Second),
		})
	}

	return issues, nil
}

func runTool(target, bin string, args []string, timeout time.Duration) error {
	_, _, err := runToolCapture(target, bin, args, timeout)
	return err
}

func runToolCapture(target, bin string, args []string, timeout time.Duration) ([]byte, int, error) {
	tctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		tctx, cancel = context.WithTimeout(context.Background(), timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(tctx, bin, args...) // #nosec G204
	cmd.Dir = target
	out, err := cmd.CombinedOutput()
	if err == nil {
		return out, 0, nil
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return out, ee.ExitCode(), nil
	}
	return out, 0, fmt.Errorf("%s execution failed: %w", bin, err)
}
