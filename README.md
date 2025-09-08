# goneat

[![Release](https://img.shields.io/github/v/release/fulmenhq/goneat?display_name=tag&sort=semver&logo=github)](https://github.com/fulmenhq/goneat/releases)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/3leaps/goneat)](go.mod)

A single CLI to make codebases neat: formatters, linters, security checks, and smart workflows — built for speed and large repositories.

## Quick Start (TL;DR)

1) Install (Go):

```bash
go install github.com/fulmenhq/goneat@latest
goneat version
```

2) Set up hooks (optional, recommended):

```bash
goneat hooks init
goneat hooks generate
goneat hooks install
```

3) Assess your repo:

```bash
goneat assess
```

4) Fix formatting (auto-fixable):

```bash
goneat format
```

Notes:
- Pre-release channel: v0.2.0-rc.X. `@latest` will prefer GA once v0.2.0 is out.
- Name clarification: This project is not affiliated with any other “goneat”. Use the full module path `github.com/fulmenhq/goneat`.

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
    https://raw.githubusercontent.com/fulmenhq/goneat/v0.2.0-rc.8/packaging/homebrew/goneat.rb
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

## Status

- Release: v0.2.0-rc.3 (tagged)
- Lifecycle Phase: RC (release candidate)
- Repo Visibility: Private (pending binary distribution verification)
- Gates: pre-commit (format+lint, fail-on=medium) passing; pre-push (format+lint+security, fail-on=high) passing
- Licensing: Audit clean (no GPL/LGPL/AGPL/MPL/CDDL); inventory maintained under `docs/licenses/`

Note: This is an active pre-release. Interfaces and outputs may evolve prior to GA.

## Highlights

- No‑hassle hooks: one manifest, one command, instant DX
- Zero‑friction tooling: automatic tool detection and installation
- JSON‑first SSOT: one structured output for CI and humans (markdown/html derived)
- Enterprise‑scale: sharded parallelism, multi-module awareness, .goneatignore filtering
- Extensible: add languages, tools, and policies without changing your hook scripts
- Diff‑Aware Assessment: prioritizes and highlights issues in your current change set
- Smart Semantic Validation (planned): detect and validate schemas beyond file extensions
- Suppression Insights: top rules/files with rich summaries for governance

## No‑hassle hooks

Goneat manages Git hooks from a single manifest — not hand-edited scripts. Update `/.goneat/hooks.yaml`, then regenerate and install with one command. Optimized for speed: staged-only scope, result caching, and parallel execution.

```bash
goneat hooks init
goneat hooks generate
goneat hooks install
```

Sensible defaults:

- Pre-commit: format + lint (fail-on medium)
- Pre-push: format + lint + security (fail-on high)
- Optimizations: only_changed_files, cache_results, parallel

Update flow:

```bash
# Edit .goneat/hooks.yaml or pull newer templates
goneat hooks generate && goneat hooks install
```

Tips:

- `GONEAT_HOOK_OUTPUT=concise|markdown|json|html` controls hook output
- Fail thresholds configurable via `--fail-on`; security concise shows `Fail-on: <level>`

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

- `goneat validate`: Schema-aware validation (preview; offline meta-validation)
- `goneat assess`: Orchestrated assessment engine (format, lint, security, static analysis, schema)
- `goneat format`: Multi-format formatting with finalizer stage (EOF/trailing spaces, line-endings, BOM)
- `goneat security`: Security scanning (gosec, govulncheck), sharded + parallel
- `goneat hooks`: Hook management (init, generate, install, validate, inspect)
- `goneat docs`: Read-only access to embedded user guides (most user-facing help)
- `goneat content`: Maintainer/curation tools for selecting and embedding docs (not for viewing)

Development note: The embed step runs during `make build` and `build-all` via `embed-assets`. Docs mirroring uses the CLI when a local binary exists; otherwise the tracked mirror is used. If you edit `docs/` or the manifest, run:

```bash
dist/goneat content embed --manifest docs/embed-manifest.yaml --root docs --target internal/assets/embedded_docs/docs
make build
```


### Doctor Command

Goneat includes a built-in doctor for automatic tool management. See the "Zero-friction tooling" section above for usage examples, or check `docs/user-guide/commands/doctor.md` for complete documentation.

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

## Lifecycle status

This project follows the Fulmen Ecosystem Lifecycle Maturity Model. Current phase: see `LIFECYCLE_PHASE` and `docs/status/lifecycle.md` for what this means operationally (coverage gates, contribution posture, and user guidance).

---

Generated by Code Scout under supervision of @3leapsdave

---

## Support & Community

- GitHub Repository: https://github.com/fulmenhq/goneat
- Issues & Feature Requests: https://github.com/fulmenhq/goneat/issues
- Releases: https://github.com/fulmenhq/goneat/releases
- Documentation: see docs/ directory in this repo
- Enterprise Support: contact 3 Leaps — support@3leaps.net

## License & Policies

- License: Apache-2.0 — see [LICENSE](LICENSE)
- OSS Policies (Code of Conduct, Security, Contributing):
  - https://github.com/3leaps/oss-policies (authoritative)
  - Code of Conduct: https://github.com/3leaps/oss-policies/blob/main/CODE_OF_CONDUCT.md
  - Security: https://github.com/3leaps/oss-policies/blob/main/SECURITY.md
  - Contributing: https://github.com/3leaps/oss-policies/blob/main/CONTRIBUTING.md

### Name Clarification

This project (github.com/fulmenhq/goneat) is not affiliated with any other projects named “goneat”. Use the full module path `github.com/fulmenhq/goneat` for `go install` and imports.

---

<div align="center">

<sub>
Built and maintained by the 3 Leaps team • Part of the <a href="https://fulmenhq.dev">Fulmen Ecosystem</a>
</sub>

</div>
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

Tip: Use `goneat docs` to learn about hooks, commands, tutorials, and workflows without leaving your terminal.


## Diff‑Aware Assessment (Change‑Set Intelligence)

For large repositories, signal‑to‑noise matters. Goneat captures git change‑set context and:

- Embeds `change_context` in assessment metadata (modified files, total changes, scope, branch/SHA)
- Marks issues as `change_related` with optional `lines_modified`
- Enables smarter CI: fail on high‑severity only when touched by the current diff

This helps reviewers and bots focus on what changed, speeding feedback and reducing churn.
