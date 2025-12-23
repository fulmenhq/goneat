---
title: "Repository Category Standards"
description: "Category-specific standards for Fulmen repository types"
author: "Schema Cartographer"
date: "2025-11-03"
last_updated: "2025-12-20"
status: "active"
tags: ["standards", "repository-category", "organization", "v0.2.26"]
---

# Repository Category Standards

## Purpose

This directory contains standards, configuration schemas, and documentation specific to each repository category in the Fulmen ecosystem. These category-specific standards complement ecosystem-wide standards and provide specialized guidance for each repository type.

## Repository Categories

Repository categories are defined in the taxonomy: `schemas/taxonomy/repository-category/v1.0.0/category-key.schema.json`

Current categories: `cli`, `codex`, `library`, `microtool`, `missive`, `pipeline`, `sdk`, `service`, `spec-host`, `workhorse`

## Structure

```
repository-category/
├── codex/
│   └── config-standard.md          # Codex configuration schema standard
├── missive/
│   └── README.md                   # Missive category requirements
├── spec-host/
│   └── README.md                   # Spec-host category requirements
├── workhorse/
│   └── (future workhorse-specific standards)
├── helper-library/
│   └── (future library-specific standards)
└── README.md                        # This file
```

## Parallel Schema Organization

Category-specific schemas are organized under `schemas/config/repository-category/`:

```
schemas/config/repository-category/
├── codex/v1.0.0/
│   └── codex-config.schema.json
├── workhorse/v1.0.0/
│   └── (future schemas)
└── helper-library/v1.0.0/
    └── (future schemas)
```

This creates symmetry between schemas and documentation:

- **Schemas**: `schemas/config/repository-category/{category}/v1.0.0/`
- **Documentation**: `docs/standards/repository-category/{category}/`

## Current Standards

### Codex

**Category**: `codex` (human-first documentation sites and knowledge hubs)

**Summary**: Documentation-first static sites with rich browsing, search, and navigation. For machine-first spec hosting, see spec-host.

**Standards**:

- [Codex Configuration Standard](codex/config-standard.md) - Configuration schema and governance

**Schemas**:

- `schemas/config/repository-category/codex/v1.0.0/codex-config.schema.json` - Configuration validation schema

**Examples**:

- `examples/config/repository-category/codex/v1.0.0/codex-config.example.json` - Reference configuration

**Applies To**:

- Forge Codex Pulsar (active)
- Future codex templates (Aurora, Nebula, etc.)

**Note**: Codex sites may layer browsable UI over a spec-host corpus.

### Spec-Host

**Category**: `spec-host` (machine-first specification artifact hosting)

**Summary**: Static hosting for versioned specification artifacts (JSON Schema, OpenAPI, AsyncAPI) with canonical URL resolution as the primary invariant.

**Standards**:

- [Spec-Host Category Standards](spec-host/README.md) - Category requirements
- [Spec Publishing Standard](../publishing/spec-publishing.md) - Publishing workflow contract

**Key Invariant**: Every `$id` (JSON Schema) or `x-fulmen-id` (OpenAPI/AsyncAPI) MUST resolve over HTTPS.

**Applies To**:

- Crucible schema publishing (planned)
- Enact spec hosting (planned)
- Future `forge-spec-host-*` templates

### Missive

**Category**: `missive` (single-page promotional/CTA sites)

**Summary**: Lightweight single-page sites for event announcements, fundraisers, product launches. Vanilla HTML/CSS first; escalate to Codex when scope grows.

**Standards**:

- [Missive Category Standards](missive/README.md) - Category requirements

**Key Principle**: Start with zero build; add complexity only when justified.

**Applies To**:

- Event announcement pages
- Charity fundraiser sites
- Product launch pages
- Future `forge-missive-*` templates

## Future Standards (Planned)

### Workhorse

**Category**: `workhorse` (application services, API servers)

**Potential Standards**:

- Server configuration patterns
- Deployment manifests
- Health check requirements
- API documentation standards

### Helper Library

**Category**: `library` (language-specific helper libraries)

**Potential Standards**:

- Module implementation patterns
- Testing requirements
- API surface consistency
- Sync procedures

## When to Add Category-Specific Standards

Use category-specific standards when:

1. **Specialized Requirements**: Requirements unique to a category (e.g., codex config, spec-host publishing)
2. **Clear Scope**: Applies to all repositories in a category, not just one instance
3. **Schema-Driven**: Configuration or validation schemas that enforce category standards
4. **Governance Value**: Standardization benefits ecosystem consistency

**Do NOT** add category-specific standards for:

- One-off requirements (use repository-specific docs)
- Ecosystem-wide concerns (use top-level `docs/standards/`)
- Implementation details (use repository README or guides)

## Relationship to Other Standards

**Ecosystem-Wide Standards** (`docs/standards/`):

- Apply to ALL repositories regardless of category
- Examples: coding standards, observability, error handling, publishing

**Category-Specific Standards** (`docs/standards/repository-category/`):

- Apply to all repositories in a specific category
- Examples: codex config, spec-host publishing requirements, missive constraints

**Repository-Specific Documentation** (in each repo):

- Apply only to that specific repository
- Examples: forge-codex-pulsar setup, tsfulmen API reference

## Adding New Category Standards

**Process**:

1. **Identify Need**: Standardization benefits multiple repositories in category
2. **Proposal**: Create issue or PR with proposed standard
3. **Review**: Crucible maintainers review with category experts
4. **Create Schema** (if applicable): Add to `schemas/config/repository-category/{category}/v1.0.0/`
5. **Document**: Create standard doc in `docs/standards/repository-category/{category}/`
6. **Examples**: Add reference implementations to `examples/config/repository-category/{category}/v1.0.0/`
7. **Release**: Include in next Crucible version

## Related Documentation

- [Repository Category Taxonomy](../../../schemas/taxonomy/repository-category/v1.0.0/category-key.schema.json) - Canonical category definitions
- [Repository Categories Config](../../../config/taxonomy/repository-categories.yaml) - Category metadata
- [Fulmen Ecosystem Guide](../../architecture/fulmen-ecosystem-guide.md) - Overview of repository types
- [Repository Structure Standards](../repository-structure/README.md) - Directory structure patterns by language
- [Publishing Standards](../publishing/README.md) - Artifact publishing standards

---

**Status**: Active (v0.2.26+)

**Maintainers**: Crucible Team
