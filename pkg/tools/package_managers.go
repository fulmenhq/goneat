package tools

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/fulmenhq/goneat/pkg/logger"
)

// PackageManager defines the interface for package manager detection and operations.
type PackageManager interface {
	Name() string
	IsAvailable() bool
	Version() (string, error)
	InstallationURL() string
	SupportedPlatforms() []string
	IsSupportedOnCurrentPlatform() bool
}

// BrewManager implements PackageManager for Homebrew.
type BrewManager struct{}

// Name returns the package manager name.
func (b *BrewManager) Name() string {
	return "brew"
}

// IsAvailable checks if brew is in PATH and executable.
func (b *BrewManager) IsAvailable() bool {
	_, err := exec.LookPath("brew")
	return err == nil
}

// Version returns the brew version string.
func (b *BrewManager) Version() (string, error) {
	if !b.IsAvailable() {
		return "", fmt.Errorf("brew not found in PATH")
	}

	cmd := exec.Command("brew", "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get brew version: %w", err)
	}

	version := parseBrewVersion(output)
	if version == "" {
		return "", fmt.Errorf("failed to parse brew version from output: %s", output)
	}

	return version, nil
}

// InstallationURL returns the URL for installing Homebrew.
func (b *BrewManager) InstallationURL() string {
	return "https://brew.sh"
}

// SupportedPlatforms returns the list of platforms where brew is supported.
func (b *BrewManager) SupportedPlatforms() []string {
	return []string{"darwin", "linux"}
}

// IsSupportedOnCurrentPlatform checks if brew is supported on the current platform.
func (b *BrewManager) IsSupportedOnCurrentPlatform() bool {
	goos := runtime.GOOS
	return goos == "darwin" || goos == "linux"
}

// parseBrewVersion extracts version from brew --version output.
// Expected format: "Homebrew 4.1.20" or "Homebrew 4.1.20\n..."
func parseBrewVersion(output []byte) string {
	lines := strings.Split(string(output), "\n")
	if len(lines) == 0 {
		return ""
	}

	// First line should contain "Homebrew X.Y.Z"
	firstLine := strings.TrimSpace(lines[0])
	parts := strings.Fields(firstLine)
	if len(parts) >= 2 && parts[0] == "Homebrew" {
		return parts[1]
	}

	return ""
}

// ScoopManager implements PackageManager for Scoop.
type ScoopManager struct{}

// Name returns the package manager name.
func (s *ScoopManager) Name() string {
	return "scoop"
}

// IsAvailable checks if scoop is in PATH and executable.
func (s *ScoopManager) IsAvailable() bool {
	_, err := exec.LookPath("scoop")
	return err == nil
}

// Version returns the scoop version string.
func (s *ScoopManager) Version() (string, error) {
	if !s.IsAvailable() {
		return "", fmt.Errorf("scoop not found in PATH")
	}

	cmd := exec.Command("scoop", "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get scoop version: %w", err)
	}

	version := parseScoopVersion(output)
	if version == "" {
		return "", fmt.Errorf("failed to parse scoop version from output: %s", output)
	}

	return version, nil
}

// InstallationURL returns the URL for installing Scoop.
func (s *ScoopManager) InstallationURL() string {
	return "https://scoop.sh"
}

// SupportedPlatforms returns the list of platforms where scoop is supported.
func (s *ScoopManager) SupportedPlatforms() []string {
	return []string{"windows"}
}

// IsSupportedOnCurrentPlatform checks if scoop is supported on the current platform.
func (s *ScoopManager) IsSupportedOnCurrentPlatform() bool {
	return runtime.GOOS == "windows"
}

// parseScoopVersion extracts version from scoop --version output.
// Expected format: "v0.3.1" or "Current Scoop version:\nv0.3.1"
func parseScoopVersion(output []byte) string {
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Version line typically starts with 'v'
		if strings.HasPrefix(line, "v") && len(line) > 1 {
			return line
		}
		// Or might be in format "Current Scoop version:" followed by version
		if strings.Contains(line, "version") {
			continue
		}
		// Try to find version pattern (vX.Y.Z)
		fields := strings.Fields(line)
		for _, field := range fields {
			if strings.HasPrefix(field, "v") && strings.Contains(field, ".") {
				return field
			}
		}
	}

	return ""
}

// GetManager returns a PackageManager instance for the given name.
// Returns an error if the manager is unknown or not supported on the current platform.
func GetManager(name string) (PackageManager, error) {
	switch name {
	case "brew":
		mgr := &BrewManager{}
		if !mgr.IsSupportedOnCurrentPlatform() {
			return nil, fmt.Errorf("brew is not supported on %s (supported: %v)",
				runtime.GOOS, mgr.SupportedPlatforms())
		}
		return mgr, nil
	case "scoop":
		mgr := &ScoopManager{}
		if !mgr.IsSupportedOnCurrentPlatform() {
			return nil, fmt.Errorf("scoop is not supported on %s (supported: %v)",
				runtime.GOOS, mgr.SupportedPlatforms())
		}
		return mgr, nil
	default:
		return nil, fmt.Errorf("unknown package manager: %s", name)
	}
}

// GetAllManagers returns all package managers supported on the current platform.
func GetAllManagers() []PackageManager {
	var managers []PackageManager

	switch runtime.GOOS {
	case "darwin", "linux":
		managers = append(managers, &BrewManager{})
	case "windows":
		managers = append(managers, &ScoopManager{})
	}

	return managers
}

// PackageManagerStatus represents the status of a package manager.
type PackageManagerStatus struct {
	Name            string
	Available       bool
	Version         string
	InstallationURL string
	PlatformSupport []string
	SupportedHere   bool
	DetectionError  error
}

// GetPackageManagerStatus returns detailed status for a package manager.
func GetPackageManagerStatus(name string) (*PackageManagerStatus, error) {
	mgr, err := GetManager(name)
	if err != nil {
		// Manager not supported on this platform
		logger.Debug("package manager not supported", logger.String("manager", name), logger.Err(err))
		return &PackageManagerStatus{
			Name:           name,
			Available:      false,
			SupportedHere:  false,
			DetectionError: err,
		}, nil
	}

	status := &PackageManagerStatus{
		Name:            name,
		Available:       mgr.IsAvailable(),
		InstallationURL: mgr.InstallationURL(),
		PlatformSupport: mgr.SupportedPlatforms(),
		SupportedHere:   true,
	}

	if status.Available {
		version, err := mgr.Version()
		if err != nil {
			logger.Debug("failed to get package manager version",
				logger.String("manager", name),
				logger.Err(err))
			status.DetectionError = err
		} else {
			status.Version = version
		}
	}

	return status, nil
}

// GetAllPackageManagerStatuses returns status for all managers on the current platform.
func GetAllPackageManagerStatuses() []*PackageManagerStatus {
	managers := GetAllManagers()
	statuses := make([]*PackageManagerStatus, 0, len(managers))

	for _, mgr := range managers {
		status, err := GetPackageManagerStatus(mgr.Name())
		if err != nil {
			logger.Debug("failed to get package manager status",
				logger.String("manager", mgr.Name()),
				logger.Err(err))
			continue
		}
		statuses = append(statuses, status)
	}

	return statuses
}
