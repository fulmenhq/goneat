/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/internal/assess"
	"github.com/fulmenhq/goneat/internal/ops"
	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/spf13/cobra"
	pflag "github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

// assessCmd represents the assess command
var assessCmd = &cobra.Command{
	Use:   "assess [target]",
	Short: "Run comprehensive assessment (format, lint, security, etc.)",
	Long: `Run a comprehensive assessment of the codebase.
The target argument is optional. If not provided, it defaults to the current directory.
You can restrict the assessment to specific categories using the --categories flag.`,
	Example: `  goneat assess                                  # Run all assessments on current directory
  goneat assess ./...                              # Run all assessments recursively
  goneat assess --categories format,lint           # Run only format and lint
  goneat assess --fix                              # Auto-fix fixable issues
  goneat assess --staged-only                      # Assess only staged files
  goneat assess --output report.html --format html # Output to HTML file
  goneat assess --fail-on high                     # Exit with error on high-severity issues
  goneat assess --priority "security=1,format=2"  # Custom priorities`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAssess,
}

var (
	assessFormat              string
	assessMode                string
	assessNoOp                bool
	assessCheck               bool
	assessFix                 bool
	assessVerbose             bool
	assessPriority            string
	assessFailOn              string
	assessTimeout             time.Duration
	assessOutput              string
	assessIncludeFiles        []string
	assessExcludeFiles        []string
	assessNoIgnore            bool
	assessForceInclude        []string
	assessSchemaEnableMeta    bool
	assessScope               bool
	assessHook                string
	assessHookManifest        string
	assessOpen                bool
	assessBenchmark           bool
	assessBenchmarkIterations int
	assessBenchmarkOutput     string
	// Concurrency flags
	assessConcurrency        int
	assessConcurrencyPercent int
	assessCategories         string
	assessStagedOnly         bool
	assessTrackSuppressions  bool
	// CI/Profiles
	assessCISummary      bool
	assessProfile        string
	assessLintNewFromRev string
	assessPackageMode    bool
	// Extended output
	assessExtended bool
)

func init() {
	rootCmd.AddCommand(assessCmd)

	// Register command in ops registry with taxonomy
	capabilities := ops.GetDefaultCapabilities(ops.GroupNeat, ops.CategoryAssessment)
	if err := ops.RegisterCommandWithTaxonomy("assess", ops.GroupNeat, ops.CategoryAssessment, capabilities, assessCmd, "Comprehensive codebase assessment and workflow planning"); err != nil {
		panic(fmt.Sprintf("Failed to register assess command: %v", err))
	}

	setupAssessCommandFlags(assessCmd)
}

// setupAssessCommandFlags configures flags for the assess command (shared with tests)
func setupAssessCommandFlags(cmd *cobra.Command) {
	// Assessment flags
	cmd.Flags().StringVar(&assessFormat, "format", "markdown", "Output format (markdown, json, html, both)")
	cmd.Flags().StringVar(&assessMode, "mode", "check", "Operation mode (no-op, check, fix)")
	cmd.Flags().BoolVarP(&assessVerbose, "verbose", "v", false, "Verbose output")
	cmd.Flags().StringVar(&assessPriority, "priority", "", "Custom priority string (e.g., 'security=1,format=2')")
	cmd.Flags().StringVar(&assessCategories, "categories", "", "Restrict assessment to specific categories (comma-separated, e.g., 'format,lint')")
	cmd.Flags().StringVar(&assessFailOn, "fail-on", "critical", "Fail if issues at or above severity (critical, high, medium, low)")
	cmd.Flags().DurationVar(&assessTimeout, "timeout", 5*time.Minute, "Assessment timeout")
	cmd.Flags().StringVarP(&assessOutput, "output", "o", "", "Output file (default: stdout)")
	cmd.Flags().StringSliceVar(&assessIncludeFiles, "include", []string{}, "Include only these files/patterns")
	cmd.Flags().StringSliceVar(&assessExcludeFiles, "exclude", []string{}, "Exclude these files/patterns")
	// Ignore override flags
	cmd.Flags().BoolVar(&assessNoIgnore, "no-ignore", false, "Disable .goneatignore/.gitignore for discovery (scan everything in scope)")
	cmd.Flags().StringSliceVar(&assessForceInclude, "force-include", []string{}, "Force-include paths or globs even if ignored (repeatable). Examples: --force-include tests/fixtures/** --force-include \"schemas/**\"")
	// Schema validation options
	cmd.Flags().BoolVar(&assessSchemaEnableMeta, "schema-enable-meta", false, "Attempt meta-schema validation using embedded drafts (may require network for remote $refs)")
	// Scoped discovery
	cmd.Flags().BoolVar(&assessScope, "scope", false, "Limit traversal scope to include paths and force-include anchors")
	// Lint controls
	cmd.Flags().StringVar(&assessLintNewFromRev, "lint-new-from-rev", "", "Report only new lint issues since a given git rev (passes to golangci-lint --new-from-rev)")
	// Concurrency
	cmd.Flags().IntVar(&assessConcurrency, "concurrency", 0, "Number of concurrent runners (0 uses --concurrency-percent)")
	cmd.Flags().IntVar(&assessConcurrencyPercent, "concurrency-percent", 50, "Percent of CPU cores to use for concurrency (1-100)")

	// Hook mode flags
	cmd.Flags().StringVar(&assessHook, "hook", "", "Run in hook mode (pre-commit, pre-push)")
	cmd.Flags().StringVar(&assessHookManifest, "hook-manifest", ".goneat/hooks.yaml", "Hook manifest file path")

	// Browser flags
	cmd.Flags().BoolVar(&assessOpen, "open", false, "Open HTML report in default browser")

	// Benchmark flags
	cmd.Flags().BoolVar(&assessBenchmark, "benchmark", false, "Run benchmark comparison")
	cmd.Flags().IntVar(&assessBenchmarkIterations, "iterations", 5, "Number of benchmark iterations")
	cmd.Flags().StringVar(&assessBenchmarkOutput, "benchmark-output", "benchmark.json", "Benchmark output file")

	// Add shorthand flags for modes
	cmd.Flags().Bool("no-op", false, "Run in no-op mode (assessment only)")
	cmd.Flags().Bool("check", false, "Run in check mode (report only)")
	cmd.Flags().Bool("fix", false, "Run in fix mode (apply fixes)")

	// File scope flags
	cmd.Flags().BoolVar(&assessStagedOnly, "staged-only", false, "Only assess staged files in git (changed and added)")
	// Suppression tracking (security)
	cmd.Flags().BoolVar(&assessTrackSuppressions, "track-suppressions", false, "Track and report security suppressions (e.g., #nosec) in assessment output")
	// CI helpers
	cmd.Flags().BoolVar(&assessCISummary, "ci-summary", false, "Print a single-line CI summary (PASS/FAIL + issue counts)")
	// Profiles
	cmd.Flags().StringVar(&assessProfile, "profile", "", "Preset profile: ci (fast, critical-only) or dev (comprehensive)")
	// Package mode for golangci-lint
	cmd.Flags().BoolVar(&assessPackageMode, "package-mode", false, "Force package-based linting mode (./pkg/...) instead of individual files")
	// Extended output
	cmd.Flags().BoolVar(&assessExtended, "extended", false, "Include detailed workplan information in output for debugging and automation")
}

func runAssess(cmd *cobra.Command, args []string) error {
	// Get flag values
	flags := cmd.Flags()
	assessFormat, _ := flags.GetString("format")

	// Suppress logs for JSON output to keep clean
	if assessFormat == "json" {
		// Reinitialize logger to only show errors for clean JSON output
		logger.Initialize(logger.Config{
			Level:     logger.ErrorLevel,
			UseColor:  false,
			JSON:      false,
			Component: "goneat",
			NoOp:      false,
		})
	}
	assessMode, _ := flags.GetString("mode")
	assessNoOp, _ = flags.GetBool("no-op")
	assessCheck, _ = flags.GetBool("check")
	assessFix, _ = flags.GetBool("fix")
	assessVerbose, _ = flags.GetBool("verbose")
	assessPriority, _ = flags.GetString("priority")
	assessFailOn, _ = flags.GetString("fail-on")
	assessTimeout, _ = flags.GetDuration("timeout")
	assessOutput, _ = flags.GetString("output")
	assessIncludeFiles, _ = flags.GetStringSlice("include")
	assessExcludeFiles, _ = flags.GetStringSlice("exclude")
	assessCategories, _ = flags.GetString("categories")
	assessHook, _ = flags.GetString("hook")
	assessHookManifest, _ = flags.GetString("hook-manifest")
	assessOpen, _ = flags.GetBool("open")
	assessConcurrency, _ = flags.GetInt("concurrency")
	assessConcurrencyPercent, _ = flags.GetInt("concurrency-percent")
	assessStagedOnly, _ = flags.GetBool("staged-only")
	assessNoIgnore, _ = flags.GetBool("no-ignore")
	assessForceInclude, _ = flags.GetStringSlice("force-include")
	assessSchemaEnableMeta, _ = flags.GetBool("schema-enable-meta")
	assessScope, _ = flags.GetBool("scope")
	assessCISummary, _ = flags.GetBool("ci-summary")
	assessProfile, _ = flags.GetString("profile")
	assessPackageMode, _ = flags.GetBool("package-mode")
	assessExtended, _ = flags.GetBool("extended")

	// Validate mode value
	switch assessMode {
	case "check", "fix", "no-op":
		// ok
	default:
		return fmt.Errorf("invalid mode: %s (must be no-op, check, or fix)", assessMode)
	}
	assessTrackSuppressions, _ = flags.GetBool("track-suppressions")

	// Determine target directory
	target := "."
	if len(args) > 0 {
		target = args[0]
	}

	// Validate target exists
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return fmt.Errorf("target directory does not exist: %s", target)
	}

	// Parse format
	var format assess.OutputFormat
	switch assessFormat {
	case "markdown":
		format = assess.FormatMarkdown
	case "json":
		format = assess.FormatJSON
	case "html":
		format = assess.FormatHTML
	case "both":
		format = assess.FormatBoth
	case "concise":
		format = assess.FormatConcise
	default:
		return fmt.Errorf("invalid format: %s (must be concise, markdown, json, html, or both)", assessFormat)
	}

	// Parse fail-on severity
	var failOnSeverity assess.IssueSeverity
	switch assessFailOn {
	case "critical":
		failOnSeverity = assess.SeverityCritical
	case "high":
		failOnSeverity = assess.SeverityHigh
	case "medium":
		failOnSeverity = assess.SeverityMedium
	case "low":
		failOnSeverity = assess.SeverityLow
	case "info":
		failOnSeverity = assess.SeverityInfo
	default:
		return fmt.Errorf("invalid fail-on severity: %s", assessFailOn)
	}

	// Parse and validate mode
	mode, err := parseAssessmentMode(assessMode, assessNoOp, assessCheck, assessFix)
	if err != nil {
		return err
	}

	// Create assessment configuration
	config := assess.AssessmentConfig{
		Mode:               mode,
		Verbose:            assessVerbose,
		Timeout:            assessTimeout,
		IncludeFiles:       assessIncludeFiles,
		ExcludeFiles:       assessExcludeFiles,
		NoIgnore:           assessNoIgnore,
		ForceInclude:       assessForceInclude,
		SchemaEnableMeta:   assessSchemaEnableMeta,
		Scope:              assessScope,
		PackageMode:        assessPackageMode,
		Extended:           assessExtended,
		PriorityString:     assessPriority,
		FailOnSeverity:     failOnSeverity,
		Concurrency:        assessConcurrency,
		ConcurrencyPercent: assessConcurrencyPercent,
		TrackSuppressions:  assessTrackSuppressions,
		LintNewFromRev:     strings.TrimSpace(assessLintNewFromRev),
	}

	// Add positional args to IncludeFiles
	if len(args) > 0 {
		config.IncludeFiles = append(config.IncludeFiles, args...)
	}

	// Apply profile defaults (non-intrusive; does not override explicitly set flags)
	if strings.TrimSpace(assessProfile) != "" {
		applyAssessProfile(strings.ToLower(strings.TrimSpace(assessProfile)), flags, &config)
	}

	// If limited to staged files, populate IncludeFiles from git staged set
	if assessStagedOnly {
		if len(config.IncludeFiles) == 0 { // do not override explicit includes
			if staged, err := getStagedFiles(); err == nil {
				if len(staged) > 0 {
					config.IncludeFiles = staged
				}
			} else {
				logger.Warn(fmt.Sprintf("Failed to resolve staged files: %v (continuing without staged-only)", err))
			}
		}
	}

	// Apply categories filter if provided
	if strings.TrimSpace(assessCategories) != "" {
		parts := strings.Split(assessCategories, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		config.SelectedCategories = parts
	}

	// Handle hook mode if specified
	if assessHook != "" {
		// Honor explicit --format. If not set, choose based on verbosity.
		formatFlagSet := flags.Changed("format")
		if !formatFlagSet {
			// Allow environment override for hook output mode
			if env := os.Getenv("GONEAT_HOOK_OUTPUT"); strings.TrimSpace(env) != "" {
				switch strings.ToLower(strings.TrimSpace(env)) {
				case "concise":
					format = assess.FormatConcise
				case "markdown":
					format = assess.FormatMarkdown
				case "json":
					format = assess.FormatJSON
				case "html":
					format = assess.FormatHTML
				case "both":
					format = assess.FormatBoth
				}
			} else if assessVerbose {
				format = assess.FormatMarkdown
			} else {
				format = assess.FormatConcise
			}
		}
		return runHookMode(cmd, assessHook, assessHookManifest, config, format)
	}

	// Hook defaults: lint new-only for pre-commit and pre-push unless overridden
	if assessHook != "" {
		if strings.TrimSpace(config.LintNewFromRev) == "" {
			// Default to HEAD~ for new-only gating
			config.LintNewFromRev = "HEAD~"
		}
	}

	// Suppress logs for JSON output to keep clean
	if format == assess.FormatJSON {
		logger.SetOutput(io.Discard)
	}

	// Create assessment engine
	engine := assess.NewAssessmentEngine()

	// Run assessment
	logger.Info(fmt.Sprintf("Starting comprehensive assessment of %s", target))
	report, err := engine.RunAssessment(cmd.Context(), target, config)
	if err != nil {
		return fmt.Errorf("assessment failed: %v", err)
	}

	// In hook mode, default to concise unless user explicitly chooses otherwise
	if assessHook != "" && assessFormat == "markdown" {
		format = assess.FormatConcise
	}

	// Format and output report
	formatter := assess.NewFormatter(format)
	formatter.SetTargetPath(target)

	if assessOutput != "" {
		// Validate output path to prevent path traversal
		assessOutput = filepath.Clean(assessOutput)
		if strings.Contains(assessOutput, "..") {
			return fmt.Errorf("invalid output path: contains path traversal")
		}
		// Write to file with restrictive permissions
		file, err := os.OpenFile(assessOutput, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			return fmt.Errorf("failed to create output file: %v", err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				logger.Warn(fmt.Sprintf("Failed to close output file: %v", err))
			}
		}()

		if err := formatter.WriteReport(file, report); err != nil {
			return fmt.Errorf("failed to write report: %v", err)
		}

		logger.Info(fmt.Sprintf("Assessment report written to %s", assessOutput))

		// Open in browser if requested
		if assessOpen && format == assess.FormatHTML {
			if err := openInBrowser(assessOutput); err != nil {
				logger.Warn(fmt.Sprintf("Failed to open report in browser: %v", err))
			} else {
				logger.Info("Report opened in default browser")
			}
		}
	} else {
		// Write to stdout
		if err := formatter.WriteReport(cmd.OutOrStdout(), report); err != nil {
			return fmt.Errorf("failed to write report: %v", err)
		}
	}

	// Concise schema summary for DX (preview)
	printSchemaSummary(report)

	// CI summary line
	if assessCISummary {
		pass := !shouldFail(report, failOnSeverity)
		c := countIssuesBySeverity(report)
		status := "FAIL"
		if pass {
			status = "PASS"
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "CI: %s | critical=%d high=%d medium=%d low=%d info=%d total=%d\n",
			status, c["critical"], c["high"], c["medium"], c["low"], c["info"], c["total"],
		)
	}

	// Check if we should fail based on severity
	if shouldFail(report, failOnSeverity) {
		msg := fmt.Sprintf("Assessment failed: found issues at or above %s severity", failOnSeverity)
		logger.Error(msg)
		return errors.New(msg)
	}

	return nil
}

// applyAssessProfile sets sensible defaults for profiles without overriding explicit flags
func applyAssessProfile(profile string, flags *pflag.FlagSet, cfg *assess.AssessmentConfig) {
	switch profile {
	case "ci":
		if !flags.Changed("fail-on") {
			cfg.FailOnSeverity = assess.SeverityCritical
		}
		if len(cfg.SelectedCategories) == 0 && !flags.Changed("categories") {
			cfg.SelectedCategories = []string{"format", "lint", "security"}
		}
		if !flags.Changed("staged-only") {
			// Favor staged-only if repository; leave discovery to include if populated elsewhere
			// No action needed - staged-only defaults are handled elsewhere
			_ = flags // Acknowledge flags parameter to avoid empty branch warning
		}
	case "dev":
		if !flags.Changed("fail-on") {
			cfg.FailOnSeverity = assess.SeverityLow
		}
		if len(cfg.SelectedCategories) == 0 && !flags.Changed("categories") {
			cfg.SelectedCategories = []string{"format", "lint", "security", "schema"}
		}
	}
}

// countIssuesBySeverity returns counts per severity for a report
func countIssuesBySeverity(report *assess.AssessmentReport) map[string]int {
	m := map[string]int{"critical": 0, "high": 0, "medium": 0, "low": 0, "info": 0, "total": 0}
	for _, cr := range report.Categories {
		for _, is := range cr.Issues {
			sev := strings.ToLower(string(is.Severity))
			if _, ok := m[sev]; ok {
				m[sev]++
			}
			m["total"]++
		}
	}
	return m
}

// shouldFail determines if the assessment should fail based on issue severity or category errors
func shouldFail(report *assess.AssessmentReport, failOnSeverity assess.IssueSeverity) bool {
	severityLevels := map[assess.IssueSeverity]int{
		assess.SeverityInfo:     0,
		assess.SeverityLow:      1,
		assess.SeverityMedium:   2,
		assess.SeverityHigh:     3,
		assess.SeverityCritical: 4,
	}

	failLevel := severityLevels[failOnSeverity]

	// Check for issues at or above the fail severity level
	for _, categoryResult := range report.Categories {
		for _, issue := range categoryResult.Issues {
			if severityLevels[issue.Severity] >= failLevel {
				return true
			}
		}
	}

	// Also check for category errors (e.g., lint config validation failures)
	for _, categoryResult := range report.Categories {
		if categoryResult.Status == "error" {
			logger.Error(fmt.Sprintf("Category %s failed with error: %s", categoryResult.Category, categoryResult.Error))
			return true
		}
	}

	return false
}

// printSchemaSummary prints a short schema issues summary (top files + first messages)
func printSchemaSummary(report *assess.AssessmentReport) {
	// Count total schema issues
	total := 0
	counts := map[string]int{}
	var first []assess.Issue
	for _, cr := range report.Categories {
		if cr.Category != assess.CategorySchema {
			continue
		}
		for _, is := range cr.Issues {
			total++
			counts[is.File]++
			if len(first) < 3 {
				first = append(first, is)
			}
		}
	}
	if total == 0 {
		return
	}
	logger.Info(fmt.Sprintf("Schema validation found %d issue(s)", total))
	// Print up to 3 top files
	printed := 0
	for file, cnt := range counts {
		logger.Info(fmt.Sprintf("  - %s: %d", file, cnt))
		printed++
		if printed >= 3 {
			break
		}
	}
	for _, is := range first {
		logger.Info(fmt.Sprintf("    %s: %s", is.SubCategory, is.Message))
	}
}

// runHookMode executes assessment in hook mode
func runHookMode(cmd *cobra.Command, hookType, manifestPath string, config assess.AssessmentConfig, outFormat assess.OutputFormat) error {
	logger.Info(fmt.Sprintf("Running assessment in hook mode: %s", hookType))

	// Validate hook type
	if hookType != "pre-commit" && hookType != "pre-push" {
		return fmt.Errorf("invalid hook type: %s (must be pre-commit or pre-push)", hookType)
	}

	// Load hook manifest if specified (full manifest to access optimization)
	var hookConfig *HookConfig
	if manifestPath != "" {
		var err error
		hookConfig, err = loadHookManifest(manifestPath)
		if err != nil {
			logger.Warn(fmt.Sprintf("Failed to load hook manifest: %v, using defaults", err))
			hookConfig = getDefaultHookConfig(hookType)
		}
	} else {
		hookConfig = getDefaultHookConfig(hookType)
	}

	// Determine effective categories for THIS hook (do not rely on global defaults)
	// Prefer categories parsed from the hook's args, then sensible per-hook defaults
	if hookConfig != nil {
		// Try to parse categories from the specific hook's args
		if cats := parseCategoriesFromHooks(hookConfig.Hooks, hookType); len(cats) > 0 {
			hookConfig.Categories = cats
		} else {
			// Fallback per hook defaults
			switch hookType {
			case "pre-push":
				hookConfig.Categories = []string{"format", "lint", "security"}
			default: // pre-commit
				hookConfig.Categories = []string{"format", "lint"}
			}
		}
		// Determine effective fail-on for this hook from args, fallback per-hook
		if val := parseFailOnFromHooks(hookConfig.Hooks, hookType); val != "" {
			hookConfig.FailOn = val
		} else {
			switch hookType {
			case "pre-push":
				hookConfig.FailOn = "high"
			default: // pre-commit
				hookConfig.FailOn = "medium"
			}
		}

		// Keep assessment config aligned for accurate metadata and behavior
		switch hookConfig.FailOn {
		case "critical":
			config.FailOnSeverity = assess.SeverityCritical
		case "high":
			config.FailOnSeverity = assess.SeverityHigh
		case "medium":
			config.FailOnSeverity = assess.SeverityMedium
		case "low":
			config.FailOnSeverity = assess.SeverityLow
		}
	}

	// Filter categories based on computed hook configuration
	config = filterCategoriesForHook(config, hookType, hookConfig)

	// Apply staged-only optimization if configured
	if hookConfig.Optimization.OnlyChangedFiles {
		if staged, err := getStagedFiles(); err == nil {
			if len(staged) > 0 {
				config.IncludeFiles = staged
			}
		} else {
			logger.Warn(fmt.Sprintf("Failed to resolve staged files: %v (continuing without staged-only)", err))
		}
	}

	// Default lint to new-only in hook mode unless explicitly set
	if strings.TrimSpace(config.LintNewFromRev) == "" {
		config.LintNewFromRev = "HEAD~"
	}

	// Set security configuration for hook mode
	config.SecurityExcludeFixtures = true
	config.SecurityFixturePatterns = []string{"tests/fixtures/", "test-fixtures/"}

	// Create assessment engine
	engine := assess.NewAssessmentEngine()

	// Run assessment
	target := "."
	report, err := engine.RunAssessment(cmd.Context(), target, config)
	if err != nil {
		return fmt.Errorf("hook assessment failed: %v", err)
	}

	// Format and output report (concise default provided by caller)
	formatter := assess.NewFormatter(outFormat)
	formatter.SetTargetPath(target)
	if err := formatter.WriteReport(cmd.OutOrStdout(), report); err != nil {
		return fmt.Errorf("failed to write hook report: %v", err)
	}

	// Fail hook when issues meet configured threshold (use .goneat/hooks.yaml to tune per-repo)
	if shouldFailHook(report, hookConfig) {
		logger.Error(fmt.Sprintf("Hook %s failed: found issues requiring attention", hookType))
		os.Exit(1)
	}

	return nil
}

// HookConfig represents hook configuration (parsed from .goneat/hooks.yaml)
type HookConfig struct {
	// Legacy/simple fields for runHookMode filtering
	Categories []string `yaml:"categories"`
	FailOn     string   `yaml:"fail_on"`

	// Schema-driven fields (subset as needed)
	Hooks map[string][]struct {
		Command  string   `yaml:"command"`
		Args     []string `yaml:"args"`
		Fallback string   `yaml:"fallback"`
	} `yaml:"hooks"`

	Optimization struct {
		OnlyChangedFiles bool   `yaml:"only_changed_files"`
		CacheResults     bool   `yaml:"cache_results"`
		Parallel         string `yaml:"parallel"`
	} `yaml:"optimization"`
}

// parseCategoriesFromHooks extracts --categories value from hook args
func parseCategoriesFromHooks(hooks map[string][]struct {
	Command  string   `yaml:"command"`
	Args     []string `yaml:"args"`
	Fallback string   `yaml:"fallback"`
}, hookType string) []string {
	var out []string
	if hookConfigs, exists := hooks[hookType]; exists {
		for _, hookConfig := range hookConfigs {
			if hookConfig.Command == "assess" {
				for i, arg := range hookConfig.Args {
					if arg == "--categories" && i+1 < len(hookConfig.Args) {
						raw := strings.TrimSpace(hookConfig.Args[i+1])
						if raw != "" {
							parts := strings.Split(raw, ",")
							for _, p := range parts {
								pp := strings.TrimSpace(p)
								if pp != "" {
									out = append(out, pp)
								}
							}
						}
						return out
					}
				}
			}
		}
	}
	return out
}

// parseFailOnFromHooks extracts --fail-on value from hook args
func parseFailOnFromHooks(hooks map[string][]struct {
	Command  string   `yaml:"command"`
	Args     []string `yaml:"args"`
	Fallback string   `yaml:"fallback"`
}, hookType string) string {
	if hookConfigs, exists := hooks[hookType]; exists {
		for _, hookConfig := range hookConfigs {
			if hookConfig.Command == "assess" {
				for i, arg := range hookConfig.Args {
					if arg == "--fail-on" && i+1 < len(hookConfig.Args) {
						return hookConfig.Args[i+1]
					}
				}
			}
		}
	}
	return ""
}

// loadHookManifest loads hook configuration from YAML file
func loadHookManifest(path string) (*HookConfig, error) {
	// Validate path to prevent path traversal
	path = filepath.Clean(path)
	if strings.Contains(path, "..") {
		return nil, fmt.Errorf("invalid manifest path: contains path traversal")
	}
	data, err := os.ReadFile(path) // #nosec G304 -- path cleaned and traversal rejected above
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var cfg HookConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	// If no simple fields provided, set sensible defaults based on hook sections
	if len(cfg.Categories) == 0 {
		switch {
		case cfg.Hooks != nil && len(cfg.Hooks["pre-push"]) > 0:
			cfg.Categories = []string{"format", "lint", "security"}
		default:
			cfg.Categories = []string{"format", "lint"}
		}
	}
	if cfg.FailOn == "" {
		// Try to parse fail-on from hook args first
		if cfg.Hooks != nil {
			if failOn := parseFailOnFromHooks(cfg.Hooks, "pre-push"); failOn != "" {
				cfg.FailOn = failOn
			} else if failOn := parseFailOnFromHooks(cfg.Hooks, "pre-commit"); failOn != "" {
				cfg.FailOn = failOn
			} else {
				// Fall back to defaults
				if len(cfg.Hooks["pre-push"]) > 0 {
					cfg.FailOn = "critical"
				} else {
					cfg.FailOn = "high"
				}
			}
		} else {
			// No hooks section, use defaults
			cfg.FailOn = "high"
		}
	}
	return &cfg, nil
}

// getDefaultHookConfig returns default hook configuration
func getDefaultHookConfig(hookType string) *HookConfig {
	switch hookType {
	case "pre-commit":
		return &HookConfig{
			Categories: []string{"format", "lint", "dates", "tools"},
			FailOn:     "high",
		}
	case "pre-push":
		return &HookConfig{
			// Align with documented defaults: include maturity and repo-status
			Categories: []string{"format", "lint", "security", "dates", "tools", "maturity", "repo-status"},
			FailOn:     "high",
		}
	default:
		return &HookConfig{
			Categories: []string{"format", "lint"},
			FailOn:     "high",
		}
	}
}

// filterCategoriesForHook filters assessment config for specific hook
func filterCategoriesForHook(config assess.AssessmentConfig, _ string, hookConfig *HookConfig) assess.AssessmentConfig {
	// Restrict to selected categories for this hook
	if len(hookConfig.Categories) > 0 {
		// Apply explicit category filter
		config.SelectedCategories = append([]string(nil), hookConfig.Categories...)

		// Also set simple priorities to prefer these categories
		priorityParts := make([]string, len(hookConfig.Categories))
		for i, category := range hookConfig.Categories {
			priorityParts[i] = fmt.Sprintf("%s=1", category)
		}
		config.PriorityString = strings.Join(priorityParts, ",")
	}

	return config
}

// getStagedFiles returns a list of staged files (Added, Copied, Modified, Renamed) for the next commit
func getStagedFiles() ([]string, error) {
	cmd := exec.Command("git", "diff", "--cached", "--name-only", "--diff-filter=ACMR")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --cached failed: %w", err)
	}
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	scanner.Split(bufio.ScanLines)
	var files []string
	for scanner.Scan() {
		path := strings.TrimSpace(scanner.Text())
		if path != "" {
			files = append(files, path)
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return nil, scanErr
	}
	return files, nil
}

// shouldFailHook determines if hook should fail based on configuration
func shouldFailHook(report *assess.AssessmentReport, config *HookConfig) bool {
	failLevel := assess.SeverityHigh // default
	switch config.FailOn {
	case "critical":
		failLevel = assess.SeverityCritical
	case "high":
		failLevel = assess.SeverityHigh
	case "medium":
		failLevel = assess.SeverityMedium
	case "low":
		failLevel = assess.SeverityLow
	}

	severityLevels := map[assess.IssueSeverity]int{
		assess.SeverityCritical: 4,
		assess.SeverityHigh:     3,
		assess.SeverityMedium:   2,
		assess.SeverityLow:      1,
		assess.SeverityInfo:     0,
	}

	failThreshold := severityLevels[failLevel]

	for _, categoryResult := range report.Categories {
		for _, issue := range categoryResult.Issues {
			if severityLevels[issue.Severity] >= failThreshold {
				return true
			}
		}
	}

	return false
}

// openInBrowser opens the HTML report in the default browser
func openInBrowser(filePath string) error {
	// Clean file path to prevent path traversal
	filePath = filepath.Clean(filePath)
	var cmd string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "linux":
		cmd = "xdg-open"
	case "windows":
		cmd = "start"
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
	return exec.Command(cmd, filePath).Start() // #nosec G204
}

// parseAssessmentMode parses and validates the assessment mode from flags
func parseAssessmentMode(modeStr string, noOp, check, fix bool) (assess.AssessmentMode, error) {

	// Count how many modes are specified
	modeCount := 0
	if noOp {
		modeCount++
	}
	if check {
		modeCount++
	}
	if fix {
		modeCount++
	}
	if modeStr != "check" {
		modeCount++
	} // Default is check, so don't count it

	if modeCount > 1 {
		return "", fmt.Errorf("multiple assessment modes specified - use only one of: --no-op, --check, --fix, or --mode")
	}

	// Determine the mode
	if noOp {
		return assess.AssessmentModeNoOp, nil
	}
	if fix {
		return assess.AssessmentModeFix, nil
	}
	if check {
		return assess.AssessmentModeCheck, nil
	}

	// Parse mode string
	switch modeStr {
	case "no-op":
		return assess.AssessmentModeNoOp, nil
	case "check":
		return assess.AssessmentModeCheck, nil
	case "fix":
		return assess.AssessmentModeFix, nil
	default:
		return "", fmt.Errorf("invalid mode: %s (must be no-op, check, or fix)", modeStr)
	}
}
