/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package cmd

import (
	"fmt"

	"github.com/fulmenhq/goneat/internal/ops"
	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/fulmenhq/goneat/pkg/ssot"
	"github.com/spf13/cobra"
)

var ssotCmd = &cobra.Command{
	Use:   "ssot",
	Short: "Manage Single Source of Truth (SSOT) asset synchronization",
	Long: `Sync documentation, schemas, and other assets from SSOT repositories.

This command enables goneat to pull canonical assets from upstream SSOT repositories
like Crucible, following the same pattern as fuldx. Configuration is managed via
.goneat/ssot-consumer.yaml (production) and .goneat/ssot-consumer.local.yaml (local overrides).

Examples:
  # Sync from crucible (uses .crucible/sync.yaml)
  goneat ssot sync

  # Sync with local override path
  goneat ssot sync --local-path ../crucible

  # Dry run to see what would be synced
  goneat ssot sync --dry-run

Configuration Priority:
  1. Command-line flags (--local-path)
  2. .goneat/ssot-consumer.local.yaml (gitignored, for local dev)
  3. .goneat/ssot-consumer.yaml (production config, committed)
  4. Environment variables (GONEAT_SSOT_CONSUMER_<SOURCE>_LOCAL_PATH)
`,
}

var ssotSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync assets from SSOT repositories",
	Long: `Sync documentation, schemas, and other assets from configured SSOT repositories.

Reads configuration from .goneat/ssot-consumer.yaml with optional local overrides from
.goneat/ssot-consumer.local.yaml. Supports both remote (git clone) and local filesystem sources.

Configuration Files:
  .goneat/ssot-consumer.yaml         - Production config (committed to git)
  .goneat/ssot-consumer.local.yaml   - Local overrides (gitignored)

The sync process:
  1. Load .goneat/ssot-consumer.yaml
  2. Merge .goneat/ssot-consumer.local.yaml if present
  3. Apply command-line flag overrides
  4. Copy assets from source to destination directories

Exit Codes:
  0 - Success
  1 - Configuration error
  2 - Source not found
  3 - Sync operation failed`,
	RunE: runSSOTSync,
}

var (
	flagSSOTLocalPath string
	flagSSOTDryRun    bool
	flagSSOTVerbose   bool
)

func init() {
	// Register ssot command under support/environment
	capabilities := ops.GetDefaultCapabilities(ops.GroupSupport, ops.CategoryEnvironment)
	if err := ops.RegisterCommandWithTaxonomy("ssot", ops.GroupSupport, ops.CategoryEnvironment, capabilities, ssotCmd, "Manage SSOT asset synchronization"); err != nil {
		panic(fmt.Sprintf("Failed to register ssot command: %v", err))
	}

	// Attach to root
	rootCmd.AddCommand(ssotCmd)

	// Subcommands
	ssotCmd.AddCommand(ssotSyncCmd)

	// Flags for sync subcommand
	ssotSyncCmd.Flags().StringVar(&flagSSOTLocalPath, "local-path", "", "Local path to source repository (overrides config)")
	ssotSyncCmd.Flags().BoolVar(&flagSSOTDryRun, "dry-run", false, "Show what would be synced without performing sync")
	ssotSyncCmd.Flags().BoolVar(&flagSSOTVerbose, "verbose", false, "Show verbose output including file-level operations")
}

func runSSOTSync(cmd *cobra.Command, args []string) error {
	// Load configuration with local override support
	config, err := ssot.LoadSyncConfig()
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to load sync configuration: %v", err))
		return fmt.Errorf("configuration error: %w", err)
	}

	// Apply command-line overrides
	if flagSSOTLocalPath != "" {
		// Apply to all sources
		for i := range config.Sources {
			config.Sources[i].LocalPath = flagSSOTLocalPath
		}
		logger.Info(fmt.Sprintf("Using local path override: %s", flagSSOTLocalPath))
	}

	// Validate source exists
	if err := ssot.ValidateSource(config); err != nil {
		logger.Error(fmt.Sprintf("Source validation failed: %v", err))
		return fmt.Errorf("source not found: %w", err)
	}

	// Perform sync
	opts := ssot.SyncOptions{
		Config:  config,
		DryRun:  flagSSOTDryRun,
		Verbose: flagSSOTVerbose,
	}

	result, err := ssot.PerformSync(opts)
	if err != nil {
		logger.Error(fmt.Sprintf("Sync failed: %v", err))
		return fmt.Errorf("sync operation failed: %w", err)
	}

	// Report results
	if flagSSOTDryRun {
		logger.Info("Dry run completed")
		logger.Info(fmt.Sprintf("Would sync %d source(s)", len(config.Sources)))
	} else {
		logger.Info("Sync completed successfully")
		if flagSSOTVerbose {
			logger.Info(fmt.Sprintf("Files copied: %d", result.FilesCopied))
			logger.Info(fmt.Sprintf("Files removed: %d", result.FilesRemoved))
			logger.Info(fmt.Sprintf("Sources synced: %d", len(result.Sources)))
		}
	}

	return nil
}
