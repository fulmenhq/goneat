package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/fulmenhq/goneat/pkg/logger"
)

// IsBunInstalled checks if bun is available in PATH
func IsBunInstalled() bool {
	_, err := exec.LookPath("bun")
	return err == nil
}

// InstallBun installs bun using the official installer script (no sudo required)
func InstallBun(dryRun bool) error {
	if dryRun {
		logger.Info("Dry-run: skipping bun installation")
		return nil
	}

	if IsBunInstalled() {
		logger.Debug("bun already installed, skipping")
		return nil
	}

	logger.Info("Installing bun...")

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// PowerShell installer for Windows
		cmd = exec.Command("powershell", "-c", "irm bun.sh/install.ps1 | iex")
	} else {
		// Bash installer for macOS/Linux
		cmd = exec.Command("bash", "-c", "curl -fsSL https://bun.sh/install | bash")
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install bun: %w", err)
	}

	// Verify installation
	bunPath := getBunPath()
	if bunPath == "" {
		return fmt.Errorf("bun installed but not found in expected locations")
	}

	// Test that it works
	// #nosec G204 -- bunPath comes from getBunPath() which uses exec.LookPath or constructs from os.UserHomeDir() + hardcoded paths, not user input
	testCmd := exec.Command(bunPath, "--version")
	if output, err := testCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("bun installed but not functional: %w\nOutput: %s", err, output)
	}

	logger.Info("bun installed successfully", logger.String("path", bunPath))
	return nil
}

// getBunPath returns the path to the bun binary, checking common locations
func getBunPath() string {
	// Check PATH first
	if bunPath, err := exec.LookPath("bun"); err == nil {
		return bunPath
	}

	// Check common installation locations
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	locations := []string{
		filepath.Join(homeDir, ".bun", "bin", "bun"),
		filepath.Join(homeDir, ".local", "bin", "bun"),
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	return ""
}

// GetBunBinPath returns the bun bin directory path
func GetBunBinPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".bun", "bin")
}
