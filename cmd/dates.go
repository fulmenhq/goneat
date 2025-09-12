/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/fulmenhq/goneat/internal/assess"
	"github.com/fulmenhq/goneat/internal/dates"
	"github.com/fulmenhq/goneat/internal/ops"
	"github.com/fulmenhq/goneat/pkg/exitcode"
	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/spf13/cobra"
)

var datesCmd = &cobra.Command{
	Use:   "dates",
	Short: "Validate and fix date consistency across your codebase",
	Long: `Dates validates dates in documentation, changelogs, and config files.
Detects future dates, stale entries, and format issues.
Supports auto-fixing for common problems.`,
}

var datesCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check for date issues without making changes",
	RunE:  runDatesCheck,
}

var datesFixCmd = &cobra.Command{
	Use:   "fix",
	Short: "Automatically fix date issues where possible",
	RunE:  runDatesFix,
}

var datesAssessCmd = &cobra.Command{
	Use:   "assess",
	Short: "Run dates as part of assessment workflow",
	RunE:  runDatesAssess,
}

func init() {
	datesCmd.AddCommand(datesCheckCmd, datesFixCmd, datesAssessCmd)

	// Global flags for dates (inherited from root, but add specific)
	datesCheckCmd.Flags().Bool("verbose", false, "Verbose output for debugging")
	datesFixCmd.Flags().Bool("dry-run", false, "Show what would be fixed")
	datesFixCmd.Flags().Bool("backup", true, "Create backups before fixing")

	rootCmd.AddCommand(datesCmd)

	// Register with ops registry
	caps := ops.GetDefaultCapabilities(ops.GroupNeat, ops.CategoryValidation)
	if err := ops.RegisterCommandWithTaxonomy("dates", ops.GroupNeat, ops.CategoryValidation, caps, datesCmd, "Validate and fix date consistency"); err != nil {
		logger.Error("Failed to register dates command", logger.Err(err))
	}
}

func runDatesCheck(cmd *cobra.Command, args []string) error {
	target, _ := cmd.Flags().GetString("target")
	if target == "" {
		target = "."
	}

	runner := dates.NewDatesRunner()
	ctx := context.Background()
	config := assess.AssessmentConfig{Mode: assess.AssessmentModeCheck}

	result, err := runner.Assess(ctx, target, config)
	if err != nil {
		return err
	}

	if result.Success {
		fmt.Println("No date issues found.")
		return nil
	}

	// Output issues
	for _, issue := range result.Issues {
		fmt.Printf("[%s] %s: %s (line %d)\n", issue.Severity, issue.File, issue.Message, issue.Line)
	}
	os.Exit(exitcode.ValidationError)
	return nil
}

func runDatesFix(cmd *cobra.Command, args []string) error {
	// Phase 1: Basic fix not implemented; stub
	fmt.Println("Fix mode: TODO - implement auto-fix logic")
	return nil
}

func runDatesAssess(cmd *cobra.Command, args []string) error {
	// Delegate to assess command with dates category
	fmt.Println("Assess mode: Run 'goneat assess --categories dates'")
	return nil
}
