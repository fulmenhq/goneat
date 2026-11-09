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

Validate schema files against embedded meta-schemas. Supports **all major JSON Schema versions**: Draft-04, Draft-06, Draft-07, 2019-09, and 2020-12. Pair this with `goneat pathfinder find --schemas` to discover candidate files first.

### Supported Schema IDs

| Schema ID | JSON Schema Version | Typical Use |
| --- | --- |
| `json-schema-draft-04` | Draft-04 (2013) | Kubernetes CRDs, legacy enterprise configs |
| `json-schema-draft-06` | Draft-06 (2017) | Transitional schemas |
| `json-schema-draft-07` | Draft-07 (2017) | Most common, community standard |
| `json-schema-2019-09` | 2019-09 | OpenAPI 3.0.x compatible |
| `json-schema-2020-12` | 2020-12 | OpenAPI 3.1, current standard |

### Flags

| Flag                 | Description                                                                                                                                     |
| -------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| `--schema-id string` | Signature id to validate against (e.g., `json-schema-draft-07`). Required for non-JSON Schema candidates until broader validator support lands. |
| `--format string`    | Output format: `text` (default) or `json`.                                                                                                      |
| `--workers int`      | Number of parallel workers (0=auto). Use `--workers 1` for deterministic sequential runs.                                                       |

### Examples

```bash
# Auto-detect draft version from $schema field (recommended for mixed directories)
goneat schema validate-schema --recursive ./schemas/

# Validate legacy Draft-04 schemas (Kubernetes, enterprise configs)
goneat schema validate-schema --schema-id json-schema-draft-04 ./k8s-schemas/

# Validate a single schema file against Draft-07
goneat schema validate-schema --schema-id json-schema-draft-07 ./schemas/config/example.json

# Validate OpenAPI 3.1 schemas (2020-12)
goneat schema validate-schema --schema-id json-schema-2020-12 ./openapi/

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

| Flag                   | Description                                                                                 |
| ---------------------- | ------------------------------------------------------------------------------------------- |
| `--schema string`      | Schema name (embedded) or canonical schema ID URL (mutually exclusive with `--schema-file`) |
| `--schema-file string` | Path to arbitrary schema file (JSON/YAML; overrides `--schema`)                             |
| `--ref-dir strings`    | Directory tree of schema files used to resolve absolute `$ref` URLs offline (repeatable)    |
| `--schema-resolution`  | `prefer-id`, `id-strict` or `path-only` (controls canonical schema ID resolution)           |
| `--data string`        | Data file to validate (required)                                                            |
| `--format string`      | Output format: `markdown` (default) or `json`                                               |

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

## Validation behaviour

Every schema check in goneat uses the same rules: `validate-schema`, `validate-data`, `validate suite`, the `schema` assessment category, and the built-in checks of goneat's own configuration, hooks manifest and source SBOMs. The engine is [santhosh-tekuri/jsonschema](https://github.com/santhosh-tekuri/jsonschema) v6.

### Dialects

- `$schema` is honoured for Draft-04, Draft-06, Draft-07, 2019-09 and 2020-12. For example, a 2020-12 schema applies keywords next to `$ref`; a Draft-07 schema ignores them.
- A schema without `$schema` is validated as Draft-07. JSON output reports it as `"dialect_source": "defaulted"`.
- The unversioned `https://json-schema.org/schema` alias and Draft-03 are refused. A custom metaschema must be supplied with `--ref-dir`.

### Formats

`format` is asserted in every dialect. Supported names:

- `date`, `time`, `date-time`: RFC 3339 syntax with calendar checks. Second `60` is accepted only when the UTC-equivalent time is 23:59:60 on the last day of a month (`structural-leap/v1`). This is a syntax check; it does not prove a leap second occurred.
- `email`: RFC 5321 mailbox syntax. `idn-email`: RFC 6531 mailbox syntax; internationalized domain labels are checked with UTS 46 registration validation, so NV8 code points (for example emoji) are accepted. No DNS lookups.
- `duration`, `hostname`, `ipv4`, `ipv6`, `uri`, `uri-reference`, `iri`, `iri-reference`, `uuid`, `uri-template`, `json-pointer`, `relative-json-pointer`, `regex` (Go RE2 syntax).

Any other format name in a schema (for example `idn-hostname` or `semver`) is a schema error rather than being ignored.

### References

`$ref` resolves within the schema, to schema files under `--ref-dir` (matched by `$id`), and to the built-in JSON Schema metaschemas. Nothing is read from other files or fetched from the network; a reference to anything else is a schema error.

### Schema errors and data errors

A schema that cannot be used is reported as a schema error, separately from data that fails validation. Causes: unsupported dialect, unknown format, unresolved reference, invalid schema, or a limit (schema 8 MiB, data 64 MiB, nesting depth 256, 512 referenced documents, 200,000 schema objects, 4 KiB per pattern).

### Data decoding

JSON data is read exactly: numbers are not rounded, and invalid UTF-8 or unpaired `\u` surrogate escapes are refused. YAML data follows YAML rules.

### Results

JSON output includes `engine`, `dialect`, `dialect_source`, `format_mode` and `leap_policy`. Error paths use dotted form (`tools.mytool.install`, `(root)` for the document). Messages are the engine's, for example `got number, want string`.

`GONEAT_OFFLINE_SCHEMA_VALIDATION` no longer has any effect: schema validation never uses the network.
