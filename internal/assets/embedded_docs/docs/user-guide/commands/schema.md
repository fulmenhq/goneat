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

### Examples

```bash
# Validate a single schema file against Draft-07
goneat schema validate-schema --schema-id json-schema-draft-07 ./schemas/config/example.json

# Pipe pathfinder results into the validator (JSON Schema only)
goneat pathfinder find --path ./schemas --schemas --schema-id json-schema-draft-07 --output text \
  | cut -d ' ' -f1 \
  | xargs -r goneat schema validate-schema --schema-id json-schema-draft-07

# Emit JSON results for machine consumption
goneat schema validate-schema --schema-id json-schema-2020-12 --format json \
  tests/fixtures/schemas/draft-2020-12/good/simple-object.yaml
```

### Exit Codes

- `0`: All provided schemas validated successfully (or output rendered as JSON without failures).
- `1`: At least one schema failed validation, or validation could not be performed (unsupported schema id, unreadable file, etc.).

Future updates will extend the command group with data validation helpers (`validate-data`) and direct manifest
lookups once the broader schema validator roadmap lands.
