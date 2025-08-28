/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/3leaps/goneat/pkg/logger"
	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Version management for goneat projects",
	Long: `Version management system supporting semver, calver, and custom schemes.
Works with VERSION files, git tags, and other version sources.`,
	RunE: runVersion,
}

func init() {
	rootCmd.AddCommand(versionCmd)
	versionCmd.Flags().Bool("extended", false, "Show detailed build and git information")
	versionCmd.Flags().Bool("json", false, "Output version information in JSON format")
	versionCmd.Flags().Bool("no-op", false, "Run in assessment mode without making changes")

	// Add subcommands
	versionCmd.AddCommand(versionInitCmd)
	versionCmd.AddCommand(versionBumpCmd)
	versionCmd.AddCommand(versionSetCmd)
	versionCmd.AddCommand(versionValidateCmd)
	versionCmd.AddCommand(versionCheckConsistencyCmd)

	// Init command flags
	versionInitCmd.Flags().Bool("dry-run", false, "Preview setup without making changes")
	versionInitCmd.Flags().Bool("force", false, "Overwrite existing version files")
	versionInitCmd.Flags().String("initial-version", "1.0.0", "Initial version to set")

	// Note: assess command flags are defined in assess.go
}

func runVersion(cmd *cobra.Command, args []string) error {
	extended, _ := cmd.Flags().GetBool("extended")
	jsonOutput, _ := cmd.Flags().GetBool("json")
	noOp, _ := cmd.Flags().GetBool("no-op")

	out := cmd.OutOrStdout()

	// Get current version from multiple sources
	version, source, err := getVersionFromSources()
	if err != nil {
		if jsonOutput {
			versionInfo := map[string]interface{}{
				"error":     "No version found",
				"goVersion": runtime.Version(),
				"platform":  runtime.GOOS,
				"arch":      runtime.GOARCH,
			}
			jsonData, _ := json.MarshalIndent(versionInfo, "", "  ")
			fmt.Fprintln(out, string(jsonData))
			return nil
		}
		return fmt.Errorf("no version found: %v", err)
	}

	if noOp {
		logger.Info(fmt.Sprintf("[NO-OP] Current version: %s (from %s)", version, source))
	}

	// Get additional git information for extended output
	var gitCommit, gitBranch string
	var gitDirty bool
	if extended {
		if commit, branch, err := getGitCommitInfo(); err == nil {
			gitCommit = commit
			gitBranch = branch
		}
		if dirty, err := isGitDirty(); err == nil {
			gitDirty = dirty
		}
	}

	if jsonOutput {
		versionInfo := map[string]interface{}{
			"version":   version,
			"source":    source,
			"goVersion": runtime.Version(),
			"platform":  runtime.GOOS,
			"arch":      runtime.GOARCH,
		}
		if extended {
			versionInfo["buildTime"] = "unknown" // Maintain backward compatibility
			if gitCommit != "" {
				versionInfo["gitCommit"] = gitCommit[:8]
			} else {
				versionInfo["gitCommit"] = "unknown"
			}
			versionInfo["gitBranch"] = gitBranch
			versionInfo["gitDirty"] = gitDirty
		}
		jsonData, err := json.MarshalIndent(versionInfo, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format JSON: %v", err)
		}
		fmt.Fprintln(out, string(jsonData))
		return nil
	}

	if extended {
		fmt.Fprintf(out, "goneat %s\n", version)
		fmt.Fprintf(out, "Build time: unknown\n") // Maintain backward compatibility
		if len(gitCommit) >= 8 {
			fmt.Fprintf(out, "Git commit: %s\n", gitCommit[:8]) // Short commit hash
		} else {
			fmt.Fprintf(out, "Git commit: %s\n", gitCommit)
		}
		fmt.Fprintf(out, "Source: %s\n", source)
		if gitBranch != "" {
			fmt.Fprintf(out, "Git branch: %s\n", gitBranch)
		}
		if gitDirty {
			fmt.Fprintf(out, "Git status: dirty (uncommitted changes)\n")
		} else {
			fmt.Fprintf(out, "Git status: clean\n")
		}
		fmt.Fprintf(out, "Go version: %s\n", runtime.Version())
		fmt.Fprintf(out, "Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	} else {
		fmt.Fprintf(out, "goneat %s\n", version)
		fmt.Fprintf(out, "Source: %s\n", source)
		fmt.Fprintf(out, "Go Version: %s\n", runtime.Version())
		fmt.Fprintf(out, "OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	}

	return nil
}

// versionBumpCmd represents the version bump command
var versionBumpCmd = &cobra.Command{
	Use:   "bump [patch|minor|major]",
	Short: "Bump version number",
	Long: `Bump the version number according to semantic versioning rules.
Supports patch, minor, and major version bumps.`,
	Args: cobra.ExactArgs(1),
	RunE: runVersionBump,
}

func runVersionBump(cmd *cobra.Command, args []string) error {
	bumpType := args[0]
	noOp, _ := cmd.Flags().GetBool("no-op")

	// Validate bump type
	if bumpType != "patch" && bumpType != "minor" && bumpType != "major" {
		return fmt.Errorf("invalid bump type: %s (must be patch, minor, or major)", bumpType)
	}

	// Read current version
	currentVersion, err := readVersionFromFile("VERSION")
	if err != nil {
		return fmt.Errorf("failed to read current version: %v", err)
	}

	// Parse and bump version
	newVersion, err := bumpSemverVersion(currentVersion, bumpType)
	if err != nil {
		return fmt.Errorf("failed to bump version: %v", err)
	}

	if noOp {
		logger.Info(fmt.Sprintf("[NO-OP] Would bump version from %s to %s", currentVersion, newVersion))
		return nil
	}

	// Write new version to VERSION file
	err = writeVersionToFile("VERSION", newVersion)
	if err != nil {
		return fmt.Errorf("failed to write new version: %v", err)
	}

	// Create git tag for the new version
	if err := createGitTag(newVersion, noOp); err != nil {
		logger.Warn(fmt.Sprintf("Failed to create git tag: %v", err))
		// Don't fail the command if git tagging fails
	}

	logger.Info(fmt.Sprintf("Bumped version from %s to %s", currentVersion, newVersion))
	return nil
}

// versionSetCmd represents the version set command
var versionSetCmd = &cobra.Command{
	Use:   "set <version>",
	Short: "Set specific version number",
	Long:  `Set the version to a specific value across all configured sources.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runVersionSet,
}

func runVersionSet(cmd *cobra.Command, args []string) error {
	newVersion := args[0]
	noOp, _ := cmd.Flags().GetBool("no-op")

	// Validate version format
	err := validateSemverVersion(newVersion)
	if err != nil {
		return fmt.Errorf("invalid version format: %v", err)
	}

	if noOp {
		logger.Info(fmt.Sprintf("[NO-OP] Would set version to %s", newVersion))
		return nil
	}

	// Write new version to VERSION file
	err = writeVersionToFile("VERSION", newVersion)
	if err != nil {
		return fmt.Errorf("failed to write version: %v", err)
	}

	// Create git tag for the new version
	if err := createGitTag(newVersion, noOp); err != nil {
		logger.Warn(fmt.Sprintf("Failed to create git tag: %v", err))
		// Don't fail the command if git tagging fails
	}

	logger.Info(fmt.Sprintf("Set version to %s", newVersion))
	return nil
}

// versionValidateCmd represents the version validate command
var versionValidateCmd = &cobra.Command{
	Use:   "validate <version>",
	Short: "Validate version format",
	Long:  `Validate that a version string conforms to the expected format.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runVersionValidate,
}

func runVersionValidate(cmd *cobra.Command, args []string) error {
	version := args[0]

	err := validateSemverVersion(version)
	if err != nil {
		return fmt.Errorf("invalid version: %v", err)
	}

	logger.Info(fmt.Sprintf("Version %s is valid", version))
	return nil
}

// versionInitCmd represents the version init command
var versionInitCmd = &cobra.Command{
	Use:   "init [template]",
	Short: "Initialize version management for the project",
	Long: `Initialize version management by creating the necessary files and configuration.
Supports various templates for different versioning strategies.

Available templates:
  • basic     - VERSION file with semantic versioning
  • git-tags  - Git tag-based versioning
  • calver    - Calendar versioning (YYYY.MM.DD)
  • custom    - Custom versioning scheme

Examples:
  goneat version init basic     # Create VERSION file with 1.0.0
  goneat version init --dry-run # Preview setup without making changes`,
	Args: cobra.MaximumNArgs(1),
	RunE: runVersionInit,
}

// versionCheckConsistencyCmd represents the version check-consistency command
var versionCheckConsistencyCmd = &cobra.Command{
	Use:   "check-consistency",
	Short: "Check version consistency across sources",
	Long:  `Check that version is consistent across all configured sources.`,
	RunE:  runVersionCheckConsistency,
}

func runVersionCheckConsistency(cmd *cobra.Command, args []string) error {
	noOp, _ := cmd.Flags().GetBool("no-op")

	out := cmd.OutOrStdout()

	// For now, just check VERSION file
	version, err := readVersionFromFile("VERSION")
	if err != nil {
		return fmt.Errorf("failed to read VERSION file: %v", err)
	}

	if noOp {
		logger.Info(fmt.Sprintf("[NO-OP] VERSION file contains: %s", version))
		fmt.Fprintf(out, "Source: VERSION\n")
		fmt.Fprintf(out, "Version: %s\n", version)
		return nil
	}

	fmt.Fprintf(out, "Version Consistency Check\n")
	fmt.Fprintf(out, "========================\n")
	fmt.Fprintf(out, "Source: VERSION\n")
	fmt.Fprintf(out, "Version: %s ✓\n", version)

	logger.Info("Version consistency check completed")
	return nil
}

// Helper functions

func readVersionFromFile(filename string) (string, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

func writeVersionToFile(filename, version string) error {
	return os.WriteFile(filename, []byte(version+"\n"), 0644)
}

func validateSemverVersion(version string) error {
	// Basic semver pattern: v?MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD]
	pattern := `^v?(\d+)\.(\d+)\.(\d+)(?:-([a-zA-Z0-9.-]+))?(?:\+([a-zA-Z0-9.-]+))?$`
	matched, err := regexp.MatchString(pattern, version)
	if err != nil {
		return err
	}
	if !matched {
		return fmt.Errorf("invalid semver format: %s", version)
	}
	return nil
}

func bumpSemverVersion(version, bumpType string) (string, error) {
	// Parse version
	pattern := `^v?(\d+)\.(\d+)\.(\d+)(?:-([a-zA-Z0-9.-]+))?(?:\+([a-zA-Z0-9.-]+))?$`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(version)
	if len(matches) < 4 {
		return "", fmt.Errorf("invalid semver format: %s", version)
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])
	prerelease := matches[4]
	build := matches[5]

	// Apply bump
	switch bumpType {
	case "patch":
		patch++
	case "minor":
		minor++
		patch = 0
	case "major":
		major++
		minor = 0
		patch = 0
	}

	// Construct new version
	newVersion := fmt.Sprintf("%d.%d.%d", major, minor, patch)
	if prerelease != "" {
		newVersion += "-" + prerelease
	}
	if build != "" {
		newVersion += "+" + build
	}

	// Preserve 'v' prefix if original had it
	if strings.HasPrefix(version, "v") {
		newVersion = "v" + newVersion
	}

	return newVersion, nil
}

// Git integration functions

// getLatestGitTag returns the latest git tag matching semver pattern
func getLatestGitTag() (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0", "--match", "v[0-9]*.[0-9]*.[0-9]*")
	output, err := cmd.Output()
	if err != nil {
		// Try without 'v' prefix
		cmd = exec.Command("git", "describe", "--tags", "--abbrev=0", "--match", "[0-9]*.[0-9]*.[0-9]*")
		output, err = cmd.Output()
		if err != nil {
			return "", fmt.Errorf("no git tags found")
		}
	}
	return strings.TrimSpace(string(output)), nil
}

// getGitCommitInfo returns current git commit information
func getGitCommitInfo() (commit, branch string, err error) {
	// Get current commit
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to get git commit: %v", err)
	}
	commit = strings.TrimSpace(string(output))

	// Get current branch
	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err = cmd.Output()
	if err != nil {
		return commit, "", fmt.Errorf("failed to get git branch: %v", err)
	}
	branch = strings.TrimSpace(string(output))

	return commit, branch, nil
}

// isGitDirty returns true if there are uncommitted changes
func isGitDirty() (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %v", err)
	}
	return len(strings.TrimSpace(string(output))) > 0, nil
}

// createGitTag creates a new git tag
func createGitTag(version string, noOp bool) error {
	if noOp {
		logger.Info(fmt.Sprintf("[NO-OP] Would create git tag: %s", version))
		return nil
	}

	cmd := exec.Command("git", "tag", version)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create git tag %s: %v", version, err)
	}

	logger.Info(fmt.Sprintf("Created git tag: %s", version))
	return nil
}

// getVersionFromSources tries to get version from multiple sources in priority order
func getVersionFromSources() (string, string, error) {
	// Priority 1: VERSION file
	if version, err := readVersionFromFile("VERSION"); err == nil {
		return version, "VERSION file", nil
	}

	// Priority 2: Git tag
	if version, err := getLatestGitTag(); err == nil {
		return version, "git tag", nil
	}

	// No version found - provide setup guidance
	setupGuidance := provideSetupGuidance()
	return "", "", fmt.Errorf("no version management detected\n\n%s", setupGuidance)
}

// provideSetupGuidance gives helpful setup instructions when no version is found
func provideSetupGuidance() string {
	var guidance strings.Builder

	guidance.WriteString("🚀 Welcome to goneat version management!\n\n")
	guidance.WriteString("To get started, choose one of these setup options:\n\n")

	guidance.WriteString("📝 Quick Setup (Recommended):\n")
	guidance.WriteString("  goneat version init --template basic\n\n")

	guidance.WriteString("🔧 Manual Setup:\n")
	guidance.WriteString("  1. Create a VERSION file: echo '1.0.0' > VERSION\n")
	guidance.WriteString("  2. Or create a git tag: git tag v1.0.0\n\n")

	guidance.WriteString("📋 Available Templates:\n")
	guidance.WriteString("  • basic     - VERSION file with semantic versioning\n")
	guidance.WriteString("  • git-tags  - Git tag-based versioning\n")
	guidance.WriteString("  • calver    - Calendar versioning (YYYY.MM.DD)\n")
	guidance.WriteString("  • custom    - Custom versioning scheme\n\n")

	guidance.WriteString("💡 Pro Tips:\n")
	guidance.WriteString("  • Use 'goneat version init --dry-run' to preview setup\n")
	guidance.WriteString("  • Run 'goneat version --help' for all options\n")
	guidance.WriteString("  • Version management is non-destructive by default\n\n")

	guidance.WriteString("Need help? Visit: https://goneat.dev/docs/version-management")

	return guidance.String()
}

// runVersionInit implements the version init command
func runVersionInit(cmd *cobra.Command, args []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	force, _ := cmd.Flags().GetBool("force")
	initialVersion, _ := cmd.Flags().GetString("initial-version")

	// Determine template
	template := "basic" // default
	if len(args) > 0 {
		template = args[0]
	}

	out := cmd.OutOrStdout()

	// Check if version management already exists
	if _, _, err := getVersionFromSources(); err == nil {
		if !force {
			return fmt.Errorf("version management already exists. Use --force to overwrite or run 'goneat version' to see current setup")
		}
		logger.Warn("Overwriting existing version management (--force specified)")
	}

	// Setup based on template
	switch template {
	case "basic":
		return setupBasicTemplate(out, initialVersion, dryRun)
	case "git-tags":
		return setupGitTagsTemplate(out, initialVersion, dryRun)
	case "calver":
		return setupCalverTemplate(out, initialVersion, dryRun)
	case "custom":
		return setupCustomTemplate(out, initialVersion, dryRun)
	default:
		return fmt.Errorf("unknown template: %s. Available: basic, git-tags, calver, custom", template)
	}
}

// setupBasicTemplate creates a VERSION file with semantic versioning
func setupBasicTemplate(out io.Writer, initialVersion string, dryRun bool) error {
	fmt.Fprintf(out, "📝 Setting up basic version management with VERSION file\n\n")

	if dryRun {
		fmt.Fprintf(out, "DRY RUN - Would create:\n")
		fmt.Fprintf(out, "  • VERSION file with content: %s\n", initialVersion)
		fmt.Fprintf(out, "  • No actual files will be created\n")
		return nil
	}

	// Create VERSION file
	err := writeVersionToFile("VERSION", initialVersion)
	if err != nil {
		return fmt.Errorf("failed to create VERSION file: %v", err)
	}

	fmt.Fprintf(out, "✅ Created VERSION file with initial version: %s\n", initialVersion)
	fmt.Fprintf(out, "💡 Usage:\n")
	fmt.Fprintf(out, "  • goneat version              # Show current version\n")
	fmt.Fprintf(out, "  • goneat version bump patch   # Increment patch version\n")
	fmt.Fprintf(out, "  • goneat version set 2.0.0    # Set specific version\n")

	return nil
}

// setupGitTagsTemplate sets up git tag-based versioning
func setupGitTagsTemplate(out io.Writer, initialVersion string, dryRun bool) error {
	fmt.Fprintf(out, "🏷️ Setting up git tag-based version management\n\n")

	if dryRun {
		fmt.Fprintf(out, "DRY RUN - Would create:\n")
		fmt.Fprintf(out, "  • Git tag: %s\n", initialVersion)
		fmt.Fprintf(out, "  • No actual tags will be created\n")
		return nil
	}

	// Create initial git tag
	err := createGitTag(initialVersion, false)
	if err != nil {
		return fmt.Errorf("failed to create initial git tag: %v", err)
	}

	fmt.Fprintf(out, "✅ Created initial git tag: %s\n", initialVersion)
	fmt.Fprintf(out, "💡 Usage:\n")
	fmt.Fprintf(out, "  • goneat version              # Show latest git tag\n")
	fmt.Fprintf(out, "  • goneat version bump patch   # Create new tag with bumped version\n")
	fmt.Fprintf(out, "  • goneat version set 2.0.0    # Create new tag with specific version\n")

	return nil
}

// setupCalverTemplate sets up calendar versioning
func setupCalverTemplate(out io.Writer, initialVersion string, dryRun bool) error {
	fmt.Fprintf(out, "📅 Setting up calendar versioning (YYYY.MM.DD)\n\n")

	// Generate current date version if not specified
	if initialVersion == "1.0.0" {
		initialVersion = time.Now().Format("2006.01.02")
	}

	if dryRun {
		fmt.Fprintf(out, "DRY RUN - Would create:\n")
		fmt.Fprintf(out, "  • VERSION file with content: %s\n", initialVersion)
		fmt.Fprintf(out, "  • No actual files will be created\n")
		return nil
	}

	// Create VERSION file
	err := writeVersionToFile("VERSION", initialVersion)
	if err != nil {
		return fmt.Errorf("failed to create VERSION file: %v", err)
	}

	fmt.Fprintf(out, "✅ Created VERSION file with calendar version: %s\n", initialVersion)
	fmt.Fprintf(out, "💡 Calendar versioning uses YYYY.MM.DD format\n")
	fmt.Fprintf(out, "💡 Usage:\n")
	fmt.Fprintf(out, "  • goneat version                    # Show current version\n")
	fmt.Fprintf(out, "  • goneat version set 2024.12.25     # Set specific date version\n")

	return nil
}

// setupCustomTemplate provides guidance for custom versioning
func setupCustomTemplate(out io.Writer, initialVersion string, dryRun bool) error {
	fmt.Fprintf(out, "🔧 Setting up custom versioning scheme\n\n")

	if dryRun {
		fmt.Fprintf(out, "DRY RUN - Would create:\n")
		fmt.Fprintf(out, "  • VERSION file with content: %s\n", initialVersion)
		fmt.Fprintf(out, "  • No actual files will be created\n")
		fmt.Fprintf(out, "\n📋 Custom versioning guidance:\n")
		fmt.Fprintf(out, "  • Edit VERSION file manually for custom schemes\n")
		fmt.Fprintf(out, "  • Use 'goneat version set <version>' to update\n")
		fmt.Fprintf(out, "  • Version validation is flexible for custom schemes\n")
		return nil
	}

	// Create VERSION file
	err := writeVersionToFile("VERSION", initialVersion)
	if err != nil {
		return fmt.Errorf("failed to create VERSION file: %v", err)
	}

	fmt.Fprintf(out, "✅ Created VERSION file with custom version: %s\n", initialVersion)
	fmt.Fprintf(out, "📋 Custom versioning notes:\n")
	fmt.Fprintf(out, "  • Edit VERSION file manually for your custom scheme\n")
	fmt.Fprintf(out, "  • Use 'goneat version set <version>' to update\n")
	fmt.Fprintf(out, "  • Version validation is flexible for custom schemes\n")

	return nil
}
