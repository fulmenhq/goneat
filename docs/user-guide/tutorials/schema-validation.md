---
title: "Tutorial: Schema Validation (Preview)"
description: "Validate repository schemas offline with goneat"
author: "@arch-eagle"
date: "2025-09-05"
last_updated: "2025-09-05"
status: "preview"
---

# Tutorial: Schema Validation (Preview)

This guide walks you through validating your repository's JSON/YAML schemas offline using goneat.

## Prerequisites

- goneat built locally: `make build`
- Curated meta-schemas are embedded; no network required

## Validate All Schemas

```bash
goneat validate --include schemas/ --format json --output validate.json
```

What you get:

- YAML/JSON syntax checks
- JSON Schema meta-validation (Draft-07/2020-12) for files under `schemas/`

## Validate Specific Files

```bash
goneat validate --include schemas/config/goneat-config-v1.0.0.yaml --format json
```

## Use Assess (Multi-Category)

```bash
goneat assess --categories schema,format --format json --output assessment.json
```

## Optional: Refresh Curated Assets

```bash
make sync-schemas  # requires network
```

See also:

- Assets architecture: `docs/architecture/assets-management.md`
- Assets standard: `docs/standards/assets-standard.md`
- Validate command reference: `docs/user-guide/commands/validate.md`

