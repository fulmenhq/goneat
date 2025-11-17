package cmd

import (
	"fmt"
	"os"

	"github.com/fulmenhq/goneat/internal/doctor"
	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/spf13/cobra"
)

var (
	envActivateFlag bool
	envShellFlag    string
	envGithubFlag   bool
)

var doctorToolsEnvCmd = &cobra.Command{
	Use:   "env",
	Short: "Show PATH activation instructions for installed package managers",
	Long: `Show shell commands to add package manager shim directories to PATH.

This command detects installed package managers that require PATH updates
(like mise, bun, scoop) and outputs the appropriate PATH modification commands
for your shell.

Usage scenarios:

  1. View activation instructions:
     goneat doctor tools env

  2. Activate in current shell (bash/zsh):
     eval "$(goneat doctor tools env --activate)"

  3. Add to shell profile permanently:
     goneat doctor tools env >> ~/.bashrc

  4. GitHub Actions integration:
     goneat doctor tools env --github >> $GITHUB_PATH

Examples:

  # Show instructions for current shell
  goneat doctor tools env

  # Activate for bash/zsh
  eval "$(goneat doctor tools env --activate)"

  # Fish shell
  goneat doctor tools env --shell fish | source

  # Add to bash profile
  goneat doctor tools env >> ~/.bashrc

  # GitHub Actions workflow
  goneat doctor tools env --github >> $GITHUB_PATH
`,
	RunE: runDoctorToolsEnv,
}

func init() {
	doctorToolsCmd.AddCommand(doctorToolsEnvCmd)

	doctorToolsEnvCmd.Flags().BoolVar(&envActivateFlag, "activate", false, "Output activation commands (for eval)")
	doctorToolsEnvCmd.Flags().StringVar(&envShellFlag, "shell", "", "Target shell (bash, zsh, fish, powershell)")
	doctorToolsEnvCmd.Flags().BoolVar(&envGithubFlag, "github", false, "Output GitHub Actions format")
}

func runDoctorToolsEnv(cmd *cobra.Command, args []string) error {
	// Load package managers config
	config, err := doctor.LoadPackageManagersConfig()
	if err != nil {
		return fmt.Errorf("failed to load package managers config: %w", err)
	}

	// Get required PATH additions
	pathMgr := doctor.NewPathManager()
	additions := doctor.GetRequiredPATHAdditions(config)

	if len(additions) == 0 {
		if !envActivateFlag && !envGithubFlag {
			logger.Info("No PATH additions required - all installed package managers are already in PATH")
			logger.Info("Or no package managers requiring PATH updates are installed")
		}
		return nil
	}

	// Detect shell if not specified
	shell := envShellFlag
	if shell == "" {
		shell = doctor.DetectCurrentShell()
	}

	// Output based on flags
	if envGithubFlag {
		// GitHub Actions format
		instructions := pathMgr.GetGitHubActionsInstructions()
		if instructions != "" {
			// Populate pathMgr with additions first
			pathMgr.AddToSessionPATH(additions...)
			instructions = pathMgr.GetGitHubActionsInstructions()
			fmt.Println(instructions)
		}
		return nil
	}

	if envActivateFlag {
		// Activation format (for eval)
		pathMgr.AddToSessionPATH(additions...)
		instructions := pathMgr.GetActivationInstructions(shell)
		fmt.Println(instructions)
		return nil
	}

	// Human-readable format (default)
	fmt.Fprintln(os.Stderr, "# PATH additions required for installed package managers")
	fmt.Fprintln(os.Stderr, "")

	// Show which package managers need PATH updates
	fmt.Fprintln(os.Stderr, "# Installed package managers with shim directories:")
	for _, pm := range config.PackageManagers {
		installed, version := doctor.DetectPackageManager(&pm)
		if !installed || !pm.RequiresPathUpdate {
			continue
		}

		shimPath := doctor.GetShimPath(pm.Name)
		if shimPath == "" {
			continue
		}

		// Check if directory exists
		if _, err := os.Stat(shimPath); err == nil {
			fmt.Fprintf(os.Stderr, "#   %s (%s): %s\n", pm.Name, version, shimPath)
		}
	}
	fmt.Fprintln(os.Stderr, "")

	// Show activation commands
	pathMgr.AddToSessionPATH(additions...)

	fmt.Fprintln(os.Stderr, "# Add these to your shell profile (~/.bashrc, ~/.zshrc, etc.):")
	instructions := pathMgr.GetActivationInstructions(shell)
	fmt.Println(instructions)
	fmt.Fprintln(os.Stderr, "")

	fmt.Fprintln(os.Stderr, "# For immediate use in current shell:")
	fmt.Fprintf(os.Stderr, "#   eval \"$(goneat doctor tools env --activate)\"\n")
	fmt.Fprintln(os.Stderr, "")

	fmt.Fprintln(os.Stderr, "# For GitHub Actions:")
	fmt.Fprintf(os.Stderr, "#   goneat doctor tools env --github >> $GITHUB_PATH\n")

	return nil
}
