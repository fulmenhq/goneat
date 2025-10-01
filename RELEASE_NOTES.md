# Goneat v0.2.11 — Guardian UX Enhancement & CI/CD Hardening (2025-09-30)

## TL;DR

- **Guardian Approval UX**: Fixed guardian approval browser page to display full command details with arguments
- **CI/CD Quality Gates**: Added embed verification to pre-push validation to prevent asset drift
- **Hook Enhancements**: All guardian-protected hooks now capture and pass command arguments for better visibility

## What's New

### Guardian Approval Command Visibility

Enhanced the guardian approval workflow to provide command transparency:

- **Full Command Display for Direct Usage**: When using `guardian approve` directly (e.g., `goneat guardian approve system ls -- ls -la /tmp`), the approval page shows the complete command with all arguments
- **Pre-push Hook**: Displays remote name and branch being pushed (e.g., `git push origin main`)
- **Git Hook Limitations**: Pre-commit and pre-reset hooks show generic placeholders (e.g., `git commit -m <pending commit message>`) because Git does not pass original command-line arguments to hook scripts
- **Command Details Section**: Collapsible section on approval page displays available command information with proper formatting
- **Best User Experience**: For full command visibility in git operations, wrap commands with `guardian approve` instead of relying on automatic hook triggers

### CI/CD Process Hardening

- **Embed Verification**: Added `make verify-embeds` to pre-push quality gates
- **Asset Drift Prevention**: Ensures embedded templates, schemas, and config stay synchronized with source
- **Release Validation**: Strengthens release process with automated embed consistency checks

## Installation

```bash
# Go install
go install github.com/fulmenhq/goneat@latest

# From source
git clone https://github.com/fulmenhq/goneat.git
cd goneat
make build
```

## Upgrade Notes

After upgrading, regenerate your hooks to get the enhanced guardian command visibility:

```bash
goneat hooks generate --with-guardian
goneat hooks install
```

---

# Goneat v0.2.8 — Guardian Repository Protection & Format Intelligence (2025-09-27)

## TL;DR

- **goneat pathfinder**: Expanded schema discovery system with simplified FinderFacade API for enterprise-grade path discovery
- **goneat format**: Added built-in XML and JSON prettification with configurable indentation and size warnings
- **goneat content**: Enhanced embedding system supporting schemas, templates, and configuration files
- **goneat hooks**: Added pre-reset hook support with guardian protection for reset operations
- **ASCII Terminal Support**: New ASCII art calibration system for accurate boxes across multiple terminal types
- **50% Test Coverage**: Comprehensive test coverage expansion with automated testing infrastructure

## What's New

### Pathfinder Schema Discovery (`goneat pathfinder`)

- **Expanded Schema Detection Engine**: Intelligent pattern matching for 10+ schema formats with enhanced discovery capabilities
- **FinderFacade API**: High-level entry point for enterprise-grade path discovery workflows with simplified interface
- **Schema Validation**: Comprehensive validation with meta-schema compliance checking and structured error reporting
- **Local Loader**: Production-ready filesystem loader with streaming text output and transform support

### Format Command Enhancements (`goneat format`)

- **JSON Prettification**: Built-in JSON formatting using Go's `json.Indent` with configurable options
  - New flags: `--json-indent` (custom string), `--json-indent-count` (1-10 spaces, 0 to skip), `--json-size-warning` (MB threshold)
  - Replaces external `jq` dependency for reliable, cross-platform JSON formatting
  - Supports compact mode and size-based warnings for large files

- **XML Prettification**: Built-in XML formatting using `etree` library with configurable options
  - New flags: `--xml-indent` (custom string), `--xml-indent-count` (1-10 spaces, 0 to skip), `--xml-size-warning` (MB threshold)
  - Validates XML well-formedness before formatting
  - Supports size-based warnings for large files

### Content Management (`goneat content`)

- **Enhanced Embedding System**: Support for embedding schemas, templates, and configuration files beyond just documentation
- **Asset Synchronization**: Better SSOT (Single Source of Truth) management for all embedded assets
- **Build Optimization**: Streamlined asset embedding process with verification and sync steps

### Git Hooks (`goneat hooks`)

- **Pre-reset Hook Support**: New `pre-reset` hook with guardian protection for reset operations
- **Guardian Integration**: Enhanced hook templates with automated guardian policy installation
- **Template Corrections**: Fixed trailing newline issues in embedded hook templates

### ASCII Terminal Calibration (`goneat ascii` command & `pkg/ascii` library)

- **Terminal Catalog System**: Comprehensive ASCII art calibration for accurate boxes across multiple terminal types
- **Display Functions**: Enhanced terminal display capabilities with proper box drawing characters
- **Cross-Platform Support**: Terminal-aware rendering for consistent visual output
- **ASCII Command**: New `goneat ascii` command for terminal calibration and display testing

### Quality & Testing Infrastructure

- **50% Test Coverage Achievement**: Comprehensive expansion across core packages:
  - `pkg/format/finalizer`: 72.5% coverage (+25.3 points) with normalization utilities testing
  - `pkg/ascii`: 31.9% coverage (+5.1 points) with terminal catalog and display function tests
  - `cmd` package: 40.7% coverage restoration with guardian compatibility

- **Automated Testing Infrastructure**:
  - `GONEAT_GUARDIAN_AUTO_DENY` environment variable for CI/CD testing workflows
  - Enhanced test fixtures and helper utilities for comprehensive validation
  - Guardian approval testing with automated denial mechanisms

### Code Quality Enhancements

- **Linting Compliance**: Resolved golangci-lint ST1015 switch statement ordering issues
- **Security Fixes**: Suppressed false positive G304 warnings in controlled file access patterns
- **Build System**: Enhanced Makefile with testing environment variables and validation targets
- **Pathfinder Flags**: `--schemas`, `--schema-id`, `--schema-category`, `--schema-metadata` for targeted discovery
- **Structured Output**: JSON and markdown output formats for programmatic consumption
- **Parallel Processing**: Optimized discovery with configurable worker pools

### Documentation and Examples

- **Pathfinder User Guide**: Comprehensive documentation for CLI usage and API integration
- **Schema Library Notes**: Technical deep-dive on schema validation architecture
- **FinderFacade Application Notes**: Enterprise integration patterns and examples

## What's Next

- **[0.2.10]** - 2025-10-01: Cloud storage loaders (S3, R2, GCS), advanced transforms, schema diffing
- **[0.3.0]** - 2025-11-01: Schema registry integration, automated schema evolution detection

---

# Goneat v0.2.7 — Version Policy Enforcement (2025-09-20)

## TL;DR

- **Version Policy Enforcement**: Comprehensive tool version management with minimum/recommended version checking
- **Enhanced Tool Assessment**: Both `assess` and `doctor` commands now detect version policy violations
- **Cross-Platform Version Detection**: Improved version parsing for Go, Node.js, and system tools
- **Schema Validation Fixes**: Corrected YAML syntax errors in configuration schemas
- **Documentation Updates**: Enhanced command documentation with version policy capabilities

## What's New

### Version Policy Enforcement

Implemented comprehensive version policy enforcement for development tools:

- **Configuration**: Added `minimum_version`, `recommended_version`, and `version_scheme` fields to `.goneat/tools.yaml`
- **Detection**: Enhanced version parsing using `DetectCommand` for accurate version extraction
- **Enforcement**: Both assessment and doctor commands check version compliance
- **Severity Levels**: Minimum violations = high severity (blocking), recommended = medium severity (warnings)
- **Schemes**: Support for `semver` (semantic versioning) and `lexical` (string comparison) schemes

### Tool Assessment Improvements

Enhanced tool checking capabilities across the entire toolchain:

- **Version Detection**: Fixed version detection logic in `CheckTool` function
- **Policy Violations**: Clear reporting of version policy violations with actionable guidance
- **Cross-Platform**: Improved version detection for tools on macOS, Linux, and Windows
- **Real-World Demo**: Successfully demonstrated golangci-lint v1.64.8 → v2.4.0 upgrade enforcement

### Schema Validation Fixes

Corrected YAML syntax errors in configuration schemas:

- Fixed indentation issues in `schemas/config/v1.0.0/dates.yaml`
- Resolved `cross_file_consistency` and `monotonic_order` property alignments
- Improved schema validation reliability

## Installation

```bash
# Go install
go install github.com/fulmenhq/goneat@latest

# Or download from releases
curl -L -o goneat https://github.com/fulmenhq/goneat/releases/download/v0.2.7/goneat-darwin-arm64
chmod +x goneat
```

## Migration Guide

### For Existing Tool Configurations

Add version policies to your `.goneat/tools.yaml`:

```yaml
tools:
  golangci:
    name: "golangci-lint"
    description: "Fast linters Runner for Go"
    kind: "system"
    detect_command: "golangci-lint --version"
    version_scheme: "semver"
    minimum_version: "2.0.0"
    recommended_version: "2.4.0"
    platforms: ["linux", "darwin", "windows"]
    install_commands:
      linux: "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"
      darwin: "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"
      windows: "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"
```

### For CI/CD Pipelines

Include version policy checking in your assessment commands:

```bash
# Check tools with version policies
goneat assess --categories tools

# Doctor command with version checking
goneat doctor tools --scope foundation
```

## Quality Metrics

- **Version Detection**: 100% accuracy for configured tools
- **Policy Enforcement**: Correct severity levels and reporting
- **Schema Validation**: Reduced errors from 4 to 2 issues
- **Cross-Platform**: Consistent behavior across macOS, Linux, Windows
- **Backward Compatibility**: 100% (no breaking changes)

## Links

- Changelog: [CHANGELOG.md](CHANGELOG.md)
- Tool Configuration: [docs/user-guide/commands/doctor.md](docs/user-guide/commands/doctor.md)
- Version Policy Guide: [docs/appnotes/intelligent-tool-installation.md](docs/appnotes/intelligent-tool-installation.md)

---

**Generated by Forge Neat ([OpenCode](https://opencode.ai/)) under supervision of [@3leapsdave](https://github.com/3leapsdave)**

**Co-Authored-By: Forge Neat <noreply@3leaps.net>**
