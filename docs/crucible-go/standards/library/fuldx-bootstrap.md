---
title: "FulDX Bootstrap Standard"
description: "Helper module contract for installing FulDX and declared tooling manifests"
author: "Schema Cartographer"
date: "2025-10-09"
last_updated: "2025-10-09"
status: "draft"
tags: ["standards", "library", "bootstrap", "fuldx", "2025.10.2"]
---

# FulDX Bootstrap Standard

## Purpose

Define how helper libraries bootstrap FulDX and the tooling manifest so downstream projects can install
dependencies consistently across platforms.

## Responsibilities

1. Install FulDX via `.goneat/tools.yaml` (or user override).
2. Respect `.goneat/tools.local.yaml` for local development overrides (gitignored).
3. Expose helper functions/CLI commands (`make bootstrap`, `fulmen bootstrap`) that download and verify FulDX
   checksums.
4. Provide utilities to invoke FulDX commands (`fuldx ssot sync`, `fuldx version bump`).

## Workflow

1. Resolve manifest path (`.goneat/tools.yaml` by default, fall back to `.goneat/tools.local.yaml` when present).
2. Read manifest (validated against `schemas/tooling/external-tools/...`).
3. Download FulDX binary matching host OS/arch, verify checksum.
4. Place binary in `./bin/fuldx` (configurable destination). Ensure executable bit on POSIX platforms.
5. Print instructions for PATH export or provide wrapper scripts.

## Helper API

| Function / Command          | Description                                                     |
| --------------------------- | --------------------------------------------------------------- |
| `Bootstrap(options)`        | Install FulDX and return installation metadata (version, path). |
| `ResolveManifest()`         | Return manifest path (prefers local override).                  |
| `VerifyChecksums(manifest)` | Validate SHA256 entries before download.                        |
| `InstallTool(tool)`         | Install additional tool entries defined in manifest (future).   |
| `FulDX()`                   | Access wrapper for invoking FulDX commands programmatically.    |

## Safety Requirements

- Never commit `.goneat/tools.local.yaml`.
- Validate manifest against schema before installation; fail fast on malformed entries.
- When running on Windows, ensure `.exe` suffix is respected and downloaded file replaces previous version
  atomically.

## Testing

- Integration tests using local HTTP server to simulate manifest downloads.
- Checks verifying overrides work (local manifest pointing to temp FulDX binary).
- Tests covering checksum mismatch and permission errors.

## Related Documents

- `docs/architecture/fulmen-helper-library-standard.md`
- `config/taxonomy/languages.yaml`
- `.plans/active/2025.10.2/library-module-manifest-design.md`
