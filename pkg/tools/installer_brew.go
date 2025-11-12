package tools

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/fulmenhq/goneat/pkg/logger"
)

// BrewInstaller handles brew-based tool installation.
type BrewInstaller struct {
	config *PackageManagerInstall
	tool   *Tool
	dryRun bool
}

// NewBrewInstaller creates a new BrewInstaller.
func NewBrewInstaller(tool *Tool, config *PackageManagerInstall, dryRun bool) *BrewInstaller {
	return &BrewInstaller{
		config: config,
		tool:   tool,
		dryRun: dryRun,
	}
}

// Install executes the brew installation.
func (b *BrewInstaller) Install() (*InstallResult, error) {
	// 1. Check brew availability
	mgr := &BrewManager{}
	if !mgr.IsAvailable() {
		return nil, fmt.Errorf("brew not found in PATH; install from %s", mgr.InstallationURL())
	}

	logger.Debug("brew detected", logger.String("tool", b.tool.Name))

	// 2. Setup tap if specified
	if b.config.Tap != "" {
		if err := b.setupTap(); err != nil {
			return nil, fmt.Errorf("failed to setup tap: %w", err)
		}
	}

	// 3. Build install command
	args := b.buildInstallArgs()

	if b.dryRun {
		cmdStr := "brew " + strings.Join(args, " ")
		logger.Info("dry run: would execute", logger.String("command", cmdStr))
		return &InstallResult{
			BinaryPath: "<dry-run>",
			Version:    "<unknown>",
			Verified:   false,
		}, nil
	}

	// 4. Execute install
	logger.Info("installing via brew",
		logger.String("package", b.config.Package),
		logger.String("tool", b.tool.Name))

	cmd := exec.Command("brew", args...) // #nosec G204 - args are constructed from validated config
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("brew install failed: %w\nOutput: %s", err, output)
	}

	logger.Debug("brew install completed", logger.String("output", string(output)))

	// 5. Verify installation using detect_command
	binaryPath, err := b.verifyInstallation()
	if err != nil {
		return nil, fmt.Errorf("verification failed: %w", err)
	}

	logger.Info("tool installed successfully",
		logger.String("tool", b.tool.Name),
		logger.String("path", binaryPath))

	return &InstallResult{
		BinaryPath: binaryPath,
		Version:    "<installed>", // Version detection can be enhanced later
		Verified:   true,
	}, nil
}

// setupTap adds a homebrew tap if not already present.
func (b *BrewInstaller) setupTap() error {
	// Check if already tapped
	cmd := exec.Command("brew", "tap")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list taps: %w", err)
	}

	taps := strings.Split(string(output), "\n")
	for _, tap := range taps {
		if strings.TrimSpace(tap) == b.config.Tap {
			logger.Debug("tap already added", logger.String("tap", b.config.Tap))
			return nil
		}
	}

	// Add tap
	if b.dryRun {
		logger.Info("dry run: would add tap", logger.String("tap", b.config.Tap))
		return nil
	}

	logger.Info("adding brew tap", logger.String("tap", b.config.Tap))
	cmd = exec.Command("brew", "tap", b.config.Tap) // #nosec G204 - tap from validated config
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add tap: %w\nOutput: %s", err, output)
	}

	logger.Debug("tap added successfully", logger.String("tap", b.config.Tap))
	return nil
}

// buildInstallArgs constructs the brew install command arguments.
func (b *BrewInstaller) buildInstallArgs() []string {
	args := []string{"install"}

	// Package type: formula (default) or cask
	packageType := b.config.PackageType
	if packageType == "" {
		packageType = "formula"
	}

	if packageType == "cask" {
		args = append(args, "--cask")
	} else {
		args = append(args, "--formula")
	}

	// Add user-specified flags
	if len(b.config.Flags) > 0 {
		args = append(args, b.config.Flags...)
	}

	// Add package name
	args = append(args, b.config.Package)

	return args
}

// verifyInstallation checks if the tool is now available after installation.
func (b *BrewInstaller) verifyInstallation() (string, error) {
	if b.tool.DetectCommand == "" {
		return "<unknown>", fmt.Errorf("no detect_command configured for tool %s", b.tool.Name)
	}

	// Parse detect command (e.g., "goneat --version" -> ["goneat", "--version"])
	parts := strings.Fields(b.tool.DetectCommand)
	if len(parts) == 0 {
		return "<unknown>", fmt.Errorf("empty detect_command for tool %s", b.tool.Name)
	}

	toolName := parts[0]

	// Try to find the tool in PATH
	binaryPath, err := exec.LookPath(toolName)
	if err != nil {
		return "<unknown>", fmt.Errorf("tool %s not found in PATH after installation: %w", toolName, err)
	}

	// Optionally run the detect command to verify it works
	if len(parts) > 1 {
		cmd := exec.Command(parts[0], parts[1:]...) // #nosec G204 - parts from validated config
		if err := cmd.Run(); err != nil {
			logger.Warn("detect command failed but tool is in PATH",
				logger.String("tool", toolName),
				logger.Err(err))
			// Don't fail here - tool is in PATH which is good enough
		}
	}

	return binaryPath, nil
}
