---
title: "Fulmen Ecosystem Guide"
description: "How the Fulmen ecosystem fits together—from schemas and tooling to libraries, forges, and operational standards"
author: "Schema Cartographer"
date: "2025-10-10"
last_updated: "2025-10-10"
status: "draft"
tags: ["fulmen", "architecture", "ecosystem", "2025.10.2"]
---

# Fulmen Ecosystem Guide

## Thrive on Scale: The Fulmen Ecosystem

Fulmen empowers teams to build enterprise systems that start fast and scale effortlessly—delivering lightning-speed development without sacrificing reliability or maintainability. Evolving from frustration with trivial boilerplate to a comprehensive ecosystem, Fulmen now integrates single-source-of-truth (SSOT) standards via Crucible, language-specific libraries, automation tools, production-ready forges, and AI-human collaboration to eliminate repeated wheel-reinvention in logging, schemas, enums, and more. Future expansions like Cosmography will extend SSOT to data modeling and topology.

**Core Layers** (building a flywheel: standards → libraries → tools → applications):

1. **SSOT Standards & Schemas (Crucible)**: Authoritative contracts, policies, docs, processes, coding rules (e.g., TS barrel exports, logging adapters), and devsecops (pre-commit gates, security scans)—ensuring persnickety excellence and zero silos.
2. **Automation Tooling (goneat, Fulward)**: CLI-driven validation, formatting, sync, and approval workflows for seamless governance.
3. **Language Foundations (`gofulmen`, `pyfulmen`, `tsfulmen`; Rust/C# planned)**: Idiomatic APIs wrapping Crucible assets, solving cross-project pains like config paths, schema validation, and observability.
4. **Fulmen Forges (Templates)**: Complete, functional starters (CLI, portal, runner) embodying CRDL (Clone → Degit → Refit → Launch) for rapid, scale-ready launches.
5. **Operational Services (brooklyn-mcp, data utilities)**: MCP orchestration, observability defaults, and AI agents (e.g., Schema Cartographer) for collaborative maintenance, per MAINTAINERS.md roles (dev, devsecops, docs, architects).

Layers compose into a virtuous cycle: Refine standards in Crucible → Sync to libraries → Automate via tools → Deploy via forges → Iterate with AI oversight. This prevents siloed reinvention, enabling teams to focus on unique value while thriving at enterprise scale.

## Evolution: From Templates to Ecosystem Flywheel

| Phase               | Timeline     | Key Evolution                                                                                                                | Outcomes                                                                                                              |
| ------------------- | ------------ | ---------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| **Inception**       | Feb–Apr 2025 | Frustration with boilerplate → Build functional forges (cockpit, runner, portal) via CRDL (Clone → Degit → Refit → Launch).  | Proven "complete systems" accelerate teams to production-scale without basics drudgery.                               |
| **Standards Forge** | May–Jul 2025 | Realized repeated pains (logging, schemas, enums) → Crucible as SSOT for contracts/docs/processes; prototype goneat/Fulward. | Centralized standards prevent reinvention; early libs (gofulmen) emerge for idiomatic access.                         |
| **Consolidation**   | Aug–Sep 2025 | Ecosystem sprawl → Align libs (pyfulmen, tsfulmen), tools (brooklyn-mcp), AI co-maintainers; draft Substaile coding norms.   | Module parity across langs; MCP integrates workflows; human-AI roles defined in MAINTAINERS.md.                       |
| **Scale Wave**      | Oct 2025+    | Full flywheel → Foundry catalogs, CalVer releases, Cosmography (data SSOT); AI stewardship for maintenance.                  | Multi-lang readiness; forges like forge-cli-pecan leverage ecosystem; "Thrive on Scale" realized via composed layers. |

## Ecosystem Map

```mermaid
graph TD
  A[Fulmen Ecosystem] --> B[Crucible SSOT]
  A --> C[Tooling & Automation]
  A --> D[Helper Libraries]
  A --> E[Forges & Templates]
  A --> F[Operational Services]

  B --> B1[Schemas]
  B --> B2[Standards & SOPs]
  B --> B3[Docs & Guides]

  C --> C1[goneat CLI]
  C --> C2[Fulward Approvals]
  C --> C3[Sync & Validation Scripts]

  D --> D1[gofulmen]
  D --> D2[pyfulmen]
  D --> D3[tsfulmen]

  E --> E1[Fulmen Portal Forge]
  E --> E2[Fulmen Runner Forge]
  E --> E3[Forge CLI (Pecan)]

  F --> F1[brooklyn-mcp]
  F --> F2[Data wrangling utilities]
  F --> F3[Observability / Substaile patterns]
```

## Layer-by-Layer Details

### 1. Standards & Schemas (Crucible)

- **Purpose**: Authoritative definition of everything Fulmen depends on—logging,
  configuration, taxonomy, helper modules, and foundry catalogs.
- **Key Assets**: `schemas/`, `docs/standards/`, `config/`, language sync targets.
- **Quality Gates**: `make check-all` (schema validation, linting, tests), Fulward
  enforcement on commits, CalVer release cadence with release notes.
- **Why it matters**: Downstream projects treat Crucible as the SSOT; language wrappers
  embed its assets so that templates and services stay consistent.

### 2. Tooling & Automation

- **goneat**: Fulmen-native CLI for formatting, linting, schema validation, and hook
  orchestration. v0.3.0 introduces stricter assessments; v0.4.x will slim the binary as
  approval workflows move elsewhere.
- **Fulward**: Successor to the guardian prototype. Resides in `../fulward` and
  centralizes approval policies for commits, releases, and other level-one operations.
  goneat retains a `guardian` subcommand temporarily until Fulward reaches general
  availability.
- **Scripts**: `scripts/validate-schemas.ts`, `scripts/sync-to-lang.ts`, and version
  tooling keep the SSOT and wrappers synchronized.
- **Future**: Additional tasks for release note generation and environment bootstraps.

### 3. Helper Libraries (`*fulmen`)

- **Repositories**: `gofulmen`, `pyfulmen`, `tsfulmen`, with planned `rsfulmen` and
  `csfulmen`.
- **Core Modules**: Config path API, three-layer config, schema validation, Crucible
  shim, SSOT sync, observability logging, foundry catalogs (patterns, HTTP statuses,
  countries, MIME types).
- **Testing Targets**: Coverage thresholds driven by `config/library/v1.0.0/module-manifest.yaml`.
- **Role**: Provide idiomatic APIs so applications don’t need to read raw YAML/JSON
  from Crucible.

### 4. Fulmen Forges (Templates)

- **Current**: `fulmen-portal-forge`, `fulmen-runner-forge`, `fulmen-cockpit`, and the
  refreshed `fulmen-forge-cli-pecan` (maintaining the tree-themed taxonomy while
  aligning with repository categories).
- **Philosophy**: Clone → degit → refit → launch. Ship production-ready systems with
  AAA, observability, testing, deployment pipelines, and docs from the start.
- **Gymnasium Experiments**: Internal playgrounds—nicknamed “gymnasiums”—bundle TUIs,
  code-metric analyzers, and other exploration tools. Candidates such as
  “forge-gymnasium-ginkgo” may emerge once the components are modularized.
- **Roadmap**: Expand to more functional verticals (workflow automation, API gateway,
  data ingestion) and integrate Crucible/goneat flows by default. Tie forge families to
  `config/taxonomy/repository-categories.yaml` so every template advertises its role
  using the shared taxonomy.
- **Docs**: Upcoming revisions will align each forge README with this guide and the
  technical manifesto.

### 5. Operational Services & AI Enablement

- **brooklyn-mcp**: MCP server that orchestrates Fulmen workflows and integrates with AI
  agents and IDEs.
- **Data utilities**: Wrangling tools, validators, and scaffolding to prepare data for
  Fulmen-based services.
- **Substaile**: Standards for documentation-enforced coding conventions (currently
  living in prototype form, with portions merged into Crucible’s coding standards).
- **AI Agents**: Schema Cartographer, Pipeline Architect, and future specialists operate
  with documented protocols (`AGENTS.md`, `MAINTAINERS.md`) to maintain the ecosystem.

## How Things Sync

| Source           | Sync Target                  | Command                               | Notes                                                        |
| ---------------- | ---------------------------- | ------------------------------------- | ------------------------------------------------------------ |
| Crucible root    | `lang/*` docs/config/schemas | `bun run sync:to-lang`                | Keeps Go, Python, TypeScript wrappers aligned.               |
| Crucible configs | Downstream libraries         | `goneat ssot pull` _(planned)_        | Future automation for consumers to update assets.            |
| Forges           | Project codebases            | `degit` or repo clone scripts         | Templates will pull latest Crucible snapshots during setup.  |
| Docs             | Ecosystem portals            | `docs/` publishing pipeline (planned) | Consolidated site similar to Codex, powered by this content. |

## Working Across the Ecosystem

1. **Author standards in Crucible** → run `make check-all` → release (CalVer) → generate
   release notes.
2. **Sync language wrappers** → cut releases of `*fulmen` libraries once they pass their
   own `make check-all`.
3. **Update forges** → incorporate new library releases, add instructions for clone →
   degit → refit flows.
4. **Validate automation** → ensure goneat tasks and Fulward policies support the new patterns.
5. **Communicate** → document in ecosystem guide/manifesto, publish release notes, loop
   in AI agents for maintenance.

## Where This Content Lives

- `docs/architecture/fulmen-technical-manifesto.md` – Deep dive on philosophy, design
  principles, and implementation examples.
- `docs/guides/` – Task-oriented docs (bootstrap guides, sync instructions).
- `docs/standards/` – Normative requirements (logging, library modules, repository
  safety).
- `release-notes/` – Per-release highlights.

Fulmen continues to be the fastest path from zero to production-grade systems—made even
stronger now that the ecosystem shares a single source of truth, automation toolkit, and
AI-assisted stewardship. Cosmography is next in line to follow Crucible’s public launch,
extending the SSOT strategy into spatial and mapping domains.

## Next Steps

- Complete forge refreshes (including `forge-cli-pecan`) using the updated library
  modules and guardian policies.
- Expand Substaile into a formal Crucible module or dedicated repository for coding
  standards.
- Automate CalVer release note generation from the checklist.
- Publish an ecosystem landing page synthesizing this guide for external readers.

Fulmen continues to be the fastest path from zero to production-grade systems—made even
stronger now that the ecosystem shares a single source of truth, automation toolkit, and
AI-assisted stewardship.
