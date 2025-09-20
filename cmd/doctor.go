/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	intdoctor "github.com/fulmenhq/goneat/internal/doctor"
	"github.com/fulmenhq/goneat/internal/ops"
	"github.com/fulmenhq/goneat/pkg/logger"
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

var (
	flagDoctorInstall           bool
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

	// Flags for tools subcommand
	doctorToolsCmd.Flags().BoolVar(&flagDoctorInstall, "install", false, "Install missing tools (non-interactive with --yes)")
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
}

func runDoctorTools(cmd *cobra.Command, _ []string) error {
	// Handle special modes first
	if flagDoctorListScopes {
		return handleListScopes(cmd)
	}

	if flagDoctorValidateConfig {
		return handleValidateConfig(cmd)
	}

	if flagDoctorDryRun {
		return handleDryRun(cmd)
	}

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

	// Process tools
	missing := 0
	installed := 0
	policyViolations := 0

	for _, tool := range selected {
		status := intdoctor.CheckTool(tool)
		policyStatus := status

		if status.Present {
			if status.Version != "" {
				logger.Info(fmt.Sprintf("%s present (%s)", tool.Name, status.Version))
			} else {
				logger.Info(fmt.Sprintf("%s present", tool.Name))
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

	// Otherwise, load with defaults and user config merging
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
		fmt.Fprintf(cmd.OutOrStdout(), "  %-15s - %s\n", scope, config.Scopes[scope].Description) //nolint:errcheck // CLI output errors are typically ignored
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
