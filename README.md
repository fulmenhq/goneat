# goneat

[![Release](https://img.shields.io/github/v/release/fulmenhq/goneat?display_name=tag&sort=semver&logo=github)](https://github.com/fulmenhq/goneat/releases)
[![CI](https://github.com/fulmenhq/goneat/actions/workflows/ci.yml/badge.svg)](https://github.com/fulmenhq/goneat/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/fulmenhq/goneat)](go.mod)
[![Lifecycle](https://img.shields.io/badge/lifecycle-alpha-orange)](docs/status/lifecycle.md)

All about smoothly delivering neat code at scale.

We bring a smooth DX layer to the business of making neat code at scale. We wrap language-specific tool chains for formatting, linting, security scanning, and similar functions. Written in Go for speed and scale, goneat enables you to solve common code and document quality problems across even large repositories.

## Language Support

Goneat provides **language-aware assessment** with automatic tool detection. When supported tools are installed, goneat seamlessly integrates them into lint and format workflows.

| Language | Lint | Format | Tool | Install |
|----------|------|--------|------|---------|
| **Go** | ✅ | ✅ | golangci-lint, gofmt | `brew install golangci-lint` |
| **Python** | ✅ | ✅ | [ruff](https://docs.astral.sh/ruff/) | `brew install ruff` |
| **JavaScript/TypeScript** | ✅ | ✅ | [biome](https://biomejs.dev/) | `brew install biome` |
| **YAML** | ✅ | ✅ | yamllint, yamlfmt | `brew install yamllint yamlfmt` |
| **Markdown** | — | ✅ | prettier | `npm install -g prettier` |
| **JSON** | — | ✅ | prettier | `npm install -g prettier` |
| **Shell** | ✅ | — | shellcheck | `brew install shellcheck` |
| _Rust_ | 🔜 | 🔜 | _planned_ | — |
| _C#_ | 🔜 | 🔜 | _planned_ | — |

**Tool-present gating**: Goneat gracefully skips tools that aren't installed—no errors, just informational logs. Install only what you need.

```bash
# Check tool availability
goneat doctor tools --scope foundation

# Run language-aware assessment
goneat assess --categories lint,format
```

See [docs/user-guide/commands/assess.md](docs/user-guide/commands/assess.md) for detailed configuration options.

## Highlights

- **Schema validation at scale**: `goneat validate suite` (bulk), `schema_path` mappings, and offline `$ref` resolution via `--ref-dir`
- **Supply-chain protection**: package cooling, license compliance, and SBOM generation (`goneat dependencies`)
- **Multi-format formatting**: Go/Markdown/YAML/JSON + finalizer (EOF/trailing whitespace)
- **Hook automation**: intelligent hooks generation + optional Guardian approvals for protected operations
- **Offline docs**: `goneat docs show release-notes` and `goneat docs show releases/latest`

## Quick Start (TL;DR)

1. **Install goneat** (pick one):
   - **Homebrew (recommended)**: `brew install fulmenhq/tap/goneat`
   - **Go install**: `go install github.com/fulmenhq/goneat@latest`
   - **Secure direct download (recommended if not using a package manager)**: `sfetch --repo fulmenhq/goneat --latest --dest-dir ~/.local/bin`
   - **Release archives**: download from [GitHub Releases](https://github.com/fulmenhq/goneat/releases) and place the binary on your `PATH`
   - Verify with `goneat version`
   - Latest release notes: `goneat docs show releases/latest`

2. **Initialize tooling config** (required in v0.3.7+):

   ```bash
   goneat doctor tools init           # local development defaults
   goneat doctor tools init --minimal # CI-safe, language-native tools only
   git add .goneat/tools.yaml && git commit -m "chore: configure goneat tools"
   ```

   This replaces all hidden defaults—goneat commands read the committed `.goneat/tools.yaml` you generate here (no manual authoring needed).

3. **Get help and explore docs**:

   ```bash
   goneat --help
   goneat docs list
   goneat docs show user-guide/install
   goneat docs show user-guide/commands/validate
   goneat docs show release-notes
   goneat docs show releases/latest
   ```

4. **Assess your repo**:

   ```bash
   goneat assess                       # Full assessment
   goneat assess --categories=format   # Just formatting issues
   ```

5. **Fix formatting** (auto-fixable):

   ```bash
   goneat format
   ```

6. **Set up hooks** (optional, recommended for teams):

   ```bash
   goneat hooks init
   goneat hooks generate --with-guardian
   goneat hooks install
   ```

Hooks automatically detect and configure format capabilities (make format-all, prettier, etc.) and include maturity validation, dirty repository protection, and optional guardian security approval workflows. See [Release Quality Management](#release-quality-management) and [Guardian Security](#guardian-security) for details.

**Notes:**

- Name clarification: This project is not affiliated with any other "goneat". Use the full module path `github.com/fulmenhq/goneat`.
- **Windows support is operational**: goneat runs on Windows with foundation tools via Scoop. Full build/dev/deployment workflows are still in progress.
- Homebrew and Scoop packages ship with each tagged release (`brew install fulmenhq/tap/goneat`, `scoop install goneat`).

## Install

### Go (cross-platform)

```bash
go install github.com/fulmenhq/goneat@latest
```

### Homebrew (macOS/Linux)

```bash
brew install fulmenhq/tap/goneat
# upgrade later
brew upgrade goneat
```

### Scoop (Windows)

```powershell
scoop bucket add fulmenhq https://github.com/fulmenhq/scoop-bucket
scoop install goneat
```

### Release archives

Download artifacts from [GitHub Releases](https://github.com/fulmenhq/goneat/releases), extract, and place `goneat` on your `PATH`.

For a high-confidence direct download (automatic signature + checksum verification), use `sfetch`:

```bash
# Install sfetch
curl -sSfL https://github.com/3leaps/sfetch/releases/latest/download/install-sfetch.sh | bash

# Install goneat
sfetch --repo fulmenhq/goneat --latest --dest-dir ~/.local/bin
```

See `docs/user-guide/bootstrap/sfetch.md`.

### After installing (required)

```bash
goneat doctor tools init           # local defaults
goneat doctor tools init --minimal # CI-safe

git add .goneat/tools.yaml && git commit -m "chore: configure goneat tools"
```

All doctor/assess commands read the committed `.goneat/tools.yaml`, so running the init command once per repository replaces every hidden default.

## Bootstrap as a Local Tool

For projects that want to manage goneat as a **repository-local tool** (keeping the binary in `./bin/goneat` within your project), you can use the bootstrap pattern with `.goneat/tools.yaml`:

**Quick Summary**:

1. Run `goneat doctor tools init --scope foundation` (or `--minimal`) to scaffold `.goneat/tools.yaml`
2. Add/modify the generated manifest to include a `goneat` entry that installs into `./bin`
3. Run `bun run scripts/bootstrap-tools.ts` (or equivalent) to download/install
4. Use `./bin/goneat` for project-specific tooling
5. Override with `.goneat/tools.local.yaml` for local development

**Full Guide**: See [Bootstrap Goneat Guide](docs/crucible-go/guides/bootstrap-goneat.md) for:

- Complete `.goneat/tools.yaml` manifest format
- Checksum verification setup
- Local override pattern with `tools.local.yaml`
- Integration with Makefiles and CI/CD
- Post-install validation checklist

This pattern is especially useful for:

- **Monorepos** where different projects use different goneat versions
- **CI/CD pipelines** that need reproducible, pinned tooling
- **Teams** that want to version-control their exact tool versions
- **Offline environments** where package managers aren't available

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

- Release: see `VERSION`
- Lifecycle Phase: Alpha (see `LIFECYCLE_PHASE`)
- Release Phase: Release (see `RELEASE_PHASE`)
- Repo Visibility: Public
- Gates: pre-commit (format+lint, fail-on=medium) passing; pre-push (format+lint+security+maturity+repo-status, fail-on=high) passing
- Licensing: Audit clean (no GPL/LGPL/AGPL/MPL/CDDL); inventory maintained under `docs/licenses/`

This is active alpha software even while release artifacts ship. See `docs/standards/lifecycle-release-phase-standard.md` for definitions, gates, and guidance before adopting in production.

## 🛡️ **NEW in v0.3.0: Dependency Protection** 🛡️

Protect your software supply chain with comprehensive dependency security and compliance features:

### Supply Chain Security (Package Cooling)

Automatically block newly published packages to prevent supply chain attacks. Enforces a configurable waiting period (default: 7 days) before adopting new dependencies.

```bash
# Enable cooling policy
goneat dependencies --cooling --fail-on high
```

**Why it matters:** 80% of supply chain attacks are detected within 7 days. Recent attacks like ua-parser-js (8M+ weekly downloads) and event-stream show why waiting matters.

### License Compliance

Automatically detect and enforce license policies across your entire dependency tree:

```bash
# Enforce license policy
goneat dependencies --licenses --fail-on high
```

Configure forbidden licenses (GPL, AGPL) and trusted sources in `.goneat/dependencies.yaml`.

### SBOM Generation

Generate Software Bill of Materials for regulatory compliance and security auditing:

```bash
# Generate SBOM in CycloneDX format
goneat dependencies --sbom
```

**Quick Start:**

1. Create `.goneat/dependencies.yaml` (see template in repo)
2. Run: `goneat dependencies --licenses --cooling`
3. Add to hooks: `goneat hooks install`

**Documentation:**

- **[Dependency Protection Overview](docs/guides/dependency-protection-overview.md)** - Complete feature guide
- **[Package Cooling Policy](docs/guides/package-cooling-policy.md)** - Supply chain security explained
- **[SBOM Workflow](docs/guides/sbom-workflow.md)** - SBOM lifecycle and best practices
- **[Troubleshooting](docs/troubleshooting/dependencies.md)** - Common issues and solutions

---

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

### Pathfinder _(Experimental during alpha)_

Pathfinder handles file discovery and resolution with loaders for multi-module repos and hierarchical ignores (like .goneatignore). Optimizes large-repo scans with glob patterns and directory traversal.

- Import: `github.com/fulmenhq/goneat/pkg/pathfinder`
- Key Features: Loaders for configs/tools, absolute/relative path handling, integration with ignore files.
- ⚠️ **Experimental**: API may change during alpha.

### Maturity Validation _(Experimental during alpha)_

The maturity package provides release lifecycle management and version consistency validation. Enables programmatic checking of repository phases and deployment readiness.

- Import: `github.com/fulmenhq/goneat/internal/maturity`
- Key Features: Phase file validation, version syntax checking, release readiness assessment.
- Usage: Integrate into CI/CD pipelines for automated release gate checks.
- ⚠️ **Experimental**: API may change during alpha.

### Assessment Runners

Extensible assessment framework with pluggable runners for different validation categories. Add custom checks by implementing the AssessmentRunner interface.

- Import: `github.com/fulmenhq/goneat/internal/assess`
- Key Features: Parallel execution, category-based assessment, JSON-first output for automation.
- Categories: format, lint, security, maturity, repo-status, and extensible for custom validations.
- Reminder: The library packages are part of the main module; no separate `go install` required—simply `go get github.com/fulmenhq/goneat` and import.

## Intelligent Hooks

Goneat manages Git hooks with intelligent format detection and zero-configuration setup. No more manual hooks.yaml editing — the system auto-detects your project's format capabilities and configures optimal workflows.

```bash
goneat hooks init                       # Auto-detects format capabilities
goneat hooks generate --with-guardian   # Add security approval workflows
goneat hooks install
```

**Important**: Git hooks live in `.git/hooks/` which is local to each clone. After cloning a repo, run `goneat hooks install` or use a Makefile with auto-install (see [hooks setup guide](docs/user-guide/bootstrap/hooks.md)).

**Smart Detection:**

- Auto-detects `make format-all`, `make format`, `make fmt` in Makefiles
- Finds npm format scripts, prettier configs, Python formatters (black, ruff)
- Configures format commands (priority 5) before assess commands (priority 10)
- No manual editing required — get project-aware configuration automatically

**Sensible defaults:**

- Pre-commit: format + assess (fail-on critical)
- Pre-push: format + assess with security + maturity + repo-status (fail-on high)
- Optimizations: cache_results, parallel execution, change-aware scoping

See [Release Quality Management](#release-quality-management) for details on maturity validation and dirty repository protection.

**Update flow:**

```bash
# Re-run init to pick up new format capabilities
goneat hooks init --force && goneat hooks generate && goneat hooks install
```

**Configuration:**

- `GONEAT_HOOK_OUTPUT=concise|markdown|json|html` controls hook output
- Fail thresholds configurable via `--fail-on`
- Guardian integration available for security-conscious teams

## Guardian Security

Protect critical git operations with policy-driven approval workflows. Guardian provides sudo-style approval for commits and pushes on protected branches, with browser-based authentication and configurable policies.

```bash
# Setup guardian protection
goneat guardian setup

# Generate hooks with guardian integration
goneat hooks generate --with-guardian
goneat hooks install

# Manual approval for protected operations
goneat guardian approve git commit -- git commit -m "protected change"
```

**Features:**

- **Browser approval**: Local web server with project branding and expiring sessions
- **Policy enforcement**: Repository-scope policies with branch-specific rules
- **Hook integration**: Automatically blocks protected operations until approved
- **Atomic execution**: Commands run only after successful approval
- **Configurable security**: Risk levels, expiry times, and approval methods

**Workflow:**

1. Protected git operations (commit/push) are blocked by hooks
2. Guardian prompts for approval via browser
3. User approves in browser with project context
4. Original command executes automatically once approved

Perfect for teams requiring approval workflows on main branches or release operations.

## ASCII Terminal Calibration

Handle Unicode character width inconsistencies across different terminal emulators with goneat's comprehensive ASCII toolkit. Fix emoji alignment issues, calibrate box drawing, and ensure consistent rendering.

```bash
# Diagnose current terminal
goneat ascii diag

# Test rendering with calibration files
goneat ascii box < tests/fixtures/ascii/calibration/emoji-grid.txt

# Detect and fix alignment issues
goneat ascii analyze --apply < tests/fixtures/ascii/calibration/width-test.txt

# Manual width corrections
goneat ascii mark --wide "🎟️" "🛠️" --term-program iTerm.app
```

**Features:**

- **Terminal detection**: Auto-detects Ghostty, iTerm2, Apple Terminal, and more
- **Width calibration**: Handles emoji variation selector rendering differences
- **Automated analysis**: Detects misalignment and generates correction commands
- **Box rendering**: Test Unicode box drawing and ASCII art alignment
- **Configuration system**: User overrides via `$GONEAT_HOME/config/terminal-overrides.yaml`

**Commands:**

- `ascii box`: Render text in aligned boxes for testing
- `ascii calibrate`: Interactive terminal width calibration
- `ascii analyze`: Automated alignment analysis with correction generation
- `ascii mark`: Manual width override configuration
- `ascii diag`: Terminal diagnostics and capability detection
- `ascii stringinfo`: Detailed character width analysis

Perfect for teams using ASCII art, box drawing, or emoji in documentation and CLIs.

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

Never deal with "tool not found" errors again. Goneat's built-in doctor automatically detects and installs required external tools — **including the package managers themselves** — no manual setup, no environment configuration hassles.

```bash
# Bootstrap everything in one command (new in v0.3.9)
# - Automatically installs brew if needed (bun is used on Windows where required)
# - Installs all foundation tools
# - Updates PATH immediately
goneat doctor tools --scope foundation --install --yes

# Check what's missing
goneat doctor tools --scope security

# Install everything automatically
goneat doctor tools --scope all --install --yes

# Get installation instructions
goneat doctor tools --scope format --print-instructions
```

Supported tools:

- **Foundation** (v0.3.15): ripgrep, jq, yq, minisign, go, go-licenses, golangci-lint, yamlfmt, yamllint, prettier
- **Security**: gosec, govulncheck, gitleaks
- **Format**: goimports, gofmt (bundled with Go)

**Automatic Package Manager Installation** (new in v0.3.9):

- Linux/macOS: prefers **Homebrew** (user-local, no sudo) for foundation tooling
- Windows: uses **bun** or **scoop** as defined in `.goneat/tools.yaml`
- Works on fresh CI runners; PATH updates are applied immediately for later steps
- PATH updated immediately — tools usable in same session

Benefits:

- **Zero setup time**: New team members can start contributing immediately
- **Consistent environments**: Same tool versions across all machines
- **Automatic updates**: Stay current with latest security tools
- **Non-intrusive**: Only installs what's needed, with clear prompts
- **CI-ready**: Works on fresh GitHub Actions runners out of the box

## Large‑repo performance

- Sharded execution (e.g., gosec across Go packages; multi-module via `go list`)
- Concurrency tuned via CPU percentage or explicit worker count
- Staged/diff scoping to minimize work on developer flows

## Commands

### Neat Commands (Core Functionality)

- `goneat assess`: Orchestrated assessment engine (format, lint, security, dependencies, static analysis, schema, date-validation, maturity, repo-status) with user-configurable assessment categories ([docs](docs/user-guide/commands/assess.md))
- `goneat ascii`: ASCII art and Unicode terminal calibration toolkit with box rendering, width analysis, and terminal-specific corrections ([docs](docs/user-guide/ascii.md))
- `goneat dates`: Validate and fix date consistency across your codebase ([docs](docs/user-guide/commands/dates.md))
- `goneat dependencies`: **NEW v0.3.0** - License compliance, package cooling (supply chain security), and SBOM generation ([docs](docs/user-guide/commands/dependencies.md) | [overview](docs/guides/dependency-protection-overview.md))
- `goneat format`: Multi-format formatting with finalizer stage (EOF/trailing spaces, line-endings, BOM) ([docs](docs/user-guide/commands/format.md))
- `goneat guardian`: Security approval workflows for protected git operations with browser-based authentication ([docs](docs/user-guide/commands/guardian.md))
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
- **Offline Schema Validation**: Automatically strips `$schema` fields from schemas and data to prevent network access during validation, enabling full offline operation
- See `docs/architecture/assets-management.md` and `docs/standards/assets-standard.md`

To refresh curated assets (optional):

```bash
make sync-schemas
```

Project configuration (preview): see `docs/configuration/schema-config.md` for configuring discovery patterns and auto-detect.

## Roadmap (Alpha)

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

# Recent release notes
   goneat docs show release-notes --format markdown | less
   goneat docs show releases/latest --format markdown | less


# Render to HTML (raw markdown wrapped in HTML)
goneat docs show user-guide/commands/hooks --format html > hooks.html
```

Tip: Use `goneat docs` to learn about hooks, commands, tutorials, and workflows without leaving your terminal.

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
