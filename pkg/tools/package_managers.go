package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

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

// BrewLocation represents different types of Homebrew installations.
type BrewLocation int

const (
	// BrewNotFound indicates no Homebrew installation was detected
	BrewNotFound BrewLocation = iota
	// BrewSystemAppleSilicon indicates Homebrew at /opt/homebrew (Apple Silicon macOS)
	BrewSystemAppleSilicon
	// BrewSystemIntel indicates Homebrew at /usr/local (Intel macOS)
	BrewSystemIntel
	// BrewSystemLinux indicates Homebrew at /home/linuxbrew/.linuxbrew (Linux standard)
	BrewSystemLinux
	// BrewUserLocal indicates Homebrew at $HOME/homebrew-local (user-local installation)
	BrewUserLocal
	// BrewCustom indicates Homebrew in PATH but at non-standard location
	BrewCustom
)

// String returns the string representation of BrewLocation.
func (l BrewLocation) String() string {
	switch l {
	case BrewNotFound:
		return "not_found"
	case BrewSystemAppleSilicon:
		return "system_apple_silicon"
	case BrewSystemIntel:
		return "system_intel"
	case BrewSystemLinux:
		return "system_linux"
	case BrewUserLocal:
		return "user_local"
	case BrewCustom:
		return "custom"
	default:
		return "unknown"
	}
}

// BrewManager implements PackageManager for Homebrew.
type BrewManager struct{}

// Name returns the package manager name.
func (b *BrewManager) Name() string {
	return "brew"
}

// IsAvailable checks if brew is in PATH and executable.
// Uses enhanced detection to find system, user-local, or custom brew installations.
func (b *BrewManager) IsAvailable() bool {
	loc, _, err := DetectBrew()
	return err == nil && loc != BrewNotFound
}

// Version returns the brew version string.
func (b *BrewManager) Version() (string, error) {
	if !b.IsAvailable() {
		return "", fmt.Errorf("brew not found in PATH")
	}

	// Add timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "brew", "--version")
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

	// Add timeout to prevent hanging on Windows
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "scoop", "--version")
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

// GetScoopBinPath returns the scoop shims directory path.
// Scoop installs tools as shims in ~/scoop/shims directory.
// Returns empty string if home directory cannot be determined.
func GetScoopBinPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, "scoop", "shims")
}

// IsScoopInstalled checks if scoop is available in PATH or in its default location.
func IsScoopInstalled() bool {
	// Check PATH first
	if _, err := exec.LookPath("scoop"); err == nil {
		return true
	}

	// Check default scoop location
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	scoopPath := filepath.Join(homeDir, "scoop", "shims", "scoop")
	if runtime.GOOS == "windows" {
		scoopPath += ".cmd"
	}
	_, err = os.Stat(scoopPath)
	return err == nil
}

// parseScoopVersion extracts version from scoop --version output.
// Expected formats:
//   - "v0.3.1"
//   - "Current Scoop version:\nv0.3.1"
//   - "Current Scoop version:\nb588a06e chore(release): Bump to version 0.5.3 (resync) (#6436)"
func parseScoopVersion(output []byte) string {
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip header lines
		if strings.Contains(line, "Current Scoop version:") || strings.Contains(line, "bucket:") {
			continue
		}

		// Check if line contains "version X.Y.Z" pattern (e.g., commit messages)
		if strings.Contains(line, "version") {
			// Extract version after "version" keyword
			parts := strings.Split(line, "version")
			if len(parts) >= 2 {
				// Look for semver pattern in the text after "version"
				fields := strings.Fields(parts[1])
				for _, field := range fields {
					// Remove common trailing characters
					field = strings.TrimRight(field, "()#,")
					// Check if this looks like a version (X.Y.Z)
					if strings.Count(field, ".") >= 2 {
						// Verify it starts with a digit
						if len(field) > 0 && field[0] >= '0' && field[0] <= '9' {
							return field
						}
					}
				}
			}
		}

		// Version line that starts with 'v' (older format)
		if strings.HasPrefix(line, "v") && len(line) > 1 && strings.Contains(line, ".") {
			return line
		}

		// Try to find version pattern (vX.Y.Z or X.Y.Z) in any field
		fields := strings.Fields(line)
		for _, field := range fields {
			field = strings.TrimRight(field, "()#,")
			if (strings.HasPrefix(field, "v") || (len(field) > 0 && field[0] >= '0' && field[0] <= '9')) && strings.Count(field, ".") >= 2 {
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

// DetectBrew detects Homebrew installations with hierarchy preference.
// Returns the location type, path to brew binary, and any error.
// Detection order (highest to lowest preference):
//  1. System brew (Apple Silicon, Intel, Linux standard locations)
//  2. User-local brew ($HOME/homebrew-local)
//  3. Custom location via PATH
func DetectBrew() (BrewLocation, string, error) {
	// 1. Check standard system locations first (most common, best performance)
	systemPaths := []struct {
		loc  BrewLocation
		path string
	}{
		{BrewSystemAppleSilicon, "/opt/homebrew/bin/brew"},
		{BrewSystemIntel, "/usr/local/bin/brew"},
		{BrewSystemLinux, "/home/linuxbrew/.linuxbrew/bin/brew"},
	}

	for _, candidate := range systemPaths {
		if fileExists(candidate.path) {
			logger.Debug("detected system brew",
				logger.String("location", candidate.loc.String()),
				logger.String("path", candidate.path))
			return candidate.loc, candidate.path, nil
		}
	}

	// 2. Check user-local installation ($HOME/homebrew-local)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		logger.Debug("failed to get home directory", logger.Err(err))
	} else {
		userLocalPath := filepath.Join(homeDir, "homebrew-local", "bin", "brew")
		if fileExists(userLocalPath) {
			logger.Debug("detected user-local brew",
				logger.String("location", BrewUserLocal.String()),
				logger.String("path", userLocalPath))
			return BrewUserLocal, userLocalPath, nil
		}
	}

	// 3. Check PATH for custom installation
	brewPath, err := exec.LookPath("brew")
	if err == nil {
		// Found in PATH - determine location type from path
		loc := classifyBrewPath(brewPath)
		logger.Debug("detected brew in PATH",
			logger.String("location", loc.String()),
			logger.String("path", brewPath))
		return loc, brewPath, nil
	}

	// No brew found
	logger.Debug("no brew installation detected")
	return BrewNotFound, "", fmt.Errorf("brew not found")
}

// GetBrewPrefix returns the HOMEBREW_PREFIX for a given brew location.
func GetBrewPrefix(loc BrewLocation) string {
	switch loc {
	case BrewSystemAppleSilicon:
		return "/opt/homebrew"
	case BrewSystemIntel:
		return "/usr/local"
	case BrewSystemLinux:
		return "/home/linuxbrew/.linuxbrew"
	case BrewUserLocal:
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		return filepath.Join(homeDir, "homebrew-local")
	default:
		return ""
	}
}

// IsUserLocalBrew checks if the given brew path is a user-local installation.
func IsUserLocalBrew(brewPath string) bool {
	if brewPath == "" {
		return false
	}
	return strings.Contains(brewPath, "homebrew-local")
}

// classifyBrewPath determines the BrewLocation type from a brew binary path.
func classifyBrewPath(brewPath string) BrewLocation {
	if strings.Contains(brewPath, "/opt/homebrew") {
		return BrewSystemAppleSilicon
	}
	if strings.Contains(brewPath, "/usr/local") && !strings.Contains(brewPath, "homebrew-local") {
		return BrewSystemIntel
	}
	if strings.Contains(brewPath, "/home/linuxbrew") {
		return BrewSystemLinux
	}
	if strings.Contains(brewPath, "homebrew-local") {
		return BrewUserLocal
	}
	return BrewCustom
}

// fileExists checks if a file exists and is accessible.
func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
