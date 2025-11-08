---
title: SSOT Sync
description: Synchronize documentation and schemas from Single Source of Truth (SSOT) repositories using Goneat.
---

# SSOT Sync Library

Goneat ships with a first-class SSOT (Single Source of Truth) synchronization workflow that replaces the legacy FulDX helper. The workflow is powered by the `ssot` command group and the `pkg/ssot` package, allowing repositories to pull canonical documentation, schemas, and other assets from upstream sources such as Crucible.

## Key Capabilities

- **Command-Line Sync**: `goneat ssot sync` copies assets described by a manifest into the current repository.
- **Typed Configuration**: `.goneat/ssot-consumer.yaml` (and optional `.goneat/ssot-consumer.local.yaml`) provide structured configuration that is validated after merge.
- **Local Overrides**: Developers can point to a sibling checkout (e.g., `../crucible`) without touching the checked-in manifest.
- **Environment Overrides**: Set `GONEAT_SSOT_CONSUMER_<SOURCE>_LOCAL_PATH` to override source paths in CI, ephemeral environments, or alternate directory layouts.
- **Schema Backed**: Configuration is validated against `schemas/config/sync-consumer-config.yaml`, ensuring consistency across the ecosystem.
- **Provenance Metadata** (v0.3.0+): Automatic generation of provenance metadata capturing source commit, version, and dirty state for audit trails and CI enforcement.

## Configuration Files

| File                               | Purpose                              | VCS | Notes                                                       |
| ---------------------------------- | ------------------------------------ | --- | ----------------------------------------------------------- |
| `.goneat/ssot-consumer.yaml`       | Primary manifest (sources + assets). | ✅  | Versioned with the repo.                                    |
| `.goneat/ssot-consumer.local.yaml` | Local override for developers.       | ❌  | Gitignored; may contain only overrides such as `localPath`. |

Example production manifest:

```yaml
version: v1.1.0

sources:
  - name: crucible
    repo: fulmenhq/crucible
    ref: main
    force_remote: false  # Optional: disable auto-detection for this source (v0.3.4+)
    sync_path_base: lang/go
    assets:
      - type: doc
        paths:
          - docs/**/*
        subdir: docs/crucible-go
      - type: schema
        paths:
          - schemas/**/*
        subdir: schemas/crucible-go
```

**Force Remote Config Option (v0.3.4+)**

Add `force_remote: true` to a source to permanently disable auto-detection:

```yaml
sources:
  - name: crucible
    repo: fulmenhq/crucible
    ref: v0.2.8
    force_remote: true  # Always use remote, never auto-detect local paths
    sync_path_base: lang/go
    assets: [...]
```

This is useful for CI environments or projects that always want remote syncs.

Local override (`.goneat/ssot-consumer.local.yaml`) can contain only the fields necessary for a developer override:

```yaml
version: v1.1.0

sources:
  - name: crucible
    localPath: ../crucible
```

> The loader skips validation on the local file until it merges with the primary manifest, so a minimal override like the example above is valid.

## Running a Sync

```bash
# Copy canonical docs/schemas into the repo
goneat ssot sync

# Dry run to preview actions
goneat ssot sync --dry-run

# Verbose output (per-file)
goneat ssot sync --verbose

# Override source path for all sources from the CLI
goneat ssot sync --local-path ../crucible

# Force remote sync (ignore local auto-detection) - v0.3.4+
goneat ssot sync --force-remote

# Force remote via environment variable
GONEAT_FORCE_REMOTE_SYNC=1 goneat ssot sync
```

Command options:

| Flag             | Description                                                                                             |
| ---------------- | ------------------------------------------------------------------------------------------------------- |
| `--local-path`   | Force all sources to read from the provided path (highest precedence).                                  |
| `--force-remote` | Force remote sync for all sources, disable local auto-detection (v0.3.4+). Mutually exclusive with `--local-path`. |
| `--dry-run`      | Show planned operations without copying files.                                                          |
| `--verbose`      | Emit per-file copy/link operations.                                                                     |

## Configuration Precedence

1. **Command-line flags** (e.g., `--local-path` or `--force-remote`).
2. **Environment variables**:
   - `GONEAT_FORCE_REMOTE_SYNC=1` - Disable local auto-detection (v0.3.4+)
   - `GONEAT_SSOT_CONSUMER_<SOURCE>_LOCAL_PATH` - Override source path
3. **Local override** (`.goneat/ssot-consumer.local.yaml`).
4. **Primary manifest** (`.goneat/ssot-consumer.yaml`).
5. **Convention-based auto-detection** (`../<source>` directory) - **only runs when `.local.yaml` exists** (v0.3.4+).

### Auto-Detection Behavior (v0.3.4+)

**Improved DX**: Auto-detection now only runs when `.goneat/ssot-consumer.local.yaml` exists, signaling local development intent.

- **`.local.yaml` present** → Auto-detection enabled: checks for `../<source>` directories
- **`.local.yaml` absent** → No auto-detection: uses production config (remote repos/refs)

This eliminates the need for `--force-remote` in the common case where you want to use the committed configuration.

**Example: TSFulmen Remote Sync**

Previously, removing `.local.yaml` was insufficient - auto-detection still ran if `../crucible` existed.
Now, simply archive/delete `.local.yaml` and `goneat ssot sync` will use the remote config.

## Schema & Validation

- Manifest validation uses the synced schema from `schemas/config/sync-consumer-config.yaml`.
- Goneat embeds the same schema under `internal/assets/embedded_schemas/...`, so the CLI can validate manifests offline.
- Producers (e.g., Crucible) also publish the schema so other tooling can interoperate.

Validate a manifest manually:

```bash
goneat schema validate \
  --schema schemas/config/sync-consumer-config.yaml \
  --file .goneat/ssot-consumer.yaml
```

## Bootstrap Workflow

Refer to Crucible's [`bootstrap-goneat.md`](../crucible-go/guides/bootstrap-goneat.md) for the end-to-end onboarding steps. In short:

1. Install Goneat (binary or `go install`).
2. Copy `.goneat/ssot-consumer.yaml` from the template or documentation.
3. Optionally create `.goneat/ssot-consumer.local.yaml` pointing at a local Crucible checkout.
4. Run `goneat ssot sync` to pull documentation/schemas.
5. Use `goneat tools install` (and related subcommands) to fetch the rest of the toolchain.

## Migration Notes

- Legacy FulDX manifests (`.fuldx/sync-consumer.yaml`) are still accepted for a transitional period. Goneat warns when it loads from a legacy location; migrate to `.goneat/ssot-consumer.yaml` to silence the warning.
- Local override filenames changed from `.fuldx/sync-consumer.local.yaml` to `.goneat/ssot-consumer.local.yaml`.
- Environment override keys changed from `GONEAT_<SOURCE>_LOCAL_PATH` to `GONEAT_SSOT_CONSUMER_<SOURCE>_LOCAL_PATH`. The old keys are still honored for compatibility but should be phased out.

## Programmatic Usage

The `pkg/ssot` package backs the CLI and can be embedded into custom workflows:

```go
import (
    "log"

    "github.com/fulmenhq/goneat/pkg/ssot"
)

func syncAssets() {
    cfg, err := ssot.LoadSyncConfig()
    if err != nil {
        log.Fatalf("failed to load SSOT config: %v", err)
    }

    if _, err := ssot.PerformSync(ssot.SyncOptions{Config: cfg}); err != nil {
        log.Fatalf("sync failed: %v", err)
    }
}
```

The library performs the same validation and merge steps as the CLI.

## Provenance Metadata (v0.3.0+)

Starting in v0.3.0, `goneat ssot sync` automatically generates provenance metadata capturing where synced assets originated. This enables audit trails, CI enforcement of clean syncs, and tracking of source versions.

### Metadata Artifacts

**Aggregate Provenance** (`.goneat/ssot/provenance.json`):

- Single JSON file containing metadata for all synced sources
- Includes commit SHAs, dirty state, version information, and asset outputs
- Schema: `schemas/ssot/provenance.v1.json`

**Per-Source Mirrors** (`.{slug}/metadata/metadata.yaml`):

- Individual YAML files for each source (e.g., `.crucible/metadata/metadata.yaml`)
- Compatible with existing helper library consumers
- Schema: `schemas/ssot/source-metadata.v1.json`

### Configuration

Provenance generation is enabled by default. Configure in `.goneat/ssot-consumer.yaml`:

```yaml
version: v1.1.0

provenance:
  enabled: true # default
  output: .goneat/ssot/provenance.json # default
  mirror_per_source: true # default - create per-source YAML mirrors
  per_source_format: yaml # default (yaml or json)

sources:
  - name: crucible
    repo: fulmenhq/crucible
    ref: main
    # ... asset definitions ...
```

### Example Provenance Output

**Aggregate** (`.goneat/ssot/provenance.json`):

```json
{
  "schema": {
    "name": "goneat.ssot.provenance",
    "version": "v1",
    "url": "https://github.com/fulmenhq/goneat/schemas/ssot/provenance.v1.json"
  },
  "generated_at": "2025-10-27T18:00:00Z",
  "sources": [
    {
      "name": "crucible",
      "slug": "crucible",
      "method": "local_path",
      "repo_url": "https://github.com/fulmenhq/crucible",
      "local_path": "../crucible",
      "ref": "main",
      "commit": "b64d22a0f0f94e4f1f128172c04fd166cf255056",
      "dirty": false,
      "forced_remote": false,
      "version": "2025.10.2",
      "version_source": "VERSION",
      "outputs": {
        "docs": "docs/crucible-go",
        "schemas": "schemas/crucible-go"
      }
    }
  ]
}
```

**Example with Force Remote** (v0.3.4+):

```json
{
  "sources": [
    {
      "name": "crucible",
      "method": "git_clone",
      "repo_url": "https://github.com/fulmenhq/crucible",
      "ref": "v0.2.8",
      "commit": "abc123...",
      "forced_remote": true,
      "forced_by": "flag"
    }
  ]
}
```

**Per-Source Mirror** (`.crucible/metadata/metadata.yaml`):

```yaml
schema:
  name: goneat.ssot.source-metadata
  version: v1
  url: https://github.com/fulmenhq/goneat/schemas/ssot/source-metadata.v1.json
generated_at: "2025-10-27T18:00:00Z"
name: crucible
slug: crucible
method: local_path
repo_url: https://github.com/fulmenhq/crucible
local_path: ../crucible
ref: main
commit: b64d22a0f0f94e4f1f128172c04fd166cf255056
dirty: false
version: 2025.10.2
version_source: VERSION
outputs:
  docs: docs/crucible-go
  schemas: schemas/crucible-go
```

### Metadata Fields

| Field            | Description                                                              |
| ---------------- | ------------------------------------------------------------------------ |
| `name`           | Source name from manifest                                                |
| `slug`           | URL-safe slug (lowercase, hyphens)                                       |
| `method`         | Sync method: `local_path`, `git_ref`, `git_tag`, or `archive`            |
| `repo_url`       | Repository URL (https://github.com/org/repo)                             |
| `local_path`     | Local filesystem path used                                               |
| `ref`            | Git branch/tag                                                           |
| `commit`         | Full 40-character Git commit SHA                                         |
| `dirty`          | Whether source had uncommitted changes                                   |
| `dirty_reason`   | Reason for dirty state: `worktree-dirty`, `non-git`, etc.                |
| `forced_remote`  | Whether force-remote was used (v0.3.4+)                                  |
| `forced_by`      | How force-remote was activated: `flag`, `env`, or `config` (v0.3.4+)    |
| `version`        | Version detected from VERSION file                                       |
| `version_source` | Source of version: filename or `not-found`                               |
| `outputs`        | Map of asset type to destination path                                    |

### Programmatic Access

```go
import "github.com/fulmenhq/goneat/pkg/ssot"

func checkSyncProvenance() error {
    result, err := ssot.PerformSync(ssot.SyncOptions{Config: cfg})
    if err != nil {
        return err
    }

    // Access metadata
    if result.Metadata != nil {
        for _, source := range result.Metadata.Sources {
            if source.Dirty {
                log.Printf("Warning: %s synced from dirty source: %s",
                    source.Name, source.DirtyReason)
            }
        }
    }

    return nil
}
```

### CI Enforcement

Use provenance metadata to enforce clean syncs in CI:

```bash
#!/bin/bash
# Check if any synced sources were dirty
if [ -f .goneat/ssot/provenance.json ]; then
    dirty=$(jq '.sources[] | select(.dirty == true) | .name' .goneat/ssot/provenance.json)
    if [ -n "$dirty" ]; then
        echo "Error: Synced from dirty sources: $dirty"
        exit 1
    fi
fi
```

### Disabling Provenance

To disable provenance generation:

```yaml
provenance:
  enabled: false
```

Or disable per-source mirrors only:

```yaml
provenance:
  enabled: true
  mirror_per_source: false # Only write aggregate manifest
```
