package sbom

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// publishSourceOutput is the final lifecycle boundary. An empty destination
// means the caller may return the validated bytes for stdout, after cleanup.
// No destination-local staging file exists until private-source cleanup succeeds.
func publishSourceOutput(ctx context.Context, content []byte, destination string, cleanup func() error) (retErr error) {
	if cleanup != nil {
		if err := cleanup(); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if destination == "" {
		return nil
	}
	absolute, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	parent := filepath.Dir(absolute)
	if err := os.MkdirAll(parent, 0o750); err != nil {
		return fmt.Errorf("create SBOM output directory: %w", err)
	}
	if info, err := os.Lstat(absolute); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("SBOM destination must be a regular non-symlink file: %s", absolute)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file, err := os.CreateTemp(parent, ".goneat-sbom-*")
	if err != nil {
		return fmt.Errorf("stage SBOM output beside destination: %w", err)
	}
	staged := file.Name()
	closed, published := false, false
	defer func() {
		if !closed {
			retErr = errors.Join(retErr, file.Close())
		}
		if !published {
			if err := os.Remove(staged); err != nil && !errors.Is(err, os.ErrNotExist) {
				retErr = errors.Join(retErr, fmt.Errorf("SBOM output cleanup failed; staged data may remain at %s: %w", staged, err))
			}
		}
	}()
	if _, err := file.Write(content); err != nil {
		return fmt.Errorf("write staged SBOM: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync staged SBOM: %w", err)
	}
	err = file.Close()
	closed = true
	if err != nil {
		return fmt.Errorf("close staged SBOM: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := replaceSourceOutput(staged, absolute); err != nil {
		return fmt.Errorf("publish SBOM output: %w", err)
	}
	published = true
	return nil
}
