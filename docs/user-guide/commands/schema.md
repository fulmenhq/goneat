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

`validate-data`, `validate suite`, the config-to-schema checks of the `schema` assessment, and goneat's own checks of its configuration, hooks manifest and source SBOMs compile schemas with one engine, [santhosh-tekuri/jsonschema](https://github.com/santhosh-tekuri/jsonschema) v6, under the rules below.

`validate-schema` is different: it checks a schema document against its draft's metaschema (the engine's built-in copy). It does not compile the schema for use, so a schema can pass `validate-schema` and still be refused when it is used to validate data (for example, because of an unknown `format` name).

### Dialects

- `$schema` is honoured for Draft-04, Draft-06, Draft-07, 2019-09 and 2020-12. For example, a 2020-12 schema applies keywords next to `$ref`; a Draft-07 schema ignores them.
- A schema without `$schema` is validated as Draft-07 (reported as `defaulted`).
- The unversioned `https://json-schema.org/schema` alias and Draft-03 are refused. A custom metaschema must be supplied with `--ref-dir`.

### Formats

`format` is asserted in every dialect, and only for strings: a non-string value passes a format-only schema. Supported names:

- `date`, `time`, `date-time` and `email`, `idn-email`: goneat's own checkers; see [Owned format checkers](#owned-format-checkers).
- `duration`, `hostname`, `ipv4`, `ipv6`, `uri`, `uri-reference`, `iri`, `iri-reference`, `uuid`, `uri-template`, `json-pointer`, `relative-json-pointer`, `regex` (Go RE2 syntax): the engine's checkers.

Any other format name in a schema (for example `idn-hostname` or `semver`) is a schema error rather than being ignored.

### References

`$ref` resolves within the schema, to schema files under `--ref-dir` (matched by `$id`), and to the built-in JSON Schema metaschemas. Nothing is read from other files or fetched from the network; a reference to anything else is a schema error. `json-schema.org` identities are reserved for the built-in metaschemas.

### Schema errors and data errors

A schema that cannot be compiled is reported as a schema error, separately from data that fails validation. Causes: unsupported dialect, unknown format, unresolved or reserved reference, invalid schema, or a schema compilation limit:

| Limit (schema compilation)                         | Value   |
| -------------------------------------------------- | ------- |
| Size of each schema document                       | 8 MiB   |
| JSON nesting depth of each schema document         | 256     |
| Referenced schema documents                        | 512     |
| Schema objects compiled                            | 200,000 |
| Length of each `pattern` / `patternProperties` key | 4 KiB   |

### Data decoding

JSON data is read exactly: numbers are not rounded, and invalid UTF-8 or unpaired `\u` surrogate escapes are refused. JSON data larger than 64 MiB is refused when it is read. Data that is not JSON text is read as YAML and follows YAML rules (including number conversion).

### Results

- `validate-data --format json` and the `pkg/schema` `Result` include `engine`, `dialect`, `dialect_source` (`declared`, `defaulted` or `metaschema`), `format_mode` and `leap_policy`. `validate-schema` and `validate suite` output do not.
- Error paths use dotted form (`tools.mytool.install`, `(root)` for the document). Messages are the engine's, for example `got number, want string`.

`GONEAT_OFFLINE_SCHEMA_VALIDATION` no longer has any effect: schema validation never uses the network.

### Owned format checkers

These are syntax checks. They do not establish that a time existed or that a mailbox can receive mail.

**`date`**: RFC 3339 `full-date` with calendar checks (month lengths, leap years). The year as written is 0000–9999.

**`time`**: RFC 3339 `full-time` (`HH:MM:SS[.frac]` with `Z` or `±HH:MM`, `z` accepted). Second `60` is accepted only when the time converted to UTC (modulo 24 hours) is 23:59:60.

**`date-time`**: RFC 3339 `date-time` (`T`/`t` separator). Second `60` is accepted only when the instant converted to UTC is 23:59:60 on the last day of its UTC month, carrying the offset across day, month and year. The year as written is 0000–9999; carrying past that range is used only for this rule. The rule is reported as `leap_policy: structural-leap/v1`. A pass does not mean a leap second occurred.

**`email`**: RFC 5321 `Mailbox`.

- Local part: dot-atom or quoted string. Quoted pairs are ASCII only; C0 control characters and DEL are never accepted.
- Domain: ASCII letter-digit-hyphen labels, or an address literal. Only untagged IPv4 (`[192.0.2.1]`) and `IPv6:` (case-insensitive) literals are accepted; `::` stands for at least two groups, as in the published RFC 5321 text.
- Lengths, in UTF-8 bytes as written: local part at most 64 (including quotes), each label at most 63, whole mailbox at most 254.

**`idn-email`**: RFC 6531 extended `Mailbox`; every valid `email` is a valid `idn-email`.

- Local part: as `email`, plus any non-ASCII character in atoms and quoted strings (including U+FFFD, C1 controls and noncharacters).
- ASCII domain labels follow the letter-digit-hyphen rule. `xn--` labels are not decoded or round-trip checked.
- Non-ASCII labels must be valid U-labels as written: no mapping or normalisation (U+3002 is not a dot, uppercase is not folded) and must already be NFC. They are checked with UTS 46 registration validation (Go `golang.org/x/net/idna`): disallowed, mapped, ignored and unassigned code points are refused, plus STD3 rules, joiner (CONTEXTJ) rules, the Bidi rule per label, and the 63-byte limit in A-label form. Code points that UTS 46 marks NV8 (for example emoji) are accepted, so this is not strict IDNA2008.
- goneat also applies the hyphen rules on code points (no leading or trailing hyphen; no `--` in the third and fourth positions, so `ü--ab` is valid and `mü--nchen` is not) and the RFC 5892 CONTEXTO rules A.3–A.9.
- Unicode tables are version 15.0.0 (pinned).
- No DNS lookups, deliverability or confusable-character checks.
