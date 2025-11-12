package tools

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/fulmenhq/goneat/pkg/logger"
)

// ScoopInstaller handles scoop-based tool installation.
type ScoopInstaller struct {
	config *PackageManagerInstall
	tool   *Tool
	dryRun bool
}

// NewScoopInstaller creates a new ScoopInstaller.
func NewScoopInstaller(tool *Tool, config *PackageManagerInstall, dryRun bool) *ScoopInstaller {
	return &ScoopInstaller{
		config: config,
		tool:   tool,
		dryRun: dryRun,
	}
}

// Install executes the scoop installation.
func (s *ScoopInstaller) Install() (*InstallResult, error) {
	// 1. Check scoop availability
	mgr := &ScoopManager{}
	if !mgr.IsAvailable() {
		return nil, fmt.Errorf("scoop not found in PATH; install from %s", mgr.InstallationURL())
	}

	logger.Debug("scoop detected", logger.String("tool", s.tool.Name))

	// 2. Setup bucket if specified
	if s.config.Bucket != "" {
		if err := s.setupBucket(); err != nil {
			return nil, fmt.Errorf("failed to setup bucket: %w", err)
		}
	}

	// 3. Build install command
	args := s.buildInstallArgs()

	if s.dryRun {
		cmdStr := "scoop " + strings.Join(args, " ")
		logger.Info("dry run: would execute", logger.String("command", cmdStr))
		return &InstallResult{
			BinaryPath: "<dry-run>",
			Version:    "<unknown>",
			Verified:   false,
		}, nil
	}

	// 4. Execute install
	logger.Info("installing via scoop",
		logger.String("package", s.config.Package),
		logger.String("tool", s.tool.Name))

	cmd := exec.Command("scoop", args...) // #nosec G204 - args are constructed from validated config
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("scoop install failed: %w\nOutput: %s", err, output)
	}

	logger.Debug("scoop install completed", logger.String("output", string(output)))

	// 5. Verify installation using detect_command
	binaryPath, err := s.verifyInstallation()
	if err != nil {
		return nil, fmt.Errorf("verification failed: %w", err)
	}

	logger.Info("tool installed successfully",
		logger.String("tool", s.tool.Name),
		logger.String("path", binaryPath))

	return &InstallResult{
		BinaryPath: binaryPath,
		Version:    "<installed>", // Version detection can be enhanced later
		Verified:   true,
	}, nil
}

// setupBucket adds a scoop bucket if not already present.
func (s *ScoopInstaller) setupBucket() error {
	// Check if already added
	cmd := exec.Command("scoop", "bucket", "list")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list buckets: %w", err)
	}

	buckets := strings.Split(string(output), "\n")
	for _, bucket := range buckets {
		// Bucket list format is typically: "bucket-name [source]"
		bucketName := strings.Fields(strings.TrimSpace(bucket))
		if len(bucketName) > 0 && bucketName[0] == s.config.Bucket {
			logger.Debug("bucket already added", logger.String("bucket", s.config.Bucket))
			return nil
		}
	}

	// Add bucket
	if s.dryRun {
		logger.Info("dry run: would add bucket", logger.String("bucket", s.config.Bucket))
		return nil
	}

	logger.Info("adding scoop bucket", logger.String("bucket", s.config.Bucket))
	cmd = exec.Command("scoop", "bucket", "add", s.config.Bucket) // #nosec G204 - bucket from validated config
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add bucket: %w\nOutput: %s", err, output)
	}

	logger.Debug("bucket added successfully", logger.String("bucket", s.config.Bucket))
	return nil
}

// buildInstallArgs constructs the scoop install command arguments.
func (s *ScoopInstaller) buildInstallArgs() []string {
	args := []string{"install"}

	// Add user-specified flags
	if len(s.config.Flags) > 0 {
		args = append(args, s.config.Flags...)
	}

	// Add package name
	args = append(args, s.config.Package)

	return args
}

// verifyInstallation checks if the tool is now available after installation.
func (s *ScoopInstaller) verifyInstallation() (string, error) {
	if s.tool.DetectCommand == "" {
		return "<unknown>", fmt.Errorf("no detect_command configured for tool %s", s.tool.Name)
	}

	// Parse detect command (e.g., "rg --version" -> ["rg", "--version"])
	parts := strings.Fields(s.tool.DetectCommand)
	if len(parts) == 0 {
		return "<unknown>", fmt.Errorf("empty detect_command for tool %s", s.tool.Name)
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
