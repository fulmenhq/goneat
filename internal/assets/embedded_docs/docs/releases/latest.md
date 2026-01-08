# Goneat v0.4.3 — Parallel Execution Across All Commands

**Release Date**: 2026-01-08
**Status**: Stable

## TL;DR

- **Parallel by default**: All major commands now default to parallel execution
- **Parallel assess**: `goneat assess` now uses 80% of CPU cores by default (was 50%)
- **Parallel format**: `goneat format` uses `--strategy parallel` by default
- **Parallel schema validation**: `goneat schema validate-schema --workers N` for parallel meta-validation
- **New validate-data command**: `goneat schema validate-data` for data file validation against schemas
- **Cached validators**: Meta-schema validators (draft-07, 2020-12) compiled once, reused across files

## What Changed

### Assess Command

- **Default parallel execution**: The assess command now uses 80% of CPU cores by default (previously 50%)
- **Consistent parallel model**: All goneat commands now default to parallel execution
- **Sequential option**: Use `--concurrency 1` for sequential execution when needed

```bash
# Default parallel (80% of CPU cores)
goneat assess

# Explicit worker count
goneat assess --concurrency 4

# Sequential execution
goneat assess --concurrency 1

# Custom percentage
goneat assess --concurrency-percent 50
```

Use `goneat envinfo` to see your system's CPU core count.

See [assess command docs](../user-guide/commands/assess.md) for configuration details.

### Format Command

- **Default parallel execution**: The format command now uses `--strategy parallel` by default
- **Configurable workers**: Use `--workers N` to control parallelism (0=auto, 1=sequential, max 8)
- **Python/JS/TS support**: Parallel formatting now includes ruff (Python) and biome (JS/TS)

```bash
# Default parallel (auto-detects CPU count)
goneat format

# Explicit worker count
goneat format --workers 4

# Sequential (legacy behavior)
goneat format --strategy sequential
```

See [format command docs](../user-guide/commands/format.md) for configuration details.

### Schema Validation

- **Parallel validate-schema**: Meta-schema validation supports `--workers` flag
- **Cached validators**: Draft-07 and 2020-12 validators compiled once and reused
- **Deterministic output**: Results maintain input file order regardless of parallelism
- **New validate-data subcommand**: Validate data files against JSON schemas

```bash
# Parallel schema validation (auto workers)
goneat schema validate-schema --workers 0 schemas/

# Validate data against schema
goneat schema validate-data --schema schemas/config.json config.yaml
```

### Performance

Benchmarks confirm meaningful speedup for large file sets:

| Operation | Sequential | Parallel (4 workers) | Speedup |
|-----------|------------|---------------------|---------|
| Schema validation (776 files) | 6.77s | 3.97s | ~1.7x |
| Format (large codebase) | varies | varies | ~1.5-2x |

For small file sets (<200 files), parallelization overhead roughly equals gains—sequential is fine.

## Upgrade Notes

### From v0.4.2

**Behavioral change**: All major commands now default to parallel execution:

- `goneat assess` uses 80% of CPU cores (was 50%). For sequential:
  ```bash
  goneat assess --concurrency 1
  ```

- `goneat format` uses parallel strategy. For sequential:
  ```bash
  goneat format --strategy sequential
  ```

- `goneat schema validate-schema` uses auto workers (CPU count). For sequential:
  ```bash
  goneat schema validate-schema --workers 1
  ```

**New subcommand**: `goneat schema validate-data` provides data validation as a first-class schema subcommand (previously only available via `goneat validate data`).

## Documentation

- **Assess command**: [docs/user-guide/commands/assess.md](../user-guide/commands/assess.md)
- **Format command**: [docs/user-guide/commands/format.md](../user-guide/commands/format.md)
- **Schema validation**: [docs/user-guide/commands/validate.md](../user-guide/commands/validate.md)
