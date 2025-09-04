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
	"slices"
	"strings"

	intdoctor "github.com/3leaps/goneat/internal/doctor"
	"github.com/3leaps/goneat/internal/ops"
	"github.com/3leaps/goneat/pkg/logger"
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
- security: gosec, govulncheck, gitleaks
- format:   goimports, gofmt (gofmt is bundled with Go toolchain)

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
	doctorToolsCmd.Flags().StringVar(&flagDoctorScope, "scope", "security", "Tool scope to target (security|format|all)")
	doctorToolsCmd.Flags().BoolVar(&flagDoctorCheckUpdates, "check-updates", false, "Check for available updates (preview; informational only)")
}

func runDoctorTools(cmd *cobra.Command, _ []string) error {
	// Helper to resolve catalog by scope
	resolveByScope := func(scope string) []intdoctor.Tool {
		switch strings.ToLower(strings.TrimSpace(scope)) {
		case "security":
			return intdoctor.KnownSecurityTools()
		case "format":
			return intdoctor.KnownFormatTools()
		case "all":
			return intdoctor.KnownAllTools()
		default:
			// Fallback to security
			return intdoctor.KnownSecurityTools()
		}
	}

	// Select tools per scope and flags
	var selected []intdoctor.Tool
	if len(flagDoctorTools) == 0 {
		// No explicit tools; use --all within the chosen scope (default security)
		selected = resolveByScope(flagDoctorScope)
		// Note: --all flag retained for backward compatibility but no trimming needed in MVP
	} else {
		unknown := []string{}
		for _, name := range flagDoctorTools {
			if t, ok := intdoctor.GetToolByName(name); ok {
				selected = append(selected, t)
			} else {
				unknown = append(unknown, strings.TrimSpace(name))
			}
		}
		if len(unknown) > 0 {
			// Build allowed list dynamically
			var allowed []string
			for _, t := range intdoctor.KnownAllTools() {
				allowed = append(allowed, t.Name)
			}
			return fmt.Errorf("unknown tool(s): %s. Allowed: %s", strings.Join(unknown, ", "), strings.Join(allowed, ", "))
		}
	}

	if len(selected) == 0 {
		logger.Info("No tools selected")
		return nil
	}

	// Preview: check-updates mode (informational; no network latest lookup yet)
	if flagDoctorCheckUpdates {
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

	missing := 0
	installed := 0

	for _, tool := range selected {
		status := intdoctor.CheckTool(tool)
		if status.Present {
			if status.Version != "" {
				logger.Info(fmt.Sprintf("%s present (%s)", tool.Name, status.Version))
			} else {
				logger.Info(fmt.Sprintf("%s present", tool.Name))
			}
		} else {
			missing++
			if strings.Contains(status.Instructions, "not in PATH") {
				logger.Warn(fmt.Sprintf("%s installed but not in PATH", tool.Name))
			} else {
				logger.Warn(fmt.Sprintf("%s missing", tool.Name))
			}

			if flagDoctorPrintInstructions && status.Instructions != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Install %s with:\n  %s\n", tool.Name, status.Instructions) //nolint:errcheck // CLI output errors are typically ignored
			}

			// Install path
			if flagDoctorInstall {
				if !flagDoctorYes {
					if !promptYes(cmd, fmt.Sprintf("Install %s now using: %s ? [y/N] ", tool.Name, status.Instructions)) {
						logger.Warn(fmt.Sprintf("Skipped install for %s", tool.Name))
						continue
					}
				}
				res := intdoctor.InstallTool(tool)
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
					// Installed but not in PATH
					installed++
					logger.Warn(fmt.Sprintf("Installed %s but not in PATH", tool.Name))
					if res.Instructions != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "Add to PATH:\n  %s\n", res.Instructions) //nolint:errcheck // CLI output errors are typically ignored
					}
				} else if res.Present {
					// Edge: command now present but not marked installed
					logger.Info(fmt.Sprintf("%s now present", tool.Name))
				}
			}
		}
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
	if missing == 0 {
		logger.Info("All requested tools are present")
		return nil
	}
	return fmt.Errorf("%d tool(s) missing after doctor run", missing)
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
