/*
Copyright © 2025 3 Leaps <info@3leaps.com>
*/
package cmd

import (
	"os"
	"strings"

	"github.com/3leaps/goneat/pkg/exitcode"
	"github.com/3leaps/goneat/pkg/logger"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
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
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		initializeLogger(cmd)
	},
}

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
	// Add global flags
	rootCmd.PersistentFlags().String("log-level", "info", "Set log level (trace|debug|info|warn|error)")
	rootCmd.PersistentFlags().Bool("json", false, "Output logs in JSON format")
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().Bool("no-op", false, "Run tasks without making changes (assessment mode)")

	// Wire Cobra's built-in --version using dynamically computed version string
	ver := "unknown"
	if v, _, err := getVersionFromSources(); err == nil && strings.TrimSpace(v) != "" {
		ver = v
	}
	rootCmd.Version = ver
	rootCmd.SetVersionTemplate("goneat {{.Version}}\n")
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
