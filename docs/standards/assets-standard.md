---
title: "Assets Curation Standard"
description: "Standards for curating and embedding validation assets"
author: "@arch-eagle"
date: "2025-09-05"
last_updated: "2025-09-05"
status: "approved"
---

# Assets Curation Standard

## Scope

Defines how goneat curates, embeds, and updates third-party validation assets (schemas and specs).

## Principles

- Offline-first: critical assets embedded; no network required by default
- Provenance: record source URL, version, checksum in docs and code registry
- Minimal footprint: store only what is necessary to validate
- Update discipline: use script + Make target; review diffs before updating

## Layout

- Curated assets: `internal/assets/<family>/<version>/...`
- Code registry: `internal/assets/registry.go` lists assets, versions, checksums
- Sync script: `scripts/sync-schemas.sh` (not required for builds)

## Licensing

- Each curated asset must have clear license/provenance
- Update `docs/licenses/inventory.md` with module/asset name, URL, license, version
