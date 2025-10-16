---
title: "Fulmen Library Module Standards"
description: "Detailed specifications for core and extension modules implemented by Fulmen helper libraries"
author: "Schema Cartographer"
date: "2025-10-09"
last_updated: "2025-10-09"
status: "draft"
tags: ["standards", "library", "modules", "2025.10.2"]
---

# Fulmen Library Module Standards

This directory contains the normative specifications for the modules implemented by Fulmen helper
libraries (`*fulmen`). Each specification defines the cross-language interface, configuration model, testing
expectations, and references to machine-readable schemas.

## Structure

- `modules/`
  - `config-path-api.md` – Platform-aware configuration path discovery helpers.
  - `three-layer-config.md` – Layered configuration loading with runtime overrides.
  - `schema-validation.md` – Schema lookup, validation, and error handling helpers.
  - `crucible-shim.md` – Asset accessors for embedded Crucible content.
  - `fuldx-bootstrap.md` – FulDX bootstrap workflow for installing tooling manifests.
  - `ssot-sync.md` – SSOT synchronization workflow using FulDX consumer manifests.
- `extensions/` – Optional modules (cloud storage, pathfinder, ascii helpers, …).
- `foundry/`
  - `README.md` – Shared pattern/catalog data (regex/glob, HTTP statuses, country codes, MIME types).
  - `interfaces.md` – Required helper interfaces (pattern accessors, MIME detection, etc.).

> Observability logging remains under `docs/standards/observability/`. Module specs reference that standard to
> avoid duplicating cross-cutting requirements.

## Machine-Readable Manifest

Module requirement levels (core vs extension), coverage targets, and language-specific overrides are tracked in
`config/library/v1.0.0/module-manifest.yaml`. Validation schema lives at
`schemas/library/module-manifest/v1.0.0/module-manifest.schema.json` and reuses the taxonomy enums under
`schemas/taxonomy/`.

## Related Resources

- `docs/architecture/modules/README.md` – High-level module catalog.
- `config/taxonomy/languages.yaml` – Canonical helper language registry.
- `config/taxonomy/repository-categories.yaml` – Repository category taxonomy.
- `.plans/active/2025.10.2/` – Feature briefs driving the v2025.10.2 upgrades.
