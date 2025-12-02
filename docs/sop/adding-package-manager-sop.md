---
title: "Adding a New Package Manager"
description: "Standard operating procedure for adding package manager support to goneat's doctor tools subsystem"
author: "Arch Eagle (@arch-eagle)"
date: "2025-12-02"
last_updated: "2025-12-02"
version: "v0.3.10"
component: "Doctor Tools / Package Manager Integration"
tags: ["package-manager", "doctor", "tool-installation", "development", "sop"]
---

# Adding a New Package Manager

## Overview

This SOP documents the required touchpoints when adding a new package manager to goneat's `doctor tools` subsystem. The package manager integration is distributed across multiple files, and missing any touchpoint causes subtle failures that are hard to diagnose.

**Why this document exists**: Between v0.3.7 and v0.3.10, we encountered repeated CI failures when adding brew support because the integration points weren't documented. Each failure revealed another missing piece:

1. **v0.3.7**: Detection worked but PATH wasn't updated → tools installed but not found
2. **v0.3.8**: PATH added during install but not when package manager was pre-existing
3. **v0.3.9**: PATH added for pre-existing but `GetShimPath()` didn't know about brew → post-install binary discovery failed
4. **v0.3.10**: Added brew to `GetShimPath()` → finally working

This brittleness is a known design issue targeted for refactoring in v0.4.x.

## Required Touchpoints Checklist

When adding a new package manager (e.g., `scoop`), you MUST update ALL of these locations:

### 1. Package Manager Interface Implementation

**File**: `pkg/tools/package_managers.go`

Create the manager struct implementing `PackageManager` interface:

```go
type ScoopManager struct{}

func (s *ScoopManager) Name() string { return "scoop" }
func (s *ScoopManager) IsAvailable() bool { /* detection logic */ }
func (s *ScoopManager) Version() (string, error) { /* version extraction */ }
func (s *ScoopManager) InstallationURL() string { return "https://scoop.sh" }
func (s *ScoopManager) SupportedPlatforms() []string { return []string{"windows"} }
func (s *ScoopManager) IsSupportedOnCurrentPlatform() bool { /* platform check */ }
```

Add a detection function if the package manager has multiple installation locations:

```go
func DetectScoop() (ScoopLocation, string, error) {
    // Check standard locations first (best performance)
    // Check user-local installations
    // Fall back to PATH lookup
}
```

### 2. Installer Implementation

**File**: `pkg/tools/installer_<name>.go` (new file)

Create the installer struct:

```go
type ScoopInstaller struct {
    config *PackageManagerInstall
    tool   *Tool
    dryRun bool
}

func NewScoopInstaller(tool *Tool, config *PackageManagerInstall, dryRun bool) *ScoopInstaller
func (s *ScoopInstaller) Install() (*InstallResult, error)
```

**CRITICAL**: Add a bin path helper function:

```go
// GetScoopBinPath returns the scoop shims directory path
func GetScoopBinPath() string {
    homeDir, err := os.UserHomeDir()
    if err != nil {
        return ""
    }
    return filepath.Join(homeDir, "scoop", "shims")
}
```

### 3. Auto-Install and PATH Setup

**File**: `cmd/doctor.go`

In `autoInstallPackageManagers()`, add logic for:

1. **Pre-existing detection with PATH extension**:

```go
// When package manager is already installed, ensure its bin is in PATH
if scoopInstalled {
    scoopBinPath := tools.GetScoopBinPath()
    if scoopBinPath != "" {
        addToCurrentPATH(scoopBinPath)
        logger.Info("Added scoop bin directory to PATH", logger.String("path", scoopBinPath))
    }
}
```

2. **Post-installation PATH extension**:

```go
// After installing the package manager, add its bin to PATH
if scoopInstalled {
    scoopBinPath := tools.GetScoopBinPath()
    if scoopBinPath != "" {
        addToCurrentPATH(scoopBinPath)
    }
}
```

> **⚠️ LOGGING REQUIREMENT**: All PATH extension messages MUST use `logger.Info()`, not `logger.Debug()`. PATH issues are the #1 cause of doctor failures in CI, and DEBUG-level logs are not visible by default. This requirement exists because we repeatedly failed to diagnose CI issues when PATH logs were at DEBUG level.

### 4. Post-Install Binary Discovery (GetShimPath)

**File**: `internal/doctor/path_manager.go`

Add a case in the `GetShimPath()` switch statement:

```go
case "scoop":
    // scoop installs to ~/scoop/shims
    return filepath.Join(homeDir, "scoop", "shims")
```

**Why this matters**: After a tool is installed via the package manager, goneat uses `GetShimPath()` to find where the binary was placed. If this case is missing, the tool appears to install successfully but goneat reports "binary not found in PATH".

### 5. Configuration Files

**File**: `config/foundation-package-managers.yaml` (if applicable)

Add the package manager definition:

```yaml
package_managers:
  - name: scoop
    platforms: [windows]
    detection_commands:
      - "scoop --version"
    bin_path: "~/scoop/shims"
    requires_path_update: true
```

### 6. Tests

**Files**:

- `pkg/tools/package_managers_test.go`
- `pkg/tools/installer_<name>_test.go` (new file)
- `cmd/doctor_test.go`

Add tests for:

- Detection logic
- Installer functionality
- PATH extension behavior
- Integration with doctor tools command

## Common Failure Patterns

### "Installed using X, but binary not found in PATH"

**Cause**: `GetShimPath()` doesn't have a case for this package manager.

**Fix**: Add the package manager to the switch statement in `internal/doctor/path_manager.go`.

### Tool installs successfully but subsequent checks fail

**Cause**: PATH wasn't extended after package manager installation.

**Fix**: Add `addToCurrentPATH()` call in `cmd/doctor.go` after installation.

### Package manager detected but tools won't install

**Cause**: PATH wasn't extended for pre-existing package manager.

**Fix**: Add PATH extension in the "already installed" branch of `autoInstallPackageManagers()`.

### False positive detection (reports installed when not)

**Cause**: Detection logic too permissive or checking wrong locations.

**Fix**: Use explicit path checks before falling back to PATH lookup.

## CI Debugging Recommendations

Since doctor operations run infrequently and failures are hard to diagnose:

1. **Use INFO-level logging in CI**: The additional verbosity is acceptable for operations that run once per workflow.

2. **Key events that should be INFO (not DEBUG)**:
   - Package manager detection results
   - PATH extensions applied
   - Binary discovery results after installation

3. **Recommended CI invocation**:
   ```yaml
   - name: Bootstrap tools
     run: |
       make build
       dist/goneat doctor tools --scope foundation --install --yes --log-level info
   ```

## Future Improvements (v0.4.x)

The current design has these known issues:

1. **Scattered touchpoints**: Package manager integration spans 4+ files
2. **No single source of truth**: Detection, bin paths, and PATH logic are duplicated
3. **Easy to miss a step**: This SOP exists because the integration is brittle

Planned improvements:

- Unified package manager registry with all metadata in one place
- Config-driven shim paths (read from `foundation-package-managers.yaml`)
- Automatic PATH discovery based on detection results

---

**Related Documents:**

- [Intelligent Tool Installation Strategy](../appnotes/intelligent-tool-installation.md) - High-level strategy
- [Tools Runner Usage Guide](../appnotes/tools-runner-usage.md) - User documentation
- [Bootstrap Patterns](../appnotes/bootstrap-patterns.md) - CI bootstrap patterns
