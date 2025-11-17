package cmd

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/fulmenhq/goneat/internal/doctor"
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
- How to install recommended package managers (v0.3.7 - manual installation only)

Examples:
  goneat doctor package-managers                    # Show all package managers
  goneat doctor package-managers --recommended      # Show only recommended ones
  goneat doctor package-managers --json             # JSON output for automation`,
	RunE: runPackageManagers,
}

var (
	pmRecommendedOnly bool
	pmJSONFormat      bool
)

func init() {
	doctorCmd.AddCommand(packageManagersCmd)
	packageManagersCmd.Flags().BoolVar(&pmRecommendedOnly, "recommended", false, "Show only recommended package managers")
	packageManagersCmd.Flags().BoolVar(&pmJSONFormat, "json", false, "Output in JSON format")
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

	// Build output structure
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

func outputPackageManagersJSON(cobraCmd *cobra.Command, output packageManagersOutput) error {
	encoder := json.NewEncoder(cobraCmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func outputPackageManagersConsole(cobraCmd *cobra.Command, output packageManagersOutput) error {
	// Header
	_, _ = fmt.Fprintf(cobraCmd.OutOrStdout(), "\nPackage Managers Status (%s)\n", output.Platform)
	_, _ = fmt.Fprintf(cobraCmd.OutOrStdout(), "═══════════════════════════════════════════════════════════════════════\n\n")

	// Summary
	_, _ = fmt.Fprintf(cobraCmd.OutOrStdout(), "📊 Summary:\n")
	_, _ = fmt.Fprintf(cobraCmd.OutOrStdout(), "   Total detected:  %d\n", output.TotalDetected)
	_, _ = fmt.Fprintf(cobraCmd.OutOrStdout(), "   Installed:       %d\n", output.TotalInstalled)
	_, _ = fmt.Fprintf(cobraCmd.OutOrStdout(), "   Recommended:     %d\n", output.RecommendedCount)
	_, _ = fmt.Fprintln(cobraCmd.OutOrStdout())

	// Recommended section
	recommendedPMs := filterRecommended(output.PackageManagers)
	if len(recommendedPMs) > 0 {
		_, _ = fmt.Fprintln(cobraCmd.OutOrStdout(), "✨ Recommended Package Managers (sudo-free, opinionated)")
		_, _ = fmt.Fprintln(cobraCmd.OutOrStdout())

		for _, pm := range recommendedPMs {
			printPackageManager(cobraCmd, pm, true)
		}
	}

	// Other package managers
	if !pmRecommendedOnly {
		otherPMs := filterNotRecommended(output.PackageManagers)
		if len(otherPMs) > 0 {
			_, _ = fmt.Fprintln(cobraCmd.OutOrStdout())
			_, _ = fmt.Fprintln(cobraCmd.OutOrStdout(), "📦 Other Package Managers")
			_, _ = fmt.Fprintln(cobraCmd.OutOrStdout())

			for _, pm := range otherPMs {
				printPackageManager(cobraCmd, pm, false)
			}
		}
	}

	// Footer
	_, _ = fmt.Fprintln(cobraCmd.OutOrStdout())
	_, _ = fmt.Fprintln(cobraCmd.OutOrStdout(), "───────────────────────────────────────────────────────────────────────")
	_, _ = fmt.Fprintln(cobraCmd.OutOrStdout(), "💡 Tip: Use --recommended to see only goneat's opinionated recommendations")
	_, _ = fmt.Fprintln(cobraCmd.OutOrStdout(), "💡 Tip: Use --json for machine-readable output")
	_, _ = fmt.Fprintf(cobraCmd.OutOrStdout(), "💡 Note: v0.3.7 shows installation instructions only - auto-install coming in v0.4.0\n")

	return nil
}

func printPackageManager(cobraCmd *cobra.Command, pm packageManagerStatus, detailed bool) {
	// Status icon and name
	statusIcon := "✗"
	if pm.Installed {
		statusIcon = "✓"
	}

	statusText := "(not installed)"
	if pm.Installed {
		statusText = fmt.Sprintf("(%s)", pm.Version)
	}

	_, _ = fmt.Fprintf(cobraCmd.OutOrStdout(), "%s %s %s\n", statusIcon, pm.Name, statusText)

	// Description
	_, _ = fmt.Fprintf(cobraCmd.OutOrStdout(), "  %s\n", pm.Description)

	// Key attributes
	if pm.RequiresSudo {
		_, _ = fmt.Fprintf(cobraCmd.OutOrStdout(), "  ⚠️  Requires sudo/admin privileges\n")
	} else {
		_, _ = fmt.Fprintf(cobraCmd.OutOrStdout(), "  ✓ Sudo-free installation\n")
	}

	if detailed || !pm.Installed {
		// Supported languages
		if len(pm.SupportedLanguages) > 0 {
			_, _ = fmt.Fprintf(cobraCmd.OutOrStdout(), "  Languages: %v\n", pm.SupportedLanguages)
		}

		// Installation instructions
		if !pm.Installed {
			_, _ = fmt.Fprintln(cobraCmd.OutOrStdout())
			_, _ = fmt.Fprintf(cobraCmd.OutOrStdout(), "  📝 Installation (%s):\n", pm.InstallMethod)

			if pm.InstallCommand != "" {
				_, _ = fmt.Fprintf(cobraCmd.OutOrStdout(), "     $ %s\n", pm.InstallCommand)
			} else {
				_, _ = fmt.Fprintf(cobraCmd.OutOrStdout(), "     Manual installation required - see package manager documentation\n")
			}
		}

		// Notes
		if pm.Notes != "" {
			_, _ = fmt.Fprintf(cobraCmd.OutOrStdout(), "  ℹ️  %s\n", pm.Notes)
		}
	}

	_, _ = fmt.Fprintln(cobraCmd.OutOrStdout())
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
