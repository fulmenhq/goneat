---
title: "docs command"
description: "Read-only access to a curated set of embedded documentation"
author: "@3leapsdave"
date: "2025-09-08"
last_updated: "2025-09-08"
status: "approved"
tags: ["cli", "docs", "offline"]
---

# docs (read-only)

The `docs` command provides offline access to a curated set of documentation embedded in the goneat binary. Use it to list and show topics even when the repository is not available.

## Usage

- List available docs (JSON-first):

```
goneat docs list --json
```

- Show a document by slug (JSON or markdown):

```
goneat docs show user-guide/commands/format --format json
goneat docs show user-guide/commands/format --format markdown
```

## Slugs

Slugs are derived from the relative path under `docs/` without the `.md` extension. For example:

```
docs/user-guide/commands/format.md → user-guide/commands/format
```

## Curation

The embedded corpus is curated via a manifest (`docs/embed-manifest.yaml`). Maintainers can propose additions by updating the manifest in a PR. The binary embeds a mirror of the curated files so `go install` users have them available.

## Output

- `list --json` emits an array of items with fields: `slug`, `path`, `title`, `description`, `size`, `tags` (when available).
- `show --format json` emits `{slug, path, title, description, tags, content}`.

