# End-to-End Setup Guide

This guide walks through integrating goneat into your CI/CD pipeline with full supply chain security. By the end, you'll have:

- Cryptographically verified tool installation (trust anchor pattern)
- Automated code quality gates (format, lint, security)
- Dependency analysis with license compliance and package cooling
- Git hooks for local development
- A reproducible, auditable pipeline

## The Big Picture

Modern polyglot repositories face a coordination problem: Go uses gofmt and golangci-lint, Python uses ruff, TypeScript uses biome, Rust uses clippy and rustfmt. Each tool has its own configuration, output format, and failure modes. CI pipelines become a patchwork of scripts.

goneat solves this by providing a single orchestration layer:

```
┌─────────────────────────────────────────────────────────────┐
│                        goneat                               │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐       │
│  │  Format  │ │   Lint   │ │ Security │ │   Deps   │       │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘       │
│       │            │            │            │              │
│  ┌────┴────────────┴────────────┴────────────┴────┐        │
│  │              Language Detection                 │        │
│  └────┬────────────┬────────────┬────────────┬────┘        │
│       │            │            │            │              │
│    gofmt        ruff        biome       clippy             │
│    golangci     black       eslint      rustfmt            │
│    gosec                    prettier    cargo-deny         │
└─────────────────────────────────────────────────────────────┘
```

One command, unified JSON output, parallel execution.

## Phase 1: Trust Anchor Installation

Supply chain security starts with your first binary. The trust anchor pattern establishes cryptographic verification from the beginning.

### Why This Matters

Every tool you install is code that runs in your pipeline. Without verification:

- Compromised package registries can serve malicious binaries
- Man-in-the-middle attacks can replace downloads
- You have no audit trail of what actually ran

The trust anchor pattern: install one verified tool, use it to verify everything else.

### Setup

```bash
# Install sfetch - verify this manually against published checksums
curl -sSfL https://github.com/3leaps/sfetch/releases/latest/download/install-sfetch.sh | bash

# Use sfetch to install goneat with signature verification
sfetch --repo fulmenhq/goneat --latest --dest-dir ~/.local/bin

# Verify installation
goneat version
```

For CI, cache the verified binaries. See [CI Examples](#ci-pipeline-examples) below.

## Phase 2: Repository Configuration

Initialize goneat for your repository:

```bash
# Generate tool manifest based on detected languages
goneat doctor tools init

# Review and commit
cat .goneat/tools.yaml
git add .goneat/tools.yaml
git commit -m "chore: initialize goneat configuration"
```

The generated `tools.yaml` declares which tools your project needs. goneat uses this for:

- `doctor tools --install` to install missing tools
- `assess` to know which checks to run
- Reproducible CI environments

## Phase 3: Tool Installation

Install the tools your project needs:

```bash
# Foundation tools (ripgrep, jq, yq, formatters)
goneat doctor tools --scope foundation --install --yes

# Language-specific tools
goneat doctor tools --scope go --install --yes        # golangci-lint, gosec, etc.
goneat doctor tools --scope python --install --yes    # ruff
goneat doctor tools --scope typescript --install --yes # biome
goneat doctor tools --scope rust --install --yes      # cargo-deny, cargo-audit

# Security tools
goneat doctor tools --scope security --install --yes  # gosec, govulncheck, gitleaks
```

Or install everything:

```bash
goneat doctor tools --scope all --install --yes
```

## Phase 4: Dependency Policy

Create `.goneat/dependencies.yaml` to define your supply chain policy:

```yaml
version: v1
licenses:
  # Forbidden licenses - hard fail
  forbidden:
    - GPL-3.0
    - AGPL-3.0
    - SSPL-1.0

  # Allowed licenses - explicit allowlist
  allowed:
    - MIT
    - Apache-2.0
    - BSD-2-Clause
    - BSD-3-Clause
    - ISC
    - MPL-2.0

cooling:
  enabled: true
  min_age_days: 7 # Block packages published < 7 days ago
```

**Why cooling?** 80% of supply chain attacks are detected within 7 days of publication. Package cooling provides a buffer for the community to identify malicious packages.

Test your policy:

```bash
# Check license compliance
goneat dependencies --licenses

# Check cooling policy
goneat dependencies --cooling

# Full dependency assessment
goneat assess --categories dependencies
```

## Phase 5: Git Hooks

Set up local development hooks:

```bash
# Initialize hooks configuration
goneat hooks init

# Generate hook scripts
goneat hooks generate

# Install to .git/hooks
goneat hooks install
```

Default behavior:

- **Pre-commit**: Format check (fast, <2s)
- **Pre-push**: Full assessment including security and dependencies

For teams requiring human approval on protected branches:

```bash
goneat hooks generate --with-guardian
```

## Phase 6: CI Pipeline Integration

### GitHub Actions Example

```yaml
name: CI
on: [push, pull_request]

jobs:
  assess:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      # Cache goneat and tools
      - uses: actions/cache@v4
        with:
          path: |
            ~/.local/bin/goneat
            ~/.local/bin/sfetch
          key: goneat-${{ runner.os }}-v0.4.5

      # Install via trust anchor (first run only)
      - name: Install goneat
        run: |
          if [ ! -f ~/.local/bin/goneat ]; then
            curl -sSfL https://github.com/3leaps/sfetch/releases/latest/download/install-sfetch.sh | bash
            sfetch --repo fulmenhq/goneat --latest --dest-dir ~/.local/bin
          fi
          echo "$HOME/.local/bin" >> $GITHUB_PATH

      # Install project tools
      - name: Install tools
        run: goneat doctor tools --scope foundation,go --install --yes

      # Run assessment
      - name: Assess
        run: goneat assess --format json --output report.json

      # Upload report
      - uses: actions/upload-artifact@v4
        with:
          name: goneat-report
          path: report.json
```

### GitLab CI Example

```yaml
assess:
  image: ubuntu:latest
  cache:
    paths:
      - .local/bin/
  script:
    - |
      if [ ! -f .local/bin/goneat ]; then
        curl -sSfL https://github.com/3leaps/sfetch/releases/latest/download/install-sfetch.sh | bash
        sfetch --repo fulmenhq/goneat --latest --dest-dir .local/bin
      fi
      export PATH="$PWD/.local/bin:$PATH"
    - goneat doctor tools --scope foundation --install --yes
    - goneat assess --fail-on high
```

## Phase 7: Ongoing Operations

### Daily Development

```bash
# Before committing - hooks run automatically, or manually:
goneat format              # Auto-fix formatting
goneat assess --staged-only # Check only staged files
```

### PR Review

```bash
# Only new issues since base branch
goneat assess --new-issues-only --new-issues-base origin/main
```

### Dependency Updates

```bash
# After updating go.mod, Cargo.toml, package.json, etc.
goneat dependencies --licenses    # Check for license changes
goneat dependencies --cooling     # Check cooling policy
goneat assess --categories dependencies --fail-on high
```

### Security Scanning

```bash
goneat security                   # Run gosec, govulncheck
goneat assess --categories security --track-suppressions
```

## Summary

| Phase | Command                         | Purpose                    |
| ----- | ------------------------------- | -------------------------- |
| 1     | `sfetch --repo fulmenhq/goneat` | Trust anchor installation  |
| 2     | `goneat doctor tools init`      | Generate tool manifest     |
| 3     | `goneat doctor tools --install` | Install required tools     |
| 4     | Create `dependencies.yaml`      | Define supply chain policy |
| 5     | `goneat hooks install`          | Set up git hooks           |
| 6     | CI pipeline config              | Automate in CI/CD          |
| 7     | `goneat assess`                 | Ongoing quality gates      |

## Next Steps

- [Command Reference](commands/) - Detailed command documentation
- [Dependency Analysis](commands/dependencies.md) - License and cooling policies
- [Hooks Configuration](commands/hooks.md) - Customizing git hooks
- [Schema Validation](commands/validate.md) - YAML/JSON schema validation

---

**Questions?** Open an issue at [github.com/fulmenhq/goneat](https://github.com/fulmenhq/goneat/issues)
