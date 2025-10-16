---
title: "Crucible Shim Standard"
description: "Helper module contract for exposing embedded Crucible assets in Fulmen libraries"
author: "Schema Cartographer"
date: "2025-10-09"
last_updated: "2025-10-09"
status: "draft"
tags: ["standards", "library", "crucible", "assets", "2025.10.2"]
---

# Crucible Shim Standard

## Purpose

Provide a stable interface for consuming projects to access Crucible documentation, schemas, templates, and
metadata embedded within helper libraries. Eliminates the need for consumers to know the on-disk layout or build
process used inside Crucible.

## Capabilities

1. Expose version metadata (`CrucibleVersion`, `CrucibleCommit`, `CrucibleSyncDate`).
2. Enumerate available asset categories (`docs`, `schemas`, `config`, `templates`).
3. Provide typed accessors for common asset types (e.g., fetch schema as JSON, fetch markdown doc).
4. Offer streaming APIs for large assets to avoid loading entire file into memory.
5. Deliver change detection utilities (checksum, last modified) to help consumers detect updates.

## API Outline

| Function / Property             | Description                                              |
| ------------------------------- | -------------------------------------------------------- |
| `CrucibleVersion()`             | Returns semantic/CalVer string from `VERSION`.           |
| `ListAssets(category, prefix?)` | Returns metadata array `{ id, path, checksum }`.         |
| `ReadAsset(id)`                 | Returns asset contents as bytes/string.                  |
| `OpenAsset(id)`                 | Stream interface (reader/async iterator).                |
| `FindSchema(id)`                | Convenience wrapper returning parsed schema + metadata.  |
| `FindDoc(id)`                   | Returns markdown content + frontmatter as struct/object. |

Helper libraries MUST ensure asset IDs remain stable between releases (`docs/standards/...` path-based).

## Implementation Notes

- **Go**: Embed assets via `//go:embed`. Provide packages `crucible/docs`, `crucible/schemas`. Maintain index
  generated during `bun run sync:to-lang` for O(1) lookup.
- **TypeScript**: Use generated `schemas.ts`/`docs.ts` modules under `lang/typescript/`. Export helper functions
  that resolve asset IDs to strings or Buffers.
- **Python**: Utilize `importlib.resources.files("pyfulmen.crucible")` to access embedded assets packaged with
  the distribution. Provide wrappers returning `str` or `bytes`.

## Error Handling

- Throw/return `ErrAssetNotFound` (Go) or equivalent exceptions (`AssetNotFoundError` in Python/TypeScript) when
  the requested ID is missing.
- Include available categories and suggestions in error messages (`Did you mean …`).

## Testing Requirements

- Unit tests verifying asset enumeration matches manifest generated during sync.
- Tests ensuring version metadata reflects `VERSION` file and sync metadata.
- Integration tests confirming schema/doc helpers return expected content for sample IDs.

## Related Documents

- `scripts/sync-to-lang.ts`
- `docs/architecture/pseudo-monorepo.md`
- `docs/architecture/sync-model.md`
