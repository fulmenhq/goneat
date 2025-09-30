---
title: "Content Command Reference"
description: "Curate and embed selected assets (docs, schemas, examples, etc.) for offline use in the goneat binary"
author: "@code-scout"
date: "2025-09-08"
last_updated: "2025-09-29"
status: "approved"
tags: ["cli", "docs", "curation", "embedding"]
---

# Content Command (Curation)

The `content` command manages curated documentation selection and embedding. It is intended for maintainers and CI; end‑users should use `goneat docs` to view content.

## Subcommands

### find

Resolve curated assets from one or more manifests. The command auto-discovers manifests under the configured root (default `docs`) and emits either a pretty table or JSON report.

```
goneat content find --format pretty                  # docs manifest only
goneat content find --all-manifests --format json    # aggregate everything
goneat content find --manifest schemas/embed-manifest.yaml --root schemas \
  --asset-type schemas --format json
```

### embed

Sync the resolved set into one or more embedded mirrors. Targets default to the asset preset (`docs`, `schemas`, `examples`, `assets`) but can be overridden per manifest or via `--target`.

```
goneat content embed                              # embed docs manifest (default target)
goneat content embed --all-manifests               # embed every discovered manifest
goneat content embed --manifest schemas/embed-manifest.yaml --root schemas \
  --target internal/assets/embedded_schemas
goneat content embed --dry-run --format json       # preview without writing files
```

### init

Interactively (or via flags) scaffold a new `v1.1.0` manifest using the built-in asset presets.

```
goneat content init --asset-type schemas --root schemas --format json
goneat content init --asset-type docs --output docs/embed-manifest.yaml --overwrite
```

### verify

Verify that the embedded mirrors match the manifest selections (presence, byte-identical, no extras). Returns non-zero on drift and surfaces per-target diagnostics.

```
goneat content verify --format json                    # verify docs manifest
goneat content verify --all-manifests --format json    # verify every manifest/target
```

### conflicts

Surface conflicts and overrides detected while resolving manifests. Use `--all-manifests` to evaluate every discovered manifest.

```
goneat content conflicts --format pretty
goneat content conflicts --format json --all-manifests
```

### manifests

List discovered manifests (respecting the same discovery rules as `find`, `embed`, and `verify`). Helpful for auditing precedence and ensuring the resolved asset types match expectations.

```
goneat content manifests --format pretty            # human-readable summary
goneat content manifests --format json              # machine readable output
goneat content manifests --all-manifests            # include discovered siblings
```

### migrate-manifest

Upgrade a legacy `v1.0.0` manifest to the asset-aware `v1.1.0` format. By default the upgraded manifest overwrites the input file; pass `--output` to write elsewhere.

```
goneat content migrate-manifest --manifest docs/embed-manifest.yaml
goneat content migrate-manifest --manifest docs/embed-manifest.yaml \
  --output docs/embed-manifest-v1.1.0.yaml
```

## Manifest

- **v1.0.0 (legacy)** – docs-only manifests with `include`/`exclude` globs. Still supported for backwards compatibility.
- **v1.1.0 (preferred)** – asset-agnostic manifest with optional `asset_type`, `content_types`, `exclude_patterns`, per-topic overrides, and custom targets. Schema: `schemas/content/v1.1.0/embed-manifest.yaml`.

Multiple manifests can coexist (e.g., `docs/embed-manifest.yaml`, `schemas/embed-manifest.yaml`). Precedence is:

1. `--manifest` flag (most specific)
2. Manifest alongside the requested root (e.g., `<root>/embed-manifest.yaml`)
3. Project-level manifests (`./embed-manifest.yaml`, `.goneat/embed-manifest.yaml`)
4. Auto-discovery (additional `embed-manifest.yaml` files under the root)

Use `goneat content manifests` to inspect the current precedence order before embedding.

Topics honour `include`/`exclude` patterns and can override the manifest asset type, content types, target directory, and conflict behaviour via `override: true`. Diagnostics captured during resolution are surfaced in the JSON output for `find`, `verify`, `manifests`, `conflicts`, and `embed` (when `--format json`), making automation and reporting easier.

## Related

- `goneat docs` — Read-only access to embedded docs for users
- `docs/sop/embedding-assets-sop.md` — SOP for embedding assets (including curated docs)
- `docs/appnotes/lib/content-embed.md` — Best practices for manifest layout, validation, and CI integration
