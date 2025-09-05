---
title: "Schema Configuration (Preview)"
description: "Configure schema-aware validation in goneat"
author: "@arch-eagle"
date: "2025-09-05"
last_updated: "2025-09-05"
status: "preview"
---

# Schema Configuration (Preview)

Goneat supports a schema-aware validation category. In preview, you can control discovery via flags.
This document outlines the upcoming config block for project-level control.

## Proposed Config Block

```yaml
schema:
  enable: true           # master switch
  auto_detect: false     # off by default; enable to scan .yaml/.yml/.json by extension
  patterns:              # config-first discovery; combine with include flags
    - schemas/**/*.yaml
    - schemas/**/*.json
  types:                 # future: type-specific options (jsonschema, openapi, asyncapi, protobuf)
    jsonschema:
      offline: true      # use embedded meta-schemas only
```

## CLI Flags (Preview)

- `goneat validate --include <paths> --exclude <paths>`
- `goneat assess --categories schema --format json`

## Roadmap

- Project-level config parsing for `schema:` block
- Type selectors and per-type options
- OpenAPI/AsyncAPI/Protobuf validation (offline-first)

