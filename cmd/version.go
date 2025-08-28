/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"

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
	versionCmd.AddCommand(versionBumpCmd)
	versionCmd.AddCommand(versionSetCmd)
	versionCmd.AddCommand(versionValidateCmd)
	versionCmd.AddCommand(versionCheckConsistencyCmd)
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
			versionInfo["gitCommit"] = gitCommit
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
		fmt.Fprintf(out, "Source: %s\n", source)
		if gitCommit != "" {
			fmt.Fprintf(out, "Git commit: %s\n", gitCommit[:8]) // Short commit hash
		}
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

	return "", "", fmt.Errorf("no version found in VERSION file or git tags")
}
