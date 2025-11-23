package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/fulmenhq/goneat/pkg/logger"
)

// InstallUserLocalBrew installs Homebrew to a user-local prefix (no sudo).
func InstallUserLocalBrew(prefix string, interactive bool, dryRun bool) error {
	if dryRun {
		logger.Info("Dry-run: skipping brew installation")
		return nil
	}
	if prefix == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		prefix = filepath.Join(home, "homebrew-local")
	}

	if err := os.MkdirAll(prefix, 0755); err != nil {
		return fmt.Errorf("failed to create prefix %s: %w", prefix, err)
	}

	logger.Info("Downloading Homebrew from GitHub...")
	downloadURL := "https://github.com/Homebrew/brew/tarball/master"

	cmd := exec.Command("curl", "-L", downloadURL)
	tarCmd := exec.Command("tar", "xz", "--strip", "1", "-C", prefix)

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %w", err)
	}
	tarCmd.Stdin = pipe

	if err := tarCmd.Start(); err != nil {
		return fmt.Errorf("failed to start tar: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start curl: %w", err)
	}
	if err := tarCmd.Wait(); err != nil {
		cmd.Wait() //nolint:errcheck // Clean up curl process if tar fails
		return fmt.Errorf("failed to extract brew: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("failed to download brew: %w", err)
	}

	brewPath := filepath.Join(prefix, "bin", "brew")
	if _, err := os.Stat(brewPath); err != nil {
		return fmt.Errorf("brew installation failed: %s not found", brewPath)
	}

	testCmd := exec.Command(brewPath, "--version")
	if output, err := testCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("brew installed but not functional: %w\nOutput: %s", err, output)
	}

	logger.Info("Homebrew installed successfully", logger.String("prefix", prefix))
	return nil
}
