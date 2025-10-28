# Goneat v0.3.0 — Dependency Protection (Release Candidate)

## TL;DR

- **Dependency Protection System**: Comprehensive license compliance, package cooling policy, and SBOM generation
- **Supply Chain Security**: Configurable package age thresholds to prevent supply chain attacks
- **License Compliance**: Policy-driven license detection with OPA integration for Go dependencies
- **SBOM Generation**: CycloneDX 1.5 artifacts via managed Syft integration
- **Assessment Integration**: Dependencies as first-class category in `goneat assess` workflow
- **Version Propagation**: Automated VERSION sync across package managers

## What's New

### Dependency Protection System (`goneat dependencies`)

The flagship feature of v0.3.0 introduces comprehensive dependency protection capabilities:

```bash
# License compliance check
goneat dependencies --licenses

# Package cooling policy enforcement
goneat dependencies --cooling

# SBOM artifact generation
goneat dependencies --sbom

# Combined analysis
goneat dependencies --licenses --cooling --sbom --fail-on=high

# Assessment integration
goneat assess --categories dependencies
```

**Key Features**:

- Multi-language analyzer framework (Go production-ready, others extensible)
- OPA policy engine for policy-as-code evaluation
- Network-aware execution with registry API integration
- Git hook integration with pre-push recommendations

### License Compliance Engine

Policy-driven license detection and enforcement:

**Configuration** (`.goneat/dependencies.yaml`):

```yaml
version: v1

licenses:
  forbidden:
    - GPL-3.0
    - AGPL-3.0
```

**Capabilities**:

- Go dependency license detection (95%+ accuracy via go-licenses)
- Forbidden license blocking with clear violation reporting
- OPA integration for advanced policy evaluation
- YAML-to-Rego policy transpilation
- Multi-language analyzer interface for future expansion

### Package Cooling Policy

Mitigate supply chain attacks by enforcing minimum package age:

**Configuration**:

```yaml
cooling:
  enabled: true
  min_age_days: 7 # Minimum package age before adoption
  min_downloads: 100 # Minimum total downloads
  min_downloads_recent: 10 # Minimum recent downloads (30 days)
  alert_only: false # Fail build on violations
  grace_period_days: 3 # Grace period for new packages

  exceptions:
    - pattern: "github.com/myorg/*"
      reason: "Internal packages are pre-vetted"
```

**Registry Integration**:

- npm registry API client
- PyPI package metadata
- crates.io for Rust dependencies
- NuGet API v3 for .NET
- Go modules proxy
- 24-hour caching layer

**Threat Protection**:

- Blocks newly published packages (configurable threshold)
- Download count validation
- Exception management for trusted sources
- Grace period for gradual adoption

### SBOM Generation

Generate Software Bill of Materials for compliance:

```bash
# Generate SBOM artifact
goneat dependencies --sbom --sbom-format cyclonedx-json

# Specify output location
goneat dependencies --sbom --sbom-output sbom/app-1.0.0.cdx.json

# With assessment integration (metadata included)
goneat assess --categories dependencies
```

**Features**:

- CycloneDX 1.5 format via managed Syft
- Automatic tool installation with SHA256 verification
- Doctor integration: `goneat doctor tools --scope sbom --install`
- Dependency graph with transitive relationships
- NTIA minimum elements compliance

### Assessment Integration

Dependencies as a first-class assessment category:

```bash
# Run dependency assessment
goneat assess --categories dependencies

# Combined with other categories
goneat assess --categories format,lint,dependencies --fail-on high
```

**Integration Points**:

- CategoryDependencies registered in assessment engine
- Priority level 2 (high risk for supply chain)
- Network-aware execution planning
- Unified reporting with other categories
- Hook integration with pre-push recommendations

### Version Propagation System

Automated VERSION file propagation across package managers:

```bash
# Propagate version from VERSION to package.json, pyproject.toml, etc.
goneat version propagate

# Check what would be updated
goneat version propagate --dry-run
```

**Features**:

- Single source of truth (VERSION file)
- Cross-language package manager support
- Staging workspace for safe multi-file updates
- Pathfinder integration for pattern matching

### SSOT Provenance Metadata

Automatic audit trail generation for SSOT sync operations:

```bash
# Sync with automatic metadata capture
goneat ssot sync

# Metadata artifacts generated:
# - .goneat/ssot/provenance.json (aggregate)
# - .crucible/metadata/metadata.yaml (per-source mirror)
```

**Features**:

- Git introspection: commit SHA, dirty state detection
- Version detection from VERSION file
- Outputs mapping (asset type → destination path)
- CI enforcement support for clean sources
- Configurable mirrors and output paths

**Example Provenance**:

```json
{
  "schema": { "name": "goneat.ssot.provenance", "version": "v1" },
  "generated_at": "2025-10-27T18:00:00Z",
  "sources": [
    {
      "name": "crucible",
      "method": "local_path",
      "commit": "b64d22a0f0f94e4f1f128172c04fd166cf255056",
      "dirty": false,
      "version": "2025.10.2",
      "outputs": { "docs": "docs/crucible-go" }
    }
  ]
}
```

**CI Enforcement**:

```bash
# Check for dirty sources
jq '.sources[] | select(.dirty == true)' .goneat/ssot/provenance.json
```

### Registry Client Library (`pkg/registry/`)

Reusable package registry API clients:

**Supported Registries**:

- npm (registry.npmjs.org)
- PyPI (pypi.org JSON API)
- crates.io (crates.io API)
- NuGet (nuget.org API v3)
- Go modules (pkg.go.dev + proxy.golang.org)

**Features**:

- Mockable HTTP transport for testing
- Rate limiting and retry logic
- 24-hour TTL caching
- Configurable timeouts

### Security Hardening

Comprehensive security audit remediation:

**Critical Fixes**:

- Decompression bomb protection (500MB extraction limit)
- Path traversal prevention in archive extraction
- Command injection vulnerability fixes (G204 audit)
- Input sanitization for git references
- Managed tool resolver with artifact verification

**Security Validations**:

- Zero command injection vulnerabilities (gosec G204)
- Path cleaning in all file operations
- Archive extraction size limits
- Tool artifact SHA256 verification

## Configuration

### Dependencies Policy File (`.goneat/dependencies.yaml`)

Complete reference configuration:

```yaml
version: v1

# License Compliance Policy
licenses:
  forbidden:
    - GPL-3.0
    - AGPL-3.0
  # Optional: explicit allow list
  # allowed:
  #   - MIT
  #   - Apache-2.0
  #   - BSD-3-Clause

# Supply Chain Security (Cooling Policy)
cooling:
  enabled: true
  min_age_days: 7
  min_downloads: 100
  min_downloads_recent: 10
  alert_only: false
  grace_period_days: 3

  exceptions:
    - pattern: "github.com/myorg/*"
      reason: "Internal packages"

# Policy Engine Configuration
policy_engine:
  type: embedded # Use embedded OPA engine (recommended)
  # Optional remote OPA server
  # type: server
  # url: "http://opa-server:8181"

# SBOM Configuration
sbom:
  format: cyclonedx-json
  include_dev_dependencies: false
```

### Hook Integration (`.goneat/hooks.yaml`)

Network-aware hook configuration:

```yaml
hooks:
  pre-commit: # Fast, offline-capable
    - command: assess
      args: ["--categories", "format,lint"]

  pre-push: # Network-dependent checks
    - command: assess
      args: ["--categories", "dependencies", "--fail-on", "high"]
```

## Performance

### Optimizations

**Registry API Caching**:

- 24-hour TTL for package metadata
- Reduces network calls for repeated checks
- Configurable cache directory

**Analysis Speed**:

- < 5s for typical projects (100 dependencies)
- < 60s for large monorepos (1000+ dependencies)
- < 2s for cached/incremental analysis

## Quality Assurance

### Linting Infrastructure Enhancements

**Enhanced Test Suite Reliability**:

- Added `.goneatignore` pattern support to lint runner for automatic test fixture exclusion
- Improved lint assessment accuracy by respecting ignore patterns and preventing false positives
- Fixed unchecked error returns in test files across multiple packages (environment variables, file operations)
- Cleaned up dates test suite by removing skipped tests and implementing proper test fixtures
- Achieved 0 lint issues and 100% health score across codebase

### Three-Tier Integration Test Protocol

**Tier 1 - Synthetic Fixtures** (CI Mandatory):

- Time: < 10s
- Dependencies: None
- When: Every commit, pre-commit, pre-push
- Command: `make test` (includes Tier 1)

**Tier 2 - Quick Validation** (Pre-Release):

- Time: ~8s warm cache, ~38s cold
- Dependencies: Hugo repository
- When: Before tagging release
- Command: `make test-integration-cooling-quick`
- Setup: `export GONEAT_COOLING_TEST_ROOT=$HOME/dev/playground`

**Tier 3 - Full Suite** (Major Releases):

- Time: ~2 minutes
- Dependencies: Hugo, OPA, Traefik, Mattermost repos
- When: Major versions (v0.3.0, v1.0.0, etc.)
- Command: `make test-integration-cooling`
- Expected: 6/8 passing (2 known non-blocking failures)

## Documentation

### New Guides

**Dependency Protection**:

- `docs/user-guide/workflows/dependency-gating.md`: Complete workflow guide
- `docs/appnotes/license-policy-hooks.md`: Hook integration patterns
- `.goneat/dependencies.yaml`: Reference configuration

**SBOM Generation**:

- Wave 4 SBOM documentation with examples
- Try-it-yourself guides for CycloneDX generation
- Doctor tool integration guide

**Integration Testing**:

- `.plans/active/v0.3.0/wave-2-phase-4-INTEGRATION-TEST-PROTOCOL.md`

## Breaking Changes

None. All new features are additive and backward compatible.

## Upgrade Notes

After upgrading to v0.3.0:

1. **Configure dependency protection** (optional):

   ```bash
   # Copy reference configuration
   cp .goneat/dependencies.yaml.example .goneat/dependencies.yaml

   # Edit policy to match your requirements
   # Customize forbidden licenses and cooling thresholds
   ```

2. **Update hooks** to include dependency checks:

   ```bash
   # Edit .goneat/hooks.yaml to add dependencies category
   # Regenerate hooks
   goneat hooks generate --with-guardian
   goneat hooks install
   ```

3. **Test SBOM generation**:

   ```bash
   # Install Syft if needed
   goneat doctor tools --scope sbom --install

   # Generate SBOM
   goneat dependencies --sbom
   ```

4. **Try assessment integration**:

   ```bash
   # Run dependency assessment
   goneat assess --categories dependencies

   # Combined workflow
   goneat assess --categories format,lint,dependencies
   ```

## Known Limitations

### Multi-Language Analyzers

**v0.3.0 Scope**:

- ✅ Go: Full production implementation (95%+ accuracy)
- ✅ Framework: Extensible multi-language analyzer interface
- ⏭️ TypeScript/Python/Rust/C#: Stub implementations (future expansion)

**Rationale**:

- Go-first approach delivers immediate value
- Framework architecture proven and extensible
- Avoids shipping untested multi-language features
- Clear upgrade path for v0.3.1+ language support

## Installation

```bash
# Go install (after release)
go install github.com/fulmenhq/goneat@v0.3.0

# From source
git clone https://github.com/fulmenhq/goneat.git
cd goneat
git checkout v0.3.0
make build
```

## What's Next (v0.3.1+)

Planned enhancements for future releases:

**Multi-Language License Detection**:

- TypeScript/JavaScript analyzer (npm packages)
- Python analyzer (PyPI packages)
- Rust analyzer (crates.io)
- C# analyzer (NuGet packages)

**SBOM Enhancements**:

- SPDX format support
- Vulnerability enrichment (OSV database)
- VEX (Vulnerability Exploitability eXchange) support
- Provenance data inclusion

**Advanced Features**:

- Typosquatting detection
- Malicious package heuristics
- Dependency update suggestions
- License compatibility analysis

## Contributors

### AI Agent Attribution

This release was developed collaboratively by the 3leaps AI agent team under human supervision:

- **🦅 Arch Eagle**: Enterprise architecture, security compliance, policy engine design
- **🔍 Code Scout**: Feature implementation, assessment integration, testing infrastructure
- **🛠️ Forge Neat**: CI/CD hardening, quality gates, release preparation

**Supervised by**: @3leapsdave

### Human Oversight

All contributions reviewed, approved, and committed by:

- Dave Thompson (@3leapsdave) - Project Lead & Primary Maintainer

## Links

- **Repository**: https://github.com/fulmenhq/goneat
- **Documentation**: https://github.com/fulmenhq/goneat/tree/main/docs
- **Issues**: https://github.com/fulmenhq/goneat/issues
- **Crucible Standards**: https://github.com/fulmenhq/crucible

---

# Previous Releases

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

### Testing Infrastructure

- **50% Test Coverage Achievement**: Comprehensive test expansion across packages
  - `pkg/format/finalizer`: 72.5% coverage with normalization utilities
  - `pkg/ascii`: 31.9% coverage with terminal display tests
  - `cmd` package: 40.7% coverage with guardian compatibility

- **Automated Testing**:
  - `GONEAT_GUARDIAN_AUTO_DENY` environment variable for CI/CD
  - Enhanced test fixtures and helper utilities
  - Guardian approval testing with automated denial mechanisms

## Bug Fixes

- **Guardian Approve Bug**: Fixed `runGuardianApprove` to always execute wrapped commands after policy checks
- **Guardian Error Messages**: Enhanced denial error handling with clear messages and proper exit codes
- **Code Quality**: Resolved golangci-lint ST1015 switch statement issues
- **Security Suppressions**: Added proper `#nosec` comments for controlled file access patterns
- **Template Formatting**: Corrected trailing newline EOF enforcement in hook templates

## Installation

See v0.2.11 installation instructions above.
