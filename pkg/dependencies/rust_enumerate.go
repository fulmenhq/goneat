/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package dependencies

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/fulmenhq/goneat/pkg/cargo"
	"github.com/fulmenhq/goneat/pkg/logger"
)

// enumerateRustCratesForCooling lists crates from Cargo.lock (preferred) or
// `cargo metadata --format-version 1` JSON. It does not call cargo-deny.
func enumerateRustCratesForCooling(ctx context.Context, target string, timeout time.Duration) []Dependency {
	root := coolingEnumerateRoot(target)

	if lock := cargo.FindLock(root); lock != "" {
		pkgs, err := cargo.ParseLockFile(lock)
		if err != nil {
			logger.Warn(fmt.Sprintf("Cargo.lock parse failed: %v", err))
		} else if len(pkgs) > 0 {
			logger.Debug(fmt.Sprintf("Cargo.lock enumerated %d crates", len(pkgs)))
			return dependenciesFromCargoPackages(pkgs)
		}
	}

	if !IsCargoAvailable() {
		return nil
	}
	data, err := runCargoMetadataJSON(ctx, root, timeout)
	if err != nil {
		logger.Debug(fmt.Sprintf("cargo metadata fallback failed: %v", err))
		return nil
	}
	pkgs, err := cargo.ParseMetadataJSON(data)
	if err != nil {
		logger.Warn(fmt.Sprintf("cargo metadata JSON parse failed: %v", err))
		return nil
	}
	logger.Debug(fmt.Sprintf("cargo metadata enumerated %d crates", len(pkgs)))
	return dependenciesFromCargoPackages(pkgs)
}

func coolingEnumerateRoot(target string) string {
	if project := DetectRustProject(target); project != nil {
		if root := project.EffectiveRoot(); root != "" {
			return root
		}
		if project.RootPath != "" {
			return project.RootPath
		}
	}
	return target
}

func runCargoMetadataJSON(ctx context.Context, dir string, timeout time.Duration) ([]byte, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	// format-version 1 is cargo's stable metadata contract (not cargo-deny).
	cmd := exec.CommandContext(ctx, "cargo", "metadata", "--format-version", "1", "--offline") // #nosec G204
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("cargo metadata: %w: %s", err, stderr.String())
	}
	return stdout.Bytes(), nil
}

func dependenciesFromCargoPackages(pkgs []cargo.Package) []Dependency {
	deps := make([]Dependency, 0, len(pkgs))
	seen := map[string]bool{}
	for _, pkg := range pkgs {
		if pkg.Name == "" {
			continue
		}
		key := pkg.Name + "@" + pkg.Version
		if seen[key] {
			continue
		}
		seen[key] = true
		deps = append(deps, Dependency{
			Module: Module{
				Name:     pkg.Name,
				Version:  pkg.Version,
				Language: LanguageRust,
			},
			Metadata: metadataFromCargoPackage(pkg),
		})
	}
	return deps
}

func metadataFromCargoPackage(pkg cargo.Package) map[string]interface{} {
	meta := map[string]interface{}{}
	switch pkg.Source {
	case cargo.SourcePath:
		meta["is_local"] = true
		meta["age_days"] = 0
	case cargo.SourceRegistry:
		if cargo.IsCratesIO(pkg.RawSource) {
			meta["registry"] = "crates.io"
		} else {
			meta["source"] = pkg.RawSource
			meta["age_unknown"] = true
			meta["registry_error"] = "not a crates.io package"
		}
	default:
		if pkg.RawSource != "" {
			meta["source"] = pkg.RawSource
		}
		meta["age_unknown"] = true
		meta["registry_error"] = "not a crates.io package"
	}
	return meta
}

func overlayLicenses(deps []Dependency, licensed []Dependency) {
	index := map[string]*License{}
	for i := range licensed {
		index[licensed[i].Name+"@"+licensed[i].Version] = licensed[i].License
	}
	for i := range deps {
		if lic, ok := index[deps[i].Name+"@"+deps[i].Version]; ok && lic != nil {
			deps[i].License = lic
		}
	}
}
