# goneat schema

description: "Reference for the goneat schema command, including schema validation helpers and meta-schema usage"

The `goneat schema` command exposes schema-focused utilities built on top of Goneat's embedded assets. The
initial release ships the `validate-schema` subcommand so you can lint JSON Schema documents against the bundled
meta-schemas without leaving your repository.

## Usage

```bash
goneat schema validate-schema [flags] <schema-file> [...schema-file]
```

## validate-schema

Validate schema files (currently JSON Schema Draft-07 and 2020-12) against the embedded meta-schemas. Pair this
with `goneat pathfinder find --schemas` to discover candidate files first.

### Flags

| Flag                 | Description                                                                                                                                     |
| -------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| `--schema-id string` | Signature id to validate against (e.g., `json-schema-draft-07`). Required for non-JSON Schema candidates until broader validator support lands. |
| `--format string`    | Output format: `text` (default) or `json`.                                                                                                      |
| `--workers int`      | Number of parallel workers (0=auto). Use `--workers 1` for deterministic sequential runs.                                                       |

### Examples

```bash
# Validate a single schema file against Draft-07
goneat schema validate-schema --schema-id json-schema-draft-07 ./schemas/config/example.json

# Validate many schema files in parallel
goneat schema validate-schema --schema-id json-schema-draft-07 --workers 8 ./schemas/**/*.json

# Pipe pathfinder results into the validator (JSON Schema only)
goneat pathfinder find --path ./schemas --schemas --schema-id json-schema-draft-07 --output text \
  | cut -d ' ' -f1 \
  | xargs -r goneat schema validate-schema --schema-id json-schema-draft-07 --workers 8

# Emit JSON results for machine consumption
goneat schema validate-schema --schema-id json-schema-2020-12 --format json --workers 8 \
  tests/fixtures/schemas/draft-2020-12/good/simple-object.yaml
```

### Exit Codes

- `0`: All provided schemas validated successfully (or output rendered as JSON without failures).
- `1`: At least one schema failed validation, or validation could not be performed (unsupported schema id, unreadable file, etc.).

## validate-data

Validate a JSON/YAML data file against either an embedded schema (by name or canonical schema ID URL) or an arbitrary schema file.

```bash
goneat schema validate-data --schema goneat-config-v1.0.0 --data .goneat/assess.yaml
```

### Flags

| Flag                    | Description |
| ----------------------- | ----------- |
| `--schema string`       | Schema name (embedded) or canonical schema ID URL (mutually exclusive with `--schema-file`) |
| `--schema-file string`  | Path to arbitrary schema file (JSON/YAML; overrides `--schema`) |
| `--ref-dir strings`     | Directory tree of schema files used to resolve absolute `$ref` URLs offline (repeatable) |
| `--schema-resolution`   | `prefer-id|id-strict|path-only` (controls canonical schema ID resolution) |
| `--data string`         | Data file to validate (required) |
| `--format string`       | Output format: `markdown` (default) or `json` |

### Examples

```bash
# Validate a config file against an embedded schema

goneat schema validate-data --schema goneat-config-v1.0.0 --data .goneat/assess.yaml

# Validate data against an arbitrary schema file with offline $ref resolution

goneat schema validate-data \
  --schema-file ./schemas/v1.0.0/my.schema.json \
  --ref-dir ./schemas/v1.0.0 \
  --data ./examples/v1.0.0/example.json
```
