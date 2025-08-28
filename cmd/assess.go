/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/3leaps/goneat/internal/assess"
	"github.com/3leaps/goneat/internal/ops"
	"github.com/3leaps/goneat/pkg/logger"
	"github.com/spf13/cobra"
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
	assessFormat       string
	assessNoOp         bool
	assessVerbose      bool
	assessPriority     string
	assessFailOn       string
	assessTimeout      time.Duration
	assessOutput       string
	assessIncludeFiles []string
	assessExcludeFiles []string
)

func init() {
	rootCmd.AddCommand(assessCmd)

	// Register command in ops registry
	ops.RegisterCommand("assess", ops.GroupUtility, assessCmd, "Comprehensive codebase assessment and workflow planning")

	// Assessment flags
	assessCmd.Flags().StringVar(&assessFormat, "format", "markdown", "Output format (markdown, json, both)")
	assessCmd.Flags().BoolVar(&assessNoOp, "no-op", false, "Run in assessment mode without making changes")
	assessCmd.Flags().BoolVarP(&assessVerbose, "verbose", "v", false, "Verbose output")
	assessCmd.Flags().StringVar(&assessPriority, "priority", "", "Custom priority string (e.g., 'security=1,format=2')")
	assessCmd.Flags().StringVar(&assessFailOn, "fail-on", "critical", "Fail if issues at or above severity (critical, high, medium, low)")
	assessCmd.Flags().DurationVar(&assessTimeout, "timeout", 5*time.Minute, "Assessment timeout")
	assessCmd.Flags().StringVarP(&assessOutput, "output", "o", "", "Output file (default: stdout)")
	assessCmd.Flags().StringSliceVar(&assessIncludeFiles, "include", []string{}, "Include only these files/patterns")
	assessCmd.Flags().StringSliceVar(&assessExcludeFiles, "exclude", []string{}, "Exclude these files/patterns")
}

func runAssess(cmd *cobra.Command, args []string) error {
	// Determine target directory
	target := "."
	if len(args) > 0 {
		target = args[0]
	}

	// Validate target exists
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return fmt.Errorf("target directory does not exist: %s", target)
	}

	// Parse output format
	var format assess.OutputFormat
	switch assessFormat {
	case "markdown":
		format = assess.FormatMarkdown
	case "json":
		format = assess.FormatJSON
	case "both":
		format = assess.FormatBoth
	default:
		return fmt.Errorf("invalid format: %s (must be markdown, json, or both)", assessFormat)
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

	// Create assessment configuration
	config := assess.AssessmentConfig{
		NoOp:           assessNoOp,
		Verbose:        assessVerbose,
		Timeout:        assessTimeout,
		IncludeFiles:   assessIncludeFiles,
		ExcludeFiles:   assessExcludeFiles,
		PriorityString: assessPriority,
		FailOnSeverity: failOnSeverity,
	}

	// Create assessment engine
	engine := assess.NewAssessmentEngine()

	// Run assessment
	logger.Info(fmt.Sprintf("Starting comprehensive assessment of %s", target))
	report, err := engine.RunAssessment(cmd.Context(), target, config)
	if err != nil {
		return fmt.Errorf("assessment failed: %w", err)
	}

	// Format and output report
	formatter := assess.NewFormatter(format)

	if assessOutput != "" {
		// Write to file
		file, err := os.Create(assessOutput)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()

		if err := formatter.WriteReport(file, report); err != nil {
			return fmt.Errorf("failed to write report: %w", err)
		}

		logger.Info(fmt.Sprintf("Assessment report written to %s", assessOutput))
	} else {
		// Write to stdout
		if err := formatter.WriteReport(cmd.OutOrStdout(), report); err != nil {
			return fmt.Errorf("failed to write report: %w", err)
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
