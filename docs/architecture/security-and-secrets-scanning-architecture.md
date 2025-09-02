---
title: "Security and Secrets Scanning Architecture"
description: "Multi-language vulnerability and secrets scanning via pluggable adapters and consistent GroupNeat UX"
author: "Code Scout (@code-scout)"
date: "2025-08-31"
status: "proposal"
tags: ["security","architecture","adapters","vulnerability","secrets"]
---

## Purpose
Define a forward-compatible architecture for security scanning in goneat that works across ecosystems (Go, JS/TS, Python, Rust, etc.), supports both dependency vulnerabilities and code-level issues, and provides consistent CLI/JSON/concise outputs. Include secrets leakage scanning for generic text assets (YAML/JSON/Markdown) and language-specific sources.

## Principles
- Adapter-driven: each tool is wrapped behind a stable interface.
- Ecosystem-aware: select tools by project manifests and file types.
- Offline-first: avoid network calls unless enabled.
- JSON-first: all tools normalized to a single issue schema with severities.
- Diff-first: prefer scanning only changed assets when possible.
- Safe-by-default: no writes; scanning is read-only.

## Scanning Dimensions
- Dependency vulnerabilities (modules, lockfiles)
- Code security findings (static analysis rules)
- Secrets leakage (API keys, tokens, credentials) in text and source files

## Adapter Interfaces

### Vulnerability scanners
```go
type VulnerabilityScanner interface {
    Name() string
    Ecosystems() []string            // e.g., ["go"], ["npm","yarn","pnpm"], ["pypi"], ["cargo"]
    IsAvailable() bool
    DetectTargets(root string) ([]Target, error) // manifests/lockfiles
    Scan(ctx context.Context, targets []Target, cfg Config) ([]assess.Issue, error)
}
```

### Code security scanners
```go
type CodeSecurityScanner interface {
    Name() string
    Languages() []string            // e.g., ["go"], ["python"], ["javascript","typescript"]
    IsAvailable() bool
    DetectFiles(root string) ([]string, error)
    Scan(ctx context.Context, files []string, cfg Config) ([]assess.Issue, error)
}
```

### Secrets scanners
```go
type SecretsScanner interface {
    Name() string
    FileGlobs() []string            // e.g., ["**/*.{yaml,yml,json,md}","**/*.env"]
    IsAvailable() bool
    DetectFiles(root string) ([]string, error)
    Scan(ctx context.Context, files []string, cfg Config) ([]assess.Issue, error)
}
```

## Initial Tooling Matrix
- Go
  - Vulnerabilities: `govulncheck`
  - Code Security: `gosec`
- Multi-language
  - Vulnerabilities: `osv-scanner` (lockfiles across npm, yarn, pnpm, pip/poetry, cargo, etc.)
  - Secrets: `gitleaks` (fast, configurable), optional `trufflehog` later
- Ecosystem-specific (future)
  - Python: `bandit` (code)
  - JS/TS: ESLint security presets or `semgrep` rulesets (code)
  - Rust: `cargo-audit` (deps)
  - Containers: `trivy` (images/filesystems)

## Selection & Scope
1) Ecosystem detection
   - Manifests/lockfiles: `go.mod`, `package.json`/`pnpm-lock.yaml`/`yarn.lock`, `requirements*.txt`/`poetry.lock`, `Cargo.toml`/`Cargo.lock`, etc.
   - Language by file extension for code scanners.
2) Filtering
   - Respect `.goneatignore`, content type limiting, and repo conventions (ignore `vendor/`, `node_modules/`, `dist/`).
   - Diff/staged-only filtering limits targets when requested.
3) Invocation
   - Build a plan of adapters per ecosystem and dimension; execute with concurrency where safe.

## Configuration
```yaml
security:
  tools: ["govulncheck","gosec","osv-scanner","gitleaks"]
  enable:
    vulnerabilities: true
    code_security: true
    secrets: true
  ecosystems: ["auto"]                 # or explicit: ["go","js","python"]
  dependency_scanners:
    go: ["govulncheck"]
    js: ["osv-scanner"]
    python: ["osv-scanner"]
    rust: ["cargo-audit"]
  code_scanners:
    go: ["gosec"]
    python: ["bandit"]
    js: ["semgrep"]
  secrets_scanners: ["gitleaks"]
  diff:
    staged_only: false
    diff_base: ""
  timeouts:
    default: 5m
    gosec: 3m
    govulncheck: 5m
    osv_scanner: 4m
    gitleaks: 2m
```

## Output Normalization
All findings mapped to `assess.Issue` with:
- Category: `security`
- SubCategory: `vulnerability` | `code` | `secret`
- Severity mapping per tool → {critical, high, medium, low, info}
- File or target (module/package) attribution

## Secrets Scanning Notes
- Default include globs: `**/*.{yaml,yml,json,md,env}`; configurable
- Rule tunings: allowlist patterns, entropy thresholds, custom regexes via config
- False positive handling: suppression file with TTL and rationale (planned in v0.1.x)

## Go-centric vs other languages
- When only Go manifests are present, run Go adapters (govulncheck/gosec) and secrets scanners on text assets
- When other ecosystems detected, add corresponding adapters (e.g., `osv-scanner`) automatically
- If no adapters available for an ecosystem, warn with remediation guidance (install tools; enable adapters)

## CLI
- `goneat security` with flags mirroring assess subset; `--ecosystems`, `--tools`, `--enable secrets|code|vuln`, `--staged-only`, `--diff-base`

## Performance & Safety
- Concurrency tuned per adapter; cache last results where possible (future)
- Read-only operations; no network unless adapter requires it and allowed

## Phasing
- v0.1.3: Go adapters (govulncheck, gosec) + secrets via `gitleaks` (if available) behind a flag; JSON/concise output
- v0.1.x: Add `osv-scanner` and per-ecosystem adapters; suppression/exposure; SARIF export

---
Generated by Code Scout under supervision of @3leapsdave