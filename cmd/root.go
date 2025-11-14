/*
Copyright © 2025 3 Leaps <info@3leaps.com>
*/
package cmd

import (
	"os"
	"strings"

	"github.com/fulmenhq/goneat/internal/ops"
	"github.com/fulmenhq/goneat/pkg/buildinfo"
	"github.com/fulmenhq/goneat/pkg/exitcode"
	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/spf13/cobra"
)

// newRootCommand creates a fresh root command instance.
// This factory pattern allows tests to create isolated command trees without shared state.
func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "goneat",
		Short: "Unified Go-based formatting and validation tool",
		Long: `Goneat is a unified tool for formatting and validating multiple languages/formats,
Inspired by Biome. It bundles existing OSS tools transparently for data engineering workflows.

Examples:
   goneat version     # Show version (use --extended for build info)
   goneat --version   # Show version (same as 'goneat version')
   goneat envinfo     # Show system information
   goneat format      # Format files
   goneat assess      # Comprehensive codebase assessment`,
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			initializeLogger(cmd)
		},
	}

	// Add global flags
	cmd.PersistentFlags().String("log-level", "info", "Set log level (trace|debug|info|warn|error)")
	cmd.PersistentFlags().Bool("json", false, "Output logs in JSON format")
	cmd.PersistentFlags().Bool("no-color", false, "Disable colored output")
	cmd.PersistentFlags().Bool("no-op", false, "Run tasks without making changes (assessment mode)")

	// Wire Cobra's built-in --version using goneat's binary version
	cmd.Version = buildinfo.BinaryVersion
	cmd.SetVersionTemplate("goneat {{.Version}}\n")

	// Grouped help by command group (Neat → Workflow → Support)
	cmd.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		reg := ops.GetRegistry()
		// Header
		cmd.Println(cmd.Long)
		cmd.Println()
		cmd.Println("Tip: Use 'goneat docs' to view built-in guides for commands, hooks, and workflows.")
		cmd.Println("Note: 'content' is for curation/building of docs; 'docs' is for viewing.")
		cmd.Println()
		cmd.Println("Functional Commands (Neat):")
		for _, c := range reg.GetCommandsByGroup(ops.GroupNeat) {
			cmd.Printf("  %-12s %s\n", c.Name, c.Description)
		}
		cmd.Println()
		cmd.Println("Workflow Commands:")
		for _, c := range reg.GetCommandsByGroup(ops.GroupWorkflow) {
			cmd.Printf("  %-12s %s\n", c.Name, c.Description)
		}
		cmd.Println()
		cmd.Println("Support Commands (use --all to list details):")
		for _, c := range reg.GetCommandsByGroup(ops.GroupSupport) {
			cmd.Printf("  %-12s %s\n", c.Name, c.Description)
		}
		cmd.Println()
		cmd.Println("Flags:")
		cmd.Print(cmd.UsageString())
	})

	return cmd
}

// registerSubcommands adds all subcommands to the root command.
// This is called from init() for production and can be called explicitly in tests.
func registerSubcommands(cmd *cobra.Command) {
	cmd.AddCommand(versionCmd)
	cmd.AddCommand(formatCmd)
	cmd.AddCommand(datesCmd)
	cmd.AddCommand(securityCmd)
	cmd.AddCommand(doctorCmd)
	cmd.AddCommand(hooksCmd)
	cmd.AddCommand(pathfinderCmd)
	cmd.AddCommand(assessCmd)
	cmd.AddCommand(validateCmd)
	cmd.AddCommand(contentCmd)
	cmd.AddCommand(docsCmd)
	cmd.AddCommand(schemaCmd)
	cmd.AddCommand(dependenciesCmd)
	cmd.AddCommand(envinfoCmd)
	cmd.AddCommand(homeCmd)
	cmd.AddCommand(prettyCmd)
	cmd.AddCommand(initCmd)
	cmd.AddCommand(infoCmd)
	cmd.AddCommand(serverCmd)
	cmd.AddCommand(ssotCmd)
	cmd.AddCommand(guardianCmd)
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = newRootCommand()

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		logger.Error("Command execution failed", logger.Err(err))
		os.Exit(exitcode.GeneralError)
	}
}

func init() {
	// Register all subcommands with the production rootCmd
	registerSubcommands(rootCmd)
}

// getVersionFromSources tries to get version from multiple sources in priority order
// Version helper functions live in version.go; root.go references them.

// printVersion prints version information and exits
// Version printing handled by Cobra's built-in mechanism via rootCmd.Version

// initializeLogger sets up the logger based on command flags
func initializeLogger(cmd *cobra.Command) {
	logLevelStr, _ := cmd.Flags().GetString("log-level")
	jsonLogs, _ := cmd.Flags().GetBool("json")
	noColor, _ := cmd.Flags().GetBool("no-color")
	noOp, _ := cmd.Flags().GetBool("no-op")

	// Parse log level
	var logLevel logger.Level
	switch strings.ToLower(logLevelStr) {
	case "trace":
		logLevel = logger.TraceLevel
	case "debug":
		logLevel = logger.DebugLevel
	case "info":
		logLevel = logger.InfoLevel
	case "warn":
		logLevel = logger.WarnLevel
	case "error":
		logLevel = logger.ErrorLevel
	default:
		logLevel = logger.InfoLevel
	}

	// Initialize logger
	config := logger.Config{
		Level:     logLevel,
		UseColor:  !noColor,
		JSON:      jsonLogs,
		Component: "goneat",
		NoOp:      noOp,
	}

	if err := logger.Initialize(config); err != nil {
		// Fallback to stderr
		if _, writeErr := os.Stderr.WriteString("Failed to initialize logger: " + err.Error() + "\n"); writeErr != nil {
			// Best effort: nothing else we can do here
			_ = writeErr
		}
		os.Exit(exitcode.ConfigError)
	}
}
