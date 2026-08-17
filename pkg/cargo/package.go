// Package cargo parses Cargo.lock and `cargo metadata` JSON into boring
// crate records (name, version, source). Hand-rolled PARSE only — no
// third-party TOML helper. Free of goneat policy, crates.io HTTP, and
// cargo-deny so it can be extracted later (e.g. gofulmen / pkg/cargolock).
package cargo

import "strings"

// SourceKind is the resolved origin of a crate.
type SourceKind string

const (
	// SourceRegistry is a registry package (usually crates.io).
	SourceRegistry SourceKind = "registry"
	// SourceGit is a git dependency.
	SourceGit SourceKind = "git"
	// SourcePath is a path or workspace package (no registry source).
	SourcePath SourceKind = "path"
)

// Package is one crate from a lockfile or cargo metadata.
type Package struct {
	Name      string
	Version   string
	Source    SourceKind
	RawSource string // original source= / metadata source; empty for path/workspace
}

// ClassifySource maps a Cargo.lock / cargo-metadata source string.
func ClassifySource(raw string) SourceKind {
	if raw == "" {
		return SourcePath
	}
	switch {
	case strings.HasPrefix(raw, "registry+") || strings.Contains(raw, "crates.io"):
		return SourceRegistry
	case strings.HasPrefix(raw, "git+") || strings.HasPrefix(raw, "git:"):
		return SourceGit
	case strings.HasPrefix(raw, "path+"):
		return SourcePath
	default:
		return SourcePath
	}
}

// IsCratesIO reports whether a raw source string points at crates.io.
func IsCratesIO(raw string) bool {
	return strings.Contains(raw, "crates.io")
}
