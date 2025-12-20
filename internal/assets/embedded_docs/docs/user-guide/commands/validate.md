# Validate Command Reference

## Overview

The `validate` command provides schema-aware validation for JSON/YAML files, including syntax checks and structural validation against embedded schemas (preview).

## Usage

```bash
goneat validate [target]
```

### Flags

- `--format`: Output format (markdown, json, html, both, concise)
- `--verbose, -v`: Verbose output
- `--fail-on`: Fail if issues at or above severity (critical, high, medium, low)
- `--timeout`: Validation timeout
- `--output, -o`: Output file (default: stdout)
- `--include`: Include only these files/patterns
- `--exclude`: Exclude these files/patterns
- `--auto-detect`: Auto-detect schema files
- `--no-ignore`: Disable .goneatignore/.gitignore
- `--force-include`: Force-include paths/globs
- `--enable-meta`: Attempt meta-schema validation
- `--scope`: Limit traversal scope
- `--list-schemas`: List available embedded schemas with drafts

## Data Validation Subcommand

Validate a data file against a specific schema:

```bash
goneat validate data --schema SCHEMA --data FILE
```

- `--schema`: Schema name for embedded (required if no --schema-file, e.g., goneat-config-v1.0.0)
- `--schema-file`: Path to arbitrary schema file (JSON/YAML; overrides --schema)
- `--ref-dir`: Directory containing schema files used to resolve remote `$ref` URLs (repeatable)
- `--data`: Data file to validate (required, YAML/JSON)
- `--format`: Output format (markdown, json)

### Supported Schemas

(Heuristics: Match schemaName to registry key without .yaml; Draft detection via $schema key. Arbitrary files via --schema-file support JSON/YAML, Draft-07/2020-12 only.)

- **goneat-config-v1.0.0** (Draft-2020-12): Main project config validation.
- **dates** (Draft-07): Dates configuration validation.
- **hooks-manifest-v1.0.0** (Draft-2020-12): Hooks manifest validation.
- **work-manifest-v1.0.0** (Draft-2020-12): Work manifest validation.

Note: Only Draft-07 and Draft-2020-12 supported. Use `goneat validate --list-schemas` to inspect available schemas and drafts.

**Examples**:

```bash
# Embedded YAML schema
goneat validate data --schema goneat-config-v1.0.0 --data .goneat.yaml --format json

# Arbitrary JSON schema file
 goneat validate data --schema-file my-schema.json --data data.yaml

# Offline $ref resolution using local schemas directory
 goneat validate data --schema-file schemas/v1.0.0/configuration/recipe.schema.json --ref-dir schemas --data examples/recipe.yaml


# Arbitrary YAML schema file
goneat validate data --schema-file my-schema.yaml --data data.json

# List first
goneat validate --list-schemas
```

Outputs validation result; fails on invalid data or unsupported drafts (e.g., Draft-04 → "unsupported $schema").

## Examples

- Validate schemas in current dir: `goneat validate . --include schemas/`
- List schemas: `goneat validate --list-schemas`
- Validate data: See subcommand above
- Meta-validation: `goneat validate schemas/ --enable-meta`

For config, see [schema-config.md](schema-config.md).
