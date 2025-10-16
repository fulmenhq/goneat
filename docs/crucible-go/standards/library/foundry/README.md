---
title: "Foundry Pattern Catalog"
description: "Contract for language-agnostic pattern catalogs consumed by Fulmen helper libraries"
author: "Schema Cartographer"
date: "2025-10-09"
last_updated: "2025-10-09"
status: "draft"
tags: ["standards", "library", "foundry", "patterns", "2025.10.2"]
---

# Foundry Pattern Catalog

## Purpose

Define the shared catalog of reusable patterns (regex, glob, literal) that every Fulmen helper library must expose
in an idiomatic way. This module reduces duplication of complex expressions (e.g., email validation, slugs,
IP addresses) across languages.

## Data Sources

- Pattern catalog: `config/library/foundry/patterns.yaml` (schema: `schemas/library/foundry/v1.0.0/patterns.schema.json`)
- HTTP status groups: `config/library/foundry/http-statuses.yaml` (schema: `schemas/library/foundry/v1.0.0/http-status-groups.schema.json`)
- ISO country codes (sample seed): `config/library/foundry/country-codes.yaml` (schema: `schemas/library/foundry/v1.0.0/country-codes.schema.json`)
- MIME types: `config/library/foundry/mime-types.yaml` (schema: `schemas/library/foundry/v1.0.0/mime-types.schema.json`)

Helper libraries MUST treat these catalogs as read-only SSOT data. Updates originate in Crucible.

## Library Responsibilities

1. Load the catalog at bootstrap and expose accessor APIs:
   - Return raw pattern strings.
   - Provide compiled/typed representations (e.g., `regex.Pattern`, `RegExp`, precompiled Go regex).
2. Enforce immutability and avoid runtime mutation of the shared catalog.
3. Surface documentation (e.g., `describe(patternId)`) pulling from the catalog’s metadata.
4. Integrate patterns into validation tooling where applicable (Pydantic validators, Go custom types, Zod schemas).
5. Normalize ISO country tokens (upper-case alpha codes, three-digit numeric strings) before indexing and maintain secondary indexes for alpha-3 and numeric codes so lookups stay case-insensitive across all ISO forms.

## Language Notes

- **Python:** Provide a `FoundryPatterns` helper that returns compiled `regex` instances and raw strings.
- **Go:** Expose typed wrappers returning `*regexp.Regexp` with lazy compilation.
- **TypeScript:** Export both the raw string and a `RegExp` instance per pattern (respecting flags).
- **Rust/C#:** Provide modules/namespaces with constants and precompiled helpers.

## Testing Requirements

- Validate catalog loading and ensure all entries are present (patterns, status groups, countries).
- Ensure accessors return compiled/typed representations without error.
- Provide positive/negative unit tests for each pattern (sample matches + mismatches).
- Verify HTTP status group helpers cover the documented codes; ensure language implementations expose convenience sets (e.g., `IsClientError(code)`).
- Verify country code lookups support alpha-2, alpha-3, and numeric forms case-insensitively (including numeric normalization) and reuse the precomputed indexes built at load time.
- Verify MIME type helpers handle lookup by id, MIME string, and extensions; add tests for unknown types.
- Maintain snapshot/parity tests to detect catalog drift across languages.
- Verify per-language flag handling (Unicode, ignore-case, etc.) in unit tests.

## Related Documents

- `docs/architecture/modules/README.md`
- `config/library/v1.0.0/module-manifest.yaml`
- `config/library/foundry/patterns.yaml`
- `config/library/foundry/http-statuses.yaml`
- `config/library/foundry/country-codes.yaml`
- `.plans/active/2025.10.2/library-module-specification-architecture-v2.md`
