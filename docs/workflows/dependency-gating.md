---
title: "Dependency Gating Overview"
description: "Quick reference for integrating goneat dependency assessments into developer workflows"
author: "@arch-eagle"
date: "2025-10-22"
last_updated: "2025-10-22"
status: "draft"
tags: ["workflow", "dependencies", "security", "sbom"]
category: "workflows"
---

# Dependency Gating Overview

Goneat v0.3.0 introduces a dedicated `dependencies` assessment category for supply-chain policy enforcement. Use this
guide as a quick reference and jump to the detailed playbook when you are ready to wire the tooling into hooks or CI.

## When to Use

- **Pre-commit (offline):** Run `goneat dependencies --licenses` to block forbidden licenses without requiring network
  access.
- **Pre-push / CI (online):** Run `goneat assess --categories dependencies --fail-on high` so cooling policy checks and
  SBOM metadata validation gate merges.
- **Release audits:** Generate SBOM artifacts with `goneat dependencies --sbom` and archive the metadata alongside
  release notes.

## Key Resources

- [Dependency Gating Workflow Guide](../user-guide/workflows/dependency-gating.md) – full hook configurations, CI
  examples, and troubleshooting tips.
- [Dependencies Command Reference](../user-guide/commands/dependencies.md) – CLI flags for license, cooling, and SBOM
  operations.
- [Assess Command Reference](../user-guide/commands/assess.md#dependencies-dependencies) – category behaviour and JSON
  schema integration.

## Security Defaults

- SBOM extraction enforces a 500 MB limit to mitigate decompression bombs (`ErrArchiveTooLarge`).
- Policy and manifest files are written with `0600` permissions to protect sensitive data.
- Syft invocation paths are resolved via `filepath.Abs` to avoid path traversal.

## Next Steps

1. Install Syft via `goneat doctor tools --scope sbom --install --yes`.
2. Configure `.goneat/dependencies.yaml` with your license and cooling policy.
3. Adopt the recommended hook definitions from the workflow guide and validate with `goneat hooks run --all`.
