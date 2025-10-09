---
title: "AI Agent Collaboration Standard"
description: "Required documentation and identity conventions for AI assistants working in Fulmen repositories"
author: "Codex Assistant"
date: "2025-10-02"
last_updated: "2025-10-02"
status: "draft"
tags: ["standards", "ai", "agents", "collaboration"]
---

# AI Agent Collaboration Standard

## Purpose

Define the documentation, identity, and communication requirements that allow Fulmen AI assistants to collaborate safely alongside human maintainers.

## Required Artifacts

Every repository MUST include the following root-level files:

| File                             | Purpose                                                                                                                                                                    |
| -------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `AGENTS.md`                      | Startup guide for AI assistants. Lists available agentic interfaces, initialization steps, session protocols, and links to additional resources.                           |
| `MAINTAINERS.md`                 | Canonical registry of human maintainers **and** AI co-maintainers (with emoji + handle). Identifies supervision relationships and attribution formats.                     |
| `REPOSITORY_SAFETY_PROTOCOLS.md` | (Recommended) Safety boundaries, guardrails, and escalation paths for high-impact operations. May be omitted for trivial repos but should be added once automation begins. |

### AGENTS.md Expectations

- Begin with a short “Read First” section linking to interface adapters (e.g., `CLAUDE.md`, `CODEX.md`, `.cline/rules/PROJECT.md`).
- Summarize known agentic interfaces and the definitive prompt file for each.
- Provide onboarding steps, session initialization requirements, and links to SOPs (e.g., session protocols, quality gates).
- Reinforce that agent identity assignments come from `MAINTAINERS.md`.

### MAINTAINERS.md Expectations

- Separate sections for human maintainers and AI co-maintainers.
- For each AI agent include: emoji, identity name, canonical handle (e.g., `@forge-neat`), specialization, supervising human, established date, attribution template, and organization email (`noreply@3leaps.net` for internal agents).
- Note any cross-repo responsibilities and the upcoming Mattermost channel(s) for inter-agent communication.

### Safety Protocols

`REPOSITORY_SAFETY_PROTOCOLS.md` documents dangerous operations, required approvals, rollback steps, and infrastructure guardrails. When omitted, the maintainer team MUST justify its absence in `README.md` or `MAINTAINERS.md`.

## Identity & Communication Rules

- **Canonical Handle**: Each AI agent must have a stable handle (`@code-scout`, `@arch-eagle`, etc.) recorded in `MAINTAINERS.md`. Handles double as Mattermost usernames for inter-agent communication.
- **Emoji Identifier**: Pair every agent with an emoji to aid quick recognition in chat threads and attribution lines.
- **Mattermost Readiness**: Agents should list their default Mattermost channel (e.g., `#agents-crucible`) once collaboration hubs are established.
- **Attribution**: Follow the [Agentic Attribution Standard](agentic-attribution.md) for commit messages, Co-Authored-By lines, and supervision references.

## Interface Adapter Files

Repositories may provide interface-specific adapters (e.g., `CLAUDE.md`, `CODEX.md`, `.cline/rules/PROJECT.md`). Each adapter should:

1. Point back to `AGENTS.md` and `MAINTAINERS.md`.
2. Describe any interface-specific constraints (token budgets, command allowlists, etc.).
3. Emphasize that agent identity is sourced from `MAINTAINERS.md` regardless of interface.

## Folder Conventions

- `.plans/` remains gitignored for local planning between agents and humans.
- Interface configuration (e.g., `.cline/rules/`) may sit in hidden directories so long as they link back to the AGENTS standard.

## Adoption Checklist

- [ ] `AGENTS.md` created with interface table + onboarding steps
- [ ] `MAINTAINERS.md` lists human + AI maintainers (emoji + handle)
- [ ] `REPOSITORY_SAFETY_PROTOCOLS.md` created or documented as intentionally absent
- [ ] Interface adapters reference `AGENTS.md`
- [ ] Agent handles registered in Mattermost (or placeholder noted)

## Related Documents

- [Agentic Attribution Standard](agentic-attribution.md)
- [Repository Structure SOP](../sop/repository-structure.md)
- [Release Checklist Standard](release-checklist-standard.md)
- `AGENTS.md`/`MAINTAINERS.md` templates in long-lived repos (e.g., `goneat`, `brooklyn-mcp`)
