---
title: "Dependency Protection Architecture"
description: "How goneat evaluates licenses, cooling policy, and SBOM metadata across standalone and assessment workflows"
author: "@arch-eagle"
date: "2025-10-22"
last_updated: "2025-10-22"
status: "draft"
tags:
  ["architecture", "dependencies", "security", "sbom", "assessment"]
category: "architecture"
---

# Dependency Protection Architecture

Goneat’s dependency protection stack combines standalone tooling, assessment orchestration, and policy-driven output. The
design supports incremental adoption (license checks only) while scaling to full supply-chain governance with SBOM
tracking and cooling policies.

## High-Level Flow

```
┌───────────────┐
│ goneat CLI    │
│ └─ dependencies│
│ └─ assess      │
└──────┬────────┘
       │ capabilities registry (ops.GroupNeat, CategoryDependencies)
       ▼
┌──────────────────────────┐
│ pkg/dependencies.Analyzer│
│  - language detectors     │
│  - license resolver       │
│  - cooling evaluator      │
└─────────┬────────────────┘
          │ AnalysisResult (issues + metrics)
          ▼
┌──────────────────────────┐
│ internal/assess runner   │
│  - severity mapping       │
│  - fail-on thresholds     │
│  - metrics aggregation    │
└─────────┬────────────────┘
          │ dependency-analysis.schema.json
          ▼
┌──────────────────────────┐
│ Reports & Hooks          │
│  - JSON / Markdown        │
│  - SBOM metadata          │
│  - Git hooks / CI         │
└──────────────────────────┘
```

## Components

### CLI Surface

- `cmd/dependencies.go` – standalone command exposing `--licenses`, `--cooling`, and `--sbom`.
- `internal/ops` – registers the dependencies category so hooks/assess discover it automatically.
- `cmd/assess.go` – surfaces `--categories dependencies`, `--fail-on`, and reporting flags.

### Analysis Engine (`pkg/dependencies`)

- **Detector:** Identifies supported languages and selects analyzers (Go today; stubs for TS/Python/Rust/C#).
- **License Analyzer:** Evaluates manifests against `.goneat/dependencies.yaml` policies and reports forbidden licenses.
- **Cooling Policy:** Fetches publish metadata, applies age/download thresholds, and honours exception lists.
- **Result Model:** Captures issues, severities, dependency metadata, and SBOM linkage fields.

### SBOM Integration (`pkg/sbom`)

- Syft invoker validates binary location (env override or managed install).
- Output paths resolved with `filepath.Abs` and guarded by decompression limits (`ErrArchiveTooLarge`).
- Assessment reports include SBOM metrics when a recent artifact exists (no automatic generation during assess).

### Assessment Runner (`internal/assess/dependencies_runner.go`)

- Wraps `Analyzer.Analyze` inside assessment lifecycle.
- Maps analyzer severities to Crucible levels and enforces `--fail-on`.
- Emits metrics that adhere to `schemas/dependencies/v1.0.0/dependency-analysis.schema.json`.
- Marks category as non-parallel (cooling policy may require network requests).

### Configuration (`pkg/config`)

- `DependenciesConfig` pulls defaults from `.goneat/dependencies.yaml`.
- Policy files and generated manifests are written with `0600` permissions for secure storage.
- Hooks manifest (`.goneat/hooks.yaml`) can reference the category directly.

## Security Controls

- **Path Validation:** All filesystem paths resolved via `filepath.Abs` before read/write to prevent traversal.
- **Executable Permissions:** Managed downloads use explicit `os.Chmod(..., 0o755)` with `#nosec` justification.
- **Archive Limits:** Tar/zip extraction capped at 500 MB per file to mitigate decompression bombs.
- **Command Execution:** Syft invocation arguments are constant and flagged with `#nosec G204`.

## Workflow Integration

- **Pre-Commit:** Run `goneat dependencies --licenses` offline for fast feedback.
- **Pre-Push / CI:** Use `goneat assess --categories dependencies --fail-on high` with network access.
- **Release Automation:** Persist SBOM metadata (`sbom/goneat-*.cdx.json`) and reference it in release notes.

Refer to [Dependency Gating Overview](../workflows/dependency-gating.md) for implementation guidance and hooks.
