# Goneat v0.4.3 — Parallel Execution for Format & Schema Validation

**Release Date**: 2026-01-08
**Status**: Stable

## TL;DR

- **Parallel format**: `goneat format` now defaults to parallel execution for faster formatting
- **Parallel schema validation**: `goneat schema validate-schema --workers N` for parallel meta-validation
- **New validate-data command**: `goneat schema validate-data` for data file validation against schemas
- **Cached validators**: Meta-schema validators (draft-07, 2020-12) compiled once, reused across files

## What Changed

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

**Behavioral change**: `goneat format` now runs in parallel by default. If you experience issues:

```bash
# Revert to sequential behavior
goneat format --strategy sequential
```

**New subcommand**: `goneat schema validate-data` provides data validation as a first-class schema subcommand (previously only available via `goneat validate data`).

## Documentation

- **Format command**: [docs/user-guide/commands/format.md](../user-guide/commands/format.md)
- **Schema validation**: [docs/user-guide/commands/validate.md](../user-guide/commands/validate.md)
