# Meta-Schema Cache

This directory stores the curated JSON Schema meta-schemas that goneat needs for
offline validation. Directory names mirror the signature tags defined in
`schemas/signatures/v1.0.0/schema-signatures.yaml` (e.g., `draft-07`,
`draft-2020-12`) so downstream automation can reason about available drafts.

## Contents

- `draft-07/` – canonical Draft-07 meta-schema (`schema.json`).
- `draft-2020-12/` – canonical Draft 2020-12 meta-schema plus offline helpers.
  - `schema.json` – raw meta-schema fetched from json-schema.org.
  - `offline.schema.json` – reduced subset used when
    `GONEAT_OFFLINE_SCHEMA_VALIDATION=true` to avoid remote fetches.
  - `meta/` – reserved for additional vocabularies (`core.json`, `validation.json`,
    etc.) should we mirror upstream structure.

## Refresh Workflow

Meta-schemas are treated as curated assets. Use the provided make target to
refresh them from upstream when network access is available:

```bash
make sync-schemas
make embed-assets
```

The `sync-schemas` target downloads the latest drafts into this directory. The
`embed-assets` target then copies the updated meta-schemas into
`internal/assets/embedded_schemas/...` so they are embedded in the binary.

> **Note**: Do not hand-edit these files unless you are updating the offline
> subset. Always regenerate from upstream to guarantee canonical content and
> ensure the json-schema.org terms of service are honored. The upstream license
> permits redistribution, so keeping copies in-repo is allowed.

## Offline Subset

`draft-2020-12/offline.schema.json` is a minimal schema that covers the portions
of the spec goneat needs for tests. It avoids `$ref` chains to `meta/*` so the
validator can operate without network access. Keep it in sync with upstream
changes as needed.
