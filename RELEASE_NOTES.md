# Goneat v0.2.8 — Guardian Repository Protection & Format Intelligence (2025-09-27)

## TL;DR

- Guardian command suite (`check`, `approve`, `setup`) protects high-risk git operations with policy-driven approvals
- Local browser approval server delivers branded, expiring approval flows with sudo-style execution
- Pre-commit/pre-push hooks now guide operators to wrap blocked commands with `guardian approve`
- Hooks system now auto-detects and configures format capabilities - no more manual hooks.yaml editing
- Complete ASCII art toolkit handles Unicode width issues across different terminal emulators
- New guardian command documentation covers CLI usage, hooks remediation, and approval UX

## What's New

### Guardian Command Suite
- Introduced `goneat guardian check`, `approve`, and `setup` commands for repository-scope approval enforcement
- `guardian approve` now requires the protected git operation after `--`, ensuring the action runs atomically once approval completes
- Rich terminal output highlights approval scope, project, reason, and expiry timing to aid maintainers

### Browser Approval Experience
- Added a local guardian approval server with cryptographic nonces, auto-expiring sessions, and localhost binding
- Terminal instructions respect branding settings and project context; the approval page now features project name as an H1 with optional custom messaging
- Sessions honor the shorter of policy `expires` and `browser_approval.timeout_seconds`, returning a clear expiration error when unattended

### Guardian Hook Integration
- Updated generated hooks for Bash, PowerShell, and CMD to surface guardian context and instruct operators to re-run blocked commands via `guardian approve`
- Removed guidance for not-yet-implemented grant workflows to keep remediation actionable today
- Hooks auto-bootstrap guardian configuration when integration is detected, keeping security posture consistent across platforms

### Intelligent Hooks Format Detection
- `goneat hooks init` now automatically detects format capabilities in your project
- Auto-configures hooks.yaml with format commands (priority 5) before assess commands (priority 10)
- Supports detection of `make format-all`, `make format`, `make fmt`, npm format scripts, prettier, and Python formatters
- Eliminates manual hooks.yaml editing while maintaining full user control over generated configuration
- Projects with format capabilities get comprehensive formatting workflow automatically

### ASCII Art Terminal Calibration System
- New `ascii` command suite: `box`, `stringinfo`, `calibrate`, `mark`, `analyze`, and `diag`
- Terminal-specific width override system handles emoji variation selector rendering differences
- Automated analysis detects alignment issues in ASCII art and generates correction commands
- Support for Ghostty, iTerm2, Apple Terminal with extensible configuration system
- Comprehensive test fixtures for emoji, symbols, and box drawing characters
- User configuration via `$GONEAT_HOME/config/terminal-overrides.yaml` with embedded defaults

### Enhanced Format Coverage
- Added `format-config` and `format-all` Makefile targets for comprehensive project formatting
- Configuration and schema files (config/, schemas/) now included in format workflow
- Integrated format targets into git hooks workflow with proper priority ordering

### Documentation
- Added `docs/user-guide/commands/guardian.md` covering guardian CLI usage, hook workflows, and troubleshooting tips
- Expanded `docs/user-guide/commands/hooks.md` to describe the new guardian remediation pattern
- Added comprehensive ASCII command documentation in `docs/user-guide/ascii.md`
- Updated format workflow documentation with new Makefile targets
- Added terminal calibration guides and troubleshooting tips

## What's Next
- The guardian DevOps and SQL extensions (`guardian-devops-foundation`, `guardian-sql-proxy`) move to the v0.2.9 planning cycle while we finalise repository coverage in v0.2.8.

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
