package ssot

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/fulmenhq/goneat/pkg/logger"
)

// PerformSync executes the sync operation based on configuration
func PerformSync(opts SyncOptions) (*SyncResult, error) {
	result := &SyncResult{
		Sources: make([]string, 0),
		Errors:  make([]error, 0),
	}

	config := opts.Config

	if opts.Verbose {
		if config.isLocal {
			logger.Info("Using local configuration override (.goneat/ssot.local.yaml)")
		}
	}

	// Process each source
	for _, source := range config.Sources {
		if err := syncSource(source, opts, result); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("source %s: %w", source.Name, err))
			continue
		}
		result.Sources = append(result.Sources, source.Name)
	}

	// Return error if all sources failed
	if len(result.Sources) == 0 && len(result.Errors) > 0 {
		return result, fmt.Errorf("all sources failed to sync")
	}

	return result, nil
}

// syncSource syncs a single source (all its assets)
func syncSource(source Source, opts SyncOptions, result *SyncResult) error {
	// Resolve source to filesystem path
	resolved, err := ResolveSource(source)
	if err != nil {
		return fmt.Errorf("failed to resolve source: %w", err)
	}

	if opts.Verbose {
		logger.Info(fmt.Sprintf("Syncing source: %s from %s", source.Name, resolved.Path))
	}

	// Process each asset
	for _, asset := range source.Assets {
		if err := syncAsset(source, asset, resolved.Path, opts, result); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("asset %s/%s: %w", source.Name, asset.Type, err))
			continue
		}
	}

	return nil
}

// syncAsset syncs a single asset (docs, schemas, etc.)
func syncAsset(source Source, asset Asset, basePath string, opts SyncOptions, result *SyncResult) error {
	mode := asset.Mode
	if mode == "" {
		mode = "copy"
	}

	destRoot := asset.Subdir
	if source.Output != "" {
		destRoot = filepath.Join(source.Output, asset.Subdir)
	}

	if opts.Verbose {
		switch mode {
		case "link":
			logger.Info(fmt.Sprintf("Asset: %s (link) -> %s (target: %s)", asset.Type, destRoot, asset.Link))
		default:
			logger.Info(fmt.Sprintf("Asset: %s -> %s (patterns: %v)", asset.Type, destRoot, asset.Paths))
		}
	}

	// In dry-run mode, just report what would happen
	if opts.DryRun {
		switch mode {
		case "link":
			logger.Info(fmt.Sprintf("[DRY RUN] Would link %s to %s", destRoot, filepath.Join(basePath, asset.Link)))
		default:
			logger.Info(fmt.Sprintf("[DRY RUN] Would sync %s assets to %s", asset.Type, destRoot))
		}
		return nil
	}

	if mode == "link" {
		return createSymlinkAsset(asset, destRoot, basePath, opts)
	}

	// Remove existing destination if prune_stale is enabled
	if opts.Config.Strategy.PruneStale {
		if err := os.RemoveAll(destRoot); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove existing destination: %w", err)
		}
	}

	// Create destination directory
	if err := os.MkdirAll(destRoot, 0750); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Copy files matching glob patterns
	filesCopied := 0

	// Determine the effective base path for pattern matching and relative path calculation
	// If source_path is specified, use it as subdirectory within basePath
	effectiveBasePath := basePath
	if asset.SourcePath != "" {
		effectiveBasePath = filepath.Join(basePath, asset.SourcePath)
	}

	for _, pattern := range asset.Paths {
		// Build full pattern relative to effective base path
		fullPattern := filepath.Join(effectiveBasePath, pattern)

		// Find matching files
		matches, err := doublestar.FilepathGlob(fullPattern)
		if err != nil {
			return fmt.Errorf("failed to glob pattern %s: %w", pattern, err)
		}

		// Copy each matching file
		for _, srcPath := range matches {
			// Skip directories
			info, err := os.Stat(srcPath)
			if err != nil {
				return fmt.Errorf("failed to stat %s: %w", srcPath, err)
			}
			if info.IsDir() {
				continue
			}

			// Calculate relative path from effective base path (not the original basePath)
			relPath, err := filepath.Rel(effectiveBasePath, srcPath)
			if err != nil {
				return fmt.Errorf("failed to calculate relative path: %w", err)
			}

			// Calculate destination path
			dstPath := filepath.Join(destRoot, relPath)

			// Create parent directory
			if err := os.MkdirAll(filepath.Dir(dstPath), 0750); err != nil {
				return fmt.Errorf("failed to create directory for %s: %w", dstPath, err)
			}

			// Copy file
			if err := copyFile(srcPath, dstPath); err != nil {
				return fmt.Errorf("failed to copy %s: %w", relPath, err)
			}

			filesCopied++
			if opts.Verbose {
				logger.Info(fmt.Sprintf("  Copied: %s", relPath))
			}
		}
	}

	result.FilesCopied += filesCopied
	return nil
}

func createSymlinkAsset(asset Asset, destPath, basePath string, opts SyncOptions) error {
	targetPath := filepath.Join(basePath, asset.Link)

	if _, err := os.Stat(targetPath); err != nil {
		return fmt.Errorf("link target not found (%s): %w", targetPath, err)
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0750); err != nil {
		return fmt.Errorf("failed to create parent directory for symlink: %w", err)
	}

	// Remove existing destination when pruning is enabled or when stale entry exists
	if opts.Config.Strategy.PruneStale {
		if err := os.RemoveAll(destPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove existing path before linking: %w", err)
		}
	} else {
		if err := os.Remove(destPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove existing path before linking: %w", err)
		}
	}

	if err := os.Symlink(targetPath, destPath); err != nil {
		return fmt.Errorf("failed to create symlink %s -> %s: %w", destPath, targetPath, err)
	}

	if opts.Verbose {
		logger.Info(fmt.Sprintf("  Linked: %s -> %s", destPath, targetPath))
	}

	return nil
}

// copyFile copies a single file from src to dst
func copyFile(src, dst string) error {
	// Open source file
	srcFile, err := os.Open(src) // #nosec G304 - caller validates paths
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() {
		if cerr := srcFile.Close(); cerr != nil {
			logger.Warn(fmt.Sprintf("Failed to close source file %s: %v", src, cerr))
		}
	}()

	// Get source file info for permissions
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	// Create destination file
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode()) // #nosec G304 - caller validates paths
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func() {
		if cerr := dstFile.Close(); cerr != nil {
			logger.Warn(fmt.Sprintf("Failed to close destination file %s: %v", dst, cerr))
		}
	}()

	// Copy contents
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	return nil
}
