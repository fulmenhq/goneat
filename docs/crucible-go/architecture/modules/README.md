---
title: "Fulmen Helper Module Catalog"
description: "Architecture index for cross-language helper modules and links to detailed specifications"
author: "Schema Cartographer"
date: "2025-10-09"
last_updated: "2025-10-09"
status: "draft"
tags: ["architecture", "modules", "helper-libraries", "2025.10.2"]
---

# Fulmen Helper Module Catalog

This catalog enumerates the cross-language helper modules every Fulmen foundation library implements or
extends. Each entry links to the normative specification under `docs/standards/library/` and identifies
whether the module is considered **core** (mandatory) or **extension** (optional, opt-in per language).

| Module                | Tier      | Summary                                               | Spec                                                                 |
| --------------------- | --------- | ----------------------------------------------------- | -------------------------------------------------------------------- |
| FulDX Bootstrap       | Core      | Shared bootstrap pattern for FulDX + tooling manifest | [FulDX Bootstrap](../../standards/library/fuldx-bootstrap.md)        |
| SSOT Sync             | Core      | Crucible asset synchronization via FulDX              | [SSOT Sync](../../standards/library/ssot-sync.md)                    |
| Crucible Shim         | Core      | Language accessors for embedded Crucible assets       | [Crucible Shim](../../standards/library/crucible-shim.md)            |
| Config Path API       | Core      | Platform-aware config/data/cache path discovery       | [Config Path API](../../standards/library/config-path-api.md)        |
| Three-Layer Config    | Core      | Default → user → runtime configuration layering       | [Three-Layer Config](../../standards/library/three-layer-config.md)  |
| Schema Validation     | Core      | Schema discovery and validation helpers               | [Schema Validation](../../standards/library/schema-validation.md)    |
| Observability Logging | Core      | Structured logging alignment with Crucible standards  | [Observability Logging](../../standards/observability/logging.md)    |
| Cloud Storage         | Extension | Common blob storage helpers                           | [Cloud Storage](../../standards/library/extensions/cloud-storage.md) |
| Pathfinder            | Extension | Path discovery and helper utilities                   | [Pathfinder](../../standards/library/extensions/pathfinder.md)       |
| ASCII Helpers         | Extension | Text formatting helpers for consoles                  | [ASCII Helpers](../../standards/library/extensions/ascii-helpers.md) |

> **Note**: Observability logging remains under `docs/standards/observability/` because it spans libraries and
> applications. The module catalog includes it for completeness and to anchor cross-language expectations.

Future modules must be registered here after their standards land and the machine-readable manifest in
`config/library/` is updated.
