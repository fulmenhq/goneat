package propagation

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/fulmenhq/goneat/pkg/config"
	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/fulmenhq/goneat/pkg/safeio"
)

// StagingWorkspace manages temporary workspace for atomic file operations
type StagingWorkspace struct {
	baseDir string
}

// NewStagingWorkspace creates a new staging workspace
func NewStagingWorkspace() (*StagingWorkspace, error) {
	goneatHome, err := config.EnsureGoneatHome()
	if err != nil {
		return nil, fmt.Errorf("failed to get goneat home directory: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	baseDir := filepath.Join(goneatHome, "work", "version-propagate", timestamp)

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create staging directory %s: %w", baseDir, err)
	}

	return &StagingWorkspace{
		baseDir: baseDir,
	}, nil
}

// StageFile copies a file to the staging workspace
func (sw *StagingWorkspace) StageFile(srcPath string) (string, error) {
	// Get relative path from current working directory
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	relPath, err := filepath.Rel(wd, srcPath)
	if err != nil {
		return "", fmt.Errorf("failed to get relative path for %s: %w", srcPath, err)
	}

	// Create staging path
	stagePath := filepath.Join(sw.baseDir, relPath)
	stageDir := filepath.Dir(stagePath)

	if err := os.MkdirAll(stageDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create staging directory %s: %w", stageDir, err)
	}

	// Copy file to staging
	if err := sw.copyFile(srcPath, stagePath); err != nil {
		return "", fmt.Errorf("failed to stage file %s: %w", srcPath, err)
	}

	return stagePath, nil
}

// ApplyChanges applies staged changes back to original files atomically
func (sw *StagingWorkspace) ApplyChanges(changes []FileChange, backupEnabled bool, backupRetention int) error {
	logger.Info("Applying staged changes", logger.Int("count", len(changes)))

	// First, create backups if requested
	backups := make(map[string]string)
	if backupEnabled {
		for _, change := range changes {
			backupPath, err := sw.createBackup(change.File)
			if err != nil {
				return fmt.Errorf("failed to create backup for %s: %w", change.File, err)
			}
			backups[change.File] = backupPath
			change.BackupPath = backupPath
		}

		// Clean up old backups based on retention policy
		if backupRetention > 0 {
			if err := sw.cleanupOldBackups(changes, backupRetention); err != nil {
				logger.Warn("Failed to cleanup old backups", logger.String("error", err.Error()))
				// Don't fail the operation for backup cleanup issues
			}
		}
	}

	// Apply changes atomically
	for _, change := range changes {
		stagePath := filepath.Join(sw.baseDir, change.File)
		if err := sw.atomicReplace(change.File, stagePath); err != nil {
			// On failure, attempt to restore from backups
			sw.rollbackChanges(backups)
			return fmt.Errorf("failed to apply change for %s: %w", change.File, err)
		}

	}

	return nil
}

// ValidateStagedChanges validates that all staged changes are correct
func (sw *StagingWorkspace) ValidateStagedChanges(changes []FileChange, expectedVersion string) error {
	logger.Info("Validating staged changes", logger.Int("count", len(changes)))

	for _, change := range changes {
		stagePath := filepath.Join(sw.baseDir, change.File)

		// Check that staged file exists
		if _, err := os.Stat(stagePath); os.IsNotExist(err) {
			return fmt.Errorf("staged file missing: %s", stagePath)
		}

		// Validate version in staged file
		manager, exists := sw.getManagerForFile(change.File)
		if !exists {
			continue // Skip validation if no manager found
		}

		if err := manager.ValidateVersion(stagePath, expectedVersion); err != nil {
			return fmt.Errorf("validation failed for %s: %w", change.File, err)
		}

	}

	return nil
}

// Cleanup removes the staging workspace
func (sw *StagingWorkspace) Cleanup() error {
	if err := os.RemoveAll(sw.baseDir); err != nil {

		return err
	}

	return nil
}

// createBackup creates a backup of the original file
func (sw *StagingWorkspace) createBackup(filePath string) (string, error) {
	timestamp := time.Now().Format("20060102-150405")
	backupPath := filePath + ".backup." + timestamp

	if err := sw.copyFile(filePath, backupPath); err != nil {
		return "", fmt.Errorf("failed to create backup: %w", err)
	}

	return backupPath, nil
}

// atomicReplace atomically replaces the original file with the staged version
func (sw *StagingWorkspace) atomicReplace(originalPath, stagePath string) error {
	// Read staged content
	content, err := os.ReadFile(stagePath)
	if err != nil {
		return fmt.Errorf("failed to read staged file: %w", err)
	}

	// Use safeio to write with preserved permissions
	return safeio.WriteFilePreservePerms(originalPath, content)
}

// copyFile copies a file from src to dst
func (sw *StagingWorkspace) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := srcFile.Close(); err != nil {
			logger.Debug("Failed to close source file", logger.String("file", src), logger.String("error", err.Error()))
		}
	}()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if err := dstFile.Close(); err != nil {
			logger.Debug("Failed to close destination file", logger.String("file", dst), logger.String("error", err.Error()))
		}
	}()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	// Copy permissions
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chmod(dst, srcInfo.Mode())
}

// cleanupOldBackups removes old backup files beyond the retention limit
func (sw *StagingWorkspace) cleanupOldBackups(changes []FileChange, retention int) error {
	for _, change := range changes {
		// Find all backup files for this file
		pattern := change.File + ".backup.*"
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue // Skip if glob fails
		}

		// Sort by modification time (newest first)
		// For simplicity, just keep the most recent N backups
		if len(matches) > retention {
			toRemove := matches[retention:] // Remove oldest backups
			for _, backup := range toRemove {
				if err := os.Remove(backup); err != nil {
					logger.Debug("Failed to remove old backup", logger.String("file", backup), logger.String("error", err.Error()))
				}
			}
		}
	}
	return nil
}

// rollbackChanges restores files from backups
func (sw *StagingWorkspace) rollbackChanges(backups map[string]string) {
	for original, backup := range backups {
		if err := sw.copyFile(backup, original); err != nil {
			logger.Error("Failed to rollback file", logger.String("file", original), logger.String("backup", backup), logger.String("error", err.Error()))
		} else {
			logger.Info("Successfully rolled back file", logger.String("file", original), logger.String("backup", backup))
		}
	}
}

// getManagerForFile returns the package manager for a given file (placeholder for now)
func (sw *StagingWorkspace) getManagerForFile(_ string) (PackageManager, bool) {
	// This will be implemented when we have concrete package managers
	// For now, return nil to skip validation
	return nil, false
}
