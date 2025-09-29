---
title: Finder Facade Guide
description: High-level discovery facade for goneat pathfinder with transforms, streaming, and worker coordination.
author: "@arch-eagle"
date: "2025-09-24"
last_updated: "2025-09-24"
status: "draft"
tags:
  [
    "pathfinder",
    "discovery",
    "facade",
    "streaming",
    "transforms",
    "cloud-ready",
  ]
category: "library"
---

# Finder Facade Guide

The `FinderFacade` is the high-level entry point for path discovery introduced in goneat v0.2.9. It keeps the enterprise-grade `PathFinder` interface intact while providing a simpler API for common workflows such as Sumpter's XML crawlers, CLI integrations, and future cloud storage loaders.

Use the facade when you need:

- Quick access to recursive discovery with include/exclude patterns
- Optional logical-path transforms (flattening, prefix mapping, asymmetric copies)
- Streamed results for large file sets
- Concurrency hints without managing worker pools by hand
- Transparent guardian enforcement and audit logging inherited from the core engine

## Quick Start

```go
package main

import (
    "context"
    "log"

    "github.com/fulmenhq/goneat/pkg/pathfinder"
)

func main() {
    pf := pathfinder.NewPathFinder()
    finder := pathfinder.NewFinderFacade(pf, pathfinder.FinderConfig{})

    results, err := finder.Find(pathfinder.FindQuery{
        Root:    "/data/reports",
        Include: []string{"**/*.xml"},
        Context: context.Background(),
    })
    if err != nil {
        log.Fatalf("failed to discover files: %v", err)
    }

    for _, r := range results {
        log.Printf("relative=%s logical=%s source=%s", r.RelativePath, r.LogicalPath, r.SourcePath)
    }
}
```

### What happens under the hood?

1. The facade validates the root path with the `SafetyValidator`.
2. It translates `FindQuery` into `DiscoveryOptions` and calls `PathFinder.DiscoverFiles`.
3. Results flow through an optional `PathTransform`, allowing you to adjust logical names without rewriting discovery.
4. Guardian constraints, audit logging, and loader behavior remain active because the underlying `PathFinder` still drives execution.

## Logical Path Transforms

Transforms let you reshape the output without tampering with `PathFinder` internals. They are ideal for Sumpter-style “flatten while copying” pipelines or any asymmetric source/destination mapping.

```go
flatten := func(result pathfinder.PathResult) pathfinder.PathResult {
    result.LogicalPath = path.Base(result.RelativePath)
    return result
}

results, _ := finder.Find(pathfinder.FindQuery{
    Root:      "/ingest/batch-01",
    Include:   []string{"**/*.ndjson"},
    Transform: flatten,
})
```

Common transform patterns:

- **Flatten directories**: Use `path.Base` to drop intermediates.
- **Prefix logical paths**: Join a deployment bucket or tenant ID before returning results.
- **Strip prefixes**: Remove archive roots (`stage/alpha/`) before handing off to copy routines.
- **Annotate metadata**: Populate `PathResult.Metadata` (e.g., dataset IDs) for downstream steps.

The CLI exposes these behaviors via `--flatten`, `--strip-prefix`, and `--logical-prefix` flags—refer to [`docs/user-guide/commands/pathfinder.md`](../../../user-guide/commands/pathfinder.md) for details.

> 💡 **Workflow pairing**: See the schema discovery + validation walkthrough in
> [`docs/user-guide/workflows/schema-discovery-validation.md`](../../../user-guide/workflows/schema-discovery-validation.md)
> for an end-to-end example that chains `goneat pathfinder find --schemas` with `goneat schema validate-schema`.

## Streaming & Large Trees

Use `FindStream` for large repositories or cloud buckets where buffering all results would be expensive.

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

results, errs := finder.FindStream(pathfinder.FindQuery{
    Root:    "/big/logs",
    Stream:  true,              // optional hint for clarity
    Context: ctx,
})

for r := range results {
    if shouldStop(r) {
        cancel()
        break
    }
    process(r)
}

if err := <-errs; err != nil && err != context.Canceled {
    log.Fatalf("stream failed: %v", err)
}
```

Key behaviors:

- The facade currently streams by buffering `Find` results internally, then sends them over a channel. Full streaming across loader boundaries is planned alongside the v0.2.10 cloud pagination work.
- Cancellation comes from the provided context; the goroutine stops pushing results and forwards the `context.Canceled` error.
- Worker hints (`FindQuery.Workers` or `FinderConfig.MaxWorkers`) map to future concurrency support in `DiscoveryOptions.Concurrency`.

## Relationship with the Core Interface

`FinderFacade` is additive, not a replacement. Reach for the underlying `PathFinder` when you need to:

- Enable audit trails (`EnableAudit`) or query historical records (`GetAuditTrail`)
- Register custom loaders or interact with loader factories directly
- Use advanced discovery filters (size ranges, time ranges, skip patterns) not exposed in the facade yet
- Coordinate audit compliance modes or retention policies

The facade intentionally keeps `PathResult.LoaderType` so you can identify the backing loader and downshift to low-level APIs when needed.

## Schema Manifest & Overrides

Schema detection pulls from the canonical manifest at `schemas/signatures/v1.0.0/schema-signatures.yaml`, embedded into the binary. The manifest entries mirror the `Signature` structs returned by `LoadManifest()` and are validated by `schema-signature-manifest.schema.yaml` (Draft 2020-12, expressed in YAML for maintainers to annotate).

At runtime we merge overrides in this order: the embedded manifest, `$GONEAT_HOME/config/signatures.yaml`, then every YAML file under `$GONEAT_HOME/signatures/`. Later definitions replace earlier ones (last wins). This lets teams ship custom signature packs without recompiling goneat.

Use `signature.LoadDefaultManifest()` to obtain the merged view, or pass your own `Manifest` to `signature.NewDetector` for bespoke pipelines.

## Guardian & Safety Considerations

- **Constraints**: Provide a `FinderConfig.Constraint` (e.g., `NewRepositoryConstraint`) to enforce path boundaries. Guardian will reject results outside approved roots.
- **Symlink rules**: `FindQuery.FollowSymlinks` mirrors the core validator. By default, symlinks are skipped to avoid policy violations.
- **Audit logging**: If the core `PathFinder` has audit logging enabled, each facade operation records entries such as `OpDiscover` or `OpDenied` as appropriate.

## CLI Integration

The `goneat pathfinder find` command is a thin wrapper over `FinderFacade`. Anything the CLI can do—pattern filters, flattening, prefix manipulation, streaming text output—you can achieve programmatically through the same query parameters. See the dedicated CLI guide for usage examples and flag descriptions.

## Roadmap

- **Concurrency control**: Hook `DiscoveryOptions.Concurrency` into `SafeWalker` for true parallel crawling.
- **Cache hints**: Respect `FinderConfig.CacheEnabled`/`CacheTTL` once the cache layer is implemented.
- **Cloud loaders**: v0.2.10 introduces S3, R2, and GCS loaders that plug into the same facade via `FinderConfig.LoaderType`.
- **Direct streaming**: Emit results as loader pages are fetched, avoiding intermediate buffering for massive buckets.

For deeper internals, continue to the planned `api-reference.md` or inspect the source in `pkg/pathfinder/`.
