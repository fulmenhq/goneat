---
title: "AppNote: Content Embed Strategies"
description: "Best practices for configuring multi-asset manifests with goneat"
author: "@arch-eagle"
date: "2025-09-30"
last_updated: "2025-09-30"
status: "draft"
---

# AppNote: Content Embed Strategies

`goneat content` curates assets (docs, schemas, examples, etc.) into the binary so downstream tools and installations have consistent offline resources. This note collects design guidelines, common manifest patterns, and operational tips that extend the command reference.

## Quick Start Checklist

1. **Start with `goneat content init`** – Scaffold a version `1.1.0` manifest using asset presets (`docs`, `schemas`, `examples`, `assets`).
2. **Document ownership** – Co-locate manifests with the assets they manage (`docs/embed-manifest.yaml`, `schemas/embed-manifest.yaml`, …).
3. **Validate locally** – Run `goneat content manifests --validate` and `goneat content conflicts` before embedding in CI.
4. **Embed from CI** – Use `goneat content embed --all-manifests` during release builds; prefer `--dry-run` in PR checks.
5. **Verify drift** – Add `goneat content verify --all-manifests --format json` to nightly jobs to detect stale mirrors.

## Manifest Organization

### Single Manifest

Pros: easy onboarding, one file to review. Cons: quickly grows noisy for mixed assets.

```yaml
version: "1.1.0"
asset_type: "docs"
target: internal/assets/embedded_docs/docs

topics:
  docs:
    include: ["**/*.md"]
  schemas:
    asset_type: "schemas"
    include: ["../schemas/**/*.json", "../schemas/**/*.yaml"]
  examples:
    asset_type: "examples"
    include: ["../examples/**/*"]
```

### Multiple Manifests

Pros: clearer ownership, targeted CI runs, easier diff reviews.

```
docs/embed-manifest.yaml
schemas/embed-manifest.yaml
examples/embed-manifest.yaml
```

When multiple manifests exist, precedence is:

1. `--manifest` flag
2. Manifest adjacent to `--root`
3. Project-level (`./embed-manifest.yaml`, `.goneat/embed-manifest.yaml`)
4. Auto-discovery under the root

Use `goneat content manifests` to inspect the resolved order.

## Asset Presets & Overrides

| Preset     | Patterns                                              | Default Target                       |
| ---------- | ----------------------------------------------------- | ------------------------------------ |
| `docs`     | `**/*.md`, `**/*.markdown`, `**/*.txt`                | `internal/assets/embedded_docs/docs` |
| `schemas`  | `**/*.json`, `**/*.yaml`, `**/*.yml`, `**/*.schema`   | `internal/assets/embedded_schemas`   |
| `examples` | `**/examples/**/*`, `**/*.example.*`, `**/*.sample.*` | `internal/assets/embedded_examples`  |
| `assets`   | `**/*` (excludes `.git`, `node_modules`)              | `internal/assets/embedded_assets`    |

Topic-level overrides allow mixing presets within a single manifest. Set `override: true` to let a topic claim files already owned by another manifest.

## CLI Workflow Patterns

### Scaffolding

```bash
goneat content init --asset-type schemas --root schemas --overwrite
```

Interactive mode (omit `--asset-type`) prompts for presets and patterns.

### Validation & Conflict Detection

```bash
goneat content manifests --all-manifests --validate --format json
goneat content conflicts --all-manifests --format pretty
```

Diagnostics from these commands surface in JSON outputs (`find`, `embed`, `verify`, `manifests`, `conflicts`), making it straightforward to wire them into CI dashboards.

### Embedding & Verification

```bash
goneat content embed --all-manifests --dry-run --format json   # CI preview
goneat content embed --all-manifests                           # Release build
goneat content verify --all-manifests --format json            # Drift detection
```

Combine with `make sync-schemas` + `make embed-assets` when upstream meta-schemas change.

## CI/CD Integration Tips

- **Dry-run in PRs** – Fail builds if the dry-run output differs from committed mirrors (compare JSON payloads).
- **Embed on release artifacts** – Run the real embed step only on trusted build stages to avoid unnecessary repository churn.
- **Verify nightly** – Scheduled `goneat content verify --json` jobs catch accidental edits or missing assets early.

## Troubleshooting

- `❌ Drift detected` – Check the JSON report for `missing`, `changed`, `extra` entries. Run `goneat content embed` locally to reconcile.
- `conflict` diagnostics – Another manifest already owns a file. Either enable `override: true` or adjust include patterns to eliminate overlap.
- `manifest outside repository` – Ensure manifests reside under the repo root; absolute paths are normalized via `normalizeManifestPath`.

## Further Reading

- [Content command reference](../../user-guide/commands/content.md)
- [Embedding assets SOP](../..//sop/embedding-assets-sop.md)
- [Schema library AppNote](schema.md) – for advanced validation workflows

---

Maintainers: update this AppNote whenever preset defaults change or new CLI flags are introduced so that offline readers receive the latest guidance.
