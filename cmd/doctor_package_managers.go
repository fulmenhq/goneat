package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"runtime"

	"strings"
	"time"

	"github.com/fulmenhq/goneat/internal/doctor"
	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/fulmenhq/goneat/pkg/tools"
	"github.com/spf13/cobra"
)

var packageManagersCmd = &cobra.Command{
	Use:   "package-managers",
	Short: "Check status of package managers and show recommendations",
	Long: `Check which package managers are installed and show goneat's opinionated recommendations.

This command helps you understand:
 - Which package managers are installed on your system
 - Which package managers goneat recommends (sudo-free, multi-language support)
 - Which package managers require sudo/admin privileges
 - How to install recommended package managers`,
	RunE: runPackageManagers,
}

var (
	flagBrewPrefix    string
	flagBrewForce     bool
	pmRecommendedOnly bool
	pmJSONFormat      bool
)

func init() {
	doctorCmd.AddCommand(packageManagersCmd)
	packageManagersCmd.Flags().BoolVar(&pmRecommendedOnly, "recommended", false, "Show only recommended package managers")
	packageManagersCmd.Flags().BoolVar(&pmJSONFormat, "json", false, "Output in JSON format")

	// Install parent command
	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install package managers",
	}
	packageManagersCmd.AddCommand(installCmd)

	// Install brew subcommand
	installBrewCmd := &cobra.Command{
		Use:   "brew",
		Short: "Install Homebrew (user-local)",
		Long: `Installs Homebrew to $HOME/homebrew-local (no sudo required).

Auto-detection in CI:
  Detects $CI environment variable and skips interactive prompts.`,
		Args: cobra.NoArgs,
		RunE: installBrew,
	}
	installBrewCmd.Flags().StringVar(&flagBrewPrefix, "prefix", "", "Installation prefix (default: $HOME/homebrew-local)")
	installBrewCmd.Flags().BoolVar(&flagBrewForce, "force", false, "Force reinstallation if already exists")
	installBrewCmd.Flags().BoolVarP(&flagDoctorYes, "yes", "y", false, "Skip confirmation prompts (auto-yes in CI)")
	installCmd.AddCommand(installBrewCmd)
}

type packageManagerStatus struct {
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	Installed          bool     `json:"installed"`
	Version            string   `json:"version,omitempty"`
	Recommended        bool     `json:"recommended"`
	RequiresSudo       bool     `json:"requires_sudo"`
	AutoInstallSafe    bool     `json:"auto_install_safe"`
	SupportedLanguages []string `json:"supported_languages"`
	InstallMethod      string   `json:"install_method"`
	InstallCommand     string   `json:"install_command,omitempty"`
	Notes              string   `json:"notes,omitempty"`
}

type packageManagersOutput struct {
	Platform         string                 `json:"platform"`
	TotalDetected    int                    `json:"total_detected"`
	TotalInstalled   int                    `json:"total_installed"`
	RecommendedCount int                    `json:"recommended_count"`
	PackageManagers  []packageManagerStatus `json:"package_managers"`
}

func runPackageManagers(cmd *cobra.Command, args []string) error {
	config, err := doctor.LoadPackageManagersConfig()
	if err != nil {
		return fmt.Errorf("failed to load package managers config: %w", err)
	}

	platform := runtime.GOOS
	var pms []doctor.PackageManager

	if pmRecommendedOnly {
		pms = doctor.GetRecommendedPackageManagers(config)
	} else {
		pms = doctor.DetectAllPackageManagers(config)
	}

	output := packageManagersOutput{
		Platform:        platform,
		TotalDetected:   len(pms),
		PackageManagers: make([]packageManagerStatus, 0, len(pms)),
	}

	for _, pm := range pms {
		if pm.Installed {
			output.TotalInstalled++
		}

		status := packageManagerStatus{
			Name:               pm.Name,
			Description:        pm.Description,
			Installed:          pm.Installed,
			Version:            pm.Version,
			Recommended:        pm.IsRecommendedOnPlatform(platform),
			RequiresSudo:       pm.RequiresSudoOnPlatform(platform),
			AutoInstallSafe:    pm.IsAutoInstallSafeOnPlatform(platform),
			SupportedLanguages: pm.SafeForLanguages,
			InstallMethod:      pm.InstallMethod,
			InstallCommand:     pm.GetInstallCommandForPlatform(platform),
			Notes:              pm.Notes,
		}

		if status.Recommended {
			output.RecommendedCount++
		}

		output.PackageManagers = append(output.PackageManagers, status)
	}

	if pmJSONFormat {
		return outputPackageManagersJSON(cmd, output)
	}

	return outputPackageManagersConsole(cmd, output)
}

func outputPackageManagersJSON(cmd *cobra.Command, output packageManagersOutput) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func outputPackageManagersConsole(cmd *cobra.Command, output packageManagersOutput) error {
	fmt.Fprintf(cmd.OutOrStdout(), "\nPackage Managers Status (%s)\n", output.Platform)                           //nolint:errcheck // CLI output errors are typically ignored
	fmt.Fprintf(cmd.OutOrStdout(), "═══════════════════════════════════════════════════════════════════════\n\n") //nolint:errcheck // CLI output errors are typically ignored

	fmt.Fprintf(cmd.OutOrStdout(), "📊 Summary:\n")                                      //nolint:errcheck // CLI output errors are typically ignored
	fmt.Fprintf(cmd.OutOrStdout(), "   Total detected:  %d\n", output.TotalDetected)    //nolint:errcheck // CLI output errors are typically ignored
	fmt.Fprintf(cmd.OutOrStdout(), "   Installed:       %d\n", output.TotalInstalled)   //nolint:errcheck // CLI output errors are typically ignored
	fmt.Fprintf(cmd.OutOrStdout(), "   Recommended:     %d\n", output.RecommendedCount) //nolint:errcheck // CLI output errors are typically ignored
	fmt.Fprintln(cmd.OutOrStdout())                                                     //nolint:errcheck // CLI output errors are typically ignored

	recommendedPMs := filterRecommended(output.PackageManagers)
	if len(recommendedPMs) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "✨ Recommended Package Managers (sudo-free, opinionated)") //nolint:errcheck // CLI output errors are typically ignored
		fmt.Fprintln(cmd.OutOrStdout())                                                            //nolint:errcheck // CLI output errors are typically ignored

		for _, pm := range recommendedPMs {
			printPackageManager(cmd, pm, true)
		}
	}

	if !pmRecommendedOnly {
		otherPMs := filterNotRecommended(output.PackageManagers)
		if len(otherPMs) > 0 {
			fmt.Fprintln(cmd.OutOrStdout())                             //nolint:errcheck // CLI output errors are typically ignored
			fmt.Fprintln(cmd.OutOrStdout(), "📦 Other Package Managers") //nolint:errcheck // CLI output errors are typically ignored
			fmt.Fprintln(cmd.OutOrStdout())                             //nolint:errcheck // CLI output errors are typically ignored

			for _, pm := range otherPMs {
				printPackageManager(cmd, pm, false)
			}
		}
	}

	fmt.Fprintln(cmd.OutOrStdout())                                                                                       //nolint:errcheck // CLI output errors are typically ignored
	fmt.Fprintln(cmd.OutOrStdout(), "───────────────────────────────────────────────────────────────────────")            //nolint:errcheck // CLI output errors are typically ignored
	fmt.Fprintln(cmd.OutOrStdout(), "💡 Tip: Use --recommended to see only goneat's opinionated recommendations")          //nolint:errcheck // CLI output errors are typically ignored
	fmt.Fprintln(cmd.OutOrStdout(), "💡 Tip: Use --json for machine-readable output")                                      //nolint:errcheck // CLI output errors are typically ignored
	fmt.Fprintf(cmd.OutOrStdout(), "💡 Note: Auto-install available: goneat doctor package-managers install brew --yes\n") //nolint:errcheck // CLI output errors are typically ignored

	return nil
}

func printPackageManager(cmd *cobra.Command, pm packageManagerStatus, detailed bool) {
	statusIcon := "✗"
	if pm.Installed {
		statusIcon = "✓"
	}

	statusText := "(not installed)"
	if pm.Installed {
		statusText = fmt.Sprintf("(%s)", pm.Version)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s\n", statusIcon, pm.Name, statusText) //nolint:errcheck // CLI output errors are typically ignored
	fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", pm.Description)                      //nolint:errcheck // CLI output errors are typically ignored

	if pm.RequiresSudo {
		fmt.Fprintf(cmd.OutOrStdout(), "  ⚠️  Requires sudo/admin privileges\n") //nolint:errcheck // CLI output errors are typically ignored
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "  ✓ Sudo-free installation\n") //nolint:errcheck // CLI output errors are typically ignored
	}

	if detailed || !pm.Installed {
		if len(pm.SupportedLanguages) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "  Languages: %v\n", pm.SupportedLanguages) //nolint:errcheck // CLI output errors are typically ignored
		}

		if !pm.Installed {
			fmt.Fprintln(cmd.OutOrStdout())                                              //nolint:errcheck // CLI output errors are typically ignored
			fmt.Fprintf(cmd.OutOrStdout(), "  📝 Installation (%s):\n", pm.InstallMethod) //nolint:errcheck // CLI output errors are typically ignored

			if pm.InstallCommand != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "     $ %s\n", pm.InstallCommand) //nolint:errcheck // CLI output errors are typically ignored
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "     Manual installation required - see package manager documentation\n") //nolint:errcheck // CLI output errors are typically ignored
			}
		}

		if pm.Notes != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  ℹ️  %s\n", pm.Notes) //nolint:errcheck // CLI output errors are typically ignored
		}
	}

	fmt.Fprintln(cmd.OutOrStdout()) //nolint:errcheck // CLI output errors are typically ignored
}

func filterRecommended(pms []packageManagerStatus) []packageManagerStatus {
	result := make([]packageManagerStatus, 0)
	for _, pm := range pms {
		if pm.Recommended {
			result = append(result, pm)
		}
	}
	return result
}

func filterNotRecommended(pms []packageManagerStatus) []packageManagerStatus {
	result := make([]packageManagerStatus, 0)
	for _, pm := range pms {
		if !pm.Recommended {
			result = append(result, pm)
		}
	}
	return result
}

func installBrew(cmd *cobra.Command, args []string) error {
	if !flagBrewForce {
		loc, brewPath, err := tools.DetectBrew()
		if err == nil && loc != tools.BrewNotFound {
			logger.Info("Brew already installed", logger.String("location", loc.String()), logger.String("path", brewPath))
			fmt.Fprintf(cmd.OutOrStdout(), "✅ Brew already installed at %s\n", brewPath) //nolint:errcheck // CLI output errors are typically ignored
			fmt.Fprintln(cmd.OutOrStdout(), "Use --force to reinstall")                  //nolint:errcheck // CLI output errors are typically ignored
			return nil
		}
	}

	interactive := os.Getenv("CI") == "" && !flagDoctorYes

	if interactive {
		fmt.Fprintln(cmd.OutOrStdout(), "This will install Homebrew to a user-local directory (no sudo required)") //nolint:errcheck // CLI output errors are typically ignored
		prefix := flagBrewPrefix
		if prefix == "" {
			prefix = "$HOME/homebrew-local"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Installation prefix: %s\n", prefix) //nolint:errcheck // CLI output errors are typically ignored
		fmt.Fprintln(cmd.OutOrStdout())                                     //nolint:errcheck // CLI output errors are typically ignored

		reader := bufio.NewReader(cmd.InOrStdin())
		fmt.Fprint(cmd.OutOrStdout(), "Proceed with brew installation? [y/N] ") //nolint:errcheck // CLI output errors are typically ignored
		line, _ := reader.ReadString('\n')
		line = strings.ToLower(strings.TrimSpace(line))
		if line != "y" && line != "yes" {
			logger.Info("Brew installation cancelled by user")
			return nil
		}
	}

	logger.Info("Installing user-local Homebrew...")
	start := time.Now()

	if err := tools.InstallUserLocalBrew(flagBrewPrefix, interactive, false); err != nil {
		return fmt.Errorf("brew installation failed: %w", err)
	}

	duration := time.Since(start)
	logger.Info("Brew installation completed", logger.String("duration", fmt.Sprintf("%.1fs", duration.Seconds())))
	fmt.Fprintf(cmd.OutOrStdout(), "✅ Homebrew installed successfully (%.1fs)\n", duration.Seconds())       //nolint:errcheck // CLI output errors are typically ignored
	fmt.Fprintln(cmd.OutOrStdout())                                                                         //nolint:errcheck // CLI output errors are typically ignored
	fmt.Fprintln(cmd.OutOrStdout(), "To activate in current shell:")                                        //nolint:errcheck // CLI output errors are typically ignored
	fmt.Fprintf(cmd.OutOrStdout(), "  eval \"$(goneat doctor tools env --activate)\"\n")                    //nolint:errcheck // CLI output errors are typically ignored
	fmt.Fprintln(cmd.OutOrStdout())                                                                         //nolint:errcheck // CLI output errors are typically ignored
	fmt.Fprintln(cmd.OutOrStdout(), "In GitHub Actions, PATH is automatically updated with --install flag") //nolint:errcheck // CLI output errors are typically ignored

	return nil
}
