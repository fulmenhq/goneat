---
title: "Toolchain Reference"
description: "Per-language tool coverage, common findings, and configuration guidance for each toolchain goneat supports"
author: "goneat contributors"
date: "2026-02-26"
last_updated: "2026-02-26"
status: "published"
tags: ["toolchains", "go", "typescript", "python", "rust", "reference"]
category: "user-guide"
---

# Toolchain Reference

goneat wraps best-in-class tools for each language behind a single interface. This
section documents what goneat actually runs for each toolchain, what findings mean,
how to configure behavior, and known version-sensitive edge cases.

## Coverage at a Glance

| Language | Lint | Format | Typecheck | Security | Dependency |
|----------|------|--------|-----------|----------|------------|
| **Go** | golangci-lint | gofmt / goimports | — | gosec, govulncheck | go-licenses |
| **TypeScript / JS** | biome | biome | tsc | — | — |
| **Python** | ruff | ruff | — | — | — |
| **Rust** | cargo-clippy | rustfmt | — | cargo-audit | cargo-deny |
| **YAML** | yamllint | yamlfmt | — | — | — |
| **Shell** | shellcheck | shfmt | — | — | — |
| **Markdown / JSON** | — | prettier | — | — | — |
| **Makefiles** | checkmake | — | — | — | — |
| **GitHub Actions** | actionlint | — | — | — | — |

Across-language security scanning (SBOM + CVE) is handled separately by
`goneat dependencies --vuln` (syft + grype) and is not language-scoped.

## Toolchain Guides

- [Go](go.md) — golangci-lint, gosec, gofmt, govulncheck; taint analysis; version notes
- [TypeScript / JS](typescript.md) — biome lint + format, tsc typecheck; monorepo nested-root behavior
- [Python](python.md) — ruff lint + format; pyproject.toml discovery
- [Rust](rust.md) — rustfmt, clippy, cargo-deny, cargo-audit; edition notes

## Installing Toolchains

Use `goneat doctor tools` to install everything for a scope:

```bash
goneat doctor tools --scope go --install --yes
goneat doctor tools --scope typescript --install --yes
goneat doctor tools --scope python --install --yes
goneat doctor tools --scope rust --install --yes
goneat doctor tools --scope all --install --yes   # everything
```

See [`doctor` command reference](../commands/doctor.md) for policy configuration
and version management.

## Version Sensitivity

Tool upgrades occasionally change what gets flagged. Each toolchain guide notes
known version-sensitive behaviors so you can anticipate findings when upgrading.
Release notes for each goneat version document toolchain changes absorbed during
that cycle.