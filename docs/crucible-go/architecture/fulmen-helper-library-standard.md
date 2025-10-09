---
title: "Fulmen Helper Library Standard"
description: "Standard structure and capabilities for gofulmen, tsfulmen, and future language helpers"
author: "Schema Cartographer"
date: "2025-10-02"
last_updated: "2025-10-07"
status: "draft"
tags: ["architecture", "helper-library", "multi-language", "local-development"]
---

# Fulmen Helper Library Standard

This document formalizes the expectations outlined in the gofulmen proposal so that every `*fulmen`
language foundation delivers the same core capabilities. The canonical list of languages and statuses
is maintained in the [Library Ecosystem taxonomy](library-ecosystem.md#language-foundations-taxonomy);
consult that table before proposing new foundations or changing lifecycle state.

## Scope

Applies to language-specific Fulmen helper libraries (gofulmen, tsfulmen, pyfulmen, csfulmen, rufulmen, etc.). Excludes SSOT repos (Crucible, Cosmography) and application/tool repos (Fulward, goneat).

## Mandatory Capabilities

1. **FulDX Bootstrap Pattern**
   - Include `.goneat/tools.yaml` with platform-specific FulDX installation (URLs, checksums).
   - Include `.goneat/tools.local.yaml.example` template for local development overrides.
   - Add `.goneat/tools.local.yaml` to `.gitignore` (never commit local paths).
   - Implement minimal language-specific bootstrap script that:
     - Prefers `.goneat/tools.local.yaml` if present (local dev iteration).
     - Falls back to `.goneat/tools.yaml` (CI/CD, production).
     - Installs FulDX to `./bin/fuldx`.
   - Let FulDX handle other tool installations (goneat, etc.) via `fuldx tools install`.
   - Provide `make bootstrap` target that runs bootstrap script.
   - Refer to the [FulDX Bootstrap Standard](../standards/library/fuldx-bootstrap.md) for normative contract.

2. **SSOT Synchronization**
   - Include `.fuldx/sync-consumer.yaml` configuration for Crucible asset sync. Example manifest:
     ```yaml
     version: "2025.10.0"
     sources:
       - name: crucible
         id: crucible.lang.<lang>
         sourceRepo: https://github.com/fulmenhq/crucible.git
         sourceRef: main
         # No localPath here - keep main config clean for CI/CD
         include:
           - docs/**/*.md
           - schemas/**/*
           - config/**/*.yaml
           - schemas/config/sync-consumer-config.yaml
           - config/sync/sync-keys.yaml
         output: .crucible
         notes: "Use language-specific keys (see config/sync/sync-keys.yaml) or add additional entries for fine-grained control."
     ```
   - Create `.fuldx/sync-consumer.local.yaml` for local development (gitignored):
     ```yaml
     sources:
       - name: crucible
         localPath: ../crucible # Local development override
     ```
   - Add to `.gitignore`:
     ```
     .fuldx/sync-consumer.local.yaml
     .fuldx/*.local.yaml
     ```
   - **Local Development Priority**: FulDX uses layered config (local overrides > env vars > main config > conventions)
   - Commit synced assets to version control (docs/crucible-<lang>, schemas/crucible-<lang>, config/crucible-<lang>, metadata) for offline availability.
   - Provide `make sync` target that runs `fuldx ssot sync`.
   - Use glob patterns such as `schemas/**/*` to capture both `.json` and `.yaml` schemas.
   - Refer to the [SSOT Sync Standard](../standards/library/ssot-sync.md) for command surface and testing guidance.

3. **Crucible Shim**
   - Provide idiomatic access to Crucible assets (docs, schemas, config defaults).
   - Re-export version constants so consumers can log/report underlying Crucible snapshot.
   - Discover available categories (`ListAvailableDocs()`, `ListAvailableSchemas()`) via embedded metadata or generated index.
   - Refer to the [Crucible Shim Standard](../standards/library/crucible-shim.md).

4. **Config Path API**
   - Implement `GetAppConfigDir`, `GetAppDataDir`, `GetAppCacheDir`, `GetAppConfigPaths`, and `GetXDGBaseDirs` (naming per language).
   - Expose Fulmen-specific helpers (`GetFulmenConfigDir`, etc.) aligned with [Fulmen Config Path Standard](../standards/config/fulmen-config-paths.md).
   - Respect platform defaults (Linux/macOS/Windows) and environment overrides.
   - Refer to the [Config Path API Standard](../standards/library/config-path-api.md).

5. **Three-Layer Config Loading**
   - Layer 1: Embed Crucible defaults from `config/{category}/vX.Y.Z/*-defaults.yaml`.
   - Layer 2: Merge user overrides from `GetFulmenConfigDir()`.
   - Layer 3: Allow application-provided config (BYOC) with explicit API hooks.
   - Refer to the [Three-Layer Configuration Standard](../standards/library/three-layer-config.md).

6. **Schema Validation Utilities**
   - Provide helpers to load, parse, and validate schemas shipped in Crucible.
   - Optional but recommended: integrate with language-native validation libraries.
   - Refer to the [Schema Validation Helper Standard](../standards/library/schema-validation.md).

7. **Observability Integration**
   - Consume logging schemas/defaults from `config/observability/logging/`.
   - Map shared severity enum and throttling settings to language-specific logging implementation.
   - Refer to the [Fulmen Logging Standard](../standards/observability/logging.md).

## Optional (Recommended) Capabilities

- Pathfinder & ASCII helpers (for languages that can support them).
- Cosmography shims once that SSOT expands.
- Registry API clients if SSOT repos expose HTTP endpoints in the future.

Module requirement levels, coverage targets, and language overrides are tracked in
`config/library/v1.0.0/module-manifest.yaml` (validated by
`schemas/library/module-manifest/v1.0.0/module-manifest.schema.json`).

## Directory Structure

### Required Directories

```
<foundation-repo>/
├── .crucible/
│   ├── tools.yaml                    # Production tool definitions (FulDX + checksums)
│   ├── tools.local.yaml.example      # Local override template (committed)
│   └── metadata/                     # Sync metadata (from fuldx ssot sync)
├── .fuldx/
│   └── sync-consumer.yaml            # SSOT sync configuration
├── docs/
│   └── crucible-<lang>/              # Synced docs (committed, regenerated via sync)
├── schemas/
│   └── crucible-<lang>/              # Synced schemas (committed, regenerated via sync)
├── config/
│   └── crucible-<lang>/              # Synced config defaults (committed, regenerated)
└── bin/                              # Installed tools (gitignored)
    ├── fuldx
    └── goneat
```

### Namespace Guidance

- SSOT-specific helpers should live under namespaces such as `crucible/logging`, `crucible/terminal`, `cosmography/maps`.
- Language-only utilities (e.g., Go reflection helpers) may live under `foundation/` namespaces.
- Provide package-level READMEs describing available modules.

## Bootstrap Strategy

### The Problem: Bootstrap the Bootstrap

Foundation libraries face a unique challenge:

- They're dependencies for sophisticated tools (goneat, fulward)
- They need DX tooling themselves (fuldx for version management, SSOT sync)
- They cannot create circular dependencies
- They must work in CI/CD without manual installation steps

### The Solution: Minimal FulDX Bootstrap + Synced Assets

1. **Commit synced assets** - Docs, schemas, configs from Crucible are committed and regenerated via `make sync`
2. **Single bootstrap entry** - `.goneat/tools.yaml` contains FulDX with platform-specific URLs/checksums
3. **Local override pattern** - `.goneat/tools.local.yaml` (gitignored) for development iteration
4. **Let FulDX handle complexity** - Use `fuldx tools install` for goneat and other sophisticated tools

### Workflows

**CI/CD (Production):**

```bash
git clone <foundation-repo>  # Includes synced assets
make bootstrap               # Installs fuldx from tools.yaml
make sync                    # Optional: update from Crucible
make test
```

**Local Development (Iteration):**

```bash
# Copy local override template
cp .goneat/tools.local.yaml.example .goneat/tools.local.yaml

# Edit to point to local fuldx build
# source: /path/to/fuldx/dist/fuldx

# Bootstrap uses local override
make bootstrap
```

### Safety: Preventing Local Path Leaks

- Add `.goneat/tools.local.yaml` to `.gitignore`
- Implement precommit hook to validate no local paths in `tools.yaml`
- Provide `tools.local.yaml.example` as template

## Testing Expectations

- Each language foundation owns its unit/integration tests.
- Crucible supplies schemas and config defaults but does not ship tests for the shims.
- Tests should cover:
  - Config path resolution (including legacy fallbacks).
  - Embedding/parsing of Crucible defaults.
  - Schema validation wrappers.
  - Logging severity/middleware mapping.
  - Bootstrap script functionality (both local and production paths).

## Documentation Requirements

- README with installation, quick start, and links to Crucible standards.
- FulDX integration guide (e.g., `docs/FULDX.md`).
- Bootstrap strategy documentation (e.g., `docs/BOOTSTRAP-STRATEGY.md`).
- API reference comments/docstrings per language norms.
- Notes on dependency flow (SSOT → foundation → consumer) to prevent circular imports.

## Version Alignment

- Foundations MUST pin the Crucible version they embed and expose it publicly.
- Consumers depend on the foundation version; no direct Crucible import required.
- When Crucible publishes new assets, foundations should sync and bump versions in tandem.
- Use FulDX for version management: `fuldx version bump --type <patch|minor|major|calver>`

## Makefile Targets (Required)

In addition to the standard targets, support overriding bootstrap with `FORCE=1` or provide a `bootstrap-force` alias so tool reinstall can be forced when iterating on local builds.

All foundation libraries MUST provide these targets:

```makefile
bootstrap:  # Install fuldx and other tools
sync:       # Sync assets from Crucible SSOT
version-bump: # Bump version (requires TYPE parameter)
test:       # Run all tests
fmt:        # Format code
lint:       # Lint/style checks
```

## Common Pitfalls & Solutions

### Pitfall 1: Syncing Only JSON Schemas

- **Issue**: Using `schemas/**/*.json` misses `.yaml` schemas.
- **Solution**: Use `schemas/**/*` or `schemas/**/*.{json,yaml}`.

### Pitfall 2: Outdated FulDX Version

- **Issue**: Old FulDX versions skip YAML schemas.
- **Solution**: Upgrade to FulDX v0.1.2+ and re-run sync.

### Pitfall 3: Local Overrides Committed

- **Issue**: `.goneat/tools.local.yaml` accidentally committed.
- **Solution**: Provide `.goneat/tools.local.yaml.example`, add `.goneat/tools.local.yaml` to `.gitignore`, enforce pre-commit checks.

### Pitfall 4: Circular Bootstrap Dependencies

- **Issue**: Using goneat during bootstrap creates cycles.
- **Solution**: Bootstrap installs FulDX only; FulDX manages other tools.

### Pitfall 5: Missing Maintainer Docs

- **Issue**: Ops/ADR info lost in commits.
- **Solution**: Maintain `ops/` directory with ADRs, runbooks, bootstrap strategy.

## References

- [Fulmen Library Ecosystem](library-ecosystem.md)
- [Fulmen Config Path Standard](../standards/config/fulmen-config-paths.md)
- [Config Defaults README](../../config/README.md)
- [FulDX Repository](https://github.com/fulmenhq/fuldx)
- `.plans/crucible/fulmen-helper-library-specification.md` (original proposal)

## Implementation Examples

- **gofulmen** - Reference implementation with Go-based bootstrap
- **tsfulmen** - TypeScript implementation (in development)

## Changelog

- **2025-10-07** - Added FulDX bootstrap pattern, SSOT sync requirements, directory structure
- **2025-10-03** - Initial draft
