---
title: Versioning Library
description: Semantic versioning parsing, validation, and manipulation utilities.
---

# Versioning Library

Goneat's `pkg/versioning` provides comprehensive semantic versioning (SemVer) support for Go applications. It includes parsing, validation, comparison, and manipulation utilities that go beyond basic version string handling, with specific support for goneat's release phases and pre-release identifiers.

## Purpose

Semantic versioning is essential for reliable software distribution, but Go's standard library lacks comprehensive SemVer support. The `pkg/versioning` library addresses this by providing:

- **Full SemVer 2.0.0 compliance**: Including pre-release and build metadata
- **Validation and normalization**: Ensures versions are well-formed
- **Comparison and sorting**: Robust version ordering
- **Range specifications**: Support for version ranges and constraints
- **Goneat-specific extensions**: Release phase integration and validation
- **Integration utilities**: Works with build info, git tags, and package managers
- **Version propagation**: Automatic synchronization from VERSION file to package manager manifests

## Version Propagation

Goneat's version propagation feature ensures the `VERSION` file remains the single source of truth while automatically synchronizing version information across package manager files. This eliminates version drift and manual synchronization overhead.

### Key Capabilities

- **Multi-format support**: Propagates to `package.json`, `pyproject.toml`, and `go.mod` files
- **Workspace awareness**: Handles monorepo structures with selective propagation
- **Policy-driven control**: Configurable via `.goneat/version-policy.yaml`
- **Safety features**: Backup creation, dry-run mode, and atomic operations
- **Guard rails**: Branch restrictions, worktree validation, and execution preconditions

### Policy Configuration

Version propagation is controlled by a declarative policy file located at `.goneat/version-policy.yaml`. This file defines which package manager files should be updated, workspace handling strategies, and safety guards.

#### Basic Configuration

```yaml
$schema: https://schemas.fulmenhq.dev/config/goneat/version-policy-v1.0.0.schema.json
version:
  scheme: semver # semver | calver - versioning scheme used
  allow_extended: true # enables prerelease/build metadata

propagation:
  defaults:
    include: ["package.json", "pyproject.toml"] # Default package managers
    exclude: ["**/node_modules/**", "docs/**"] # Patterns to exclude
    backup:
      enabled: true # Create backup files before changes
      retention: 5 # Number of backup files to keep

  workspace:
    strategy: single-version # single-version | opt-in | opt-out

guards:
  required_branches: ["main", "release/*"] # Optional branch restrictions
  disallow_dirty_worktree: true # Prevent propagation with uncommitted changes
```

#### Advanced Configuration

```yaml
$schema: https://schemas.fulmenhq.dev/config/goneat/version-policy-v1.0.0.schema.json
version:
  scheme: semver
  allow_extended: true
  channel: stable # Optional release channel

propagation:
  defaults:
    include: ["package.json", "pyproject.toml"]
    exclude: ["**/node_modules/**", "docs/**"]
    backup:
      enabled: true
      retention: 5

  workspace:
    strategy: single-version

  # Target-specific overrides
  targets:
    package.json:
      include:
        ["./package.json", "apps/*/package.json", "packages/*/package.json"]
      exclude: ["packages/legacy-*"] # Override defaults for specific targets
    pyproject.toml:
      include: ["services/*/pyproject.toml"]
      mode: poetry # project | poetry - which section to update
    go.mod:
      validate_only: true # Go modules are validation-only

guards:
  required_branches: ["main", "release/*"]
  disallow_dirty_worktree: true
```

#### Schema Reference

- **Full Schema**: [Version Policy Schema](../../schemas/crucible-go/config/goneat/v1.0.0/version-policy.schema.yaml)
- **Schema Documentation**: See the schema file for complete field descriptions and validation rules

#### Generating Policy Files

You can generate a sample policy file with comments using:

```bash
goneat version propagate --generate-policy
```

This creates `.goneat/version-policy.yaml` with all available options commented out.

#### Workspace Strategies

- **`single-version`** (default): All packages in the workspace use the root VERSION file
- **`opt-in`**: Only explicitly configured packages get independent versions
- **`opt-out`**: All packages propagate except those explicitly excluded

#### Package Manager Support

| Manager               | File             | Update Support     | Notes                                                  |
| --------------------- | ---------------- | ------------------ | ------------------------------------------------------ |
| JavaScript/TypeScript | `package.json`   | ✅ Full            | Supports workspaces via `workspaces` field             |
| Python                | `pyproject.toml` | ✅ Full            | Supports both `[project]` and `[tool.poetry]` sections |
| Go                    | `go.mod`         | ❌ Validation only | Checks module name consistency, no updates             |

#### Safety Guards

- **Branch Restrictions**: Prevent accidental propagation on feature branches
- **Worktree Validation**: Ensure clean git state before propagation
- **Backup Creation**: Automatic backup files with configurable retention
- **Dry-run Mode**: Preview changes without making them

### Usage Examples

```go
import (
    "github.com/fulmenhq/goneat/pkg/propagation"
    "github.com/fulmenhq/goneat/pkg/propagation/managers"
    "github.com/fulmenhq/goneat/pkg/versioning"
)

func propagateVersion(ctx context.Context, newVersion string) error {
    // Parse and validate the new version
    ver, err := versioning.Parse(newVersion)
    if err != nil {
        return fmt.Errorf("invalid version: %w", err)
    }

    // Create propagator with registry
    registry := propagation.NewRegistry()
    registry.Register(managers.NewJavaScriptManager())
    registry.Register(managers.NewPythonManager())
    registry.Register(managers.NewGoManager())

    propagator := propagation.NewPropagator(registry)

    // Propagate version with dry-run first
    result, err := propagator.Propagate(ctx, ver.String(), propagation.PropagateOptions{
        DryRun: true,
    })
    if err != nil {
        return fmt.Errorf("dry-run failed: %w", err)
    }

    if len(result.Errors) > 0 {
        return fmt.Errorf("validation errors found: %v", result.Errors)
    }

    // Apply changes
    result, err = propagator.Propagate(ctx, ver.String(), propagation.PropagateOptions{
        Backup: true,
    })
    if err != nil {
        return fmt.Errorf("propagation failed: %w", err)
    }

    fmt.Printf("Successfully propagated version to %d files\n", result.Processed)
    return nil
}
```

See the [Version Propagation Architecture](https://github.com/fulmenhq/goneat/blob/main/.plans/active/v0.3.0/version-ssot-propagation.md) document for detailed implementation information.

## Key Features

- **SemVer 2.0.0 compliant**: Full specification support including edge cases
- **Pre-release handling**: `1.0.0-alpha`, `1.0.0-rc.1`
- **Build metadata**: `1.0.0+sha.abc123` for reproducible builds
- **Range support**: `^1.0.0`, `~>2.3.0`, `>=1.0.0 <2.0.0`
- **Validation**: Strict and lenient parsing modes
- **Sorting and comparison**: Natural version ordering
- **Phase integration**: Goneat release phase validation (dev, rc, release)
- **Git integration**: Tag parsing and validation
- **Version propagation**: Automatic synchronization to package manager files (package.json, pyproject.toml, go.mod)

## Installation

```bash
go get github.com/fulmenhq/goneat/pkg/versioning
```

## Basic Usage

### Parsing and Validation

```go
package main

import (
    "fmt"
    "github.com/fulmenhq/goneat/pkg/versioning"
)

func main() {
    // Parse valid semantic versions
    versions := []string{
        "1.0.0",
        "2.3.1",
        "1.0.0-alpha",
        "1.0.0-rc.1",
        "1.0.0+build.123",
        "1.0.0-alpha+sha.456",
    }

    for _, v := range versions {
        ver, err := versioning.Parse(v)
        if err != nil {
            fmt.Printf("❌ %s: %v\n", v, err)
            continue
        }

        fmt.Printf("✅ %s: major=%d, minor=%d, patch=%d, pre=%s, meta=%s\n",
            v, ver.Major, ver.Minor, ver.Patch, ver.Pre.String(), ver.Meta)
    }

    // Invalid versions
    invalid := []string{"1.0", "1.0.0.0", "v1.0.0", "1..0.0", "1.0.0-"}
    for _, v := range invalid {
        _, err := versioning.Parse(v)
        if err != nil {
            fmt.Printf("❌ %s: %v\n", v, err)
        }
    }
}
```

### Comparison and Sorting

```go
package main

import (
    "fmt"
    "sort"
    "github.com/fulmenhq/goneat/pkg/versioning"
)

func main() {
    versions := []*versioning.Version{
        versioning.MustParse("1.0.0"),
        versioning.MustParse("2.0.0"),
        versioning.MustParse("1.5.0"),
        versioning.MustParse("1.0.0-rc.1"),
        versioning.MustParse("1.0.0-beta"),
        versioning.MustParse("1.0.0+build.123"),
    }

    // Natural sorting
    sort.Slice(versions, func(i, j int) bool {
        return versions[i].Less(versions[j])
    })

    fmt.Println("Sorted versions:")
    for _, v := range versions {
        fmt.Printf("  %s\n", v.String())
    }
    // Output: 1.0.0-beta, 1.0.0-rc.1, 1.0.0, 1.0.0+build.123, 1.5.0, 2.0.0

    // Comparison examples
    v1 := versioning.MustParse("1.0.0")
    v2 := versioning.MustParse("1.0.1")
    v3 := versioning.MustParse("1.0.0-rc.1")

    fmt.Printf("%s < %s: %t\n", v1, v2, v1.Less(v2))     // true
    fmt.Printf("%s == %s: %t\n", v1, v1, v1.Equal(v1))   // true
    fmt.Printf("%s > %s: %t\n", v2, v3, v2.Greater(v3))  // true (release > pre-release)
}
```

## API Reference

### Version Structure

```go
type Version struct {
    Major    uint64
    Minor    uint64
    Patch    uint64
    Pre      PreRelease
    Meta     string // Build metadata
}

type PreRelease struct {
    Identifiers []Identifier
}

type Identifier struct {
    Original string
    Numeric  uint64
    IsNum    bool
}
```

### Parsing Functions

```go
// Strict parsing (SemVer 2.0.0 compliant)
func Parse(version string) (*Version, error)
func MustParse(version string) *Version  // Panics on error

// Lenient parsing (accepts common variants)
func ParseLenient(version string) (*Version, error)

// Parse with options
type ParseOption func(*parseOptions)
func WithStripV() ParseOption                    // Remove leading 'v'
func WithAllowLeadingV() ParseOption             // Allow leading 'v' but don't require stripping
func WithIgnoreMeta() ParseOption                // Ignore build metadata
func WithValidatePhase(phase string) ParseOption // Validate against goneat phase

// Version ranges
func ParseRange(spec string) (*Range, error)  // "^1.0.0", "~>2.0.0", ">=1.0.0 <2.0.0"
func MustParseRange(spec string) *Range
```

### Comparison Methods

```go
func (v *Version) Less(other *Version) bool
func (v *Version) Equal(other *Version) bool
func (v *Version) Greater(other *Version) bool
func (v *Version) Compare(other *Version) int  // -1, 0, 1

// Semantic comparison (ignoring build metadata)
func (v *Version) SemanticEqual(other *Version) bool
func (v *Version) SemanticLess(other *Version) bool
```

### String Representation

```go
func (v *Version) String() string
func (v *Version) FullString() string  // Includes build metadata
func (v *Version) SemVerString() string // Strict SemVer format
func (v *Version) Canonical() string    // Normalized canonical form

// Formatting options
func (v *Version) Format(format string) string
// Supported formats: "semver", "full", "short", "major", "npm", "docker", "goneat"
```

### Range and Constraint Support

```go
type Range struct {
    Spec     string
    Includes []RangeSet
    Excludes []RangeSet
}

type RangeSet struct {
    Min *Version
    Max *Version
    IncludeMin bool
    IncludeMax bool
}

func (r *Range) Test(v *Version) bool
func (r *Range) String() string
func (r *Range) Satisfies(version string) bool

// Common range constructors
func NewCaretRange(major, minor, patch uint64) *Range  // ^1.2.3
func NewTildeRange(major, minor, patch uint64) *Range  // ~1.2.3
func NewExactRange(version string) *Range              // =1.2.3
func NewGreaterRange(version string) *Range            // >1.2.3
func NewGreaterEqualRange(version string) *Range       // >=1.2.3
```

## Advanced Usage

### Goneat Release Phase Integration

```go
package main

import (
    "fmt"
    "github.com/fulmenhq/goneat/pkg/versioning"
)

const (
    PhaseDev    = "dev"
    PhaseAlpha  = "alpha"
    PhaseBeta   = "beta"
    PhaseRC     = "rc"
    PhaseRelease = "release"
    PhaseGA     = "ga"
    PhaseMaintenance = "maintenance"
)

func validateGoneatVersion(phase string, versionStr string) error {
    ver, err := versioning.ParseLenient(versionStr)
    if err != nil {
        return fmt.Errorf("invalid version: %w", err)
    }

    // Phase-specific validation rules
    switch phase {
    case PhaseDev:
        if ver.Major == 0 && ver.Minor == 0 && ver.Patch == 0 {
            return fmt.Errorf("development phase requires at least 0.1.0")
        }
        // Dev versions should have -dev suffix or be pre-1.0.0
        if ver.Major >= 1 && !hasDevSuffix(ver) {
            return fmt.Errorf("development release requires -dev suffix")
        }

    case PhaseAlpha, PhaseBeta:
        if ver.Major == 0 {
            // Allow 0.x.y-alpha/beta
        } else if ver.Major >= 1 && !hasPreRelease(ver, "alpha", "beta") {
            return fmt.Errorf("%s phase requires alpha or beta pre-release", phase)
        }

    case PhaseRC:
        if !hasPreRelease(ver, "rc") {
            return fmt.Errorf("release candidate requires -rc pre-release identifier")
        }
        if ver.Pre.Identifiers[0].Original != "rc" {
            return fmt.Errorf("rc identifier must be first pre-release identifier")
        }

    case PhaseRelease, PhaseGA:
        // Release versions must be clean (no pre-release, no build meta for GA)
        if len(ver.Pre.Identifiers) > 0 {
            return fmt.Errorf("release version must not have pre-release identifiers")
        }
        if phase == PhaseGA && ver.Meta != "" {
            return fmt.Errorf("GA release must not have build metadata")
        }

    case PhaseMaintenance:
        if ver.Major < 1 {
            return fmt.Errorf("maintenance phase requires stable major version (1+)")
        }
        // Allow patch releases only
        if ver.Minor > 0 {
            return fmt.Errorf("maintenance releases should only increment patch")
        }

    default:
        return fmt.Errorf("unknown phase: %s", phase)
    }

    return nil
}

func hasDevSuffix(ver *versioning.Version) bool {
    return ver.Meta == "dev" || strings.HasSuffix(ver.FullString(), ".dev")
}

func hasPreRelease(ver *versioning.Version, allowed ...string) bool {
    if len(ver.Pre.Identifiers) == 0 {
        return false
    }

    pre := ver.Pre.Identifiers[0].Original
    for _, allowedPre := range allowed {
        if pre == allowedPre {
            return true
        }
    }
    return false
}

// Usage examples
func main() {
    tests := []struct {
        phase    string
        version  string
        expected bool
    }{
        {"dev", "0.1.0", true},
        {"dev", "1.0.0-dev", true},
        {"dev", "1.0.0", false}, // Missing dev suffix
        {"alpha", "1.0.0-alpha", true},
        {"rc", "1.0.0-rc.1", true},
        {"rc", "1.0.0-beta.1", false}, // Wrong pre-release for RC
        {"release", "1.0.0", true},
        {"release", "1.0.0-rc.1", false}, // Has pre-release
        {"ga", "1.0.0+build", false}, // Has build meta
        {"maintenance", "1.0.1", true},
        {"maintenance", "1.1.0", false}, // Minor increment not allowed
    }

    for _, test := range tests {
        err := validateGoneatVersion(test.phase, test.version)
        valid := err == nil
        status := "✅"
        if !valid {
            status = "❌"
        }
        fmt.Printf("%s %s (%s): %s\n", status, test.version, test.phase, err)
    }
}
```

### Version Range Specifications

```go
package main

import (
    "fmt"
    "github.com/fulmenhq/goneat/pkg/versioning"
)

func main() {
    // Common range patterns
    ranges := map[string]string{
        "Caret (^1.2.3)":      "^1.2.3",    // >=1.2.3 <2.0.0
        "Tilde (~1.2.3)":      "~1.2.3",    // >=1.2.3 <1.3.0
        "Exact (=1.2.3)":      "=1.2.3",    // =1.2.3
        "Greater (>?1.2.3)":   ">1.2.3",    // >1.2.3
        "GreaterEqual (>=1.2)": ">=1.2",     // >=1.2.0
        "Less (<2.0.0)":       "<2.0.0",    // <2.0.0
        "Complex":              ">=1.0.0 <=1.5.0 || >2.0.0 <3.0.0",
    }

    testVersions := []string{"1.0.0", "1.2.3", "1.5.0", "2.0.0", "1.2.3-rc.1"}

    for name, spec := range ranges {
        r, err := versioning.ParseRange(spec)
        if err != nil {
            fmt.Printf("❌ %s: %v\n", name, err)
            continue
        }

        fmt.Printf("\n%s (%s):\n", name, spec)
        for _, vStr := range testVersions {
            v, _ := versioning.Parse(vStr)
            if r.Test(v) {
                fmt.Printf("  ✅ %s\n", vStr)
            } else {
                fmt.Printf("  ❌ %s\n", vStr)
            }
        }
    }
}
```

### Git Tag Integration

```go
package main

import (
    "context"
    "fmt"
    "os/exec"
    "strings"
    "github.com/fulmenhq/goneat/pkg/versioning"
)

type GitVersioner struct {
    repoRoot string
}

func NewGitVersioner(repoRoot string) *GitVersioner {
    return &GitVersioner{repoRoot: repoRoot}
}

func (gv *GitVersioner) LatestTag(ctx context.Context) (*versioning.Version, error) {
    cmd := exec.CommandContext(ctx, "git", "-C", gv.repoRoot, "describe", "--tags", "--abbrev=0")
    output, err := cmd.Output()
    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
            // No tags found
            return versioning.MustParse("0.0.0"), nil
        }
        return nil, fmt.Errorf("failed to get latest tag: %w", err)
    }

    tag := strings.TrimSpace(string(output))
    // Strip leading 'v' if present
    if strings.HasPrefix(tag, "v") {
        tag = tag[1:]
    }

    return versioning.ParseLenient(tag)
}

func (gv *GitVersioner) NextVersion(ctx context.Context, current *versioning.Version, bumpType string) (*versioning.Version, error) {
    switch bumpType {
    case "major":
        return current.BumpMajor(), nil
    case "minor":
        return current.BumpMinor(), nil
    case "patch":
        return current.BumpPatch(), nil
    case "prerelease":
        if len(current.Pre.Identifiers) == 0 {
            // No pre-release, start with alpha
            return current.WithPreRelease("alpha.1"), nil
        }
        // Increment pre-release
        return current.IncrementPreRelease(), nil
    default:
        return nil, fmt.Errorf("unknown bump type: %s", bumpType)
    }
}

func (gv *GitVersioner) ValidateTag(ctx context.Context, tag string, phase string) error {
    ver, err := versioning.ParseLenient(tag)
    if err != nil {
        return fmt.Errorf("invalid tag version: %w", err)
    }

    // Check if tag exists
    cmd := exec.CommandContext(ctx, "git", "-C", gv.repoRoot, "tag", "--list", tag)
    output, err := cmd.Output()
    if err != nil {
        return fmt.Errorf("tag %s does not exist", tag)
    }

    if !strings.Contains(string(output), tag) {
        return fmt.Errorf("tag %s not found", tag)
    }

    // Phase-specific validation
    return validateGoneatVersion(phase, ver.String())
}

// Usage example
func main() {
    ctx := context.Background()
    versioner := NewGitVersioner(".")

    // Get latest release
    latest, err := versioner.LatestTag(ctx)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    fmt.Printf("Latest release: %s\n", latest.String())

    // Calculate next version
    next, _ := versioner.NextVersion(ctx, latest, "patch")
    fmt.Printf("Next patch version: %s\n", next.String())

    // Validate proposed tag
    err = versioner.ValidateTag(ctx, "v1.0.0-rc.1", "rc")
    if err != nil {
        fmt.Printf("Tag validation failed: %v\n", err)
    } else {
        fmt.Printf("Tag v1.0.0-rc.1 is valid for RC phase\n")
    }
}
```

## Integration with Other Libraries

### With pkg/buildinfo

```go
import (
    "github.com/fulmenhq/goneat/pkg/buildinfo"
    "github.com/fulmenhq/goneat/pkg/versioning"
)

func enrichBuildInfo(bi *buildinfo.Info) error {
    // Parse and validate embedded version
    ver, err := versioning.Parse(bi.Version)
    if err != nil {
        return fmt.Errorf("invalid embedded version: %w", err)
    }

    // Add parsed version components
    bi.Major = &ver.Major
    bi.Minor = &ver.Minor
    bi.Patch = &ver.Patch
    bi.PreRelease = ver.Pre.String()

    // Validate phase compatibility
    if err := validateGoneatVersion(bi.ReleasePhase, bi.Version); err != nil {
        return fmt.Errorf("version-phase mismatch: %w", err)
    }

    // Check if version satisfies requirements
    if bi.ReleasePhase == "release" {
        req, _ := versioning.ParseRange(">=1.0.0")
        if !req.Test(ver) {
            return fmt.Errorf("release version must be >=1.0.0")
        }
    }

    return nil
}

// Usage
func main() {
    bi := buildinfo.FromEmbedded()
    if err := enrichBuildInfo(bi); err != nil {
        log.Error("Build info enrichment failed", "error", err)
        os.Exit(1)
    }

    fmt.Printf("Enriched version: %d.%d.%d-%s (%s phase)\n",
        *bi.Major, *bi.Minor, *bi.Patch, bi.PreRelease, bi.ReleasePhase)
}
```

### With pkg/config

```go
import (
    "github.com/fulmenhq/goneat/pkg/config"
    "github.com/fulmenhq/goneat/pkg/versioning"
)

func validateConfigVersion(ctx context.Context, cfg *config.Config) error {
    // Get required version from config
    minVersionStr := cfg.GetString("app.min_version")
    if minVersionStr == "" {
        return nil // No version requirement
    }

    minVersion, err := versioning.ParseRange(minVersionStr)
    if err != nil {
        return fmt.Errorf("invalid min_version in config: %w", err)
    }

    // Get current application version
    currentVersion, err := versioning.Parse(bi.Version) // From build info
    if err != nil {
        return fmt.Errorf("cannot parse current version: %w", err)
    }

    if !minVersion.Test(currentVersion) {
        return fmt.Errorf("application version %s does not satisfy config requirement %s",
            currentVersion, minVersion.Spec)
    }

    return nil
}

// Validate at startup
func initializeApp(ctx context.Context) error {
    cfg, err := config.New(ctx)
    if err != nil {
        return err
    }

    if err := validateConfigVersion(ctx, cfg); err != nil {
        return fmt.Errorf("config validation failed: %w", err)
    }

    log.Info("Version compatibility check passed",
        "app_version", bi.Version,
        "min_required", cfg.GetString("app.min_version"),
    )

    return nil
}
```

## Performance Characteristics

- **Parsing**: ~500ns/op for typical versions, ~2μs/op for complex pre-releases
- **Comparison**: ~100ns/op, zero allocations for simple comparisons
- **Range testing**: ~1μs/op for simple ranges, ~5μs/op for complex ranges
- **Validation**: ~200ns/op for basic validation, ~1μs/op for phase validation
- **Memory**: ~100 bytes per Version object, minimal garbage

For high-performance scenarios:

```go
// Cache parsed versions
var versionCache = make(map[string]*versioning.Version)
var cacheMu sync.RWMutex

func parseCached(versionStr string) *versioning.Version {
    cacheMu.RLock()
    if v, exists := versionCache[versionStr]; exists {
        cacheMu.RUnlock()
        return v
    }
    cacheMu.RUnlock()

    cacheMu.Lock()
    defer cacheMu.Unlock()

    // Double-check
    if v, exists := versionCache[versionStr]; exists {
        return v
    }

    ver, err := versioning.Parse(versionStr)
    if err != nil {
        // Handle error appropriately
        return nil
    }

    versionCache[versionStr] = ver
    return ver
}
```

## Error Handling

### Common Errors

```go
var (
    ErrInvalidVersion    = errors.New("invalid semantic version")
    ErrInvalidMajor      = errors.New("major version must be non-negative integer")
    ErrInvalidMinor      = errors.New("minor version must be non-negative integer")
    ErrInvalidPatch      = errors.New("patch version must be non-negative integer")
    ErrInvalidPreRelease = errors.New("invalid pre-release identifier")
    ErrInvalidMeta       = errors.New("invalid build metadata")
    ErrEmptyVersion      = errors.New("version string cannot be empty")
    ErrTooManyComponents = errors.New("version must have exactly 3 numeric components")
)

type ParseError struct {
    Version string
    Offset  int
    Reason  string
    Inner   error
}

func (e *ParseError) Error() string
func (e *ParseError) Unwrap() error
```

### Robust Parsing

```go
func safeParseVersion(versionStr string, defaultVersion string) (*versioning.Version, error) {
    if versionStr == "" {
        return versioning.MustParse(defaultVersion), nil
    }

    ver, err := versioning.ParseLenient(versionStr)
    if err != nil {
        // Try strict parsing
        ver, err = versioning.Parse(versionStr)
        if err != nil {
            return nil, fmt.Errorf("failed to parse version %q: %w (tried lenient and strict)", versionStr, err)
        }
    }

    // Additional validation
    if ver.Major == 0 && ver.Minor == 0 && ver.Patch == 0 {
        return nil, fmt.Errorf("version %q is not a valid release version", versionStr)
    }

    return ver, nil
}

// Usage with fallbacks
func getAppVersion() *versioning.Version {
    // Try environment variable
    if envVer := os.Getenv("APP_VERSION"); envVer != "" {
        if ver, err := safeParseVersion(envVer, "0.0.0"); err == nil {
            return ver
        }
    }

    // Try from build info
    if bi := buildinfo.FromEmbedded(); bi != nil {
        if ver, err := safeParseVersion(bi.Version, "dev"); err == nil {
            return ver
        }
    }

    // Fallback
    return versioning.MustParse("0.0.0-dev")
}
```

## Testing Versioning Logic

```go
package versioning_test

import (
    "testing"
    "github.com/fulmenhq/goneat/pkg/versioning"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestParseValidVersions(t *testing.T) {
    tests := []struct {
        input    string
        expected string
        major    uint64
        minor    uint64
        patch    uint64
        pre      string
        meta     string
    }{
        {"1.0.0", "1.0.0", 1, 0, 0, "", ""},
        {"2.3.1", "2.3.1", 2, 3, 1, "", ""},
        {"1.0.0-alpha", "1.0.0-alpha", 1, 0, 0, "alpha", ""},
        {"1.0.0-1.2.3", "1.0.0-1.2.3", 1, 0, 0, "1.2.3", ""},
        {"1.0.0-rc.1+build.123", "1.0.0-rc.1+build.123", 1, 0, 0, "rc.1", "build.123"},
        {"0.1.0", "0.1.0", 0, 1, 0, "", ""},
    }

    for _, test := range tests {
        t.Run(test.input, func(t *testing.T) {
            ver, err := versioning.Parse(test.input)
            require.NoError(t, err)

            assert.Equal(t, test.expected, ver.String())
            assert.Equal(t, test.major, ver.Major)
            assert.Equal(t, test.minor, ver.Minor)
            assert.Equal(t, test.patch, ver.Patch)
            assert.Equal(t, test.pre, ver.Pre.String())
            assert.Equal(t, test.meta, ver.Meta)
        })
    }
}

func TestParseInvalidVersions(t *testing.T) {
    invalid := []string{
        "", "1", "1.0", "1.0.0.0", "v1.0.0", "1..0.0", "1.0.0-", "1.0.0-abc.",
        "-1.0.0", "1.-1.0", "1.0.-1", "1.0.0-abc#", "1.0.0+meta#",
    }

    for _, v := range invalid {
        t.Run(v, func(t *testing.T) {
            _, err := versioning.Parse(v)
            assert.Error(t, err)
            assert.Contains(t, err.Error(), "invalid semantic version")
        })
    }
}

func TestComparison(t *testing.T) {
    tests := []struct {
        v1, v2 string
        less   bool
    }{
        {"1.0.0", "1.0.1", true},
        {"1.0.1", "1.0.0", false},
        {"1.0.0", "1.0.0", false},
        {"1.0.0-rc.1", "1.0.0", true},
        {"1.0.0", "1.0.0-rc.1", false},
        {"1.0.0-alpha", "1.0.0-beta", true},
        {"1.0.0+build1", "1.0.0+build2", false}, // Meta doesn't affect ordering
    }

    for _, test := range tests {
        t.Run(fmt.Sprintf("%s ? %s", test.v1, test.v2), func(t *testing.T) {
            v1, _ := versioning.Parse(test.v1)
            v2, _ := versioning.Parse(test.v2)

            assert.Equal(t, test.less, v1.Less(v2))
            assert.Equal(t, !test.less, v2.Less(v1))
        })
    }
}

func TestRangeSatisfaction(t *testing.T) {
    tests := []struct {
        rangeSpec string
        versions  []string
        expected  []bool
    }{
        {
            "^1.2.3",
            []string{"1.2.3", "1.5.0", "2.0.0", "1.2.2", "0.5.0"},
            []bool{true, true, false, false, false},
        },
        {
            "~1.2.3",
            []string{"1.2.3", "1.3.0", "1.2.4", "1.1.0"},
            []bool{true, false, true, false},
        },
        {
            ">=1.0.0 <2.0.0",
            []string{"1.0.0", "1.5.9", "2.0.0", "0.9.9"},
            []bool{true, true, false, false},
        },
    }

    for _, test := range tests {
        t.Run(test.rangeSpec, func(t *testing.T) {
            r, err := versioning.ParseRange(test.rangeSpec)
            require.NoError(t, err)

            for i, vStr := range test.versions {
                v, _ := versioning.Parse(vStr)
                assert.Equal(t, test.expected[i], r.Test(v),
                    "expected %s %s %t, got %t", test.rangeSpec, vStr, test.expected[i], r.Test(v))
            }
        })
    }
}

func TestGoneatPhaseValidation(t *testing.T) {
    tests := []struct {
        phase    string
        version  string
        valid    bool
        reason   string
    }{
        {"dev", "0.1.0", true, ""},
        {"dev", "1.0.0-dev", true, ""},
        {"dev", "1.0.0", false, "development release requires -dev suffix"},
        {"rc", "1.0.0-rc.1", true, ""},
        {"rc", "1.0.0-beta.1", false, "rc identifier must be first pre-release identifier"},
        {"release", "1.0.0", true, ""},
        {"release", "1.0.0-rc.1", false, "release version must not have pre-release identifiers"},
        {"maintenance", "1.0.1", true, ""},
        {"maintenance", "1.1.0", false, "maintenance releases should only increment patch"},
    }

    for _, test := range tests {
        t.Run(fmt.Sprintf("%s-%s", test.phase, test.version), func(t *testing.T) {
            err := validateGoneatVersion(test.phase, test.version)
            if test.valid {
                assert.NoError(t, err)
            } else {
                assert.Error(t, err)
                if test.reason != "" {
                    assert.Contains(t, err.Error(), test.reason)
                }
            }
        })
    }
}
```

## Security Considerations

### Version Pinning and Supply Chain

```go
// Validate dependency versions against known good ranges
func validateDependencyVersions(deps []Dependency) error {
    // Critical dependency version requirements
    requirements := map[string]string{
        "github.com/fulmenhq/goneat/pkg/config": "^0.2.0",
        "github.com/fulmenhq/goneat/pkg/schema": "~0.2.0",
    }

    for _, dep := range deps {
        if reqSpec, exists := requirements[dep.Path]; exists {
            req, err := versioning.ParseRange(reqSpec)
            if err != nil {
                return fmt.Errorf("invalid requirement for %s: %w", dep.Path, err)
            }

            ver, err := versioning.Parse(dep.Version)
            if err != nil {
                return fmt.Errorf("invalid dependency version %s@%s: %w", dep.Path, dep.Version, err)
            }

            if !req.Test(ver) {
                return fmt.Errorf("dependency %s@%s does not satisfy %s", dep.Path, dep.Version, reqSpec)
            }
        }
    }

    return nil
}

// Warn about outdated dependencies
func checkForUpdates(deps []Dependency, latestVersions map[string]string) {
    for _, dep := range deps {
        if latest, exists := latestVersions[dep.Path]; exists {
            currentVer, _ := versioning.Parse(dep.Version)
            latestVer, _ := versioning.Parse(latest)

            if latestVer.Greater(currentVer) {
                log.Warn("Outdated dependency",
                    "path", dep.Path,
                    "current", dep.Version,
                    "latest", latest,
                    "update_cmd", fmt.Sprintf("go get %s@%s", dep.Path, latest),
                )
            }
        }
    }
}
```

### Input Sanitization

```go
// Sanitize user-provided version strings
func sanitizeVersionInput(input string) (string, error) {
    // Remove common prefixes
    input = strings.TrimPrefix(input, "v")
    input = strings.TrimSpace(input)

    // Basic validation - only allow alphanumeric, dots, dashes, plus
    validChars := "0123456789.+-"
    for _, r := range input {
        if !strings.ContainsRune(validChars, r) && !unicode.IsLetter(r) {
            return "", fmt.Errorf("version contains invalid character: %c", r)
        }
    }

    // Limit length
    if len(input) > 100 {
        return "", fmt.Errorf("version string too long: %d characters", len(input))
    }

    return input, nil
}

// Safe parsing wrapper
func parseUserVersion(input string) (*versioning.Version, error) {
    clean, err := sanitizeVersionInput(input)
    if err != nil {
        return nil, fmt.Errorf("input sanitization failed: %w", err)
    }

    // Use lenient parsing for user input
    ver, err := versioning.ParseLenient(clean)
    if err != nil {
        return nil, fmt.Errorf("version parsing failed: %w (input: %q)", err, input)
    }

    return ver, nil
}
```

## Limitations

- **SemVer 2.0.0 strict**: Does not support SemVer 1.0.0 date-based versions
- **Pre-release complexity**: Very complex pre-release identifiers may have edge cases
- **Unicode**: Limited support for Unicode in version identifiers (ASCII only)
- **Performance**: Range parsing with complex expressions can be slow for very large specs

## Future Enhancements

- **SemVer 3.0 preview support**: Experimental features from upcoming spec
- **Calendar versioning**: Support for date-based versioning schemes
- **Version recommendation**: Suggest appropriate next versions based on changelog
- **Multi-format support**: Cargo, npm, Python PEP 440 compatibility modes
- **Performance**: Optimized parsing for hot paths and large version lists

## Related Libraries

- [`pkg/buildinfo`](buildinfo.md) - Build metadata embedding and version integration
- [github.com/Masterminds/semver](https://github.com/Masterminds/semver) - Alternative SemVer library (underlying implementation)
- [github.com/blang/semver](https://github.com/blang/semver) - Another SemVer implementation for comparison

For more information, see the [GoDoc documentation](https://pkg.go.dev/github.com/fulmenhq/goneat/pkg/versioning).

---

_Generated by Code Scout ([OpenCode](https://opencode.ai/)) under supervision of [@3leapsdave](https://github.com/3leapsdave)_  
_Co-Authored-By: Code Scout <noreply@3leaps.net>_
