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
```

Command options:

| Flag           | Description                                                            |
| -------------- | ---------------------------------------------------------------------- |
| `--local-path` | Force all sources to read from the provided path (highest precedence). |
| `--dry-run`    | Show planned operations without copying files.                         |
| `--verbose`    | Emit per-file copy/link operations.                                    |

## Configuration Precedence

1. **Command-line flags** (e.g., `--local-path`).
2. **Environment variables** (`GONEAT_SSOT_CONSUMER_<SOURCE>_LOCAL_PATH`).
3. **Local override** (`.goneat/ssot-consumer.local.yaml`).
4. **Primary manifest** (`.goneat/ssot-consumer.yaml`).
5. **Convention-based fallback** (`../<source>` when a repo is declared but no path override is provided).

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
