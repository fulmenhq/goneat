---
title: "Docs Embed Manifest"
description: "Curating embedded documentation via YAML manifest validated by JSON Schema"
author: "@3leapsdave"
date: "2025-09-08"
last_updated: "2025-09-08"
status: "approved"
tags: ["configuration", "docs", "embed"]
---

# Docs Embed Manifest

Goneat embeds a curated set of Markdown docs for offline use. The selection is defined in `docs/embed-manifest.yaml`, validated against `schemas/content/docs-embed-manifest-v1.0.0.json` (JSON Schema 2020‑12).

## Manifest Structure

```yaml
version: "1.0.0"
topics:
  <topic-key>:
    tags: [tag1, tag2]
    include:
      - path/pattern/*.md
      - path/pattern/**/*.md
    exclude:
      - path/to/skip/**
```

Notes:
- Paths are relative to the `docs/` directory.
- Only Markdown (`.md`) files are considered for embedding.
- `**` patterns select recursively; `*` selects direct children.
- Tags are optional, single-token (no spaces).

## Workflow

1. Edit `docs/embed-manifest.yaml` to add or adjust content.
2. Build and embed mirrors: `make build` (prefers CLI‑driven embedding when available).
3. Verify mirrors are drift‑free in CI (`verify-embeds`).
4. Commit SSOT changes and embedded mirrors together.

## CLI Support

- `goneat content find --manifest docs/embed-manifest.yaml --root docs --json`
- `goneat content embed --manifest docs/embed-manifest.yaml --root docs --target internal/assets/embedded_docs/docs`
- `goneat content verify --manifest docs/embed-manifest.yaml --root docs --target internal/assets/embedded_docs/docs`
- `goneat docs list` and `goneat docs show` provide read‑only access at runtime.
