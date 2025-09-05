---
title: "Validate Command Reference"
description: "Schema-aware validation (preview) for JSON/YAML with assess integration"
author: "@arch-eagle"
date: "2025-09-05"
last_updated: "2025-09-05"
status: "preview"
tags: ["cli", "schema", "validation", "commands"]
category: "user-guide"
---

# Validate Command Reference (Preview)

The `goneat validate` command runs schema-aware validation using the same engine as `goneat assess`, focused on the `schema` category.

## Overview

- Syntax-first: YAML/JSON syntax checks to catch broken files quickly.
- Schema-aware (roadmap): Draft 2020-12 JSON Schema, OpenAPI/AsyncAPI, Protobuf.
- Offline-first and deterministic; config-first discovery planned.

## Command Structure

```bash
goneat validate [target] [flags]
```

## Common Use

```bash
# Validate current directory
goneat validate

# Validate a specific file
goneat validate --include schemas/config/goneat-config-v1.0.0.yaml

# JSON output for automation
goneat validate --format json --output validate.json
```

## Flags

- `--format`: Output format (`markdown`, `json`, `html`, `both`, `concise`)
- `--output`: Output file path (default stdout)
- `--include`: Include files/patterns
- `--exclude`: Exclude files/patterns
- `--fail-on`: Fail gate (`critical`, `high`, `medium`, `low`, `info`)
- `--timeout`: Validation timeout (default 3m)
- `--auto-detect`: Preview option to scan `.yaml/.yml/.json` by extension

## Project Config (Preview)

You can configure discovery via the project config using a `schema:` block.
See `docs/configuration/schema-config.md` for proposed keys (enable, auto_detect, patterns, types).

## Output

Results are identical to `goneat assess` with issues under `categories.schema`.

- Syntax errors:
  - `sub_category`: `yaml_syntax` or `json_syntax`
  - `severity`: `high`

## Roadmap

- Config-first patterns (`schema.patterns`) and type selectors
- JSON Schema meta-validation (Draft 2020-12)
- OpenAPI/AsyncAPI/Protobuf validation (offline-first)
