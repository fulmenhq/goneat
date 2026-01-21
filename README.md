# goneat

[![Release](https://img.shields.io/github/v/release/fulmenhq/goneat?display_name=tag&sort=semver&logo=github)](https://github.com/fulmenhq/goneat/releases)
[![CI](https://github.com/fulmenhq/goneat/actions/workflows/ci.yml/badge.svg)](https://github.com/fulmenhq/goneat/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/fulmenhq/goneat)](go.mod)

**One CLI to orchestrate code quality across your polyglot codebase.**

Stop juggling twelve tools across four languages. goneat wraps best-in-class toolchains—gofmt, ruff, biome, clippy, and more—behind a single interface with parallel execution, unified output, and zero configuration drama.

Beyond formatting and linting, goneat handles the hard parts of CI/CD: dependency analysis, license compliance, package cooling policies, security scanning, and git hooks. For teams building with Go, Python, TypeScript, and Rust, it's the orchestration layer that brings order to polyglot pipelines.

**New to goneat?** Start with `brew install fulmenhq/tap/goneat` to explore. When you're ready to integrate into CI/CD, see [End-to-End Setup](docs/user-guide/end-to-end-setup.md) for the full trust-anchored workflow.

## Why goneat?

| Challenge | goneat Solution |
|-----------|-----------------|
| **Tool sprawl** | One CLI wraps gofmt, golangci-lint, ruff, biome, clippy, prettier, and more |
| **Slow CI** | Parallel execution uses all CPU cores—format 800+ files in seconds |
| **YAML schemas ignored** | Validates YAML-defined schemas that traditional tools skip |
| **Supply chain risk** | Package cooling blocks newly published dependencies until vetted |
| **Known vulnerabilities** | SBOM-based scanning via grype detects CVEs across Go, Rust, Python, TypeScript |
| **Hook fragmentation** | Language-neutral hooks infrastructure works across your entire repo |
| **Agent integration** | JSON Schema-backed output for AI agents and automation |

## Performance at Scale

Real benchmarks on public repositories:

| Repository | Files | Time | CPU Utilization |
|------------|-------|------|-----------------|
| [Hugo](https://github.com/gohugoio/hugo) | 883 Go files | 14.4s | 716% (7 cores) |
| [Crucible](https://github.com/fulmenhq/crucible) | 1,743 YAML/MD files | 7.6s | 651% (6 cores) |
| [SchemaStore](https://github.com/SchemaStore/schemastore) | 776 JSON schemas | 2.5s | 316% (3 cores) |

goneat defaults to 80% CPU utilization with configurable worker pools. Your CI stays fast even on large monorepos.

## Quick Start

```bash
# Install (pick one)
brew install fulmenhq/tap/goneat          # macOS/Linux
go install github.com/fulmenhq/goneat@latest  # Cross-platform

# Initialize tooling config (one-time per repo)
goneat doctor tools init
git add .goneat/tools.yaml && git commit -m "chore: configure goneat"

# Assess your codebase
goneat assess                    # Full assessment
goneat assess --categories lint  # Just linting
goneat format                    # Auto-fix formatting

# Set up hooks (recommended)
goneat hooks init && goneat hooks install
```

## Language Support

goneat provides **language-aware assessment** with automatic tool detection:

| Language | Lint | Format | Typecheck | Tool | Install |
|----------|------|--------|-----------|------|---------|
| **Go** | Yes | Yes | — | golangci-lint, gofmt | `brew install golangci-lint` |
| **Python** | Yes | Yes | — | [ruff](https://docs.astral.sh/ruff/) | `brew install ruff` |
| **TypeScript/JS** | Yes | Yes | Yes | [biome](https://biomejs.dev/), tsc | `brew install biome` |
| **Rust** | Yes | Yes | — | cargo-clippy, rustfmt, cargo-deny | `rustup component add clippy rustfmt` |
| **YAML** | Yes | Yes | — | yamllint, yamlfmt | `brew install yamllint yamlfmt` |
| **Markdown/JSON** | — | Yes | — | prettier | `npm install -g prettier` |
| **Shell** | Yes | — | — | shellcheck, shfmt | `brew install shellcheck shfmt` |
| **Makefiles** | Yes | — | — | checkmake | `brew install checkmake` |
| **GitHub Actions** | Yes | — | — | actionlint | `brew install actionlint` |

**TypeScript type checking** (v0.5.0+): Run `goneat assess --categories typecheck` to catch type errors via `tsc --noEmit`. Complements biome's lint/format with full type analysis.

**Graceful degradation**: Missing a tool? goneat skips it and logs what was skipped—no errors, no broken builds. Add tools incrementally as your needs grow.

**Automatic installation**: Use [doctor tools](docs/user-guide/commands/doctor.md) to install everything at once:

```bash
goneat doctor tools init                           # Generate .goneat/tools.yaml for your repo
goneat doctor tools --scope foundation --install   # Install rg, jq, yq, prettier, yamlfmt...
goneat doctor tools --scope security --install     # Install gosec, govulncheck, gitleaks
goneat doctor tools --scope all --install          # Install everything
```

The `foundation` scope includes common DX tools (ripgrep, jq, yq) plus language-specific formatters. All tool configuration lives in `.goneat/tools.yaml`—no hidden defaults.

## Core Capabilities

### Multi-Language Format & Lint

```bash
goneat assess --categories format,lint   # Check across all languages
goneat format --fix                       # Auto-fix everything fixable
goneat assess --categories lint --new-issues-only  # Only new issues since last commit
```

### Schema Validation

Unlike traditional tools, goneat validates **YAML-defined schemas**—not just `.json` files. Both meta-validation (is this a valid schema?) and data validation (does this config match its schema?):

```bash
# Validate schema files themselves (meta-validation)
goneat schema validate-schema --recursive ./schemas/

# Use glob patterns for targeted validation
goneat schema validate-schema "schemas/**/*.json"

# Validate config files against schemas
goneat validate data --schema schemas/config.yaml config/app.yaml

# Bulk validate with parallel workers
goneat validate suite --workers 4 --mapping .goneat/schema-mappings.yaml
```

Supports **all major JSON Schema versions**: Draft-04, Draft-06, Draft-07, 2019-09, and 2020-12—with **offline `$ref` resolution** and no network calls needed. Validate legacy Kubernetes schemas alongside modern OpenAPI 3.1 specs. Embedded meta-schemas enable air-gapped CI environments.

### Supply Chain Security

Protect against dependency attacks with vulnerability scanning, package cooling, and license compliance:

```bash
# Scan for known vulnerabilities (NEW in v0.5.0)
goneat dependencies --vuln

# Check license compliance
goneat dependencies --licenses

# Enforce cooling policy (block packages < 7 days old)
goneat dependencies --cooling --fail-on high

# Generate SBOM for compliance
goneat dependencies --sbom

# Full supply chain assessment in CI
goneat assess --categories dependencies --fail-on high
```

**Vulnerability scanning** uses syft (SBOM generation) and grype (CVE detection) to identify known vulnerabilities across Go, Rust, Python, and TypeScript dependencies. Configure policy in `.goneat/dependencies.yaml`:

```yaml
vulnerabilities:
  enabled: true
  fail_on: high
  allow:
    - id: CVE-2024-12345
      until: 2026-06-30
      reason: "Vendor patch pending"
```

**Package cooling** blocks newly published dependencies until vetted. 80% of supply chain attacks are detected within 7 days—the ua-parser-js attack (8M+ weekly downloads) would have been blocked.

### Language-Neutral Hooks

One hook system works across Go, Python, TypeScript, and Rust:

```bash
goneat hooks init                    # Auto-detect project capabilities
goneat hooks generate                # Generate hook scripts
goneat hooks generate --with-guardian  # Add browser-based approval prompts
goneat hooks install                 # Install to .git/hooks
```

Pre-commit runs format checks. Pre-push adds security, maturity, and dependency validation.

**Guardian integration**: Use `--with-guardian` to add an optional friction layer that requires browser-based approval before commits or pushes to protected branches. Prevents fully autonomous operations when you want human oversight. See [Guardian](#guardian-approval-workflows) for configuration.

## For DevSecOps Teams

### Dependency Policy Enforcement

Create `.goneat/dependencies.yaml`:

```yaml
version: v1
licenses:
  forbidden: [GPL-3.0, AGPL-3.0]
  allowed: [MIT, Apache-2.0, BSD-3-Clause]
cooling:
  enabled: true
  min_age_days: 7
```

Then enforce in CI:

```bash
goneat assess --categories dependencies --fail-on high
```

### Security Scanning

```bash
goneat security                    # Run gosec, govulncheck
goneat assess --categories security --track-suppressions  # Track #nosec comments
```

### Guardian Approval Workflows

Protect critical operations with browser-based approval:

```bash
goneat guardian setup
goneat hooks generate --with-guardian
```

## For Large Repositories

### Parallel Execution

```bash
goneat assess --concurrency-percent 80   # Use 80% of CPU cores (default)
goneat format --workers 8                 # Explicit worker count
```

### Diff-Aware Assessment

Only assess what changed:

```bash
goneat assess --staged-only              # Only staged files
goneat assess --new-issues-only          # Only new issues since HEAD~
```

### Scoped Discovery

```bash
goneat assess --include "src/**" --exclude "vendor/**"
goneat assess --force-include "tests/fixtures/**"  # Override .gitignore
```

## Commands

| Category | Commands |
|----------|----------|
| **Assessment** | `assess`, `format`, `security`, `validate`, `dependencies`, `dates` |
| **Workflow** | `hooks`, `guardian`, `repository`, `maturity` |
| **Tooling** | `doctor`, `schema`, `pathfinder`, `ssot` |
| **Setup** | `init`, `doctor tools init`, `hooks init` |
| **Support** | `docs`, `version`, `envinfo`, `info` |

```bash
goneat --help              # Full command list
goneat docs list           # Browse embedded documentation
goneat docs show assess    # Read command guide offline
```

## CI/CD Integration

### Zero-Friction Tool Installation

Fresh CI runner? One command installs everything:

```bash
goneat doctor tools --scope foundation --install --yes
```

Automatically installs package managers (brew/bun) and all required tools. Idempotent and fast on subsequent runs.

### JSON-First Output

All commands produce structured JSON for automation:

```bash
goneat assess --format json --output report.json
goneat dependencies --format json | jq '.issues'
```

## Install

### Quick Start

```bash
# Homebrew (macOS/Linux)
brew install fulmenhq/tap/goneat

# Go (cross-platform)
go install github.com/fulmenhq/goneat@latest

# Scoop (Windows)
scoop bucket add fulmenhq https://github.com/fulmenhq/scoop-bucket
scoop install goneat
```

### For CI/CD: Trust Anchor Pattern

For production pipelines, we recommend the **trust anchor pattern**—cryptographic verification from the first binary you install. This establishes an auditable chain of trust for your entire build process.

The idea: install one verified tool ([sfetch](https://github.com/3leaps/sfetch)), then use it to install everything else with signature verification. No unsigned binaries in your pipeline.

```bash
# Step 1: Install sfetch (the trust anchor) - verify this checksum manually
curl -sSfL https://github.com/3leaps/sfetch/releases/latest/download/install-sfetch.sh | bash

# Step 2: Use sfetch to install goneat with minisign verification
sfetch --repo fulmenhq/goneat --latest --dest-dir ~/.local/bin
```

This pattern is how goneat itself manages supply chain security. When goneat runs `doctor tools --install`, it uses the same verification approach for downstream tools. See [End-to-End Setup](docs/user-guide/end-to-end-setup.md) for the full workflow.

### Binary Download

Download from [GitHub Releases](https://github.com/fulmenhq/goneat/releases). All releases include minisign and PGP signatures for manual verification.

## Configuration

goneat uses `.goneat/` directory for project configuration:

| File | Purpose |
|------|---------|
| `tools.yaml` | Tool manifest for `doctor tools` |
| `hooks.yaml` | Git hook orchestration |
| `assess.yaml` | Lint/assessment tuning |
| `dependencies.yaml` | License and cooling policies |
| `schema-mappings.yaml` | Config-to-schema mappings |

## Documentation

```bash
goneat docs list                    # Available topics
goneat docs show user-guide/install # Read offline
goneat docs show releases/latest    # Latest release notes
```

Full documentation: [docs/](docs/)

## Status

- **Version**: See [VERSION](VERSION)
- **Lifecycle**: Beta (previously Alpha, 25+ releases)
- **Platforms**: macOS, Linux, Windows (operational)

## Support

- **Repository**: [github.com/fulmenhq/goneat](https://github.com/fulmenhq/goneat)
- **Issues**: [GitHub Issues](https://github.com/fulmenhq/goneat/issues)
- **Enterprise**: support@3leaps.net

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and [3 Leaps OSS Policies](https://github.com/3leaps/oss-policies).

## License

Apache-2.0 — see [LICENSE](LICENSE)

**Trademarks**: "Fulmen", "goneat", and "3 Leaps" are trademarks of 3 Leaps, LLC. Use the full module path `github.com/fulmenhq/goneat` for imports (this project is not affiliated with any other "goneat").

---

**Built by the [3 Leaps](https://3leaps.net) team** | Part of the [Fulmen Ecosystem](https://fulmenhq.dev)
