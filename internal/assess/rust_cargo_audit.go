package assess

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fulmenhq/goneat/pkg/logger"
)

const cargoAuditMinVersion = "0.18.0"

type cargoAuditAdapter struct {
	runner     *SecurityAssessmentRunner
	moduleRoot string
	cfg        AssessmentConfig
}

func (c *cargoAuditAdapter) Name() string { return "cargo-audit" }

func (c *cargoAuditAdapter) IsApplicable() bool {
	project := DetectRustProject(c.moduleRoot)
	return project != nil && project.CargoTomlPath != ""
}

func (c *cargoAuditAdapter) IsAvailable() bool {
	if !IsCargoAvailable() {
		return false
	}
	if !c.IsApplicable() {
		return false
	}
	presence := CheckRustToolPresence("cargo-audit", cargoAuditMinVersion)
	if presence.Present && !presence.MeetsMin && presence.Version != "" {
		logger.Warn(fmt.Sprintf("cargo-audit %s below minimum %s; results may be unreliable", presence.Version, cargoAuditMinVersion))
	}
	return presence.Present
}

func (c *cargoAuditAdapter) Run(ctx context.Context) ([]Issue, error) {
	issues, _, err := c.RunWithWarnings(ctx)
	return issues, err
}

func (c *cargoAuditAdapter) RunWithWarnings(ctx context.Context) ([]Issue, map[string][]json.RawMessage, error) {
	project := DetectRustProject(c.moduleRoot)
	if project == nil || project.CargoTomlPath == "" {
		return nil, nil, nil
	}
	root := project.EffectiveRoot()
	if root == "" {
		root = c.moduleRoot
	}

	args := []string{"audit", "--json"}
	rctx, cancel := c.runner.effectiveToolContext(ctx, c.cfg.Timeout, 0)
	defer cancel()
	cmd := exec.CommandContext(rctx, "cargo", args...)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	processErr := cmd.Run()
	exitCode := 0
	var exitErr *exec.ExitError
	if errors.As(processErr, &exitErr) && exitErr.ExitCode() >= 0 && rctx.Err() == nil {
		exitCode = exitErr.ExitCode()
		processErr = nil // Completion is decided only after report validation.
	}
	processErr = errors.Join(processErr, rctx.Err())
	report := parseCargoAuditReport(bytes.NewReader(stdout.Bytes()))
	completionErr := cargoAuditCompletionError(report, exitCode, processErr)
	issues := make([]Issue, 0, len(report.vulnerabilities))
	reportFile := rustIssueFile(project)
	for _, vuln := range report.vulnerabilities {
		msg := strings.TrimSpace(vuln.Advisory.Title)
		if msg == "" {
			msg = "cargo-audit advisory"
		}
		if vuln.Package.Name != "" {
			msg = fmt.Sprintf("%s (crate: %s %s)", msg, vuln.Package.Name, vuln.Package.Version)
		}
		if vuln.Advisory.URL != "" {
			msg = fmt.Sprintf("%s - %s", msg, vuln.Advisory.URL)
		}

		issues = append(issues, Issue{
			File:        filepath.ToSlash(reportFile),
			Severity:    mapCargoAuditSeverity(vuln.Advisory.Severity),
			Message:     fmt.Sprintf("cargo-audit(%s): %s", vuln.Advisory.ID, msg),
			Category:    CategorySecurity,
			SubCategory: "rust:cargo-audit",
			AutoFixable: false,
		})
	}

	return issues, report.warnings, completionErr
}

type cargoAuditVuln struct {
	Advisory struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Severity string `json:"severity"`
		URL      string `json:"url"`
	} `json:"advisory"`
	Package struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"package"`
}

func mapCargoAuditSeverity(sev string) IssueSeverity {
	switch strings.ToLower(strings.TrimSpace(sev)) {
	case "critical":
		return SeverityCritical
	case "high":
		return SeverityHigh
	case "medium", "moderate":
		return SeverityMedium
	case "low":
		return SeverityLow
	default:
		return SeverityMedium
	}
}

func rustIssueFile(project *RustProject) string {
	if project == nil {
		return ""
	}
	root := project.EffectiveRoot()
	if root == "" && project.CargoTomlPath != "" {
		root = filepath.Dir(project.CargoTomlPath)
	}
	if root != "" {
		lockPath := filepath.Join(root, "Cargo.lock")
		if _, err := os.Stat(lockPath); err == nil {
			return lockPath
		}
		tomlPath := filepath.Join(root, "Cargo.toml")
		if _, err := os.Stat(tomlPath); err == nil {
			return tomlPath
		}
	}
	if project.CargoTomlPath != "" {
		return project.CargoTomlPath
	}
	return ""
}

func init() {
	RegisterSecurityTool("cargo-audit", "vuln", func(r *SecurityAssessmentRunner, moduleRoot string, cfg AssessmentConfig) SecurityTool {
		return &cargoAuditAdapter{runner: r, moduleRoot: moduleRoot, cfg: cfg}
	})
}
