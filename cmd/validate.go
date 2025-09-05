package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/3leaps/goneat/internal/assess"
	"github.com/3leaps/goneat/internal/ops"
	"github.com/3leaps/goneat/pkg/logger"
	"github.com/spf13/cobra"
)

var (
	validateFormat       string
	validateVerbose      bool
	validateFailOn       string
	validateTimeout      time.Duration
	validateOutput       string
	validateIncludeFiles []string
	validateExcludeFiles []string
	validateAutoDetect   bool
)

var validateCmd = &cobra.Command{
	Use:   "validate [target]",
	Short: "Schema-aware validation (preview)",
	Long:  "Validate JSON/YAML files with syntax checks and schema-aware processing (preview).",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runValidate,
}

func init() {
	rootCmd.AddCommand(validateCmd)

	caps := ops.GetDefaultCapabilities(ops.GroupNeat, ops.CategoryValidation)
	if err := ops.RegisterCommandWithTaxonomy("validate", ops.GroupNeat, ops.CategoryValidation, caps, validateCmd, "Schema-aware validation (preview)"); err != nil {
		panic(fmt.Sprintf("Failed to register validate command: %v", err))
	}

	validateCmd.Flags().StringVar(&validateFormat, "format", "markdown", "Output format (markdown, json, html, both)")
	validateCmd.Flags().BoolVarP(&validateVerbose, "verbose", "v", false, "Verbose output")
	validateCmd.Flags().StringVar(&validateFailOn, "fail-on", "high", "Fail if issues at or above severity (critical, high, medium, low)")
	validateCmd.Flags().DurationVar(&validateTimeout, "timeout", 3*time.Minute, "Validation timeout")
	validateCmd.Flags().StringVarP(&validateOutput, "output", "o", "", "Output file (default: stdout)")
	validateCmd.Flags().StringSliceVar(&validateIncludeFiles, "include", []string{}, "Include only these files/patterns")
	validateCmd.Flags().StringSliceVar(&validateExcludeFiles, "exclude", []string{}, "Exclude these files/patterns")
	validateCmd.Flags().BoolVar(&validateAutoDetect, "auto-detect", false, "Auto-detect schema files (preview; uses extensions)")
}

func runValidate(cmd *cobra.Command, args []string) error {
	target := "."
	if len(args) > 0 {
		target = args[0]
	}
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return fmt.Errorf("target does not exist: %s", target)
	}

	// Output format
	var outFmt assess.OutputFormat
	switch strings.ToLower(validateFormat) {
	case "markdown":
		outFmt = assess.FormatMarkdown
	case "json":
		outFmt = assess.FormatJSON
	case "html":
		outFmt = assess.FormatHTML
	case "both":
		outFmt = assess.FormatBoth
	case "concise":
		outFmt = assess.FormatConcise
	default:
		return fmt.Errorf("invalid format: %s", validateFormat)
	}

	// Fail-on severity
	var failOn assess.IssueSeverity
	switch validateFailOn {
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
		return fmt.Errorf("invalid fail-on severity: %s", validateFailOn)
	}

	cfg := assess.AssessmentConfig{
		Mode:               assess.AssessmentModeCheck,
		Verbose:            validateVerbose,
		Timeout:            validateTimeout,
		IncludeFiles:       validateIncludeFiles,
		ExcludeFiles:       validateExcludeFiles,
		FailOnSeverity:     failOn,
		SelectedCategories: []string{string(assess.CategorySchema)},
	}

	// Engine and run
	engine := assess.NewAssessmentEngine()
	logger.Info(fmt.Sprintf("Validating schema files in %s", target))
	report, err := engine.RunAssessment(cmd.Context(), target, cfg)
	if err != nil {
		return fmt.Errorf("validation failed: %v", err)
	}

	// Output
	formatter := assess.NewFormatter(outFmt)
	formatter.SetTargetPath(target)
	if validateOutput != "" {
		f, err := os.Create(validateOutput)
		if err != nil {
			return fmt.Errorf("failed to create output: %v", err)
		}
		defer f.Close()
		if err := formatter.WriteReport(f, report); err != nil {
			return fmt.Errorf("write report: %v", err)
		}
	} else {
		if err := formatter.WriteReport(cmd.OutOrStdout(), report); err != nil {
			return fmt.Errorf("write report: %v", err)
		}
	}

	// Fail-on gate
	if shouldFail(report, failOn) {
		return fmt.Errorf("validation failed: found issues at or above %s", failOn)
	}
	return nil
}
