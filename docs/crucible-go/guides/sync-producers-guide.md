---
title: "Sync Producers Guide"
description: "How SSOT repositories expose assets for FulDX consumers"
author: "Schema Cartographer"
date: "2025-10-03"
last_updated: "2025-10-03"
status: "draft"
tags: ["sync", "producer", "guide"]
---

# Sync Producers Guide

SSOT repositories (Crucible, Cosmography, etc.) should publish the metadata required by downstream consumers to sync assets consistently via FulDX.

## Responsibilities

1. **Expose Assets** – Organize docs/schemas/config defaults in predictable directories.
2. **Publish Sync Keys** – Provide a machine-readable list of keys (`config/sync/sync-keys.yaml`).
3. **Document Usage** – Describe recommended keys and output locations in docs (consumer guide).
4. **Version Assets** – Use CalVer or SemVer directories (e.g., `v1.0.0`) for reproducibility.
5. **Schema Support** – Publish `sync-consumer-config.yaml` schema so consumers can validate manifests.

## Sync Keys Metadata

Example `config/sync/sync-keys.yaml`:

```yaml
version: "2025.10.0"
keys:
  - id: crucible.docs
    description: "General documentation under docs/"
    basePath: docs/
    recommendedOutput: docs/crucible
    tags: [docs]
  - id: crucible.schemas.terminal
    description: "Terminal schemas"
    basePath: schemas/terminal/
    recommendedOutput: schemas/terminal
    tags: [schemas]
  - id: crucible.lang.go
    description: "Go language wrapper assets"
    basePath: lang/go/
    recommendedOutput: lang/go
    tags: [language, lang:go]
```

### Schema Snippet for Keys

```yaml
$schema: https://json-schema.org/draft/2020-12/schema
$id: https://schemas.fulmenhq.dev/config/sync-keys-v1.0.0.json
title: Sync Keys Metadata
description: Published by SSOT repos to describe available sync keys.
type: object
required: [version, keys]
properties:
  version:
    type: string
  keys:
    type: array
    items:
      type: object
      required: [id, description, basePath]
      properties:
        id:
          type: string
        description:
          type: string
        basePath:
          type: string
        recommendedOutput:
          type: string
```

(Full schema will live under `schemas/config/sync-keys.yaml`).

## Publishing Checklist

- [ ] Update `config/README.md` with new defaults if applicable.
- [ ] Add entries to `config/sync/sync-keys.yaml` for new asset families.
- [ ] Mention new keys in consumer docs (`sync-consumers-guide.md`) and include tags/metadata to aid filtering.
- [ ] Validate keys file against schema (future CI).
- [ ] Coordinate with FulDX team for command updates.

## Future Extensions

- Locale-specific keys (`crucible.docs.locale.en`, etc.).
- Support for push/sync workflows (beyond read-only consumers).
- API endpoints for dynamic discovery (e.g., `fuldx sync profiles` fetching from HTTP).

## See Also

- [Sync Consumers Guide](sync-consumers-guide.md)
- [FulDX Bootstrap Plan](../../fuldx/.plans/bootstrap.md)
