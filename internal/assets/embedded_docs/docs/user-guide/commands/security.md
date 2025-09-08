---
title: "Security Command"
description: "Run security scanners (vulnerabilities, code security) with JSON-first output"
author: "Code Scout (@code-scout)"
date: "2025-08-31"
last_updated: "2025-08-31"
status: "draft"
---

# goneat security

Run security scanners through goneat's GroupNeat interface.

## Summary

- Vulnerabilities: govulncheck (Go), future multi-ecosystem adapters
- Code security: gosec (Go), future Bandit/Semgrep/etc.
- Secrets: gitleaks (initial), future trufflehog
- JSON-first: use `--format json` for automation; concise/markdown/html also supported

## Usage

```bash
goneat security [target]
```

## Flags

- `--format`: concise | markdown | json | html | both (default: markdown)
- `--fail-on`: critical | high | medium | low | info (default: high)
- `--tools`: comma-separated tools to run (e.g., `gosec,govulncheck`)
- Secrets scanning: add `gitleaks` to `--tools` and enable the `secrets` dimension
- `--enable`: one or more of `vuln,code,secrets` (default: vuln,code)
- `--staged-only`: restrict to staged files (code scanners)
- `--diff-base`: restrict to files changed since ref (e.g., origin/main) (code scanners)
- `--concurrency`: explicit worker count for gosec sharding (default: derived)
- `--concurrency-percent`: percent of CPU cores to use when `--concurrency` is 0 (default: 50)
- `--timeout`: global assessment timeout for security (default: 5m)
- `--gosec-timeout`: per-tool timeout for gosec (0 = inherit global)
- `--govulncheck-timeout`: per-tool timeout for govulncheck (0 = inherit global)
- `--track-suppressions`: include inline suppression tracking and summary (e.g., `#nosec`)
- `--profile`: ci | dev (apply sensible defaults for fail-on)
- `--exclude-fixtures`: exclude common test fixture paths (default: true)
- `--fixture-patterns`: additional substrings to exclude from results (e.g., `tests/fixtures/`)
- `--max-issues`: limit displayed issues per category for non‑JSON output (0 = unlimited)
- `--output`: write report to file
- `--ignore-missing-tools`: skip missing security tools (otherwise fail fast if tool is explicitly requested via `--tools`)

## Environment

- `GONEAT_SECURITY_FAIL_ON`: surfaced in concise header for visibility

## Defaults & Best Practices

- Pre-commit: `--fail-on medium`
- Pre-push: `--fail-on high`
- CI (PR/main): `--fail-on high` (project-dependent)
- Release gates: `--fail-on critical` for vulnerabilities; high+ for code security in sensitive repos

## Examples

```bash
# Quick scan (concise)
goneat security --format concise

# JSON report for CI
goneat security --format json --output security.json

# Vulnerabilities only
goneat security --enable vuln --tools govulncheck --format concise

# Code security only on staged files
goneat security --enable code --tools gosec --staged-only --format concise

# Diff-based scan since origin/main
goneat security --enable code --tools gosec --diff-base origin/main --format concise

# Secrets scanning (gitleaks)
goneat security --enable secrets --tools gitleaks --format concise
goneat security --enable secrets --tools gitleaks --format json --output secrets.json

## Quick secrets alias

For a fast secrets-only scan using gitleaks:

```
goneat security secrets
```

# Quick preset (fast checks)
goneat security --enable vuln --tools govulncheck --format concise --timeout 2m

# Control sharding
goneat security --enable code --tools gosec --concurrency-percent 75 --format concise

# Per-tool timeouts
goneat security --enable code --tools gosec --gosec-timeout 1m --timeout 3m   # effective=1m
goneat security --enable vuln --tools govulncheck --govulncheck-timeout 4m     # effective=4m (global default 5m)

# Track suppressions (e.g., #nosec)
goneat security --track-suppressions --format json > security.json
 # JSON includes a suppression_report for the security category

# Limit log noise in non-JSON output
goneat security --format markdown --max-issues 50
```

## Install Tools With Doctor

Use the built-in Doctor command to check and install required tools:

```bash
# Print install instructions for missing tools
goneat doctor tools --scope security --tools gosec,govulncheck,gitleaks --print-instructions

# Install missing tools non-interactively (requires Go toolchain + network)
goneat doctor tools --scope security --tools gosec,govulncheck,gitleaks --install --yes
```

Notes:

- gitleaks module path is `github.com/zricethezav/gitleaks/v8@latest`.
- If your PATH doesn’t include `$(go env GOPATH)/bin` or `$GOBIN`, add it so installed tools are discoverable.

## Notes

- Tools must be installed and on PATH.
  - If you explicitly request a tool via `--tools` and it is not installed, the command fails fast with a helpful message suggesting installation (e.g., `go install golang.org/x/vuln/cmd/govulncheck@latest`) and a future `goneat doctor` flow. Use `--ignore-missing-tools` to skip missing tools.
  - If you do not specify `--tools`, absent tools are skipped when not selected by defaults.
- govulncheck scans at module scope; gosec operates on source files and can be scoped via staged/diff.

### Security Configuration Keys

```yaml
security:
  # Global timeouts and concurrency
  timeout: 5m
  concurrency: 0
  concurrency_percent: 50

  # Enable dimensions
  enable:
    code: true
    vuln: true
    secrets: false

  # Suppress fixture noise
  exclude_fixtures: true
  fixture_patterns:
    - "tests/fixtures/"
    - "test-fixtures/"

  # Per-tool timeouts
  tool_timeouts:
    gosec: 0s
    govulncheck: 0s

  # Fail threshold and suppressions
  track_suppressions: false
  fail_on: high
```
