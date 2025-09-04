/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/3leaps/goneat/internal/assess"
	"github.com/3leaps/goneat/internal/ops"
	cfgpkg "github.com/3leaps/goneat/pkg/config"
	"github.com/3leaps/goneat/pkg/logger"
	"github.com/spf13/cobra"
)

// securityCmd represents the security command (GroupNeat)
var securityCmd = &cobra.Command{
	Use:   "security [target]",
	Short: "Security scanning (vulnerabilities, code security)",
	Long: `Run security scanners via goneat (GroupNeat).

Includes gosec (code) and govulncheck (dependencies) for Go projects.
Outputs structured reports suitable for CI and hooks.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSecurity,
}

var (
	securityFormat             string
	securityFailOn             string
	securityOutput             string
	securityStaged             bool
	securityDiffBase           string
	securityTools              []string
	securityEnable             []string
	securityConcurrency        int
	securityConcurrencyPercent int
	securityMaxIssues          int
	securityTimeout            time.Duration
	securityGosecTimeout       time.Duration
	securityGovulnTimeout      time.Duration
	securityTrackSuppressions  bool
)

func init() {
	rootCmd.AddCommand(securityCmd)

	// Register command in ops registry with taxonomy
	capabilities := ops.GetDefaultCapabilities(ops.GroupNeat, ops.CategoryAssessment)
	if err := ops.RegisterCommandWithTaxonomy("security", ops.GroupNeat, ops.CategoryAssessment, capabilities, securityCmd, "Security scanning (vulnerabilities, code security)"); err != nil {
		panic(fmt.Sprintf("Failed to register security command: %v", err))
	}

	securityCmd.Flags().StringVar(&securityFormat, "format", "markdown", "Output format (concise, markdown, json, html, both)")
	securityCmd.Flags().StringVar(&securityFailOn, "fail-on", "high", "Fail if issues at or above severity (critical, high, medium, low, info)")
	securityCmd.Flags().StringVarP(&securityOutput, "output", "o", "", "Output file (default: stdout)")
	securityCmd.Flags().BoolVar(&securityStaged, "staged-only", false, "Restrict to staged files (code scanners)")
	securityCmd.Flags().StringVar(&securityDiffBase, "diff-base", "", "Restrict to files changed since this ref (e.g., origin/main)")
	securityCmd.Flags().StringSliceVar(&securityTools, "tools", []string{}, "Security tools to run (e.g., gosec,govulncheck)")
	securityCmd.Flags().StringSliceVar(&securityEnable, "enable", []string{"vuln", "code"}, "Enable dimensions: vuln, code, secrets")
	securityCmd.Flags().IntVar(&securityConcurrency, "concurrency", 0, "Explicit worker count for security sharding (default derived from CPU%)")
	securityCmd.Flags().IntVar(&securityConcurrencyPercent, "concurrency-percent", 50, "Percent of CPU cores for sharding (1-100), used when --concurrency is 0")
	securityCmd.Flags().IntVar(&securityMaxIssues, "max-issues", 0, "Limit displayed issues per category for non-JSON output (0 = unlimited)")
	securityCmd.Flags().DurationVar(&securityTimeout, "timeout", 5*time.Minute, "Assessment timeout for security (overrides default)")
	// Per-tool timeouts (inherit global when 0); effective timeout = min(global, per-tool)
	securityCmd.Flags().DurationVar(&securityGosecTimeout, "gosec-timeout", 0, "Per-tool timeout for gosec (0 = inherit global)")
	securityCmd.Flags().DurationVar(&securityGovulnTimeout, "govulncheck-timeout", 0, "Per-tool timeout for govulncheck (0 = inherit global)")
	// Doctor-isolated behavior: optionally skip missing tools (otherwise fail fast if explicitly requested and missing)
	securityCmd.Flags().Bool("ignore-missing-tools", false, "Skip missing security tools (otherwise fail if explicitly requested via --tools and missing)")
	securityCmd.Flags().BoolVar(&securityTrackSuppressions, "track-suppressions", false, "Track and report security suppressions (e.g., #nosec comments)")
}

func runSecurity(cmd *cobra.Command, args []string) error {
	// Determine target directory
	target := "."
	if len(args) > 0 {
		target = args[0]
	}
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return fmt.Errorf("target directory does not exist: %s", target)
	}

	// Parse format
	var outFmt assess.OutputFormat
	switch strings.ToLower(securityFormat) {
	case "concise":
		outFmt = assess.FormatConcise
	case "markdown":
		outFmt = assess.FormatMarkdown
	case "json":
		outFmt = assess.FormatJSON
	case "html":
		outFmt = assess.FormatHTML
	case "both":
		outFmt = assess.FormatBoth
	default:
		return fmt.Errorf("invalid format: %s", securityFormat)
	}

	// Suppress logs for JSON output to keep clean
	if outFmt == assess.FormatJSON {
		logger.SetOutput(io.Discard)
	}

	// Parse fail-on severity
	var failOn assess.IssueSeverity
	switch strings.ToLower(securityFailOn) {
	case "critical":
		failOn = assess.SeverityCritical
	case "high":
		failOn = assess.SeverityHigh
	case "medium":
		failOn = assess.SeverityMedium
	case "low":
		failOn = assess.SeverityLow
	case "info":
		failOn = assess.SeverityInfo
	default:
		return fmt.Errorf("invalid fail-on severity: %s", securityFailOn)
	}

	// Handle missing tool policy (doctor-isolated behavior)
	ignoreMissing, _ := cmd.Flags().GetBool("ignore-missing-tools")
	// Fail fast only when the user explicitly requested tools via --tools
	if len(securityTools) > 0 && !ignoreMissing {
		for _, tname := range securityTools {
			tn := strings.TrimSpace(strings.ToLower(tname))
			if tn == "" {
				continue
			}
			if _, err := exec.LookPath(tn); err != nil {
				return fmt.Errorf("%s not found but was requested via --tools.\nInstall with:\n  - gosec:       go install github.com/securego/gosec/v2/cmd/gosec@latest\n  - govulncheck: go install golang.org/x/vuln/cmd/govulncheck@latest\n  - gitleaks:    go install github.com/zricethezav/gitleaks/v8@latest\nOr run: goneat doctor tools --install %s\nTip: use --ignore-missing-tools to skip missing tool(s)", tn, tn)
			}
		}
	}

	// Load project config (optional)
	projCfg, _ := cfgpkg.LoadProjectConfig()

	// If --fail-on not provided, honor config.security.fail_on
	if !cmd.Flags().Changed("fail-on") && projCfg != nil {
		switch strings.ToLower(strings.TrimSpace(projCfg.Security.FailOn)) {
		case "critical":
			failOn = assess.SeverityCritical
		case "high":
			failOn = assess.SeverityHigh
		case "medium":
			failOn = assess.SeverityMedium
		case "low":
			failOn = assess.SeverityLow
		case "info":
			failOn = assess.SeverityInfo
		}
	}

	// Build assessment config
	cfg := assess.DefaultAssessmentConfig()
	cfg.Mode = assess.AssessmentModeCheck
	cfg.FailOnSeverity = failOn
	cfg.Verbose = false
	// Timeout: from config unless flag provided
	if f := cmd.Flags(); f.Changed("timeout") {
		cfg.Timeout = securityTimeout
	} else if projCfg != nil && projCfg.Security.Timeout > 0 {
		cfg.Timeout = projCfg.Security.Timeout
	} else {
		cfg.Timeout = securityTimeout
	}
	// Concurrency tuning for runner sharding
	if f := cmd.Flags(); f.Changed("concurrency") {
		cfg.Concurrency = securityConcurrency
	} else if projCfg != nil {
		cfg.Concurrency = projCfg.Security.Concurrency
	} else {
		cfg.Concurrency = securityConcurrency
	}
	if f := cmd.Flags(); f.Changed("concurrency-percent") {
		cfg.ConcurrencyPercent = securityConcurrencyPercent
	} else if projCfg != nil && projCfg.Security.ConcurrencyPercent > 0 {
		cfg.ConcurrencyPercent = projCfg.Security.ConcurrencyPercent
	} else {
		cfg.ConcurrencyPercent = securityConcurrencyPercent
	}
	// Apply tool/dimension flags without touching schema
	if cmd.Flags().Changed("tools") && len(securityTools) > 0 {
		cfg.SecurityTools = append([]string(nil), securityTools...)
	} else if projCfg != nil && len(projCfg.Security.Tools) > 0 {
		cfg.SecurityTools = append([]string(nil), projCfg.Security.Tools...)
	}
	// Defaults already vuln+code; adjust per --enable
	if cmd.Flags().Changed("enable") {
		cfg.EnableVuln, cfg.EnableCode, cfg.EnableSecrets = false, false, false
		for _, dim := range securityEnable {
			switch strings.ToLower(strings.TrimSpace(dim)) {
			case "vuln", "vulnerability", "vulnerabilities":
				cfg.EnableVuln = true
			case "code", "codesec", "security":
				cfg.EnableCode = true
			case "secrets", "secret":
				cfg.EnableSecrets = true
			}
		}
	} else if projCfg != nil {
		cfg.EnableCode = projCfg.Security.Enable.Code
		cfg.EnableVuln = projCfg.Security.Enable.Vuln
		cfg.EnableSecrets = projCfg.Security.Enable.Secrets
	}
	// Per-tool timeouts
	if cmd.Flags().Changed("gosec-timeout") {
		cfg.SecurityGosecTimeout = securityGosecTimeout
	} else if projCfg != nil {
		cfg.SecurityGosecTimeout = projCfg.Security.ToolTimeouts.Gosec
	}
	if cmd.Flags().Changed("govulncheck-timeout") {
		cfg.SecurityGovulncheckTimeout = securityGovulnTimeout
	} else if projCfg != nil {
		cfg.SecurityGovulncheckTimeout = projCfg.Security.ToolTimeouts.Govulncheck
	}
	if cmd.Flags().Changed("track-suppressions") {
		cfg.TrackSuppressions = securityTrackSuppressions
	} else if projCfg != nil {
		cfg.TrackSuppressions = projCfg.Security.TrackSuppressions
	}

	// Limit to security category only
	cfg.SelectedCategories = []string{"security"}

	// Apply diff-only or staged-only scoping for code scanners
	if securityStaged {
		if staged, err := getStagedFiles(); err == nil {
			cfg.IncludeFiles = staged
		} else {
			logger.Warn(fmt.Sprintf("Failed to resolve staged files: %v", err))
		}
	} else if strings.TrimSpace(securityDiffBase) != "" {
		if changed, err := getChangedFiles(securityDiffBase); err == nil {
			cfg.IncludeFiles = changed
		} else {
			logger.Warn(fmt.Sprintf("Failed to resolve changed files from %s: %v", securityDiffBase, err))
		}
	}

	// Create assessment engine and run
	engine := assess.NewAssessmentEngine()
	// Suppress log for JSON output to keep clean
	if outFmt != assess.FormatJSON {
		logger.Info(fmt.Sprintf("Starting security assessment of %s (workers=%d)", target, max(1, runtime.NumCPU()/2)))
	}
	report, err := engine.RunAssessment(cmd.Context(), target, cfg)
	if err != nil {
		return fmt.Errorf("security assessment failed: %v", err)
	}

	// Output (export fail-on for concise header context)
	_ = os.Setenv("GONEAT_SECURITY_FAIL_ON", strings.ToLower(string(failOn)))
	// Optional cap for non-JSON outputs to keep logs readable
	if securityMaxIssues > 0 {
		_ = os.Setenv("GONEAT_MAX_ISSUES_DISPLAY", fmt.Sprintf("%d", securityMaxIssues))
	}
	formatter := assess.NewFormatter(outFmt)
	formatter.SetTargetPath(target)
	out := cmd.OutOrStdout()
	if securityOutput != "" {
		// Validate output path to prevent path traversal
		securityOutput = filepath.Clean(securityOutput)
		if strings.Contains(securityOutput, "..") {
			return fmt.Errorf("invalid output path: contains path traversal")
		}
		f, ferr := os.Create(securityOutput)
		if ferr != nil {
			return fmt.Errorf("failed to create output file: %v", ferr)
		}
		defer func() {
			if cerr := f.Close(); cerr != nil {
				logger.Warn(fmt.Sprintf("failed to close output file: %v", cerr))
			}
		}()
		out = f
	}
	if err := formatter.WriteReport(out, report); err != nil {
		return fmt.Errorf("failed to write report: %v", err)
	}

	// Enforce fail-on
	if shouldFail(report, failOn) {
		logger.Error(fmt.Sprintf("Security scan failed: found issues at or above %s severity", failOn))
		os.Exit(1)
	}
	return nil
}

// getChangedFiles returns files changed since a given base ref
func getChangedFiles(baseRef string) ([]string, error) {
	// Validate baseRef to prevent command injection
	if strings.ContainsAny(baseRef, ";|&`") {
		return nil, fmt.Errorf("invalid baseRef: contains dangerous characters")
	}
	refArg := fmt.Sprintf("%s...HEAD", baseRef)
	cmd := exec.Command("git", "diff", "--name-only", refArg) // #nosec G204
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w", err)
	}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	scanner.Split(bufio.ScanLines)
	var files []string
	for scanner.Scan() {
		p := strings.TrimSpace(scanner.Text())
		if p != "" {
			files = append(files, p)
		}
	}
	if serr := scanner.Err(); serr != nil {
		return nil, serr
	}
	return files, nil
}
