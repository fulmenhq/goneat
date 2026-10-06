package sbom

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// sourceCollector owns a neutral working directory and explicit configuration.
// It reuses capture privacy, identity, reconciliation and cleanup guarantees,
// but never becomes part of the scanned source or artifact subject.
type sourceCollector struct {
	owned       *sourceCapture
	config      string
	environment []string
}

const sourceCollectorPolicy = "invocation-owned-config-cwd; SYFT-environment-removed; version-default-catalogers"

func newSourceCollector(ctx context.Context, target, temporaryParent string) (_ *sourceCollector, retErr error) {
	parent, err := filepath.Abs(temporaryParent)
	if err != nil {
		return nil, err
	}
	parent, err = filepath.EvalSymlinks(parent)
	if err != nil {
		return nil, fmt.Errorf("resolve collector working parent: %w", err)
	}
	realTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		return nil, err
	}
	if containedSourcePath(realTarget, parent) {
		return nil, fmt.Errorf("collector working directory must be outside target %s", target)
	}
	name, err := os.MkdirTemp(parent, "goneat-source-collector-")
	if err != nil {
		return nil, fmt.Errorf("create private collector working directory: %w", err)
	}
	owned := &sourceCapture{path: name, limits: sourceLimits{entries: sourceMaxEntries, bytes: sourceMaxBytes}}
	defer func() {
		if retErr != nil {
			retErr = errors.Join(retErr, owned.cleanup())
		}
	}()
	owned.root, err = os.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	owned.created, err = owned.root.Stat(".")
	if err != nil {
		return nil, err
	}
	if err := makeSourcePrivate(name); err != nil {
		return nil, err
	}
	file, err := owned.root.OpenFile("syft.yaml", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create private collector config: %w", err)
	}
	// No cataloger override: retain this Syft version's full default catalogers.
	// Explicit config plus neutral CWD plus the environment below form the policy;
	// an empty config on its own would not defeat ambient environment settings.
	_, writeErr := file.WriteString("{}\n")
	ownerErr := setSourceChildOwner(file)
	if err := errors.Join(writeErr, ownerErr, file.Close()); err != nil {
		return nil, fmt.Errorf("write private collector config: %w", err)
	}
	owned.manifest, err = scanSourceTree(ctx, owned.root, nil, owned.limits)
	if err != nil {
		return nil, err
	}
	if err := owned.verify(ctx); err != nil {
		return nil, err
	}
	return &sourceCollector{owned: owned, config: filepath.Join(name, "syft.yaml"), environment: sourceCollectorEnvironment(os.Environ())}, nil
}

func sourceCollectorEnvironment(environment []string) []string {
	result := make([]string, 0, len(environment))
	for _, value := range environment {
		key, _, _ := strings.Cut(value, "=")
		// Case-insensitive removal also covers Windows environment semantics.
		// There are currently no allowed SYFT_* variables. Unrelated OS/auth
		// values are preserved and are never logged or written to provenance.
		if !strings.HasPrefix(strings.ToUpper(key), "SYFT_") {
			result = append(result, value)
		}
	}
	return result
}
