---
title: "Assets Management Architecture"
description: "Design for curated, embedded, and cached validation assets"
author: "@arch-eagle"
date: "2025-09-05"
last_updated: "2025-09-05"
status: "approved"
---

# Assets Management Architecture

## Goals

- Deterministic, offline validation for critical schemas and specs
- Minimal maintenance overhead with clear provenance and update paths
- Extensible to additional validators (OpenAPI, AsyncAPI, Protobuf, K8s)

## Strategy

1. Curated core assets in-repo (embedded)
   - JSON Schema meta-schemas (Draft-07, Draft 2020-12)
   - Templates directory (hooks/bash/\*.tmpl)
   - Schemas directory (config/_.yaml, output/_.yaml, work/\*.yaml)
   - Stored under `internal/assets/embedded_*` subdirectories and embedded via `go:embed`
   - Accessed via `GetTemplatesFS()`, `GetSchemasFS()`, and `GetJSONSchemaMeta()`
   - Used by default for offline meta-validation and template rendering

2. Optional local cache (opt-in)
   - Path: `~/.goneat/cache/schemas`
   - Used when explicit flags/config allow remote resolution for external catalogs

3. Sync tooling (not required for builds)
   - `scripts/sync-schemas.sh` fetches/pins curated assets
   - `make sync-schemas` runs the script

## Determinism & Security

- Embedded assets ensure reproducible results in CI/CD and developer machines
- Remote refs disabled by default; enabling requires explicit flags/config
- Cache guarded by timeouts and checksums; never required for default flows

## Extensibility

- Additional assets follow the same pattern: curate → embed → optional cache for large catalogs
- The embedded registry (`internal/assets/registry.go`) enumerates available assets for discovery/debugging
