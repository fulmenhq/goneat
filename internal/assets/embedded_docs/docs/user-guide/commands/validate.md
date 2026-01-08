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
- `--enable-meta`: Attempt meta-schema validation (meta-validation runs in parallel when validating many schema files)
- `--scope`: Limit traversal scope
- `--list-schemas`: List available embedded schemas with drafts

Performance note: `goneat validate` does not currently expose a `--workers` flag. For explicit concurrency control, use `goneat assess --categories schema --concurrency N` (or `goneat validate suite --workers N`).

## Data Validation Subcommand

Validate a single data file against a specific schema:

```bash
goneat validate data --schema SCHEMA --data FILE
```

- `--schema`: Schema name for embedded (required if no --schema-file, e.g., goneat-config-v1.0.0)
- `--schema-file`: Path to arbitrary schema file (JSON/YAML; overrides --schema)
- `--ref-dir`: Directory tree of schema files used to resolve absolute `$ref` URLs offline (repeatable). Safe if it also contains `--schema-file`
- `--data`: Data file to validate (required, YAML/JSON)
- `--format`: Output format (markdown, json)

## Suite Validation Subcommand (Bulk)

Validate many data files in one run, mapped to schemas via a schema mapping manifest. This is the recommended workflow for schema ecosystems with example suites.

```bash
goneat validate suite --data DIR --manifest .goneat/schema-mappings.yaml --format json
```

Common flags:

- `--data`: Root directory of data/examples to validate (required)
- `--manifest`: Schema mapping manifest path (defaults to `.goneat/schema-mappings.yaml`)
- `--ref-dir`: Directory tree of schema files used to resolve absolute `$ref` URLs offline (repeatable)
- `--schema-resolution`: `prefer-id|id-strict|path-only` (controls how canonical schema IDs are resolved offline)
- `--expect-fail`: Glob of files expected to fail (repeatable)
- `--skip`: Glob of files to skip (repeatable)
- `--workers`: Max parallel workers (defaults to CPU count). Applies to both data validation and (when `--enable-meta` is set) schema meta-validation.
- `--format`: Output format (markdown, json)

### Example: Offline `$ref` resolution for a full examples suite

```bash
goneat validate suite \
  --data examples/v1.0.0 \
  --manifest .goneat/schema-mappings.yaml \
  --ref-dir schemas/v1.0.0 \
  --expect-fail "**/invalid/**" \
  --format json
```

This enables friction-free validation when schemas use canonical `$id` / absolute `$ref` URLs but the schema registry host is offline or not deployed yet.

Note: JSON Schema catches structural issues (types, required fields). It will not catch taxonomy/slug semantics unless those are encoded into the schema.

## Recommended CI Strategy (Dual-Run)

Canonical schema IDs are a _contract_. In many ecosystems the canonical spec-host may be offline, not deployed yet, or CI may be intentionally no-network.

Goneat recommends a two-phase validation strategy:

### 1) Pre-deploy (required): offline `id-strict` against the corpus

This proves your schema corpus is internally consistent and that canonical `$id` values can be resolved **without network**.

```bash
goneat validate suite \
  --data examples/v1.0.0 \
  --ref-dir schemas/v1.0.0 \
  --schema-resolution id-strict \
  --format json
```

### 2) Post-deploy (optional for build correctness; critical operationally): verify the live spec-host

After you publish the corpus to your spec-host (staging or production), run a separate job to prove the **deployed** canonical URLs actually resolve.

- This is optional from the standpoint of “is the build artifact OK?”
- This is critical to know the spec-host is up and serving the intended content

A simple starting point is to `curl` a few canonical URLs and verify a `200` response.

Example minimal post-deploy probe (copy/paste):

```bash
set -euo pipefail

# Base URL for your deployed spec-host (staging or prod)
SPEC_HOST_BASE_URL="https://schemas.example.org"

# A few canonical IDs you expect to resolve post-deploy
CANONICAL_IDS=(
  "${SPEC_HOST_BASE_URL}/v1.0.0/configuration/recipe.schema.json"
  "${SPEC_HOST_BASE_URL}/v1.0.0/state/inventory.schema.json"
)

for url in "${CANONICAL_IDS[@]}"; do
  echo "Probing: $url"
  curl -fsSIL "$url" >/dev/null
  echo "  ✅ 200 OK"
done
```

> Future: goneat can provide a first-class `spec-host probe` command, but the dual-run strategy remains the same.

### Writing a manifest for local schemas

The schema mapping manifest lives at `.goneat/schema-mappings.yaml` by default.

**Simplest (recommended): use `schema_path` in mappings**

```yaml
version: "1.0.0"

mappings:
  - pattern: "**/recipe*.yaml"
    schema_path: schemas/v1.0.0/configuration/recipe.schema.json
    priority: high

  - pattern: "**/inventory*.yaml"
    schema_path: schemas/v1.0.0/state/inventory.schema.json
    priority: high
```

**Canonical IDs (optional): use `overrides` + `schema_id`**

Use this when you want stable names like `enact-recipe-v1.0.0` in output.

**Canonical URL IDs (offline registry mode): use `schema_id` + `source: external`**

Use this when you want the manifest to look like a real schema registry (canonical URL IDs), but still run in no-network CI using `--ref-dir`.

```yaml
version: "1.0.0"

mappings:
  - pattern: "**/recipe*.yaml"
    schema_id: "https://schemas.example.org/v1.0.0/configuration/recipe.schema.json"
    source: external
```

Run:

```bash
goneat validate suite \
  --data examples/v1.0.0 \
  --ref-dir schemas/v1.0.0 \
  --schema-resolution id-strict \
  --format json
```

```yaml
version: "1.0.0"

overrides:
  - schema_id: enact-recipe-v1.0.0
    source: local
    path: schemas/v1.0.0/configuration/recipe.schema.json

mappings:
  - pattern: "**/recipe*.yaml"
    schema_id: enact-recipe-v1.0.0
    source: local
    priority: high
```

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
