# Goneat v0.3.3 — Cryptographic Release Signing

**Release Date**: 2025-10-28
**Status**: Release

## TL;DR

- **Release Signing Infrastructure**: Mandatory PGP/GPG signing for all release artifacts
- **Supply Chain Security**: Cryptographic verification establishes artifact authenticity
- **Homebrew/Scoop Ready**: Signing infrastructure prerequisite for package manager distribution

## What's New

### Cryptographic Release Signing Infrastructure

Goneat v0.3.3 establishes the foundation for cryptographically signed releases, ensuring users can verify the authenticity and integrity of all release artifacts. This is a critical prerequisite for distribution through package managers like Homebrew and Scoop.

**Key Components**:

1. **FulmenHQ Release Signing Key**: Official PGP keypair with hardware-backed signing subkey
2. **Manual Signing Workflow**: Documented process for maintainers using YubiKey
3. **User Verification**: Complete instructions for verifying artifact signatures
4. **CI Prerequisites**: GitHub Actions workflow updated with GPG tooling

**Documentation**:
- **Security Guide**: `docs/security/release-signing.md` - Complete signing and verification guide
- **Release Checklist**: Updated with cryptographic signing steps
- **Key Management**: Documented custodianship, rotation, and revocation procedures

### For Users: Verifying Releases

Starting with v0.3.3, all release artifacts will be signed. Verify authenticity before installing:

```bash
# Download artifact and signature
curl -LO https://github.com/fulmenhq/goneat/releases/download/v0.3.3/goneat-darwin-arm64.tar.gz
curl -LO https://github.com/fulmenhq/goneat/releases/download/v0.3.3/goneat-darwin-arm64.tar.gz.asc

# Import FulmenHQ public key
curl -L https://github.com/fulmenhq/goneat/releases/download/v0.3.3/fulmenhq-release-signing-key.asc | gpg --import

# Verify signature
gpg --verify goneat-darwin-arm64.tar.gz.asc goneat-darwin-arm64.tar.gz
```

**Expected Output**:
```
gpg: Good signature from "FulmenHQ Release Signing <security@fulmenhq.dev>"
```

### For Maintainers: Signing Workflow

Release managers will sign artifacts using the manual workflow:

1. **Build and Package**: Standard release build
2. **Generate Checksums**: `sha256sum *.tar.gz *.zip > SHA256SUMS`
3. **Sign Artifacts**: Using YubiKey-backed subkey
4. **Verify Locally**: Test all signatures before upload
5. **Publish**: Upload binaries, signatures, and public key

See `docs/security/release-signing.md` for complete workflow.

### Infrastructure Updates

**GitHub Actions** (`.github/workflows/release.yml`):
- ✅ GPG tooling prerequisites installed (gnupg2)
- ✅ Workflow prepared for future signing automation
- ⏳ Actual signing deferred to post-v0.3.3 (manual workflow first)

**Security Documentation**:
- ✅ `docs/security/release-signing.md` - Comprehensive signing guide
- ✅ Key management and rotation procedures
- ✅ Verification instructions for users
- ✅ Troubleshooting and emergency procedures

## Roadmap: Signing Automation

**Phase 1 (v0.3.3)**: ✅ Manual signing + infrastructure
- Manual signing workflow with YubiKey
- CI tooling prerequisites installed
- Documentation complete

**Phase 2 (v0.3.4+)**: Automated CI signing
- Deploy CI signing subkey
- Secrets management (OIDC/Vault)
- Automated signature generation

**Phase 3 (v0.4.0+)**: Verification gates
- Automated signature verification in CI
- Pre-merge verification gates

**Phase 4 (v0.5.0+)**: Advanced provenance
- Sigstore integration
- SLSA provenance attestation

## Installation

```bash
# Go install (recommended)
go install github.com/fulmenhq/goneat@v0.3.3

# Verify installation
goneat version
```

## Upgrade Notes

No breaking changes. Simply upgrade to v0.3.3:

```bash
go install github.com/fulmenhq/goneat@v0.3.3
```

**Recommendation**: Verify signatures for all future downloads to ensure authenticity and protect against supply chain attacks.

## Security

For security concerns or to report key compromise:
- Email: security@fulmenhq.dev
- GitHub Security Advisories: https://github.com/fulmenhq/goneat/security

## Links

- **Repository**: https://github.com/fulmenhq/goneat
- **CHANGELOG**: See [CHANGELOG.md](CHANGELOG.md) for full details
- **Signing Documentation**: [docs/security/release-signing.md](docs/security/release-signing.md)
- **v0.3.2 Release**: See [docs/releases/v0.3.2.md](docs/releases/v0.3.2.md)

---

# Goneat v0.3.2 — Version Conflict Management

**Release Date**: 2025-10-28
**Status**: Release

## TL;DR

- **Version Conflict Detection**: New `goneat doctor versions` command to detect and manage multiple goneat installations
- **Automatic Conflict Resolution**: Purge stale global installations or update to latest with single command
- **Developer Experience**: Solves common version conflict issues when using multiple repositories

## What's New

### Version Conflict Detection and Management

Users working with multiple repositories may encounter version conflicts when goneat is installed both globally (`go install`) and locally (project bootstrap). This release introduces comprehensive version management capabilities to detect and resolve these conflicts.

**New Command**: `goneat doctor versions`

```bash
# Detect all goneat installations and identify conflicts
goneat doctor versions

# Remove stale global installation
goneat doctor versions --purge --yes

# Update global installation to latest
goneat doctor versions --update --yes

# JSON output for automation
goneat doctor versions --json
```

**Detection Coverage**:

- **Global installations**: GOPATH/bin (from `go install`)
- **Project-local**: ./bin/goneat (bootstrap pattern)
- **Development builds**: ./dist/goneat
- **PATH scanning**: All directories in system PATH

**What It Does**:

1. **Scans** your system for all goneat binaries
2. **Compares** versions and identifies the currently running binary
3. **Reports** version conflicts with clear visual indicators
4. **Recommends** solutions based on conflict type
5. **Resolves** conflicts automatically with `--purge` or `--update` flags

**Example Output**:

```
Goneat Version Analysis
=======================

Current running version: v0.3.2
Current binary path: /Users/you/project/dist/goneat

Detected installations:
   v0.2.11      | global | /Users/you/go/bin/goneat
▶️ v0.3.2       | development | /Users/you/project/dist/goneat

⚠️  Warning: 1 version conflict(s) detected

Recommendations:
1. Remove stale global installation:
   goneat doctor versions --purge --yes

2. Or update global installation to latest:
   goneat doctor versions --update --yes

3. Or use project-local installations (recommended):
   - Bootstrap to ./bin/goneat per project
   - See: goneat docs show user-guide/bootstrap
```

### Problem Solved

**Scenario**: Developer has multiple repositories on their machine:
- Repository A uses goneat v0.3.0 (bootstrapped to ./bin/goneat)
- Repository B just added goneat v0.3.2
- Developer previously ran `go install goneat@v0.2.11` (now in ~/go/bin)

**Before v0.3.2**:
- Commands might use the wrong version depending on PATH order
- Hooks might call stale global version
- Difficult to diagnose which version is running where
- Manual removal required understanding of GOPATH/bin location

**After v0.3.2**:
- Single command shows all installations and conflicts
- One-line fix to purge stale versions
- Clear recommendations for resolution strategy
- JSON output for CI/CD integration

### Use Cases

1. **Multi-Repository Development**: Detect when different repos use different versions
2. **Onboarding**: New team members can quickly identify and fix version mismatches
3. **CI/CD**: Validate no stale installations in build environments
4. **Troubleshooting**: Diagnose unexpected behavior due to version conflicts

### SSOT Dirty Detection Fix

Fixed a false positive bug in SSOT provenance dirty state detection that caused crucible repositories to incorrectly show as "dirty" when files matched only global gitignore patterns.

**The Bug**:
- go-git's `Status().IsClean()` includes ALL untracked files, even those matched by global gitignore (`~/.config/git/ignore`)
- This differs from git CLI behavior, which only checks repository `.gitignore`
- **Example**: `.claude/settings.local.json` in global gitignore but not repo `.gitignore` triggered false positive

**The Fix**:
- Now filters untracked files through repository `.gitignore` patterns only
- Repository `.gitignore` is the source of truth (matches CI/CD behavior)
- Includes `.git/info/exclude` for repository-local excludes
- Verified with 3-pass testing demonstrating correct behavior

**Impact**:
- ✅ **Team Consistency**: All developers see the same dirty state
- ✅ **CI/CD Alignment**: Local detection matches CI/CD behavior
- ✅ **Prepush Validation**: Correctly blocks only real uncommitted changes
- ✅ **Developer Experience**: Add common patterns (`.claude/`, `.vscode/`) to repo `.gitignore` for proper ignore behavior

**Design Decision**: See [ADR-0002](docs/architecture/decisions/adr-0002-ssot-dirty-detection.md) for detailed rationale on why repository `.gitignore` is the correct source of truth over global gitignore.

**Before Fix**:
```bash
$ cd crucible && git status
working tree clean  # Git CLI says clean

$ cd ../goneat && make sync-ssot
crucible: dirty (false positive from .claude/settings.local.json)
```

**After Fix**:
```bash
$ cd crucible && git status
working tree clean

$ cd ../goneat && make sync-ssot
crucible: clean ✅ (correctly ignores files in repo .gitignore)
```

## Installation

```bash
# Go install (recommended)
go install github.com/fulmenhq/goneat@v0.3.2

# From source
git clone https://github.com/fulmenhq/goneat.git
cd goneat
git checkout v0.3.2
make build
```

## Upgrade Notes

No configuration changes required. Upgrade to v0.3.2 and run `goneat doctor versions` to audit your installations:

```bash
# Upgrade
go install github.com/fulmenhq/goneat@v0.3.2

# Audit your installations
goneat doctor versions

# Clean up if conflicts detected
goneat doctor versions --purge --yes
```

**Recommended Practice**: After upgrading, run `goneat doctor versions` to ensure clean state across all your development environments.

---

# Goneat v0.3.1 — Build System Fix

**Release Date**: 2025-10-28
**Status**: Release

## TL;DR

- **Build System Fix**: Resolved chicken-and-egg dependency preventing fresh checkouts from building
- **Fast Follow**: Critical bug fix for v0.3.0 embed-assets workflow

## What's Fixed

### Build System Chicken-and-Egg Dependency

**Problem**: v0.3.0 introduced a circular dependency that prevented fresh repository checkouts from building:

1. `make build` requires `embed-assets` target to run first
2. `embed-assets.sh` script was trying to use `dist/goneat` binary
3. But `dist/goneat` doesn't exist until after build completes
4. Result: Fresh checkouts couldn't complete `make build` without manual intervention

**Solution**: Changed embed and verify scripts to use `go run .` instead of requiring the prebuilt binary:

- `scripts/embed-assets.sh`: Now uses `go run . content embed` instead of `dist/goneat content embed`
- `scripts/verify-embeds.sh`: Now uses `go run . content verify` instead of `dist/goneat content verify`
- Added explanatory notes in Makefile documenting the approach

**Impact**: Fresh checkouts can now run `make build` successfully without any manual steps.

## Installation

```bash
# Go install
go install github.com/fulmenhq/goneat@v0.3.1

# From source
git clone https://github.com/fulmenhq/goneat.git
cd goneat
git checkout v0.3.1
make build  # Now works on fresh checkouts!
```

## Upgrade Notes

No configuration changes required. Simply upgrade to v0.3.1 to get the build system fix:

```bash
go install github.com/fulmenhq/goneat@v0.3.1
```

---

# Goneat v0.3.0 — Dependency Protection

**Release Date**: 2025-10-28
**Status**: Release

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

## Documentation

### Comprehensive User Guides (1,700+ lines)

This release includes extensive documentation to help teams adopt dependency protection features:

**Core Guides**:

- **`docs/guides/dependency-protection-overview.md`** (397 lines)
  - Complete feature overview with quick start (5 minutes to production)
  - Real-world attack examples (ua-parser-js, event-stream, node-ipc)
  - Integration patterns decision tree with Mermaid diagrams
  - Clear network requirements and offline/online considerations
  - Cross-linked navigation to all related documentation

- **`docs/guides/package-cooling-policy.md`** (600 lines)
  - Detailed supply chain security threat model
  - Cooling timeline and validation flow diagrams (Mermaid)
  - Step-by-step setup guide with copy-paste commands
  - Complete policy configuration reference
  - Exception patterns with approval templates
  - Best practices and quarterly review guidelines

- **`docs/troubleshooting/dependencies.md`** (665 lines)
  - Comprehensive troubleshooting for all common issues
  - License compliance problems and diagnostic commands
  - Package cooling errors with step-by-step solutions
  - SBOM generation issues and fixes
  - Hook integration debugging
  - Performance optimization tips
  - Quick reference table of common fixes

**Dogfooding & Reference Implementation**:

- **`docs/appnotes/dogfooding-dependency-protection.md`** (410 lines)
  - How goneat uses its own dependency protection features
  - Real-world configuration with actual file paths and license counts
  - Operational patterns: daily development workflow, adding dependencies, pre-release validation
  - Current dependency health status (93 deps, 0 violations, 100% compliant)
  - Lessons learned: what works, what was rejected, common pitfalls
  - Implementation checklist for teams adopting the features

**Enhanced Configuration**:

- **`.goneat/dependencies.yaml`** (200+ lines of inline documentation)
  - Production-ready configuration used by goneat itself
  - Comprehensive field-by-field explanations
  - Exception pattern examples with approval attribution
  - Network requirements clearly called out
  - Quick troubleshooting section in footer
  - Strict allowlist approach: MIT, Apache-2.0, BSD, ISC, 0BSD, Unlicense
  - MPL-2.0 added to forbidden list (copyleft concerns documented)

**README Updates**:

- Prominent "NEW in v0.3.0" section highlighting dependency protection
- Supply chain security explained for non-technical readers
- Quick start with 3 simple steps
- Documentation navigation tree
- Commands section updated with dependencies highlighted

**Quality Features**:

- ✅ Beginner-friendly: Explains "what" and "why" before "how"
- ✅ Visual diagrams: 3 Mermaid diagrams for complex workflows
- ✅ Real examples: Actual attack cases with dates and impact
- ✅ Cross-linked: Every doc links to related documentation
- ✅ Actionable: Step-by-step guides with copy-paste commands
- ✅ Troubleshooting-first: Common issues prominently documented
- ✅ Offline access: All docs embedded via `goneat docs`

### Documentation Validation

All documentation has been validated through dogfooding:

- goneat's own `.goneat/dependencies.yaml` uses strict policies documented in guides
- All examples tested against goneat's 93 dependencies
- Troubleshooting scenarios derived from actual implementation issues
- Performance numbers from real goneat repository testing

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

- **🦅 Arch Eagle**: Enterprise architecture, security compliance, policy engine design, implementation planning
- **🔍 Code Scout**: Feature implementation, assessment integration, testing infrastructure, dogfooding implementation
- **🛠️ Forge Neat**: Documentation authorship (1,700+ lines), CI/CD hardening, quality gates, release preparation

**Supervised by**: @3leapsdave

**Documentation Contributions**:

Forge Neat authored the comprehensive documentation suite for v0.3.0:
- Dependency protection overview with quick start and decision trees
- Package cooling policy guide with threat model and Mermaid diagrams
- Complete troubleshooting guide covering all common scenarios
- Enhanced `.goneat/dependencies.yaml` with 200+ lines of inline docs
- README feature highlights and cross-linked navigation
- Validated through Code Scout's dogfooding appnote (goneat using its own features)

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
