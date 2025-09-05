/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/3leaps/goneat/internal/assess"
	"github.com/3leaps/goneat/internal/ops"
	"github.com/3leaps/goneat/pkg/logger"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// assessCmd represents the assess command
var assessCmd = &cobra.Command{
	Use:   "assess [target]",
	Short: "Comprehensive codebase assessment and workflow planning",
	Long: `Assess performs a comprehensive analysis of your codebase using all available
formatting, linting, and analysis tools. It generates structured reports with
prioritized remediation workflows and parallelization opportunities.

Examples:
  goneat assess                    # Assess current directory
  goneat assess /path/to/project   # Assess specific directory
  goneat assess --format json      # JSON output for automation
  goneat assess --no-op            # Assessment mode only
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
}

func runAssess(cmd *cobra.Command, args []string) error {
	// Get flag values
	flags := cmd.Flags()
	assessFormat, _ = flags.GetString("format")
	assessMode, _ = flags.GetString("mode")
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
		PriorityString:     assessPriority,
		FailOnSeverity:     failOnSeverity,
		Concurrency:        assessConcurrency,
		ConcurrencyPercent: assessConcurrencyPercent,
		TrackSuppressions:  assessTrackSuppressions,
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
		// Write to file
		file, err := os.Create(assessOutput)
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

	// Check if we should fail based on severity
	if shouldFail(report, failOnSeverity) {
		logger.Error(fmt.Sprintf("Assessment failed: found issues at or above %s severity", failOnSeverity))
		os.Exit(1)
	}

	return nil
}

// shouldFail determines if the assessment should fail based on issue severity
func shouldFail(report *assess.AssessmentReport, failOnSeverity assess.IssueSeverity) bool {
	severityLevels := map[assess.IssueSeverity]int{
		assess.SeverityInfo:     0,
		assess.SeverityLow:      1,
		assess.SeverityMedium:   2,
		assess.SeverityHigh:     3,
		assess.SeverityCritical: 4,
	}

	failLevel := severityLevels[failOnSeverity]

	for _, categoryResult := range report.Categories {
		for _, issue := range categoryResult.Issues {
			if severityLevels[issue.Severity] >= failLevel {
				return true
			}
		}
	}

	return false
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

	// Filter categories based on hook type
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

	// Check if we should fail based on hook configuration
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
	data, err := os.ReadFile(path)
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
			Categories: []string{"format", "lint"},
			FailOn:     "high",
		}
	case "pre-push":
		return &HookConfig{
			Categories: []string{"format", "lint", "security"},
			FailOn:     "critical",
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
