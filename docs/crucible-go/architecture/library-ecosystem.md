---
title: "Fulmen Library Ecosystem"
description: "Architecture overview of shared standards, data repos, and language foundations"
author: "Codex Assistant"
date: "2025-10-02"
last_updated: "2025-10-02"
status: "draft"
tags: ["architecture", "ecosystem", "libraries", "observability"]
---

# Fulmen Library Ecosystem

## Overview

FulmenHQ maintains a constellation of repositories that separate concerns between source-of-truth assets, language-specific foundations, and application/tooling consumers. This document maps those relationships so maintainers understand how to compose dependencies without creating cycles.

```
SSOT Repositories          Language Foundations            Consumers / Tools
--------------------------------------------------------------------------------
crucible  ──┐
            ├──▶ gofulmen ─────┐
cosmography ─┘                 │
                                ├──▶ Application repos (fulward, etc.)
                                └──▶ Tooling (goneat CLI)

crucible ──┐
            ├──▶ tsfulmen ─────▶ Web/Node services
cosmography ─┘
```

- **SSOT Repositories** (e.g., `crucible`, `cosmography`) contain standards, schemas, documentation, and templates.
- **Language Foundations** (`gofulmen`, `tsfulmen`, future `{lang}fulmen`) provide ergonomic APIs that wrap SSOT assets for a given language.
- **Consumers** (Fulward, goneat, services) depend on the foundation layer rather than importing SSOT repos directly.

## Repository Roles

### Language Foundations Taxonomy

The Fulmen foundation libraries follow a shared roadmap and identity schema. This table captures the
canonical language set so downstream standards stay aligned during the v2025.10.2 cycle.

| Language   | Library Name | Status  | Minimum Runtime | Notes                                                         |
| ---------- | ------------ | ------- | --------------- | ------------------------------------------------------------- |
| Go         | gofulmen     | Active  | Go 1.23+        | Reference implementation for helper standards                 |
| Python     | pyfulmen     | Active  | Python 3.12+    | Productionizing logging/context upgrades in 2025.10.2         |
| TypeScript | tsfulmen     | Active  | TypeScript 5.0+ | Ships Bun/Node-compatible bundles with ESM as default         |
| Rust       | rsfulmen     | Planned | Rust 1.70+      | Specification work begins once logging/core modules stabilize |
| C#         | csfulmen     | Planned | .NET 8.0+       | Targets ASP.NET + worker scenarios after rust foundation      |

The table replaces ad-hoc language lists in individual feature briefs. Any future additions (e.g., Java,
Ruby) must update this canonical source before new standards reference them. The machine-readable registry
for tooling lives at `config/taxonomy/languages.yaml`, and category-specific repository expectations are
documented in `docs/standards/repository-structure/README.md`.

### Crucible

- SSOT for standards, schemas, templates, documentation.
- Provides Go/TypeScript packages but intended primarily as asset store.
- Observability/logging standards live under `docs/standards/observability/`.

### Cosmography (planned reuse)

- Analogous SSOT for data modeling / topology assets.

### gofulmen

- Go foundation: exposes typed access to Crucible/other SSOT assets.
- Houses reusable libraries (logging, schemas, pathfinder, config paths, etc.).
- Implements the shared config-path API (`config.GetAppConfigDir`, etc.) defined in the Fulmen Config Path Standard.
- Should not import consumer tools (e.g., goneat) to avoid cycles.

### tsfulmen

- TypeScript foundation mirroring gofulmen capabilities.

### goneat

- CLI tooling (format, lint, security) depending on gofulmen/Crucible.
- Not imported as a library; accessed as external tool (CLI).

## Namespace Patterns

To keep imports clear:

- Reserve top-level package name (e.g., `gofulmen/pathfinder`, `gofulmen/observability/logging`).
- Nest SSOT-specific wrappers under namespace reflecting origin, e.g., `gofulmen/crucible/logging` or `gofulmen/foundations/crucible/logging`. Decision pending; see "Namespace Decision" below.
- Provide convenience re-exports for frequently used assets.

## Namespace Decision (Pending)

**Option A:** `foundation/crucible/logging`, `foundation/cosmography/*`

- Pros: groups all SSOT adapters under foundation.
- Cons: deeper import path.

**Option B:** `crucible/logging`, `cosmography/*`

- Pros: shorter imports, obvious source.
- Cons: mixing root packages for assets and non-asset utilities.

**Recommendation:** adopt Option B for clarity (`gofulmen/crucible/logging`, `gofulmen/cosmography/maps`), reserving `foundation/` for language-level utilities (e.g., `foundation/config/paths`) that aggregate cross-SSOT helpers such as the config-path API. Decision to be confirmed with gofulmen maintainers.

## Packaging Guidance

- Consumers depend on gofulmen/tsfulmen.
- Foundations pin compatible Crucible versions internally.
- Tools like goneat are adopted as external binaries via manifests (see `.goneat/tools.yaml`).

## Related Docs

- [Crucible Pseudo-Monorepo Playbook](pseudo-monorepo.md)
- [Crucible Sync Model](sync-model.md)
- [Makefile Standard](../standards/makefile-standard.md)
- [Fulmen Config Path Standard](../standards/config/fulmen-config-paths.md)
- [Fulmen Helper Library Standard](fulmen-helper-library-standard.md)
- [Fulmen Ecosystem Guide](fulmen-ecosystem-guide.md)
- [Fulmen Technical Manifesto](fulmen-technical-manifesto.md)
- [Observability Standards](../standards/observability/README.md)
