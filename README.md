# goneat

[![Release](https://img.shields.io/github/v/release/fulmenhq/goneat?display_name=tag&sort=semver&logo=github)](https://github.com/fulmenhq/goneat/releases)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/3leaps/goneat)](go.mod)

All about smoothly delivering neat code at scale

We bring a smooth DX layer to the business of making neat code at scale. We wrap language-specific tool chains for formatting, linting, security scanning and other similar functions. Written in Go for speed and scale, we include in the package some of our additions as well, goneat enables you to solve common code and document quality problems across even large repositories.

## Quick Start (TL;DR)

1. **Install goneat**:

**Option A: Download binary** (recommended for most users)

- Visit [Releases](https://github.com/fulmenhq/goneat/releases) and download for your platform
- Extract and add to PATH, or:

```bash
# macOS/Linux example - adjust for your platform and version
curl -L -o goneat https://github.com/fulmenhq/goneat/releases/download/v0.2.7/goneat-darwin-arm64
chmod +x goneat
sudo mv goneat /usr/local/bin/
```

**Option B: Go install**

```bash
go install github.com/fulmenhq/goneat@latest
```

Verify: `goneat version`

2. **Get help and explore docs**:

```bash
# Built-in help system
goneat --help
goneat docs list                    # See available docs
goneat docs show user-guide/getting-started  # First recommended read
goneat docs show user-guide/commands/assess  # Deep dive on assessment
```

3. **Assess your repo**:

```bash
goneat assess                       # Full assessment
goneat assess --categories=format   # Just formatting issues
```

4. **Fix formatting** (auto-fixable):

```bash
goneat format                       # Fix all format issues
```

5. **Set up hooks** (optional, recommended for teams):

```bash
goneat hooks init
goneat hooks generate
goneat hooks install
```

Hooks now include maturity validation and dirty repository protection to prevent careless version pushes and ensure release readiness. See [Release quality checking](#release-quality-checking) for details.

**Notes:**

- Name clarification: This project is not affiliated with any other "goneat". Use the full module path `github.com/fulmenhq/goneat`.
- **Windows support is experimental until v0.3.0** - While goneat provides Windows binaries and basic functionality works, full compatibility testing and optimization is ongoing. Use with caution in production Windows environments.
- Upcoming: Homebrew and Scoop packages will be available soon for easier installation.

## Install

- Go (recommended):

```bash
go install github.com/fulmenhq/goneat@latest
```

- Releases (prebuilt binaries): https://github.com/fulmenhq/goneat/releases

- Homebrew (rc.8+):
  - After the tap is published:

  ```bash
  brew install fulmenhq/goneat/goneat
  ```

  - During RC bring-up (temporary), you can install directly from the raw formula for a specific tag:

  ```bash
  brew install --formula \
    https://raw.githubusercontent.com/fulmenhq/goneat/v0.2.3/packaging/homebrew/goneat.rb
  ```

- Scoop (rc.8+):
  - After the bucket is published:
  ```powershell
  scoop bucket add fulmenhq https://github.com/fulmenhq/scoop-bucket
  scoop install goneat
  ```

Verify install:

```bash
goneat version
```

## Developer Quick Start

**For contributors and those building from source:**

1. **Clone and build**:

```bash
git clone https://github.com/fulmenhq/goneat.git
cd goneat
make build          # Builds to dist/goneat
```

2. **Set up hooks** (recommended for development):

```bash
./dist/goneat hooks init
./dist/goneat hooks generate
./dist/goneat hooks install
```

3. **Development workflow**:

```bash
# Run tests
make test

# Run full assessment
./dist/goneat assess

# Format code
make fmt            # Uses goneat itself (dogfooding)

# Build for all platforms
make build-all
```

4. **Embedded docs development**:

```bash
# If you edit docs/ or manifest, sync embedded docs:
./dist/goneat content embed --manifest docs/embed-manifest.yaml --root docs --target internal/assets/embedded_docs/docs
make build
```

## Status

- Release: v0.2.6 (per `VERSION` file)
- Lifecycle Phase: GA (per `LIFECYCLE_PHASE` file)
- Release Phase: Release (per `RELEASE_PHASE` file)
- Repo Visibility: Public
- Gates: pre-commit (format+lint, fail-on=medium) passing; pre-push (format+lint+security+maturity+repo-status, fail-on=high) passing
- Licensing: Audit clean (no GPL/LGPL/AGPL/MPL/CDDL); inventory maintained under `docs/licenses/`

Note: This is alpha software in RC release phase. See `docs/standards/lifecycle-release-phase-standard.md` for phase definitions and operational details on coverage gates, contribution posture, and user guidance.

## Highlights

- **Multi-function text formatter**: handles Go code files, markdown, YAML, JSON with a general text mode for EOF and whitespace trimming at EOL
- **No‑hassle hooks**: one manifest, one command, instant DX
- **Zero‑friction tooling**: automatic tool detection and installation
- **JSON‑first SSOT**: one structured output for CI and humans (markdown/html derived)
- **Enterprise‑scale**: sharded parallelism, multi-module awareness, .goneatignore filtering
- **Extensible**: add languages, tools, and policies without changing your hook scripts
- **Diff‑Aware Assessment**: prioritizes and highlights issues in your current change set
- **Maturity Validation**: prevents version/phase mismatches and ensures release readiness ([see below](#release-quality-checking))
- **Dirty Repository Protection**: blocks pushes with unstaged changes to prevent careless releases ([see below](#release-quality-checking))
- **Smart Semantic Validation** (planned): detect and validate schemas beyond file extensions
- **Suppression Insights**: top rules/files with rich summaries for governance
- **Library Functions**: Reusable Go packages for schema validation and path resolution, enabling integration into custom tools without separate installation.

## Developer Libraries

Goneat provides reusable Go libraries for common DX patterns. See the [libraries guide](docs/user-guide/libraries.md) for details on available packages, integration patterns, and API documentation.

Key libraries include:

- **Configuration**: Hierarchical YAML/JSON loading with schema validation
- **Pathfinder**: Safe file discovery with gitignore support (experimental)
- **Schema**: Offline JSON/YAML schema validation
- **Safe I/O**: Secure file operations with traversal protection
- **Versioning**: Full SemVer 2.0.0 support with phase integration

**Single import covers everything**: If you've already `go install github.com/fulmenhq/goneat@latest` for the CLI tool, you don't need separate imports for libraries—they're included in the main module. Simply import the specific packages in your code:

```go
import (
    "github.com/fulmenhq/goneat/pkg/config"
    "github.com/fulmenhq/goneat/pkg/schema"
    // etc.
)
```

No duplicate `go install` commands needed—goneat's libraries are part of the main module and follow the same release cadence with backward compatibility guarantees.

For detailed documentation, see [docs/appnotes/lib/](docs/appnotes/lib/).

### Schema Management

Goneat's schema package provides fast, offline JSON Schema validation (Draft-07/2020-12) with embedded meta-schemas. Supports hierarchical configs and error reporting for enterprise-scale validation.

- Import: `github.com/fulmenhq/goneat/pkg/schema`
- Key Features: Validator rework for performance, schema discovery via patterns.
- Reminder: No separate `go install` needed—use as library in your Go projects via `go get github.com/fulmenhq/goneat`.

### Pathfinder _(Experimental until v0.3.0)_

Pathfinder handles file discovery and resolution with loaders for multi-module repos and hierarchical ignores (like .goneatignore). Optimizes large-repo scans with glob patterns and directory traversal.

- Import: `github.com/fulmenhq/goneat/pkg/pathfinder`
- Key Features: Loaders for configs/tools, absolute/relative path handling, integration with ignore files.
- ⚠️ **Experimental**: API may change before v0.3.0 stabilization.

### Maturity Validation _(Experimental until v0.3.0)_

The maturity package provides release lifecycle management and version consistency validation. Enables programmatic checking of repository phases and deployment readiness.

- Import: `github.com/fulmenhq/goneat/internal/maturity`
- Key Features: Phase file validation, version syntax checking, release readiness assessment.
- Usage: Integrate into CI/CD pipelines for automated release gate checks.
- ⚠️ **Experimental**: API may change before v0.3.0 stabilization.

### Assessment Runners

Extensible assessment framework with pluggable runners for different validation categories. Add custom checks by implementing the AssessmentRunner interface.

- Import: `github.com/fulmenhq/goneat/internal/assess`
- Key Features: Parallel execution, category-based assessment, JSON-first output for automation.
- Categories: format, lint, security, maturity, repo-status, and extensible for custom validations.
- Reminder: The library packages are part of the main module; no separate `go install` required—simply `go get github.com/fulmenhq/goneat` and import.

## No‑hassle hooks

Goneat manages Git hooks from a single manifest — not hand-edited scripts. Update `/.goneat/hooks.yaml`, then regenerate and install with one command. Optimized for speed: staged-only scope, result caching, and parallel execution.

```bash
goneat hooks init
goneat hooks generate
goneat hooks install
```

Sensible defaults:

- Pre-commit: format + lint (fail-on medium)
- Pre-push: format + lint + security + maturity + repo-status (fail-on high)
- Optimizations: only_changed_files, cache_results, parallel

See [Release quality checking](#release-quality-checking) for details on maturity validation and dirty repository protection.

Update flow:

```bash
# Edit .goneat/hooks.yaml or pull newer templates
goneat hooks generate && goneat hooks install
```

Tips:

- `GONEAT_HOOK_OUTPUT=concise|markdown|json|html` controls hook output
- Fail thresholds configurable via `--fail-on`; security concise shows `Fail-on: <level>`

## Release Quality Management

Goneat provides comprehensive release quality management through repository phase handling, maturity validation, and state checks. These features ensure your project progresses smoothly from development to production, integrating with git hooks and CI/CD for automated enforcement.

### Repository Phases

Manage project lifecycle phases (dev, rc, release, hotfix) and release phases (alpha, beta, ga, maintenance) to enforce appropriate standards at each stage.

**Commands:**

- `goneat repository phase set --release rc --lifecycle beta` - Transition to release candidate.
- `goneat repository phase show` - Display current phases and rules.
- `goneat repository policy show` - View phase-specific policies (e.g., min coverage, git cleanliness).
- `goneat repository policy validate --level error` - Validate against current state.

**Policies Example:**

- Dev: 50% coverage, dirty git allowed, "-dev" suffix.
- RC: 75% coverage, clean git required, "-rc.1" suffix, docs mandatory.
- Release: 90% coverage, no suffixes, full validation.

Configure in `.goneat/phases.yaml`.

### Maturity Validation

Validate repository health based on phases, checking git state, versions, docs, coverage, and schemas.

**Commands:**

- `goneat maturity validate --level warn` - Comprehensive check with warnings.
- `goneat maturity release-check --phase rc --strict` - Phase-specific readiness (fails on issues).
- Integrate via `goneat assess --categories maturity`.

**Checks Include:**

- Git cleanliness and branch state.
- Version suffix matching phase (e.g., no "-rc" in release).
- Required docs (CHANGELOG.md, RELEASE_NOTES.md).
- Coverage thresholds with exceptions (e.g., node_modules=0%).
- Schema validity for configs.

**JSON Output for CI:**

```json
{
  "ready": true,
  "issues": [],
  "phase": "rc"
}
```

### Dirty Repository Protection

Blocks pushes with uncommitted changes to prevent incomplete releases.

- Runs `git status --porcelain` in hooks.
- Fails pre-push if dirty (configurable per phase).
- Clear fixes: "git add . && git commit".

### Workflow Integration

Follow phases in your release process:

1. **Dev**: `goneat repository phase set --release dev`; lenient checks.
2. **RC**: Set to rc/beta; run `goneat maturity release-check --phase rc --strict`.
3. **Release**: Set to release/ga; full `goneat assess --categories all`.
4. **Hotfix**: 80% coverage, focused security checks.

**Hooks Setup:**

- Pre-commit: `goneat maturity validate --level warn`.
- Pre-push: Full `release-check --strict` + assess.

**CI Example (GitHub Actions):**

```yaml
- run: goneat maturity release-check --phase rc --strict --json | jq '.ready'
- run: if [ "$(goneat assess --categories maturity --json | jq '.issues | length')" -gt 0 ]; then exit 1; fi
```

**Benefits:**

- Enforces standards per phase for enterprise-scale releases.
- Prevents version drifts, dirty pushes, and doc gaps.
- JSON-first for agentic/CI integration.
- Customizable via `.goneat/phases.yaml` for multi-language repos.

For full workflows, see [Release Readiness Guide](docs/user-guide/workflows/release-readiness.md).

## Zero‑friction tooling

Never deal with "tool not found" errors again. Goneat's built-in doctor automatically detects and installs required external tools — no manual setup, no environment configuration hassles.

```bash
# Check what's missing
goneat doctor tools --scope security

# Install everything automatically
goneat doctor tools --scope all --install --yes

# Get installation instructions
goneat doctor tools --scope format --print-instructions
```

Supported tools:

- **Security**: gosec, govulncheck, gitleaks
- **Format**: goimports, gofmt (bundled with Go)
- **Future**: Multi-language formatters and linters

Benefits:

- **Zero setup time**: New team members can start contributing immediately
- **Consistent environments**: Same tool versions across all machines
- **Automatic updates**: Stay current with latest security tools
- **Non-intrusive**: Only installs what's needed, with clear prompts

## Large‑repo performance

- Sharded execution (e.g., gosec across Go packages; multi-module via `go list`)
- Concurrency tuned via CPU percentage or explicit worker count
- Staged/diff scoping to minimize work on developer flows

## Commands

### Neat Commands (Core Functionality)

- `goneat assess`: Orchestrated assessment engine (format, lint, security, static analysis, schema, date-validation, maturity, repo-status) with user-configurable assessment categories ([docs](docs/user-guide/commands/assess.md))
- `goneat dates`: Validate and fix date consistency across your codebase ([docs](docs/user-guide/commands/dates.md))
- `goneat format`: Multi-format formatting with finalizer stage (EOF/trailing spaces, line-endings, BOM) ([docs](docs/user-guide/commands/format.md))
- `goneat security`: Security scanning (gosec, govulncheck), sharded + parallel ([docs](docs/user-guide/commands/security.md))
- `goneat validate`: Schema-aware validation (preview; offline meta-validation) ([docs](docs/user-guide/commands/validate.md))

### Workflow Commands (Repository Management)

- `goneat hooks`: Hook management (init, generate, install, validate, inspect) ([docs](docs/user-guide/commands/hooks.md))
- `goneat maturity`: Repository maturity validation and release readiness checks ([docs](docs/user-guide/commands/maturity.md))
- `goneat repository`: Repository phase and policy management ([docs](docs/user-guide/commands/repository.md))

### Content Commands (Documentation)

- `goneat content`: Curate and embed documentation content ([docs](docs/user-guide/commands/content.md))
- `goneat docs`: Read-only access to embedded user guides (most user-facing help) ([docs](docs/user-guide/commands/docs.md))

### Support Commands (Utilities)

- `goneat doctor`: Diagnostics and tooling checks ([docs](docs/user-guide/commands/doctor.md))
- `goneat envinfo`: Display environment and system information
- `goneat home`: Manage user configuration and preferences
- `goneat info`: Display informational content and metadata
- `goneat version`: Show goneat version information ([docs](docs/user-guide/commands/version.md))

Development note: The embed step runs during `make build` and `build-all` via `embed-assets`. Docs mirroring uses the CLI when a local binary exists; otherwise the tracked mirror is used. If you edit `docs/` or the manifest, run:

```bash
dist/goneat content embed --manifest docs/embed-manifest.yaml --root docs --target internal/assets/embedded_docs/docs
make build
```

### Doctor Command

Goneat includes a built-in doctor for automatic tool management. See the "Zero-friction tooling" section above for usage examples, or check `docs/user-guide/commands/doctor.md` for complete documentation.

## User Configuration

Goneat supports user configuration through `.goneat/` directory in your project root. Each assessment category can be customized with YAML or JSON configuration files:

- **Date Validation**: `.goneat/dates.yaml` - Configure file patterns, date formats, and validation rules
- **Format**: `.goneat/format.yaml` - Customize formatting rules and file types
- **Security**: `.goneat/security.yaml` - Configure security scanning rules and exclusions

Goneat uses three distinct **[config file resolution patterns](docs/configuration/config-file-resolution-patterns.md)** to ensure consistent, predictable behavior:

1. **User-extensible-from-default** (goneat configs) - Project overrides user overrides defaults
2. **Repo root only** (tool configs like `.golangci.yml`) - Working directory resolution
3. **Hierarchical ignore files** (like `.goneatignore`) - Directory traversal with precedence

All configuration files use JSON Schema validation with fast-fail error handling. Invalid configurations fall back to sensible defaults with warning messages.

## JSON‑first SSOT

All commands produce structured JSON with rich metadata for programmatic processing. Perfect for CI/CD integration, automated workflows, and agentic processing systems.

```json
{
  "metadata": {
    "tool": "goneat",
    "version": "1.0.0",
    "execution_time": 48660125,
    "commands_run": ["format", "lint"]
  },
  "summary": {
    "total_issues": 63,
    "overall_health": 0.37,
    "parallel_groups": 13
  },
  "categories": {
    "format": {
      "issues": [
        {
          "file": "cmd/doctor.go",
          "auto_fixable": true,
          "estimated_time": 30000000000
        }
      ]
    }
  }
}
```

**Features:**

- Rich metadata for routing and prioritization
- Auto-fixable issue detection
- Parallel processing optimization
- CI/CD pipeline integration
- Agentic backend compatibility

### Date Semantics (optional)

- Semantic date validation for key files (e.g., CHANGELOG): future-date detection, stale entries, and optional descending-order (monotonic) enforcement.
- Disabled by default for compatibility. Enable and customize in `.goneat/dates.yaml`:

```yaml
# .goneat/dates.yaml
enabled: true
date_patterns:
  - regex: "(\\d{4})-(\\d{2})-(\\d{2})"
    order: "YMD"
rules:
  future_dates:
    enabled: true
    max_skew: "24h" # also supports 5d, 30d
    severity: "error"
  monotonic_order:
    enabled: true
    files:
      - "CHANGELOG.md"
      - "docs/releases/**"
    severity: "warning"
```

See `docs/configuration/date-validation-config.md` for full configuration details.

## Offline Assets

Goneat embeds critical validation assets to ensure deterministic, offline runs:

- JSON Schema meta-schemas: Draft-07, 2020-12
- See `docs/architecture/assets-management.md` and `docs/standards/assets-standard.md`

To refresh curated assets (optional):

```bash
make sync-schemas
```

Project configuration (preview): see `docs/configuration/schema-config.md` for configuring discovery patterns and auto-detect.

## Roadmap (v0.1.x)

- Deeper finalizer capabilities and shared sharding utilities
- Secrets scanning (gitleaks) and multi-ecosystem dependency scanners (osv-scanner)
- Concurrency manager and telemetry for cross-category budgeting

## Built-in Docs (Offline)

No repo? No problem. Goneat embeds a curated set of documentation for offline use:

```bash
# Discover available topics
goneat docs list --format json | jq '.[].slug'

# Read a command guide (stream to pager)
goneat docs show user-guide/commands/format --format markdown | less

# Quick alias for command help
goneat docs help format | less

# Render to HTML (raw markdown wrapped in HTML)
goneat docs show user-guide/commands/hooks --format html > hooks.html
```

Tip: Use `goneat docs` to learn about hooks, commands, tutorialsdocs/user-guide/workflows/release-readiness.md, and workflows without leaving your terminal.

## Diff‑Aware Assessment (Change‑Set Intelligence)

For large repositories, signal‑to‑noise matters. Goneat captures git change‑set context and:

- Embeds `change_context` in assessment metadata (modified files, total changes, scope, branch/SHA)
- Marks issues as `change_related` with optional `lines_modified`
- Enables smarter CI: fail on high‑severity only when touched by the current diff

This helps reviewers and bots focus on what changed, speeding feedback and reducing churn.

## Lifecycle Status

This project follows the Fulmen Ecosystem Lifecycle Maturity Model. Current phase: see `LIFECYCLE_PHASE` and `docs/status/lifecycle.md` for what this means operationally (coverage gates, contribution posture, and user guidance).

## Support & Community

- GitHub Repository: https://github.com/fulmenhq/goneat
- Issues & Feature Requests: https://github.com/fulmenhq/goneat/issues
- Releases: https://github.com/fulmenhq/goneat/releases
- Documentation: see docs/ directory in this repo
- Enterprise Support: contact 3 Leaps — support@3leaps.net

---

## 📜 **License & Legal**

**Open Source**: Apache-2.0 License - see [LICENSE](LICENSE) for details.

**Trademarks**: "Fulmen™", "goneat", and "3 Leaps®" are trademarks of 3 Leaps, LLC. While code is open source, please use distinct names for derivative works to prevent confusion.

### Name Clarification

This project (github.com/fulmenhq/goneat) is not affiliated with any other projects named "goneat". Use the full module path `github.com/fulmenhq/goneat` for `go install` and imports.

### OSS Policies (Organization-wide)

- Authoritative policies repository: https://github.com/3leaps/oss-policies/
- Code of Conduct: https://github.com/3leaps/oss-policies/blob/main/CODE_OF_CONDUCT.md
- Security Policy: https://github.com/3leaps/oss-policies/blob/main/SECURITY.md
- Contributing Guide: https://github.com/3leaps/oss-policies/blob/main/CONTRIBUTING.md
- Third-party notices are generated per release (see `docs/licenses/` for current inventory).

### Enterprise Support

For enterprise support, custom integrations, or commercial licensing inquiries, contact: support@3leaps.net

---

<div align="center">

⚡ **All about smoothly delivering neat code at scale** ⚡

_Multi-function formatting, linting, and assessment for enterprise development_

<br><br>

**Built with 🛠️ by the 3 Leaps team**
**Part of the [Fulmen Ecosystem](https://fulmenhq.dev) - Lightning-fast enterprise development**

</div>
