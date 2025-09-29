---
title: "Pathfinder Command Reference"
description: "Reference guide for the goneat pathfinder command, covering discovery workflows, transforms, and streaming output"
author: "@arch-eagle"
date: "2025-09-24"
last_updated: "2025-09-24"
status: "draft"
tags:
  [
    "cli",
    "pathfinder",
    "discovery",
    "facade",
    "streaming",
    "transforms",
  ]
category: "user-guide"
---

# Pathfinder Command Reference

The `goneat pathfinder` command exposes the finder facade from `pkg/pathfinder` so teams can perform consistent discovery without writing Go code. It powers Sumpter's XML crawlers, ad-hoc directory inspections, and the upcoming cloud-loader workflows with the same safety guarantees as the library.

## Command Structure

```bash
goneat pathfinder find [flags]
```

The top-level `pathfinder` command currently offers the `find` subcommand and will gain additional helpers (transfer planning, auditing) in later releases.

## Highlights

- **Pattern-based discovery** with doublestar (`**/*.xml`) include/exclude filters
- **Logical-path transforms** (flatten, strip prefix, prepend prefix) matching the Go facade
- **Schema-aware discovery** with canonical signatures for JSON Schema, OpenAPI, AsyncAPI, Avro, Cue, etc.
- **Streaming text output** for large directories and future cloud buckets
- **Guardian-aware** enforcement via the underlying `PathFinder`
- **Consistent JSON output** for scripting and automation

## Quick Start

```bash
# List XML assets relative to the current repo
 goneat pathfinder find \
   --path ./data \
   --include "**/*.xml" \
   --output text

# Get JSON output for scripting
 goneat pathfinder find \
   --path ./downloads \
   --include "**/*.csv" \
   --output json > csv_index.json
```

## Flags

| Flag | Description |
|------|-------------|
| `--path` | Root directory or loader source to search (default `.` for local loader). |
| `--include` | One or more glob patterns to include (doublestar syntax). |
| `--exclude` | Patterns to exclude from the result set. |
| `--skip-dir` | Substrings; matching directories are skipped entirely. |
| `--max-depth` | Maximum traversal depth (`-1` for unlimited). Depth counts directory segments beneath the root. |
| `--follow-symlinks` | Follow symbolic links (default skips symlinks for safety). |
| `--workers` | Worker hint for future parallel traversal (0 uses the facade default). |
| `--stream` | Stream results as they are discovered (text output emits progressively; JSON currently buffers). |
| `--output` | Output format: `json` (default) or `text`. |
| `--show-source` | With `--output text`, append the underlying source path (`logical -> source`). |
| `--strip-prefix` | Remove a leading prefix from logical paths (useful for flattening archives). |
| `--logical-prefix` | Prepend a prefix to logical paths (e.g., target bucket or tenant). |
| `--flatten` | Set the logical path to the base filename, ignoring directories. Overrides `--strip-prefix`. |
| `--loader` | Loader type (`local`, `s3`, `r2`, `gcs`, etc.). v0.2.9 ships `local`; cloud loaders arrive in v0.2.10. |
| `--schemas` | Enable schema signature mode (filters results to recognised schemas). |
| `--schema-id` | Restrict schema discovery to specific signature IDs or aliases. |
| `--schema-category` | Restrict schema discovery to categories (e.g., `json-schema`, `openapi`, `avro`). |
| `--schema-metadata` | Include full signature metadata (match diagnostics, docs links). Enabled automatically for JSON output. |

## Schema Discovery Mode

The signature manifest lives at `schemas/signatures/v1.0.0/schema-signatures.yaml` and ships inside the binary. It recognises JSON Schema drafts (04, 06, 07, 2019-09, 2020-12), OpenAPI 3.x, AsyncAPI 2.x, Avro, Cue modules, Protobuf schemas, and more.

Teams can extend or override signatures by adding YAML manifests under:

1. `$GONEAT_HOME/config/signatures.yaml`
2. `$GONEAT_HOME/signatures/*.yaml`

Files are merged in that order (last definition wins) so local packs can override embedded defaults without rebuilding goneat.

```bash
# Inventory every recognised schema with metadata in JSON form
goneat pathfinder find --path ./schemas --schemas --output json

# Restrict to JSON Schema drafts and emit annotated text output
goneat pathfinder find --path ./schemas --schemas \
  --schema-category json-schema --output text

# Pipe schema candidates to validation
goneat pathfinder find --schemas --schema-id json-schema-draft-07 \
  | xargs -r goneat schema validate-schema --schema-id json-schema-draft-07
```

When schema mode is enabled, `PathResult.metadata.schema` includes the detected signature id, category, score, source (embedded or override), and optional docs links. Text output annotates the logical path (`schema.json [schema:json-schema-draft-07]`), while JSON output carries the full metadata for automation.

## Transform Recipes

Logical-path rewrites mirror the Go `PathTransform` callbacks:

```bash
# Drop the leading "stage/" segment and prepend a tenant prefix
goneat pathfinder find \
  --path ./stage/tenant-42 \
  --include "**/*.ndjson" \
  --strip-prefix "stage/tenant-42" \
  --logical-prefix "tenants/42" \
  --output text

# Flatten nested assets for copy jobs
goneat pathfinder find \
  --path ./assets \
  --include "**/*.png" \
  --flatten \
  --output text
```

## Streaming Output

```bash
# Stream logical -> source pairs so downstream tooling can react immediately
goneat pathfinder find \
  --path ./logs \
  --include "**/*.log" \
  --output text \
  --show-source \
  --stream
```

- Text streaming writes each line as soon as the finder emits it.
- JSON streaming will buffer until the initial facade streaming work lands (planned alongside v0.2.10 cloud pagination).
- Cancel the command with Ctrl+C when you have collected enough entries; the facade respects context cancellation.

## Guardian & Safety

The CLI inherits guardian protections from the library:

- Repository and workspace constraints remain enforced; attempting to traverse outside the allowed roots produces guarded errors.
- Symlinks are skipped by default. Pass `--follow-symlinks` only when policy allows it and you trust the target tree.
- Audit logging (when enabled in the binary) records discovery operations (`OpDiscover`, `OpDenied`).

## JSON Output Schema

Each entry matches the `pathfinder.PathResult` structure:

```json
{
  "relative_path": "data/nested/report.xml",
  "source_path": "./data/nested/report.xml",
  "logical_path": "data/nested/report.xml",
  "loader_type": "local",
  "metadata": null
}
```

Forthcoming cloud loaders will populate `metadata` with provider-specific fields (ETag, generation, storage class).

## Integration Tips

- Combine `--schemas --output json` with `jq` for schema inventory reports, or `--output json` for general discovery.
- Pair with `goneat format` or copy jobs by reusing the logical path decisions, keeping flatten/strip rules synchronized.
- For programmatic use, call the facade directly—see [`docs/appnotes/lib/pathfinder/finder_facade.md`](../../../appnotes/lib/pathfinder/finder_facade.md).

## Roadmap

| Release | Planned Enhancements |
|---------|----------------------|
| v0.2.9  | Local loader, transforms, streaming text output *(delivered)* |
| v0.2.10 | S3/R2/GCS loaders, credential selection, pagination-aware streaming |
| v0.3.x  | Transfer planning (`copy`, `mirror`), audit reporting, cache controls |

Stay tuned to guardian release notes for expanded enforcement messaging that will surface directly in CLI output.
