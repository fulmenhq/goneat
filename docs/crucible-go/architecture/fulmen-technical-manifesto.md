---
title: "Fulmen Technical Manifesto"
description: "Principles, architecture patterns, and quality bars that define the Fulmen ecosystem"
author: "Schema Cartographer"
date: "2025-10-10"
last_updated: "2025-10-10"
status: "draft"
tags: ["fulmen", "architecture", "manifesto", "2025.10.2"]
---

# Fulmen Technical Manifesto

## Why This Manifesto Exists

Fulmen’s promise: **Thrive on Scale**—start fast with systems engineered for enterprise reliability and growth. This manifesto codifies our commitments across the evolved ecosystem: from CRDL-optimized forges to Crucible SSOT, language foundations, tools like goneat/brooklyn-mcp, and AI-human collaboration (via MAINTAINERS.md roles). Refined over eight months from template frustration to a flywheel preventing repeated pains (e.g., logging, schemas), it guides building without silos.

## Core Beliefs

### 1. Production First, Refitting Second

- Ship complete, running systems—not starter kits. Every forge must boot with auth,
  observability, testing, and deployment stories already working.
- Refitting (removing or reshaping) is faster than inventing from scratch. Design docs,
  toggles, and blueprints anticipate the first waves of customization.

### 2. Standards Drive Speed

- Crucible is the SSOT for schemas, terminology, process, and documentation. If it is not codified there, it does not exist.
- Tooling (goneat, scripts, guardian hooks) automates compliance with those standards.
  Manual checklists are a failure mode.

### 3. Type Safety and Contracts Everywhere

- Prefer compile-time guarantees (TypeScript strict mode, Go static analysis, Python
  typing + runtime validation) and backstop with schema validation.
- Foundry catalogs (patterns, HTTP status groups, countries, MIME types) capture shared
  domain knowledge so libraries can enforce the same rules across languages.

### 4. Observability is Table Stakes

- Logging, metrics, tracing, and diagnostics are built in—never optional add-ons.
- Default dashboards and log schemas live in Crucible. Forges wire those defaults on
  day zero.

### 5. Collaborative Human-AI Stewardship

- AI agents (e.g., Schema Cartographer for standards, Pipeline Architect for workflows) partner with human roles (dev, devsecops, docs, architects per MAINTAINERS.md) under unified protocols: planning in .plans/, guardian approvals, audits, and supervision.
- Rich SSOT docs (manifesto, guides, SOPs) enable AI to accelerate maintenance while upholding safety—e.g., refusing malicious code, enforcing quality gates—fostering a model where humans innovate and AI ensures consistency at scale.

## Architecture Tenets

| Layer              | Tenet                        | Implementation                                                                                                                |
| ------------------ | ---------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| **Crucible**       | SSOT over synchronization    | Author once; sync to language wrappers via `bun run sync:to-lang`; downstream consumers pull tagged releases.                 |
| **Tooling**        | Automated governance         | `goneat assess`, guardian approvals, make targets—even release notes creation—are scripted.                                   |
| **Libraries**      | Idiomatic by default         | `*fulmen` packages wrap Crucible assets in language-native APIs, follow coverage targets, and expose the same module surface. |
| **Forges**         | Production from commit zero  | Templates include AAA, CI/CD, packaging, deployment docs, and sample telemetry wiring before customization.                   |
| **Ops & Services** | Observability and resilience | brooklyn-mcp, data utilities, and supporting services integrate with shared logging schemas and guardian policies.            |

## Technical Pillars in Practice

### Schemas & Contracts

- CalVer releases ensure compatibility expectations (e.g., `2025.10.x`).
- Schema evolution follows “additive first” principle—removals require deprecation
  windows and documentation.
- `scripts/validate-schemas.ts` enforces catalog integrity before any build/test run.

### Code Quality & Testing

- Minimum coverage expectations: `gofulmen` ≥95%, `pyfulmen` ≥90%, `tsfulmen` ≥85%.
- Forges embed per-layer testing (unit, integration, E2E) with CI pipelines reused
  across downstream deployments.
- Goneat’s guardian prohibits commits when quality gates are skipped or fail.

### Security & Compliance

- Zero-trust defaults: config layering, secret handling patterns, RBAC and audit trails
  defined in Crucible.
- Build-in AAA scaffolding (even if disabled) so production upgrades are additive.
- Guardian risk classifications align with `docs/standards/repository-safety-framework.md`.

### Observability & Operations

- Logging schemas (`config/observability/logging/`) and telemetry standards are consumed
  by libraries and forges.
- Release process includes generating per-release notes and verifying downstream sync.
- MCP, CLI, and guardrails use shared metrics to monitor ecosystem health.

## How to Apply the Manifesto

1. **Start with Standards**: Before writing code, check Crucible and Substaile docs.
   Submit missing standards upstream rather than re-implementing locally.
2. **Automate Everything**: Extend goneat, make targets, or scripts when you find manual
   tasks. Automation is the default path to compliance.
3. **Publish the Blueprint**: Update changelog, release notes, ecosystem guide, and
   relevant standards whenever you introduce new capabilities.
4. **Refit Responsibly**: When customizing a forge, document divergence, add tests for
   new behavior, and keep telemetry + AAA in place.
5. **Partner with AI Agents**: Provide context (plans, docs) so Schema Cartographer and
   peers can execute safely. Treat AI outputs as first-class contributions subject to the
   same review rigor.

## The Road Ahead

- **Templates**: Expand forge coverage (CLI, background services, multi-tenant portals)
  and align them with the refreshed standards.
- **Libraries**: Deliver parity across languages and bring in additional modules
  (e.g., health reporting, data catalogs).
- **Tooling**: Evolve goneat tasks, finalize the Fulward approval workflow, add
  release-note generation, and automate crucible-to-consumer sync pipelines.
- **Documentation**: Add other external-facing, SSOT information archives that further
  accelerate the mission of FulmenHQ.

Fulmen is a living system centered at [FulmenHQ](https://github.com/fulmenhq). This manifesto keeps everyone—from humans to AI agents—aligned
on the bar we set for ourselves and the promises we make to teams adopting the ecosystem.

> **Trademark Notice**: “Fulmen” and “FulmenHQ” are trademarks of 3 Leaps LLC. Refer to
> `README.md` and `LICENSE` for usage guidelines when creating derivatives or new
> distributions.
