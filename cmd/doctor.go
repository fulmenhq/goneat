/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	intdoctor "github.com/fulmenhq/goneat/internal/doctor"
	"github.com/fulmenhq/goneat/internal/ops"
	"github.com/fulmenhq/goneat/pkg/buildinfo"
	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/fulmenhq/goneat/pkg/tools"
	"github.com/fulmenhq/goneat/pkg/tools/metadata"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnostics and tooling checks",
	Long:  "Run diagnostics and verify/install external tools required by goneat features.",
}

var doctorToolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Check or install external tools required by goneat",
	Long: `Verify (and optionally install) external tools used by goneat features.

Current scopes:
- security:     gosec, govulncheck, gitleaks
- format:       goimports, gofmt (gofmt is bundled with Go toolchain)
- foundation: ripgrep, jq, go-licenses (cross-platform CLI tools)
- all:          all tools from all scopes

Package Manager Installation:
- --install-package-managers: Install missing package managers (scoop on Windows)
- Requires --yes for non-interactive installation
- Package managers are installed before tools to ensure PATH is updated

PATH Troubleshooting:
If tools are installed but not found, check your PATH:
- Go installs tools to $GOPATH/bin or $GOBIN (default: ~/go/bin)
- Add to PATH: export PATH="$PATH:$(go env GOPATH)/bin"
- Or: export PATH="$PATH:$(go env GOBIN)"`,
	RunE: runDoctorTools,
}

var doctorEnvCmd = &cobra.Command{
	Use:   "env",
	Short: "Show Go environment and PATH information",
	Long:  `Display Go environment variables and PATH information to help diagnose tool installation issues.`,
	RunE:  runDoctorEnv,
}

var doctorVersionsCmd = &cobra.Command{
	Use:   "versions",
	Short: "Detect and manage multiple goneat installations",
	Long: `Detect all goneat installations on the system and help manage version conflicts.

This command scans your system for goneat binaries in:
- GOPATH/bin (global go install location)
- Project-local ./bin/goneat (bootstrap pattern)
- Project-local ./dist/goneat (development build)
- All directories in PATH

It reports version conflicts and offers to:
- Purge stale global installations
- Update global installation to latest version`,
	RunE: runDoctorVersions,
}

var (
	flagDoctorVersionsPurge  bool
	flagDoctorVersionsUpdate bool
)

var (
	flagDoctorInstall           bool
	flagDoctorUpgrade           bool
	flagDoctorAll               bool
	flagDoctorTools             []string
	flagDoctorPrintInstructions bool
	flagDoctorYes               bool
	flagDoctorScope             string
	flagDoctorCheckUpdates      bool
	flagDoctorInstallPkgMgr     bool
	flagDoctorConfig            string
	flagDoctorListScopes        bool
	flagDoctorValidateConfig    bool
	flagDoctorDryRun            bool
	flagDoctorNoCooling         bool
)

func init() {
	// Register doctor root under support/environment
	capabilities := ops.GetDefaultCapabilities(ops.GroupSupport, ops.CategoryEnvironment)
	if err := ops.RegisterCommandWithTaxonomy("doctor", ops.GroupSupport, ops.CategoryEnvironment, capabilities, doctorCmd, "Diagnostics and tooling checks"); err != nil {
		panic(fmt.Sprintf("Failed to register doctor command: %v", err))
	}

	// Attach to root
	rootCmd.AddCommand(doctorCmd)

	// Subcommands
	doctorCmd.AddCommand(doctorToolsCmd)
	doctorCmd.AddCommand(doctorEnvCmd)
	doctorCmd.AddCommand(doctorVersionsCmd)

	// Flags for tools subcommand
	doctorToolsCmd.Flags().BoolVar(&flagDoctorInstall, "install", false, "Install missing tools (non-interactive with --yes)")
	doctorToolsCmd.Flags().BoolVar(&flagDoctorUpgrade, "upgrade", false, "Upgrade installed tools below recommended version")
	doctorToolsCmd.Flags().BoolVar(&flagDoctorAll, "all", false, "Target all known tools in this scope")
	doctorToolsCmd.Flags().StringSliceVar(&flagDoctorTools, "tools", []string{}, "Comma-separated list of tools (e.g., gosec,govulncheck,goimports,gofmt)")
	doctorToolsCmd.Flags().BoolVar(&flagDoctorPrintInstructions, "print-instructions", false, "Print install instructions for missing tools")
	doctorToolsCmd.Flags().BoolVar(&flagDoctorYes, "yes", false, "Assume 'yes' for prompts (non-interactive)")
	doctorToolsCmd.Flags().StringVar(&flagDoctorScope, "scope", "security", "Tool scope to target (security|format|foundation|all)")
	doctorToolsCmd.Flags().BoolVar(&flagDoctorCheckUpdates, "check-updates", false, "Check for available updates (preview; informational only)")
	doctorToolsCmd.Flags().BoolVar(&flagDoctorInstallPkgMgr, "install-package-managers", false, "Install missing package managers (requires --yes for non-interactive)")
	doctorToolsCmd.Flags().StringVar(&flagDoctorConfig, "config", "", "Path to custom tools configuration file")
	doctorToolsCmd.Flags().BoolVar(&flagDoctorListScopes, "list-scopes", false, "List available scopes and exit")
	doctorToolsCmd.Flags().BoolVar(&flagDoctorValidateConfig, "validate-config", false, "Validate configuration file and exit")
	doctorToolsCmd.Flags().BoolVar(&flagDoctorDryRun, "dry-run", false, "Show what would be installed without installing")
	doctorToolsCmd.Flags().BoolVar(&flagDoctorNoCooling, "no-cooling", false, "Disable package cooling policy checks (for offline/air-gapped environments)")

	// Flags for versions subcommand
	doctorVersionsCmd.Flags().BoolVar(&flagDoctorVersionsPurge, "purge", false, "Remove stale global installation from GOPATH/bin")
	doctorVersionsCmd.Flags().BoolVar(&flagDoctorVersionsUpdate, "update", false, "Update global installation to latest version")
	doctorVersionsCmd.Flags().BoolVar(&flagDoctorYes, "yes", false, "Assume 'yes' for prompts (non-interactive)")
}

func runDoctorTools(cmd *cobra.Command, _ []string) error {
	// Handle special modes first
	if flagDoctorListScopes {
		return handleListScopes(cmd)
	}

	if flagDoctorValidateConfig {
		return handleValidateConfig(cmd)
	}

	// Mutual exclusion: --install and --upgrade cannot be used together
	if flagDoctorInstall && flagDoctorUpgrade {
		return fmt.Errorf("--install and --upgrade are mutually exclusive")
	}

	// Dry-run for install mode (non-upgrade); upgrade handles dry-run internally
	if flagDoctorDryRun && !flagDoctorUpgrade {
		return handleDryRun(cmd)
	}

	// Load configuration early for auto-install
	config, err := loadToolsConfiguration()
	if err != nil {
		return fmt.Errorf("failed to load tools configuration: %w", err)
	}

	// If in GitHub Actions and we're installing tools, automatically update GITHUB_PATH
	// This makes tools immediately usable in subsequent workflow steps
	// Note: PATH is already extended globally in PersistentPreRun for all goneat commands
	if flagDoctorInstall && os.Getenv("GITHUB_ACTIONS") == "true" {
		pkgMgrConfig, pkgMgrErr := intdoctor.LoadPackageManagersConfig()
		if pkgMgrErr == nil {
			additions := intdoctor.GetRequiredPATHAdditions(pkgMgrConfig)
			if len(additions) > 0 {
				githubPath := os.Getenv("GITHUB_PATH")
				if githubPath != "" {
					if err := updateGitHubActionsPath(githubPath, additions); err != nil {
						logger.Warn(fmt.Sprintf("Failed to update GITHUB_PATH: %v", err))
					} else {
						logger.Info("Updated GITHUB_PATH for subsequent workflow steps")
					}
				}
			}
		}
	}

	// Convert configuration tools to legacy Tool format for compatibility
	selected, err := selectToolsFromConfig(config)
	if err != nil {
		return fmt.Errorf("failed to select tools: %w", err)
	}

	if len(selected) == 0 {
		logger.Info("No tools selected")
		return nil
	}

	// Auto-install missing package managers if --install flag set (before tool checks) using actual selection.
	if flagDoctorInstall && !flagDoctorDryRun {
		if err := autoInstallPackageManagers(cmd, selected); err != nil {
			logger.Warn("Package manager auto-install failed", logger.Err(err))
		}
	}

	// Initialize shared metadata registry for cooling policy checks
	// This registry is reused across all tool checks to benefit from 24-hour cache
	// Prevents redundant GitHub API calls and reduces rate-limit risk
	metadataRegistry := metadata.NewRegistry(24 * time.Hour)
	githubFetcher := metadata.NewGitHubFetcher(
		os.Getenv("GITHUB_TOKEN"),
		30*time.Second,
	)
	metadataRegistry.RegisterFetcher("github", githubFetcher)
	pypiFetcher := metadata.NewPyPIFetcher(30 * time.Second)
	metadataRegistry.RegisterFetcher("pypi", pypiFetcher)

	// Foundation scope validation - proactive checks for common issues
	if flagDoctorScope == "foundation" {
		if warnings := intdoctor.ValidateFoundationTools(); len(warnings) > 0 {
			logger.Warn("Foundation scope validation warnings:")
			for _, warning := range warnings {
				fmt.Fprintf(cmd.ErrOrStderr(), "⚠️  %s\n", warning) //nolint:errcheck // CLI output errors are typically ignored
			}
			fmt.Fprintln(cmd.ErrOrStderr()) //nolint:errcheck // CLI output errors are typically ignored
		}
	}

	// Preview: check-updates mode (informational; no network latest lookup yet)
	if flagDoctorCheckUpdates {
		return handleCheckUpdates(cmd, selected)
	}

	// Upgrade mode: upgrade installed tools below recommended version
	if flagDoctorUpgrade {
		return handleUpgrade(cmd, selected)
	}

	// Process tools
	missing := 0
	installed := 0
	policyViolations := 0

	for _, tool := range selected {
		// CRITICAL: Platform filtering MUST occur before checking tools.
		// Without this check, platform-specific tools (e.g., scoop on Windows, mise on Linux/macOS)
		// will be reported as "missing" on incompatible platforms, causing false failures in:
		// - Multi-platform CI/CD pipelines (same config, different runners)
		// - Template repositories targeting multiple platforms
		// - Make targets like `make bootstrap` that use shared tool scopes
		//
		// Historical context: Windows-only tools like "scoop" were incorrectly being checked
		// on macOS/Linux systems, causing `goneat doctor tools` to fail with exit code 1
		// even when all platform-applicable tools were present.
		if !intdoctor.SupportsCurrentPlatform(tool) {
			// Tool not applicable to current platform - skip silently
			// Do NOT check, do NOT report as missing, do NOT count toward failure
			logger.Debug(fmt.Sprintf("Skipping %s (not applicable to %s platform)", tool.Name, runtime.GOOS))
			continue
		}

		status := intdoctor.CheckTool(tool)
		policyStatus := status

		if status.Present {
			if status.Version != "" {
				logger.Info(fmt.Sprintf("%s present (%s)", tool.Name, status.Version))
			} else {
				logger.Info(fmt.Sprintf("%s present", tool.Name))
			}

			// Optional: Check cooling policy for informational purposes on present tools
			// This helps users understand if their currently-installed tools meet cooling requirements
			if !flagDoctorNoCooling {
				coolingResult := intdoctor.CheckToolCoolingPolicy(tool, flagDoctorNoCooling, &metadataRegistry)
				if coolingResult != nil && !coolingResult.Disabled && !coolingResult.Passed {
					logger.Warn(fmt.Sprintf("%s present but does not meet cooling policy", tool.Name))
					if len(coolingResult.Violations) > 0 {
						fmt.Fprintf(cmd.ErrOrStderr(), "%s\n", intdoctor.FormatCoolingViolation(tool.Name, coolingResult)) //nolint:errcheck
					}
				}
			}
		} else {
			missing++
			if strings.Contains(status.Instructions, "not in PATH") || strings.Contains(status.Instructions, "is installed at") {
				logger.Warn(fmt.Sprintf("%s installed but not accessible (PATH issue)", tool.Name))
			} else {
				logger.Warn(fmt.Sprintf("%s missing", tool.Name))
			}

			if flagDoctorPrintInstructions && status.Instructions != "" {
				if strings.Contains(status.Instructions, "not in PATH") || strings.Contains(status.Instructions, "is installed at") {
					fmt.Fprintf(cmd.OutOrStdout(), "Fix PATH for %s:\n%s\n", tool.Name, status.Instructions) //nolint:errcheck // CLI output errors are typically ignored
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "Install %s with:\n  %s\n", tool.Name, status.Instructions) //nolint:errcheck // CLI output errors are typically ignored
				}
			}

			if flagDoctorInstall {
				// Check cooling policy before installation
				coolingResult := intdoctor.CheckToolCoolingPolicy(tool, flagDoctorNoCooling, &metadataRegistry)
				if coolingResult != nil && !coolingResult.Disabled && !coolingResult.Passed {
					logger.Warn(fmt.Sprintf("Cooling policy check failed for %s", tool.Name))
					if len(coolingResult.Violations) > 0 {
						fmt.Fprintf(cmd.ErrOrStderr(), "%s\n", intdoctor.FormatCoolingViolation(tool.Name, coolingResult)) //nolint:errcheck
					}

					// Get effective cooling config to check alert-only mode
					effectiveCoolingConfig, err := tool.GetEffectiveCoolingConfig(flagDoctorNoCooling)
					blockInstallation := true
					if err == nil && effectiveCoolingConfig != nil && effectiveCoolingConfig.AlertOnly {
						blockInstallation = false
					}

					if blockInstallation {
						logger.Error(fmt.Sprintf("Installation blocked for %s: cooling policy violation", tool.Name))
						logger.Info("To bypass cooling checks, use: --no-cooling flag")
						policyViolations++
						continue
					}
					// AlertOnly mode: warn but allow installation
					logger.Warn(fmt.Sprintf("Cooling policy violation for %s (alert-only mode: installation proceeding)", tool.Name))
				}

				if !flagDoctorYes {
					if !promptYes(cmd, fmt.Sprintf("Install %s now using: %s ? [y/N] ", tool.Name, status.Instructions)) {
						logger.Warn(fmt.Sprintf("Skipped install for %s", tool.Name))
						policyViolations += summarizePolicy(tool, policyStatus)
						continue
					}
				}
				res := intdoctor.InstallTool(tool)
				policyStatus = res
				if res.Error != nil {
					logger.Error(fmt.Sprintf("Install failed for %s: %v", tool.Name, res.Error))
					if res.Instructions != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "Try manual install:\n  %s\n", res.Instructions) //nolint:errcheck // CLI output errors are typically ignored
					}
				} else if res.Installed && res.Present {
					installed++
					if res.Version != "" {
						logger.Info(fmt.Sprintf("Installed %s (%s)", tool.Name, res.Version))
					} else {
						logger.Info(fmt.Sprintf("Installed %s", tool.Name))
					}
				} else if res.Installed && !res.Present {
					installed++
					logger.Warn(fmt.Sprintf("Installed %s but not accessible (PATH issue)", tool.Name))
					if res.Instructions != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "Fix PATH access:\n%s\n", res.Instructions)           //nolint:errcheck // CLI output errors are typically ignored
						fmt.Fprintf(cmd.OutOrStdout(), "For detailed PATH diagnostics: goneat doctor env\n") //nolint:errcheck // CLI output errors are typically ignored
					}
				} else if res.Present {
					logger.Info(fmt.Sprintf("%s now present", tool.Name))
				}
			}
		}

		policyViolations += summarizePolicy(tool, policyStatus)
	}

	// Re-check if we attempted installs
	if flagDoctorInstall && missing > 0 {
		// Refresh statuses
		finalMissing := 0
		for _, tool := range selected {
			if !intdoctor.CheckTool(tool).Present {
				finalMissing++
			}
		}
		missing = finalMissing
	}

	// Summary
	switch {
	case missing == 0 && policyViolations == 0:
		logger.Info("All requested tools are present")
		return nil
	case missing > 0 && policyViolations > 0:
		return fmt.Errorf("%d tool(s) missing and %d tool(s) violate version policy", missing, policyViolations)
	case missing > 0:
		return fmt.Errorf("%d tool(s) missing after doctor run", missing)
	default:
		return fmt.Errorf("%d tool(s) violate version policy requirements", policyViolations)
	}
}
func summarizePolicy(tool intdoctor.Tool, status intdoctor.Status) int {
	if status.PolicyError != nil {
		logger.Warn(fmt.Sprintf("%s version check skipped: %v", tool.Name, status.PolicyError))
		return 0
	}
	if status.PolicyEvaluation == nil {
		return 0
	}
	eval := status.PolicyEvaluation
	if eval.IsDisallowed {
		logger.Error(fmt.Sprintf("%s version %s is disallowed by policy", tool.Name, eval.ActualVersion))
		return 1
	}
	if !eval.MeetsMinimum {
		logger.Warn(fmt.Sprintf("%s version %s below minimum %s", tool.Name, eval.ActualVersion, eval.MinimumVersion))
		return 1
	}
	if !eval.MeetsRecommended {
		recommended := eval.RecommendedVersion
		if recommended == "" {
			recommended = "latest"
		}
		logger.Warn(fmt.Sprintf("%s version %s below recommended %s (run 'goneat doctor tools --install %s' to upgrade)", tool.Name, eval.ActualVersion, recommended, tool.Name))
	}
	return 0
}

func promptYes(cmd *cobra.Command, message string) bool {
	out := cmd.OutOrStdout()
	fmt.Fprint(out, message) //nolint:errcheck // CLI output errors are typically ignored
	reader := bufio.NewReader(cmd.InOrStdin())
	line, _ := reader.ReadString('\n')
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes"
}

// getGoBinPath returns the Go bin directory where tools are installed
func getGoBinPath() string {
	// First check GOBIN environment variable
	if goBin := os.Getenv("GOBIN"); goBin != "" {
		return goBin
	}

	// Then check GOPATH/bin
	if goPath := os.Getenv("GOPATH"); goPath != "" {
		return filepath.Join(goPath, "bin")
	}

	// Default to ~/go/bin (Go 1.8+ default)
	if homeDir, err := os.UserHomeDir(); err == nil {
		return filepath.Join(homeDir, "go", "bin")
	}

	return ""
}

// updateGitHubActionsPath appends paths to GitHub Actions GITHUB_PATH file
// This makes tools installed to shim directories immediately available in subsequent workflow steps
func updateGitHubActionsPath(githubPathFile string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	// Open file in append mode
	// #nosec G302 G304 - githubPathFile comes from GITHUB_PATH env var managed by GitHub Actions
	// Permission 0644 is the GitHub Actions standard for $GITHUB_PATH file
	// File path is controlled by GitHub Actions runtime, not user input
	f, err := os.OpenFile(githubPathFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open GITHUB_PATH file: %w", err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			logger.Warn(fmt.Sprintf("Failed to close GITHUB_PATH file: %v", closeErr))
		}
	}()

	// Write each path on a new line
	for _, path := range paths {
		if _, err := fmt.Fprintln(f, path); err != nil {
			return fmt.Errorf("failed to write to GITHUB_PATH: %w", err)
		}
	}

	return nil
}

func runDoctorEnv(cmd *cobra.Command, _ []string) error {
	out := cmd.OutOrStdout()

	fmt.Fprintln(out, "Go Environment Information:") //nolint:errcheck // CLI output errors are typically ignored
	fmt.Fprintln(out, "===========================") //nolint:errcheck // CLI output errors are typically ignored

	// Check if Go is available
	if _, err := exec.LookPath("go"); err != nil {
		fmt.Fprintf(out, "❌ Go toolchain not found in PATH\n")    //nolint:errcheck // CLI output errors are typically ignored
		fmt.Fprintf(out, "   Install Go: https://go.dev/dl/\n\n") //nolint:errcheck // CLI output errors are typically ignored
		return nil
	}

	// Get Go environment variables
	envVars := []string{"GOPATH", "GOBIN", "GOROOT", "GOOS", "GOARCH"}
	for _, env := range envVars {
		if value := os.Getenv(env); value != "" {
			fmt.Fprintf(out, "%s=%s\n", env, value) //nolint:errcheck // CLI output errors are typically ignored
		} else {
			fmt.Fprintf(out, "%s=(not set)\n", env) //nolint:errcheck // CLI output errors are typically ignored
		}
	}

	// Show Go version
	if version, err := exec.Command("go", "version").Output(); err == nil {
		fmt.Fprintf(out, "\nGo Version: %s", strings.TrimSpace(string(version))) //nolint:errcheck // CLI output errors are typically ignored
	}

	// Show current PATH
	fmt.Fprintf(out, "\nPATH contains:\n") //nolint:errcheck // CLI output errors are typically ignored
	pathDirs := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))
	for _, dir := range pathDirs {
		if dir == "" {
			continue
		}
		fmt.Fprintf(out, "  %s", dir) //nolint:errcheck // CLI output errors are typically ignored
		// Check if this directory exists
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			fmt.Fprintf(out, " ❌ (does not exist)") //nolint:errcheck // CLI output errors are typically ignored
		} else {
			fmt.Fprintf(out, " ✅") //nolint:errcheck // CLI output errors are typically ignored
		}
		fmt.Fprintln(out) //nolint:errcheck // CLI output errors are typically ignored
	}

	// Show Go bin directory
	fmt.Fprintln(out, "\nGo Tool Installation:") //nolint:errcheck // CLI output errors are typically ignored
	goBinPath := getGoBinPath()
	if goBinPath != "" {
		if _, err := fmt.Fprintf(out, "Expected Go bin directory: %s\n", goBinPath); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		if _, err := os.Stat(goBinPath); os.IsNotExist(err) {
			if _, err := fmt.Fprintf(out, "❌ Directory does not exist\n"); err != nil {
				return fmt.Errorf("failed to write output: %w", err)
			}
		} else {
			if _, err := fmt.Fprintf(out, "✅ Directory exists\n"); err != nil {
				return fmt.Errorf("failed to write output: %w", err)
			}

			// List tools in the directory
			if entries, err := os.ReadDir(goBinPath); err == nil {
				if _, err := fmt.Fprintf(out, "Installed tools: "); err != nil {
					return fmt.Errorf("failed to write output: %w", err)
				}
				toolCount := 0
				for _, entry := range entries {
					if !entry.IsDir() {
						if toolCount > 0 {
							if _, err := fmt.Fprintf(out, ", "); err != nil {
								return fmt.Errorf("failed to write output: %w", err)
							}
						}
						if _, err := fmt.Fprintf(out, "%s", entry.Name()); err != nil {
							return fmt.Errorf("failed to write output: %w", err)
						}
						toolCount++
					}
				}
				if toolCount == 0 {
					if _, err := fmt.Fprintf(out, "(none)"); err != nil {
						return fmt.Errorf("failed to write output: %w", err)
					}
				}
				if _, err := fmt.Fprintln(out); err != nil {
					return fmt.Errorf("failed to write output: %w", err)
				}
			}
		}
	} else {
		if _, err := fmt.Fprintf(out, "❌ Could not determine Go bin directory\n"); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
	}

	// Check if Go bin is in PATH
	if goBinPath != "" {
		inPath := slices.Contains(pathDirs, goBinPath)
		if inPath {
			if _, err := fmt.Fprintf(out, "✅ Go bin directory is in PATH\n"); err != nil {
				return fmt.Errorf("failed to write output: %w", err)
			}
		} else {
			if _, err := fmt.Fprintf(out, "❌ Go bin directory is NOT in PATH\n"); err != nil {
				return fmt.Errorf("failed to write output: %w", err)
			}
			if _, err := fmt.Fprintf(out, "   Add to PATH: export PATH=\"$PATH:%s\"\n", goBinPath); err != nil {
				return fmt.Errorf("failed to write output: %w", err)
			}
		}
	}

	fmt.Fprintln(out, "\nTroubleshooting Tips:")                                                               //nolint:errcheck // CLI output errors are typically ignored
	fmt.Fprintln(out, "- If tools are installed but not found, restart your shell or source your profile")     //nolint:errcheck // CLI output errors are typically ignored
	fmt.Fprintln(out, "- Check that ~/.bashrc, ~/.zshrc, or ~/.profile includes the Go bin directory in PATH") //nolint:errcheck // CLI output errors are typically ignored
	fmt.Fprintln(out, "- On macOS/Linux, you may need to add: export PATH=\"$PATH:$(go env GOPATH)/bin\"")     //nolint:errcheck // CLI output errors are typically ignored
	if _, err := fmt.Fprintf(out, "- On Windows, use: set PATH=%%PATH%%;%%GOPATH%%\\bin\n"); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	return nil
}

// loadToolsConfiguration loads the tools configuration
func loadToolsConfiguration() (*intdoctor.ToolsConfig, error) {
	// If custom config specified, load it directly
	if flagDoctorConfig != "" {
		// Validate path to prevent directory traversal
		cleanPath := filepath.Clean(flagDoctorConfig)
		if strings.Contains(cleanPath, "..") {
			return nil, fmt.Errorf("config path contains invalid path traversal")
		}

		configBytes, err := os.ReadFile(cleanPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		// Validate the config
		if err := intdoctor.ValidateToolsConfig(configBytes); err != nil {
			return nil, fmt.Errorf("config validation failed: %w", err)
		}

		// Parse the config
		config, err := intdoctor.ParseConfig(configBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse config: %w", err)
		}

		return config, nil
	}

	// Otherwise, use LoadToolsConfig which searches upward and falls back to embedded default
	return intdoctor.LoadToolsConfig()
}

// selectToolsFromConfig selects tools based on configuration and flags
func selectToolsFromConfig(config *intdoctor.ToolsConfig) ([]intdoctor.Tool, error) {
	var selected []intdoctor.Tool

	if len(flagDoctorTools) == 0 {
		toolConfigs, err := config.GetToolsForScope(flagDoctorScope)
		if err != nil {
			return nil, fmt.Errorf("failed to get tools for scope '%s': %w", flagDoctorScope, err)
		}

		for _, toolConfig := range toolConfigs {
			tool, err := convertToolConfigToTool(toolConfig)
			if err != nil {
				return nil, fmt.Errorf("failed to parse tool definition for %s: %w", toolConfig.Name, err)
			}
			selected = append(selected, tool)
		}
	} else {
		unknown := []string{}
		for _, name := range flagDoctorTools {
			toolConfig, exists := config.GetTool(name)
			if !exists {
				unknown = append(unknown, strings.TrimSpace(name))
				continue
			}
			tool, err := convertToolConfigToTool(toolConfig)
			if err != nil {
				return nil, fmt.Errorf("failed to parse tool definition for %s: %w", toolConfig.Name, err)
			}
			selected = append(selected, tool)
		}
		if len(unknown) > 0 {
			var allowed []string
			for name := range config.Tools {
				allowed = append(allowed, name)
			}
			return nil, fmt.Errorf("unknown tool(s): %s. Allowed: %s", strings.Join(unknown, ", "), strings.Join(allowed, ", "))
		}
	}

	return selected, nil
}

// convertToolConfigToTool converts ToolConfig to legacy Tool format
func convertToolConfigToTool(toolConfig intdoctor.ToolConfig) (intdoctor.Tool, error) {
	tool := intdoctor.Tool{
		Name:           toolConfig.Name,
		Kind:           toolConfig.Kind,
		InstallPackage: toolConfig.InstallPackage,
		VersionArgs:    toolConfig.VersionArgs,
		CheckArgs:      toolConfig.CheckArgs,
		Description:    toolConfig.Description,
		Platforms:      toolConfig.Platforms,
		DetectCommand:  toolConfig.DetectCommand,
	}

	if policy, err := toolConfig.VersionPolicy(); err != nil {
		return intdoctor.Tool{}, err
	} else {
		tool.VersionPolicy = policy
	}

	if len(toolConfig.InstallCommands) > 0 {
		tool.InstallCommands = make(map[string]string, len(toolConfig.InstallCommands))
		tool.InstallMethods = make(map[string]intdoctor.InstallMethod)
		for key, command := range toolConfig.InstallCommands {
			tool.InstallCommands[key] = command
			switch key {
			case "darwin", "linux", "windows", "all":
				cmdCopy := command
				detectCmd := toolConfig.DetectCommand
				tool.InstallMethods[key] = intdoctor.InstallMethod{
					Detector: func() (string, bool) {
						parts := strings.Fields(detectCmd)
						if len(parts) == 0 {
							return "", false
						}
						return intdoctor.TryCommand(parts[0], parts[1:]...)
					},
					Installer: func() error {
						return intdoctor.ExecuteInstallCommand(cmdCopy)
					},
					Instructions: command,
				}
			}
		}
	}

	if len(toolConfig.InstallerPriority) > 0 {
		tool.InstallerPriority = make(map[string][]string, len(toolConfig.InstallerPriority))
		for platform, priorities := range toolConfig.InstallerPriority {
			tool.InstallerPriority[platform] = append([]string(nil), priorities...)
		}
	}

	if toolConfig.Artifacts != nil {
		tool.Artifacts = toolConfig.Artifacts
	}

	// Copy cooling policy configuration
	if toolConfig.Cooling != nil {
		tool.Cooling = toolConfig.Cooling
	}

	// Copy recommended version for metadata fetching
	if toolConfig.RecommendedVersion != "" {
		tool.RecommendedVersion = toolConfig.RecommendedVersion
	}

	return tool, nil
}

// handleListScopes handles the --list-scopes flag
func handleListScopes(cmd *cobra.Command) error {
	config, err := loadToolsConfiguration()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	scopes := config.GetAllScopes()

	// Honor --json for structured output
	if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
		payload := map[string]any{
			"scopes": scopes,
		}
		data, _ := json.MarshalIndent(payload, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data)) //nolint:errcheck // CLI output errors are typically ignored
		return nil
	}

	// Human-readable output
	fmt.Fprintln(cmd.OutOrStdout(), "Available scopes:") //nolint:errcheck // CLI output errors are typically ignored
	for _, scope := range scopes {
		desc := config.Scopes[scope].Description
		if desc == "" && scope == "all" {
			desc = "All tools from all scopes"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %-15s - %s\n", scope, desc) //nolint:errcheck // CLI output errors are typically ignored
	}

	return nil
}

// handleValidateConfig handles the --validate-config flag
func handleValidateConfig(cmd *cobra.Command) error {
	configPath := flagDoctorConfig
	if configPath == "" {
		configPath = ".goneat/tools.yaml"
	}

	err := intdoctor.ValidateConfigFile(configPath)
	if err != nil {
		fmt.Fprintf(cmd.OutOrStderr(), "❌ Configuration validation failed: %v\n", err) //nolint:errcheck // CLI output errors are typically ignored
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✅ Configuration is valid: %s\n", configPath) //nolint:errcheck // CLI output errors are typically ignored
	return nil
}

// handleCheckUpdates handles the --check-updates flag
func handleCheckUpdates(cmd *cobra.Command, selected []intdoctor.Tool) error {
	// Collect local info for output
	type toolInfo struct {
		Name    string `json:"name"`
		Present bool   `json:"present"`
		Version string `json:"version,omitempty"`
	}
	infos := make([]toolInfo, 0, len(selected))
	for _, tool := range selected {
		// Platform filtering: skip tools not applicable to current platform
		if !intdoctor.SupportsCurrentPlatform(tool) {
			logger.Debug(fmt.Sprintf("Skipping %s (not applicable to %s platform)", tool.Name, runtime.GOOS))
			continue
		}

		st := intdoctor.CheckTool(tool)
		ver := st.Version
		if ver == "" && st.Present {
			ver = "unknown"
		}
		infos = append(infos, toolInfo{Name: tool.Name, Present: st.Present, Version: ver})
	}

	// Honor --json for structured output
	if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
		payload := map[string]any{
			"tools":    infos,
			"note":     "Upgrade checks will report latest versions in v0.1.x (preview)",
			"scope":    flagDoctorScope,
			"selected": flagDoctorTools,
		}
		data, _ := json.MarshalIndent(payload, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data)) //nolint:errcheck // CLI output errors are typically ignored
		return nil
	}

	// Human-readable summary
	for _, ti := range infos {
		if ti.Present {
			fmt.Fprintf(cmd.OutOrStdout(), "%-12s present (version: %s)\n", ti.Name, ti.Version) //nolint:errcheck // CLI output errors are typically ignored
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "%-12s missing\n", ti.Name) //nolint:errcheck // CLI output errors are typically ignored
		}
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Note: Upgrade checks will report latest versions in v0.1.x. This is an informational preview.") //nolint:errcheck // CLI output errors are typically ignored
	return nil
}

// handleDryRun handles the --dry-run flag
func handleDryRun(cmd *cobra.Command) error {
	// Load configuration
	config, err := loadToolsConfiguration()
	if err != nil {
		return fmt.Errorf("failed to load tools configuration: %w", err)
	}

	// Convert configuration tools to legacy Tool format for compatibility
	selected, err := selectToolsFromConfig(config)
	if err != nil {
		return fmt.Errorf("failed to select tools: %w", err)
	}

	if len(selected) == 0 {
		logger.Info("No tools selected")
		return nil
	}

	// Check tools and collect installation information
	type dryRunInfo struct {
		Name           string `json:"name"`
		Present        bool   `json:"present"`
		Version        string `json:"version,omitempty"`
		WouldInstall   bool   `json:"would_install"`
		InstallCommand string `json:"install_command,omitempty"`
		Instructions   string `json:"instructions,omitempty"`
		Error          string `json:"error,omitempty"`
	}

	infos := make([]dryRunInfo, 0, len(selected))
	wouldInstallCount := 0

	for _, tool := range selected {
		// Platform filtering: skip tools not applicable to current platform
		if !intdoctor.SupportsCurrentPlatform(tool) {
			logger.Debug(fmt.Sprintf("Skipping %s (not applicable to %s platform)", tool.Name, runtime.GOOS))
			continue
		}

		status := intdoctor.CheckTool(tool)
		info := dryRunInfo{
			Name:    tool.Name,
			Present: status.Present,
			Version: status.Version,
		}

		if !status.Present {
			info.WouldInstall = true
			info.InstallCommand = getInstallCommand(tool)
			info.Instructions = status.Instructions
			wouldInstallCount++
		}

		infos = append(infos, info)
	}

	// Honor --json for structured output
	if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
		payload := map[string]any{
			"dry_run":        true,
			"tools":          infos,
			"would_install":  wouldInstallCount,
			"total_tools":    len(selected),
			"scope":          flagDoctorScope,
			"selected_tools": flagDoctorTools,
		}
		data, _ := json.MarshalIndent(payload, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data)) //nolint:errcheck // CLI output errors are typically ignored
		return nil
	}

	// Human-readable output
	fmt.Fprintln(cmd.OutOrStdout(), "Dry run: Tools that would be installed") //nolint:errcheck // CLI output errors are typically ignored
	fmt.Fprintln(cmd.OutOrStdout(), "=====================================")  //nolint:errcheck // CLI output errors are typically ignored

	if wouldInstallCount == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "✅ All requested tools are already present") //nolint:errcheck // CLI output errors are typically ignored
		return nil
	}

	for _, info := range infos {
		if info.Present {
			if info.Version != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "✅ %-15s present (%s)\n", info.Name, info.Version) //nolint:errcheck // CLI output errors are typically ignored
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "✅ %-15s present\n", info.Name) //nolint:errcheck // CLI output errors are typically ignored
			}
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "📦 %-15s would install\n", info.Name) //nolint:errcheck // CLI output errors are typically ignored
			if info.InstallCommand != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "   Command: %s\n", info.InstallCommand) //nolint:errcheck // CLI output errors are typically ignored
			}
			if info.Instructions != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "   Instructions: %s\n", info.Instructions) //nolint:errcheck // CLI output errors are typically ignored
			}
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nSummary: %d tool(s) would be installed out of %d total\n", wouldInstallCount, len(selected)) //nolint:errcheck // CLI output errors are typically ignored
	return nil
}

// handleUpgrade handles the --upgrade flag: upgrades installed tools below recommended version
func handleUpgrade(cmd *cobra.Command, selected []intdoctor.Tool) error {
	upgraded := 0
	skippedCurrent := 0
	warnings := 0
	failed := 0

	for _, tool := range selected {
		if !intdoctor.SupportsCurrentPlatform(tool) {
			logger.Debug(fmt.Sprintf("Skipping %s (not applicable to %s platform)", tool.Name, runtime.GOOS))
			continue
		}

		status := intdoctor.CheckTool(tool)

		// Not installed → skip
		if !status.Present {
			logger.Info(fmt.Sprintf("%s not installed (use --install to install)", tool.Name))
			continue
		}

		// Version empty/unparseable → skip with warning
		if status.Version == "" || status.PolicyError != nil {
			if status.PolicyError != nil {
				logger.Warn(fmt.Sprintf("%s version check skipped: %v", tool.Name, status.PolicyError))
			} else {
				logger.Warn(fmt.Sprintf("%s version not detectable, skipping upgrade", tool.Name))
			}
			warnings++
			continue
		}

		// No policy configured → nothing to compare against
		if status.PolicyEvaluation == nil {
			logger.Info(fmt.Sprintf("%s %s (no version policy configured)", tool.Name, status.Version))
			skippedCurrent++
			continue
		}

		eval := status.PolicyEvaluation

		// Already meets recommended → skip
		if eval.MeetsRecommended {
			recommended := eval.RecommendedVersion
			if recommended == "" {
				recommended = eval.MinimumVersion
			}
			logger.Info(fmt.Sprintf("%s %s meets recommended %s", tool.Name, eval.ActualVersion, recommended))
			skippedCurrent++
			continue
		}

		// Upgrade candidate
		recommended := eval.RecommendedVersion
		if recommended == "" {
			recommended = eval.MinimumVersion
		}

		if flagDoctorDryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s -> would upgrade (recommended: %s)\n", tool.Name, eval.ActualVersion, recommended) //nolint:errcheck
			upgraded++
			continue
		}

		if !flagDoctorYes {
			if !promptYes(cmd, fmt.Sprintf("Upgrade %s from %s? (recommended: %s) [y/N] ", tool.Name, eval.ActualVersion, recommended)) {
				logger.Warn(fmt.Sprintf("Skipped upgrade for %s", tool.Name))
				continue
			}
		}

		logger.Info(fmt.Sprintf("%s %s -> upgrading (recommended: %s)...", tool.Name, eval.ActualVersion, recommended))
		res := intdoctor.UpgradeTool(tool)
		if res.Error != nil {
			logger.Error(fmt.Sprintf("Upgrade failed for %s: %v", tool.Name, res.Error))
			failed++
			continue
		}

		// Re-detect version after upgrade
		newStatus := intdoctor.CheckTool(tool)
		if newStatus.Version != "" {
			logger.Info(fmt.Sprintf("%s upgraded to %s", tool.Name, newStatus.Version))
		} else {
			logger.Info(fmt.Sprintf("%s upgraded (version detection unavailable)", tool.Name))
		}
		upgraded++
	}

	// Summary
	if flagDoctorDryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "\nDry run: Would upgrade %d tool(s) | Skipped: %d (current) | Warnings: %d (unparseable)\n", upgraded, skippedCurrent, warnings) //nolint:errcheck
		return nil
	}

	logger.Info(fmt.Sprintf("Upgraded: %d | Failed: %d | Skipped: %d (current) | Warnings: %d (unparseable)", upgraded, failed, skippedCurrent, warnings))

	if failed > 0 {
		return fmt.Errorf("%d tool upgrade(s) failed", failed)
	}
	return nil
}

// getInstallCommand returns the install command for a tool
func getInstallCommand(tool intdoctor.Tool) string {
	if tool.Kind == "go" {
		return fmt.Sprintf("go install %s", tool.InstallPackage)
	}

	if tool.Kind == "system" {
		platform := runtime.GOOS
		if method, ok := tool.InstallMethods[platform]; ok {
			return method.Instructions
		}
		// Try fallback platforms
		for fallbackPlatform, method := range tool.InstallMethods {
			if fallbackPlatform == "all" || fallbackPlatform == "*" {
				return method.Instructions
			}
		}
	}

	return "Manual installation required"
}

// GoneatInstallation represents a detected goneat installation
type GoneatInstallation struct {
	Path    string
	Version string
	Type    string // "global", "project-local", "development", "path"
	Current bool   // whether this is the currently running binary
}

// runDoctorVersions detects and manages multiple goneat installations
func runDoctorVersions(cmd *cobra.Command, _ []string) error {
	out := cmd.OutOrStdout()
	jsonOut, _ := cmd.Flags().GetBool("json")

	// Detect all goneat installations
	installations, err := detectGoneatInstallations()
	if err != nil {
		return fmt.Errorf("failed to detect goneat installations: %w", err)
	}

	// Get current running version
	currentExePath, _ := os.Executable()
	currentVersion := buildinfo.BinaryVersion

	// Mark current installation
	for i := range installations {
		if installations[i].Path == currentExePath {
			installations[i].Current = true
		}
	}

	// JSON output
	if jsonOut {
		payload := map[string]interface{}{
			"current_version": currentVersion,
			"current_path":    currentExePath,
			"installations":   installations,
			"conflict_count":  countVersionConflicts(installations, currentVersion),
		}
		data, _ := json.MarshalIndent(payload, "", "  ")
		fmt.Fprintln(out, string(data)) //nolint:errcheck // CLI output errors are typically ignored
		return nil
	}

	// Human-readable output
	fmt.Fprintln(out, "Goneat Version Analysis")                        //nolint:errcheck // CLI output errors are typically ignored
	fmt.Fprintln(out, "=======================")                        //nolint:errcheck // CLI output errors are typically ignored
	fmt.Fprintf(out, "\nCurrent running version: %s\n", currentVersion) //nolint:errcheck // CLI output errors are typically ignored
	fmt.Fprintf(out, "Current binary path: %s\n\n", currentExePath)     //nolint:errcheck // CLI output errors are typically ignored

	if len(installations) == 0 {
		fmt.Fprintln(out, "No goneat installations detected on system") //nolint:errcheck // CLI output errors are typically ignored
		return nil
	}

	fmt.Fprintln(out, "Detected installations:") //nolint:errcheck // CLI output errors are typically ignored
	conflicts := []GoneatInstallation{}
	for _, inst := range installations {
		marker := "  "
		if inst.Current {
			marker = "▶️"
		}
		fmt.Fprintf(out, "%s %-12s | %s | %s\n", marker, inst.Version, inst.Type, inst.Path) //nolint:errcheck // CLI output errors are typically ignored

		// Track conflicts (different versions than current)
		if inst.Version != currentVersion && inst.Version != "unknown" {
			conflicts = append(conflicts, inst)
		}
	}

	// Handle conflicts
	if len(conflicts) > 0 {
		fmt.Fprintf(out, "\n⚠️  Warning: %d version conflict(s) detected\n", len(conflicts)) //nolint:errcheck // CLI output errors are typically ignored

		// Check if there's a global installation conflict
		globalConflict := false
		var globalPath string
		for _, inst := range conflicts {
			if inst.Type == "global" {
				globalConflict = true
				globalPath = inst.Path
				break
			}
		}

		if globalConflict {
			fmt.Fprintln(out, "\nRecommendations:")                                     //nolint:errcheck // CLI output errors are typically ignored
			fmt.Fprintln(out, "1. Remove stale global installation:")                   //nolint:errcheck // CLI output errors are typically ignored
			fmt.Fprintf(out, "   goneat doctor versions --purge --yes\n")               //nolint:errcheck // CLI output errors are typically ignored
			fmt.Fprintln(out, "\n2. Or update global installation to latest:")          //nolint:errcheck // CLI output errors are typically ignored
			fmt.Fprintf(out, "   goneat doctor versions --update --yes\n")              //nolint:errcheck // CLI output errors are typically ignored
			fmt.Fprintln(out, "\n3. Or use project-local installations (recommended):") //nolint:errcheck // CLI output errors are typically ignored
			fmt.Fprintln(out, "   - Bootstrap to ./bin/goneat per project")             //nolint:errcheck // CLI output errors are typically ignored
			fmt.Fprintln(out, "   - See: goneat docs show user-guide/bootstrap")        //nolint:errcheck // CLI output errors are typically ignored

			// Handle --purge flag
			if flagDoctorVersionsPurge {
				if !flagDoctorYes {
					if !promptYes(cmd, fmt.Sprintf("\nRemove %s? [y/N] ", globalPath)) {
						fmt.Fprintln(out, "Cancelled") //nolint:errcheck // CLI output errors are typically ignored
						return nil
					}
				}
				if err := os.Remove(globalPath); err != nil {
					return fmt.Errorf("failed to remove %s: %w", globalPath, err)
				}
				fmt.Fprintf(out, "✅ Removed: %s\n", globalPath) //nolint:errcheck // CLI output errors are typically ignored
				return nil
			}

			// Handle --update flag
			if flagDoctorVersionsUpdate {
				if !flagDoctorYes {
					if !promptYes(cmd, "\nUpdate global installation with 'go install github.com/fulmenhq/goneat@latest'? [y/N] ") {
						fmt.Fprintln(out, "Cancelled") //nolint:errcheck // CLI output errors are typically ignored
						return nil
					}
				}
				updateCmd := exec.Command("go", "install", "github.com/fulmenhq/goneat@latest")
				updateCmd.Stdout = out
				updateCmd.Stderr = cmd.ErrOrStderr()
				if err := updateCmd.Run(); err != nil {
					return fmt.Errorf("failed to update: %w", err)
				}
				fmt.Fprintln(out, "✅ Global installation updated to latest") //nolint:errcheck // CLI output errors are typically ignored
				return nil
			}
		}
	} else {
		fmt.Fprintln(out, "\n✅ No version conflicts detected") //nolint:errcheck // CLI output errors are typically ignored
	}

	return nil
}

// detectGoneatInstallations scans the system for goneat binaries
func detectGoneatInstallations() ([]GoneatInstallation, error) {
	var installations []GoneatInstallation
	seen := make(map[string]bool) // Deduplicate by path

	// 1. Check GOPATH/bin (global go install location)
	goBinPath := getGoBinPath()
	if goBinPath != "" {
		globalPath := filepath.Join(goBinPath, "goneat")
		if runtime.GOOS == "windows" {
			globalPath += ".exe"
		}
		if version, found := getGoneatVersion(globalPath); found {
			installations = append(installations, GoneatInstallation{
				Path:    globalPath,
				Version: version,
				Type:    "global",
			})
			seen[globalPath] = true
		}
	}

	// 2. Check project-local ./bin/goneat
	localBinPath := filepath.Join(".", "bin", "goneat")
	if runtime.GOOS == "windows" {
		localBinPath += ".exe"
	}
	if absPath, err := filepath.Abs(localBinPath); err == nil {
		if !seen[absPath] {
			if version, found := getGoneatVersion(absPath); found {
				installations = append(installations, GoneatInstallation{
					Path:    absPath,
					Version: version,
					Type:    "project-local",
				})
				seen[absPath] = true
			}
		}
	}

	// 3. Check project-local ./dist/goneat (development build)
	distPath := filepath.Join(".", "dist", "goneat")
	if runtime.GOOS == "windows" {
		distPath += ".exe"
	}
	if absPath, err := filepath.Abs(distPath); err == nil {
		if !seen[absPath] {
			if version, found := getGoneatVersion(absPath); found {
				installations = append(installations, GoneatInstallation{
					Path:    absPath,
					Version: version,
					Type:    "development",
				})
				seen[absPath] = true
			}
		}
	}

	// 4. Scan PATH for goneat binaries
	pathDirs := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))
	for _, dir := range pathDirs {
		if dir == "" {
			continue
		}
		pathGoneat := filepath.Join(dir, "goneat")
		if runtime.GOOS == "windows" {
			pathGoneat += ".exe"
		}
		if absPath, err := filepath.Abs(pathGoneat); err == nil {
			if !seen[absPath] {
				if version, found := getGoneatVersion(absPath); found {
					installations = append(installations, GoneatInstallation{
						Path:    absPath,
						Version: version,
						Type:    "path",
					})
					seen[absPath] = true
				}
			}
		}
	}

	return installations, nil
}

// getGoneatVersion gets the version from a goneat binary
func getGoneatVersion(path string) (string, bool) {
	// Check if file exists and is executable
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return "", false
	}

	// Try to run `goneat version`
	cmd := exec.Command(path, "version")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = nil // Suppress errors

	if err := cmd.Run(); err != nil {
		return "unknown", true // Exists but can't get version
	}

	// Parse version from output (first line, first word after "goneat")
	output := strings.TrimSpace(out.String())
	lines := strings.Split(output, "\n")
	if len(lines) == 0 {
		return "unknown", true
	}

	firstLine := lines[0]
	// Expected format: "goneat v0.3.1" or "goneat dev" or just "v0.3.1"
	parts := strings.Fields(firstLine)
	if len(parts) == 0 {
		return "unknown", true
	}

	// Return the version (could be "v0.3.1", "dev", etc.)
	if len(parts) >= 2 && strings.HasPrefix(parts[0], "goneat") {
		return parts[1], true
	}
	return parts[0], true
}

// countVersionConflicts counts installations with different versions
func countVersionConflicts(installations []GoneatInstallation, currentVersion string) int {
	conflicts := 0
	for _, inst := range installations {
		if inst.Version != currentVersion && inst.Version != "unknown" {
			conflicts++
		}
	}
	return conflicts
}

// displayPackageManagerStatus shows package manager availability.
//
//nolint:unused // This function might be useful for future diagnostics even if currently unused
func displayPackageManagerStatus(cmd *cobra.Command) {
	statuses := intdoctor.GetAllPackageManagerStatuses()
	if len(statuses) == 0 {
		return
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\nPackage Managers:") //nolint:errcheck // CLI output errors are typically ignored

	for _, status := range statuses {
		if status.Available {
			fmt.Fprintf(cmd.OutOrStdout(), "  ✅ %-10s %s (detected)\n", status.Name, status.Version) //nolint:errcheck // CLI output errors are typically ignored
		} else if status.SupportedHere {
			fmt.Fprintf(cmd.OutOrStdout(), "  ❌ %-10s not found\n", status.Name)                     //nolint:errcheck // CLI output errors are typically ignored
			fmt.Fprintf(cmd.OutOrStdout(), "                 Install: %s\n", status.InstallationURL) //nolint:errcheck // CLI output errors are typically ignored
		}
	}

	fmt.Fprintln(cmd.OutOrStdout()) //nolint:errcheck // CLI output errors are typically ignored
}

func autoInstallPackageManagers(cmd *cobra.Command, selected []intdoctor.Tool) error {
	logger.Debug("autoInstallPackageManagers: starting")

	// Load package manager config
	pmConfig, err := intdoctor.LoadPackageManagersConfig()
	if err != nil {
		logger.Warn("Failed to load package manager config", logger.Err(err))
		return err
	}

	// On Windows, ensure scoop shim path is in PATH for tool discovery
	if runtime.GOOS == "windows" && tools.IsScoopInstalled() {
		scoopBinPath := tools.GetScoopBinPath()
		if scoopBinPath != "" {
			addToCurrentPATH(scoopBinPath)
			logger.Info("Added scoop bin directory to PATH", logger.String("path", scoopBinPath))
		}
	}

	needsBrewResult, needsBunResult := computeNeededPackageManagers(selected)
	brewAutoInstallSafe := getPackageManagerAutoInstallSafe(pmConfig, "brew")
	bunAutoInstallSafe := getPackageManagerAutoInstallSafe(pmConfig, "bun")

	// Log at INFO level so users can see what package managers are needed
	logger.Info(fmt.Sprintf("Package manager check: needsBrew=%v, needsBun=%v, platform=%s",
		needsBrewResult, needsBunResult, runtime.GOOS))
	logger.Debug("autoInstallPackageManagers: checking conditions",
		logger.Bool("needsBrew", needsBrewResult),
		logger.Bool("needsBun", needsBunResult),
		logger.Bool("brewAutoInstallSafe", brewAutoInstallSafe),
		logger.Bool("bunAutoInstallSafe", bunAutoInstallSafe),
		logger.String("platform", runtime.GOOS))

	var brewInstalled, bunInstalled bool
	var brewErr, bunErr error

	// Try to install bun first (simpler, fewer dependencies, priority 1)
	if needsBunResult && bunAutoInstallSafe {
		if !tools.IsBunInstalled() {
			logger.Info("Auto-installing bun...")
			if err := tools.InstallBun(false); err != nil {
				bunErr = err
				logger.Warn("Failed to auto-install bun", logger.Err(err))
			} else {
				bunInstalled = true
				logger.Info("bun auto-install completed")
				// Add bun to PATH for current session
				bunBinPath := tools.GetBunBinPath()
				if bunBinPath != "" {
					addToCurrentPATH(bunBinPath)
				}
			}
		} else {
			logger.Debug("autoInstallPackageManagers: bun already installed")
			bunInstalled = true
			// Ensure bun is in PATH for current session (may be detected but not in PATH)
			bunBinPath := tools.GetBunBinPath()
			if bunBinPath != "" {
				addToCurrentPATH(bunBinPath)
				logger.Info("Added bun bin directory to PATH", logger.String("path", bunBinPath))
			}
		}
	}

	// Try brew if bun isn't available/needed and brew is needed
	if needsBrewResult && brewAutoInstallSafe && !bunInstalled {
		loc, brewPath, err := tools.DetectBrew()
		logger.Debug("autoInstallPackageManagers: brew detection result",
			logger.String("location", loc.String()),
			logger.String("path", brewPath),
			logger.Err(err))
		if err == nil && loc != tools.BrewNotFound {
			logger.Debug("autoInstallPackageManagers: brew already installed")
			brewInstalled = true
			// Ensure brew is in PATH for current session (may be detected but not in PATH)
			if brewPath != "" {
				brewBinDir := filepath.Dir(brewPath)
				addToCurrentPATH(brewBinDir)
				logger.Info("Added brew bin directory to PATH", logger.String("path", brewBinDir))
			}
		} else {
			logger.Info("Auto-installing user-local Homebrew...")
			interactive := !isCI() && !flagDoctorYes
			if err := tools.InstallUserLocalBrew("", interactive, false); err != nil {
				brewErr = err
				logger.Warn("Failed to auto-install brew", logger.Err(err))
			} else {
				brewInstalled = true
				logger.Info("Brew auto-install completed")
				// Add brew to PATH for current session
				homeDir, _ := os.UserHomeDir()
				if homeDir != "" {
					brewBinPath := filepath.Join(homeDir, "homebrew-local", "bin")
					addToCurrentPATH(brewBinPath)
				}
			}
		}
	}

	// Report overall status
	if !bunInstalled && !brewInstalled && (needsBunResult || needsBrewResult) {
		errMsg := "no package managers could be auto-installed"
		if bunErr != nil {
			errMsg += fmt.Sprintf("; bun: %v", bunErr)
		}
		if brewErr != nil {
			errMsg += fmt.Sprintf("; brew: %v", brewErr)
		}
		logger.Error(errMsg)
		return errors.New(errMsg)
	}

	// Log success if we needed and installed package managers
	if needsBrewResult && brewInstalled {
		logger.Info("brew is available for tool installation")
	}
	if needsBunResult && bunInstalled {
		logger.Info("bun is available for tool installation")
	}
	if !needsBrewResult && !needsBunResult {
		logger.Debug("No package managers needed for selected scope")
	}

	return nil
}

func getPackageManagerAutoInstallSafe(pmConfig *intdoctor.PackageManagersConfig, pmName string) bool {
	for _, pm := range pmConfig.PackageManagers {
		if pm.Name == pmName {
			return pm.IsAutoInstallSafeOnPlatform(runtime.GOOS)
		}
	}
	return false
}

// computeNeededPackageManagers inspects the installer priorities for the selected tools
// and derives which package managers are actually needed on the current platform.
func computeNeededPackageManagers(selected []intdoctor.Tool) (needsBrew bool, needsBun bool) {
	platform := runtime.GOOS

	for _, tool := range selected {
		var priorities []string
		if p, ok := tool.InstallerPriority[platform]; ok {
			priorities = append(priorities, p...)
		}
		if p, ok := tool.InstallerPriority["all"]; ok {
			priorities = append(priorities, p...)
		}

		for _, pm := range priorities {
			switch strings.TrimSpace(strings.ToLower(pm)) {
			case "brew":
				needsBrew = true
			case "bun":
				needsBun = true
			}
		}
	}

	return needsBrew, needsBun
}

func isCI() bool {
	return os.Getenv("CI") != ""
}

// addToCurrentPATH adds a directory to the current PATH environment variable
// This is needed after installing a package manager to make it immediately available
func addToCurrentPATH(dir string) {
	if dir == "" {
		return
	}

	currentPATH := os.Getenv("PATH")
	if strings.Contains(currentPATH, dir) {
		logger.Debug("PATH already contains directory", logger.String("dir", dir))
		return
	}

	newPATH := dir + string(os.PathListSeparator) + currentPATH
	if err := os.Setenv("PATH", newPATH); err != nil {
		logger.Warn("Failed to update PATH", logger.Err(err))
		return
	}
	logger.Debug("Added to PATH for current session", logger.String("dir", dir))
}
