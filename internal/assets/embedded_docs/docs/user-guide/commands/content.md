---
title: "Content Command Reference"
description: "Curate and embed selected documentation for offline use in the goneat binary"
author: "@code-scout"
date: "2025-09-08"
last_updated: "2025-09-08"
status: "approved"
tags: ["cli", "docs", "curation", "embedding"]
---

# Content Command (Curation)

The `content` command manages curated documentation selection and embedding. It is intended for maintainers and CI; end‑users should use `goneat docs` to view content.

## Subcommands

### find

Resolve a curated set of docs from a YAML manifest (validated by JSON Schema). Outputs either a pretty table or a JSON report.

```
goneat content find --manifest docs/embed-manifest.yaml --root docs --format pretty
goneat content find --manifest docs/embed-manifest.yaml --root docs --format json
```

JSON schema: `schemas/output/content-find-report-v1.0.0.json`

### embed

Sync the resolved set into the embedded mirror. This mirror is tracked and embedded into the binary so `go install` users have docs offline.

```
goneat content embed --manifest docs/embed-manifest.yaml --root docs \
  --target internal/assets/embedded_docs/docs
```

### verify

Verify the embedded mirror matches the manifest selection (presence, byte‑identical, no extras). Returns non‑zero on drift.

```
goneat content verify --manifest docs/embed-manifest.yaml --root docs \
  --target internal/assets/embedded_docs/docs --format json
```

## Manifest

The manifest (`docs/embed-manifest.yaml`) defines topics with include/exclude globs (supports `**`). See `docs/configuration/embed-manifest.md` for structure and examples.

## Related

- `goneat docs` — Read-only access to embedded docs for users
- `docs/sop/embedding-assets-sop.md` — SOP for embedding assets (including curated docs)
