---
title: "Pathfinder Extension"
description: "Optional helper module for path discovery and filesystem traversal with checksum support"
author: "Schema Cartographer"
date: "2025-10-09"
last_updated: "2025-10-23"
status: "draft"
tags:
  [
    "standards",
    "library",
    "extensions",
    "pathfinder",
    "fulhash",
    "checksums",
    "2025.10.3",
  ]
---

# Pathfinder Extension

## Scope

Deliver ergonomic helpers for scanning filesystem trees, applying inclusion/exclusion globs, and producing
metadata used by Fulmen tools (e.g., goneat). Pathfinding remains optional but widely useful for CLI tools.

## Capabilities

- Recursive scanning with inclusive/exclusive glob patterns.
- Ability to honor `.fulmenignore`-style files.
- Metadata collection: file size, modification time, and optional checksums.
- Hooks for pluggable processors (e.g., apply validation per file).
- Optional checksum calculation using [FulHash](../modules/fulhash.md) for integrity verification and change detection.

## Interoperability

- **Error Handling**: The shared error module (`schemas/error-handling/v1.0.0/error-response.schema.json`) extends the Pathfinder error envelope via `$ref`. Libraries emitting Pathfinder errors gain the optional telemetry fields automatically when they adopt the wrapper—no breaking changes for existing consumers.
- **Logging**: When Pathfinder operations surface errors, coordinate with Observability Logging to propagate `correlation_id` and `severity` so downstream pipelines can link events.

## Implementation Notes

- **Go**: Build atop `filepath.WalkDir` with concurrency controls and context cancellation.
- **Python**: Use `pathlib.Path.rglob` / `os.scandir`. Provide async variant when running under asyncio.
- **TypeScript**: Use `fast-glob` or `@nodelib/fs.walk` for efficient traversal.

## Testing

- Fixture-based tests with nested directories verifying glob matching.
- Performance benchmarks to guard against regressions.
- Windows path handling tests (drive letters, UNC paths).

## Checksum Support

**Version**: 2025.10.3+

Pathfinder integrates with [FulHash](../modules/fulhash.md) to provide optional file checksum calculation during traversal.

### Configuration

Extend Pathfinder configuration with checksum options:

```jsonc
{
  "includePatterns": ["**/*.go", "**/*.ts"],
  "excludePatterns": ["**/node_modules/**"],
  "calculateChecksums": false, // Enable checksum calculation
  "checksumAlgorithm": "xxh3-128", // Default: xxh3-128, also supports: sha256
  "checksumEncoding": "hex", // Default: hex (lowercase)
}
```

**Fields**:

- `calculateChecksums` (boolean): Enable/disable checksum calculation. Default: `false`
- `checksumAlgorithm` (string): Hash algorithm. Options: `"xxh3-128"` (default), `"sha256"`
- `checksumEncoding` (string): Output encoding. Currently only `"hex"` supported (lowercase)

### Metadata Schema

When checksums enabled, `PathResult` metadata includes:

```json
{
  "path": "src/main.go",
  "metadata": {
    "size": 12345,
    "modified": "2025-10-23T17:20:00Z",
    "checksum": "xxh3-128:abc123def456...",
    "checksumAlgorithm": "xxh3-128"
  }
}
```

**New Fields**:

- `checksum` (string, optional): Formatted checksum with algorithm prefix (`<algorithm>:<hex>`)
- `checksumAlgorithm` (string, optional): Algorithm identifier for quick filtering

**Backward Compatibility**: When `calculateChecksums: false`, these fields are omitted and existing metadata structure unchanged.

### Performance Considerations

- **Streaming**: Implementations MUST use streaming hashing for files to avoid loading entire file into memory
- **Target Overhead**: <10% performance overhead on mixed workloads (small/large files)
- **Buffer Management**: Reuse buffers to minimize allocations during traversal
- **Concurrency**: May parallelize checksum calculation across files (language-dependent)

### Error Handling

**Checksum Calculation Failures**:

When checksum calculation fails (permissions, I/O error):

1. Log warning with file path and error reason
2. Continue traversal (do not fail entire operation)
3. Omit checksum fields from metadata for affected files
4. Optionally include `checksumError` field in metadata:

```json
{
  "path": "protected/file.bin",
  "metadata": {
    "size": 5678,
    "modified": "2025-10-23T12:00:00Z",
    "checksumError": "Permission denied"
  }
}
```

### Implementation Requirements

**FulHash Integration**:

```python
# Python example
from pyfulmen.fulhash import hash_file, Algorithm

if config.calculate_checksums:
    try:
        digest = await hash_file(
            path,
            algorithm=Algorithm[config.checksum_algorithm.upper().replace('-', '_')]
        )
        metadata.checksum = digest.formatted
        metadata.checksum_algorithm = config.checksum_algorithm
    except OSError as e:
        logger.warning(f"Checksum failed for {path}: {e}")
        metadata.checksum_error = str(e)
```

**Cross-Language Validation**:

- All implementations MUST produce identical checksums for same file
- Use FulHash shared fixtures for testing
- Include integration tests with known files

### Testing Requirements

**Unit Tests**:

- Checksum calculation enabled/disabled
- Both algorithms (`xxh3-128`, `sha256`)
- Error handling (permission denied, missing file)
- Streaming correctness (large files)

**Integration Tests**:

- Cross-language checksum parity
- Performance benchmarks within target overhead
- Concurrent traversal with checksums

**Fixtures**:
Use FulHash shared fixtures (`config/library/fulhash/fixtures.yaml`) for validation.

## Status

- Optional; recommended for CLI-heavy foundations. Document adoption in module manifest overrides.
- Checksum support added in 2025.10.3 via FulHash integration.
