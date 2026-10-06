package sbom

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// artifactIdentity bounds an explicit-file collection with pathname identity
// and content checks. It is not protection against a hostile concurrent writer.
type artifactIdentity struct {
	path   string
	info   fs.FileInfo
	digest string
}

func inspectArtifact(ctx context.Context, path string) (_ *artifactIdentity, retErr error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(filepath.Dir(absolute))
	if err != nil {
		return nil, err
	}
	defer func() { retErr = errors.Join(retErr, root.Close()) }()
	// Root.Lstat takes its identity from a handle, including on Windows.
	info, err := root.Lstat(filepath.Base(absolute))
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("SBOM artifact must be a regular non-symlink file: %s", absolute)
	}
	file, err := root.Open(filepath.Base(absolute))
	if err != nil {
		return nil, err
	}
	opened, statErr := file.Stat()
	if err := errors.Join(statErr, file.Close()); err != nil {
		return nil, err
	}
	if !sameSourceFile(info, opened) {
		return nil, fmt.Errorf("SBOM artifact changed while opening: %s", absolute)
	}
	// File.Stat retains the handle's ID rather than a lazily reopened pathname.
	identity := &artifactIdentity{path: absolute, info: opened}
	identity.digest, err = identity.hash(ctx)
	if err != nil {
		return nil, err
	}
	return identity, nil
}

func (identity *artifactIdentity) hash(ctx context.Context) (digest string, retErr error) {
	// Check the absolute pathname as well as the rooted handle: replacement of
	// the parent directory must not cause us to verify an unlinked old subject.
	before, err := os.Lstat(identity.path)
	if err != nil || !sameSourceFile(identity.info, before) {
		return "", fmt.Errorf("SBOM artifact pathname changed: %s", identity.path)
	}
	root, err := os.OpenRoot(filepath.Dir(identity.path))
	if err != nil {
		return "", err
	}
	defer func() { retErr = errors.Join(retErr, root.Close()) }()
	digest, err = copySourceFile(ctx, root, nil, filepath.Base(identity.path), identity.info, identity.info.Size())
	if err != nil {
		return "", err
	}
	after, err := os.Lstat(identity.path)
	if err != nil || !sameSourceFile(identity.info, after) {
		return "", fmt.Errorf("SBOM artifact pathname changed: %s", identity.path)
	}
	return digest, nil
}

func (identity *artifactIdentity) verify(ctx context.Context) error {
	digest, err := identity.hash(ctx)
	if err != nil {
		return err
	}
	if digest != identity.digest {
		return fmt.Errorf("SBOM artifact content changed: %s", identity.path)
	}
	return nil
}
