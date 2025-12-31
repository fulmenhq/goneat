---
title: "Feature Gates (Configuration)"
description: "Scalable, per-command behavioral toggles with clear precedence and safe defaults"
author: "goneat contributors"
date: "2025-09-01"
last_updated: "2025-09-01"
status: "draft"
tags: ["configuration", "feature-gates", "flags", "env", "schema"]
---

# Feature Gates (Configuration)

Feature gates are per-command behavioral toggles that enable or disable optional capabilities in Goneat (e.g., import alignment, file normalization). This document defines a scalable, single place for feature-gate concepts, precedence, naming, and current gates.

## Goals

- Provide a consistent pattern to toggle features in group commands (format, assess, security).
- Offer a clear precedence model and safe, programmed defaults when config/env are absent.
- Scale across languages (Go, Python, TS/JS, Rust) without churn to call sites.

## Precedence Model

When the same gate is controllable by multiple mechanisms, the effective value resolves in this order:

1. CLI flag (explicitly set)
2. Environment variable (future)
3. Project config (future)
4. Programmed default (in code)

A small helper will be introduced to centralize resolution (planned): [pkg/config.FeatureGateResolver](pkg/config:1)

## Naming Scheme

- Environment variables (future):
  - Pattern: `GONEAT_<COMMAND>_<GATE>[__SUBGATE]`
  - Examples:
    - `GONEAT_FORMAT_USE_GOIMPORTS=on|off`
    - `GONEAT_FORMAT_FINALIZER_ENSURE_EOF=on|off`
    - `GONEAT_FORMAT_FINALIZER_LINE_ENDINGS=auto|lf|crlf|off`
  - Boolean parsing: on/off, true/false, 1/0, yes/no (case-insensitive)
  - Enumerations must match schema enums

- Configuration schema (future):
  - Feature gates live under command section:
  - Example for format:
    ```yaml
    format:
      feature_gates:
        finalizer:
          ensure_eof: true
          trim_trailing_whitespace: false
          normalize_line_endings: "auto" # enum: auto|lf|crlf|off
          remove_bom: true
        organizers:
          go:
            use_goimports: false
          python:
            use_isort: true
            use_black: true
            isort_profile: "black"
          ts:
            organize_imports: true
    ```

See current schema anchor: [schemas/config/goneat-config-v1.0.0.yaml](schemas/config/goneat-config/goneat-config-v1.0.0.yaml)

## Programmed Defaults

Always define a hard default in code to ensure safe behavior if config/env are unavailable.
Example (current): go import alignment defaults to off unless the CLI flag is provided.

- CLI flag and sequential flow implemented in: [cmd.RunFormat()](cmd/format.go:72), [cmd.formatGoFile()](cmd/format.go:298)
- Finalizer options and operations: [finalizer.NormalizationOptions](pkg/format/finalizer/finalizer.go:217), [finalizer.ComprehensiveFileNormalization()](pkg/format/finalizer/finalizer.go:176)

## Current Gates

### Format Command

- Go Import Alignment (organizer)
  - CLI: `--use-goimports` (default false)
  - Sequential pipeline order: gofmt → goimports → finalizer
  - Parallel strategy: currently warns and skips; to be added later
  - Code:
    - Flag and flow: [cmd.RunFormat()](cmd/format.go:72)
    - Go formatter path: [cmd.formatGoFile()](cmd/format.go:298)
- Finalizer (cross-type normalization)
  - CLI:
    - `--finalize-eof`
    - `--finalize-trim-trailing-spaces`
    - `--finalize-line-endings`
    - `--finalize-remove-bom`
  - Engine: [finalizer.ComprehensiveFileNormalization()](pkg/format/finalizer/finalizer.go:176)

### Assess Command (alignment)

- The Format assessment surfaces normalization sub-areas (EOF, whitespace, line endings, BOM).
  See: [assess.(\*FormatAssessmentRunner).Assess()](internal/assess/format_runner.go:32)

- Imports sub-area (for goimports) will be added next, with assess fix-mode delegating to:
  - [cmd.RunFormat()](cmd/format.go:72) + `--use-goimports` (resolved via precedence)

## Extensibility Guidance

- Per language, follow pattern: formatter → organizer → finalizer
- Add organizer gates under `format.feature_gates.organizers.<lang>`
- Respect command-level filters: `--types`, `.goneatignore`, staged-only
- Keep defaults conservative (off) for organizers, unless community norms suggest “on” and risk is low

## Document Sources of Truth

- Environment variables (SSOT): [docs/environment-variables.md](docs/environment-variables.md)
- Configuration schema: [schemas/config/goneat-config-v1.0.0.yaml](schemas/config/goneat-config/goneat-config-v1.0.0.yaml)
- Command docs:
  - Format: [docs/user-guide/commands/format.md](docs/user-guide/commands/format.md)
  - Assess: [docs/user-guide/commands/assess.md](docs/user-guide/commands/assess.md)

## Roadmap

- Add env variable for goimports: `GONEAT_FORMAT_USE_GOIMPORTS`
- Add schema fields for format.feature_gates (finalizer + organizers)
- Add resolver utility to centralize precedence
- Extend parallel path to honor gates
- Add assess “imports” subcategory and fix-mode delegation
