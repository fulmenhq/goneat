---
title: "Fulmen Forge Workhorse Standard"
description: "Standard structure and capabilities for Fulmen Workhorse forges - production-ready templates for robust, general-purpose applications"
author: "Fulmen Enterprise Architect (@fulmen-ea-steward)"
date: "2025-10-20"
last_updated: "2025-10-20"
status: "draft"
tags: ["architecture", "forge", "workhorse", "template", "2025.10.2"]
---

# Fulmen Forge Workhorse Standard

This document defines the standardized structure and pre-integrated capabilities for Fulmen Workhorse forges. Workhorse forges provide production-ready templates for robust, general-purpose applications (e.g., servers, workers, long-running processes) that require reliable tooling out-of-the-box. They embody the CRDL philosophy (Clone → Degit → Refit → Launch) and align with the repository category taxonomy (`workhorse` key from [category-key.schema.json](schemas/taxonomy/repository-category/v1.0.0/category-key.schema.json)).

Workhorse forges are distinguished from other categories (e.g., `cli` for command-line tools, `service` for microservices) by their focus on durable, scalable backends with emphasis on observability, config management, and error resilience. The prototype `forge-cli-pecan` evolves into `forge-workhorse-pecan` as the first canonical implementation, using tree-themed naming (`pecan` for workhorse) to delineate types without implying hierarchy.

The canonical list of forge categories and statuses is maintained in the [Repository Category Taxonomy](schemas/taxonomy/repository-category/v1.0.0/README.md); consult that before proposing new forges or changing lifecycle states.

## Scope

Applies to Workhorse-specific forge templates (e.g., `forge-workhorse-pecan`, future variants like `forge-workhorse-oak`). Excludes other categories (e.g., `cli` for interactive tools, `library` for reusable code). Forges are not SSOT repos or full applications but starters that integrate Fulmen ecosystem components (Crucible via helpers, goneat optional) to accelerate development while enforcing standards.

Core philosophy: Ship "batteries-included" templates that handle 80% of boilerplate (logging, config, telemetry, bootstrap) so users focus on business logic. No "useful" functionality (e.g., no domain-specific code); just scalable foundations.

**CDRL Guide**: Users follow Clone → Degit → Refit → Launch; see `docs/development/fulmen_cdrl_guide.md` for details.

Implementers MUST comply with ecosystem standards in Crucible's `docs/standards/` (e.g., coding conventions, API patterns, repository structure) to ensure consistency.

## Mandatory Capabilities

Workhorse forges MUST pre-integrate these ecosystem components, providing a launch-ready skeleton. Crucible access is indirect via the language-specific helper library (e.g., pyfulmen for Python forges), eliminating direct SSOT sync. Goneat is optional for DX tooling but not required for core bootstrap. All SSOT assets (Crucible, Cosmography, etc.) accessed via helper library shims; extend helpers for new SSOT (e.g., Cosmography shim for data ELT/analytics).

Implementers MUST comply with ecosystem standards in Crucible's `docs/standards/` (e.g., coding conventions, API patterns, repository structure) to ensure consistency.

1. **Helper Library Integration (Primary Bootstrap)**
   - Depend on and bootstrap via language-specific Fulmen helper library (e.g., `go install gofulmen` or `uv add pyfulmen`).
   - Use helper library's Crucible Shim for all asset access (schemas, docs, configs)—no direct Crucible sync or goneat SSOT in forges.
   - Pre-configure Three-Layer Config (embed defaults via helper, load user overrides, support BYOC), Schema Validation, and Documentation Module.
   - Include a simple `make bootstrap` script that installs the helper library and verifies Crucible access (e.g., `crucible.GetVersion()`).
   - Pin SSOT versions (e.g., Crucible v2025.10.2) in deps/lockfiles; expose via `/version` endpoint.
   - For emerging SSOT like Cosmography, use/extend helper shims (e.g., `cosmography.GetMap(id)`); no direct repo access.
   - Refer to [Fulmen Helper Library Standard](docs/architecture/fulmen-helper-library-standard.md) for integration patterns.

2. **Makefile (Mandatory)**
   - Always include Makefile following [Makefile Standard](docs/standards/makefile-standard.md).
   - Targets: `bootstrap` (helper install), `run` (CLI serve), `build` (binary), `test`, `lint`, `version-bump` (CalVer).
   - No SSOT sync targets; optional `dx` for goneat if included.
   - Ensure cross-platform (even for Python/TS via shell fallbacks).

3. **Goneat as Optional DX Tool**
   - Optionally include `.goneat/tools.yaml` for local development (e.g., linting, validation via goneat tasks)—no `ssot-consumer.yaml` or sync config.
   - Provide `make bootstrap-dx` for goneat installation if desired, but do NOT implement SSOT sync targets (`make sync-ssot` prohibited to avoid confusion with libraries).
   - Local overrides (`.goneat/tools.local.yaml`) gitignored; use only for non-Crucible tooling.
   - Refer to [Goneat Bootstrap Guide](docs/guides/bootstrap-goneat.md) for optional setup.

4. **Observability & Telemetry**
   - Pre-wire structured logging using Crucible logging schemas (SIMPLE/STRUCTURED profiles) via helper library.
   - Integrate metrics export (counters/gauges/histograms) via Telemetry/Metrics module.
   - Default middleware: Request ID correlation, severity mapping, throttling.
   - Expose health/version endpoints per API standards.
   - Refer to [Observability Logging](docs/standards/observability/logging.md) and [Telemetry/Metrics](docs/standards/library/modules/telemetry-metrics.md).

5. **Error Handling & Propagation**
   - Use standardized error types from Error Handling module (extend Pathfinder with severity/correlation).
   - Wrap errors uniformly for logging/export (JSON responses for APIs).
   - Refer to [Error Handling Standard](docs/standards/library/modules/error-handling-propagation.md).

6. **Config Path & Management**
   - Use Config Path API from helper library for discovering Fulmen/app directories.
   - Implement Three-Layer Config explicitly: Layer 1 (Crucible defaults via helper), Layer 2 (user from app dir), Layer 3 (runtime BYOC).
   - Pre-load/validate configs against schemas; support env var overrides (e.g., `FULMEN_CONFIG_HOME`).
   - Refer to [Config Path API](docs/standards/library/modules/config-path-api.md) and [Three-Layer Config](docs/standards/library/modules/three-layer-config.md).

7. **Env Var & .env Support**
   - Use a required env var prefix based on app name (e.g., `{APP_NAME}_` where APP*NAME is uppercase kebab-case from CDRL refit, default `WORKHORSE_PECAN*`).
   - Include `.env.example` with standard vars (e.g., `{APP_NAME}_PORT=8080`, `{APP_NAME}_LOG_LEVEL=info`, `{APP_NAME}_CONFIG_PATH=./config/app.yaml`); gitcommitted, user copies to `.env` (gitignored).
   - Load .env via three-layer (Layer 2: from app config dir; parse with helper or lang-native like python-dotenv).
   - In fulmen*cdrl_guide.md, instruct users to rename prefix (e.g., change `WORKHORSE_PECAN*`to`MY*APP*` in code/.env.example).
   - Validate prefix in CLI (`--env-prefix` flag optional); env vars override config (Layer 3).
   - Standard vars: Port, log level, metrics port, health port, config path; extend for app-specific.
   - Refer to Three-Layer Config for integration.

8. **Docscribe Module Integration**
   - Embed examples using docscribe module for frontmatter parsing and clean doc reads.
   - Include runtime doc serving (e.g., /docs endpoint) for self-documenting apps.
   - Refer to [Docscribe Standard](docs/standards/library/modules/docscribe.md).

9. **Standard Endpoints & Message Patterns**
   - **HTTP/gRPC Patterns**: Implement REST/gRPC backends with standard routes/methods:
     - `/health`: Liveness/readiness (JSON: `{status: "healthy", version: str}`).
     - `/version`: Full version info (integrate Crucible/SSOT versions from helper).
     - `/metrics`: Prometheus/OpenTelemetry export.
     - Error responses: JSON per [API HTTP Standards](docs/standards/api/http-rest-standards.md) (e.g., `{error: {code: str, message: str, details: any}}`).
     - gRPC: Use proto defs from Crucible schemas; unary/streaming with metadata propagation.
   - **Messages**: Structured payloads validated against schemas (e.g., log events, metrics). Use helper's Foundry for patterns (e.g., HTTP status groups, MIME types).
   - Refer to [API Standards](docs/standards/api/README.md).

10. **CLI Surface for Server Invocation**
    - Provide a standard CLI wrapper (e.g., via cobra/click/argparse) for backend server:
      - `workhorse-pecan serve [flags]`: Starts server.
      - Standard flags: `--config <path>` (Three-Layer), `--port <int>`, `--log-level <str>` (trace/debug/info/warn/error), `--metrics-port <int>`, `--health-port <int>`, `--env-prefix <str>` (default from app name), `--version` (print and exit), `--help`.
      - Subcommands:
        - `serve` (default): Starts server.
        - `version` / `version --extended`: Basic version; extended shows full info (app version, SSOT/Crucible versions from helper, build date, git commit).
        - `envinfo`: Dumps effective env vars (filtered by prefix), config layers, SSOT versions (helpful for debugging).
        - `doctor`: Runs checks/scaffolding (health self-test, config validation, missing deps, suggest fixes; e.g., "Missing .env? Copy .env.example").
        - `health`: Self-check (mirrors /health endpoint).
      - Defaults from Crucible configs via helper; env var overrides (e.g., `PORT=8080`); load .env via three-layer.
    - Integrate with helper library for config/logging/SSOT info.
    - Refer to [Repository Structure: Workhorse](docs/standards/repository-structure/typescript/workhorse.md) for patterns; see goneat CLI for version/envinfo/doctor examples.

11. **CI/CD & Release Readiness**
    - Include basic GitHub Actions for lint/test/build.
    - Pre-commit hooks via helper library or simple scripts (no goneat dependency).
    - Versioning: CalVer support with `make version-bump`.
    - Refer to [Repository Lifecycle](docs/standards/repository-lifecycle.md).

## Runtime Patterns

Workhorse forges MUST implement these for production reliability:

- **Graceful Shutdown**: Handle OS signals (SIGINT/SIGTERM) with context cancellation; allow clean resource cleanup (e.g., DB connections, goroutines in Go; asyncio in Python).
- **Monitoring & Debug**: Expose /debug/pprof (Go) or equivalent; integrate with observability for runtime metrics (CPU/memory, goroutine count).
- **Migration Support**: If DB-integrated, include migration tooling (e.g., goose for Go, alembic for Python); config flag for auto-migrate on start.
- **DB Integration**: Standard config for connections (DSN via three-layer); pool management; health checks for DB readiness.

Refer to language standards for implementation (e.g., Go context, Python signal handling).

## UX Support

- **Browser-Based UX Flag**: Config `enable_ui: bool` (default false) to toggle /ui endpoint (e.g., simple dashboard for metrics/health using embedded assets).
- If enabled, serve static UI from /ui (use helper's Documentation for dynamic content); secure with auth if production.
- CDRL guide: Instruct on enabling/customizing UI post-refit.

## Refactoring Guide (Sumpter Example)

To refactor existing tools like sumpter (Go CLI for data extraction/inspection) to workhorse standard:

- **Helper Integration**: Add gofulmen dep; replace direct schemas/docs with Crucible Shim (e.g., load extract schemas via `crucible.LoadSchema("extract", "v0.1.0", "record-match")`); use three-layer for config (embed defaults, user .yaml, env).
- **Remove Direct SSOT**: Eliminate goneat SSOT sync; embed via gofulmen (internal/assets now from helper).
- **Standardize CLI**: Sumpter already has version/envinfo/doctor—extend version --extended for SSOT/build info; add serve subcommand for backend mode (e.g., API for extract/validate).
- **Add Endpoints**: Implement /health, /version, /metrics; /extract (POST for recipes) using gRPC/HTTP patterns.
- **Env/ .env**: Adopt {APP*NAME}* prefix (e.g., SUMPTER\_); add .env.example with DSN/log-level.
- **Runtime**: Add graceful shutdown (context in main); pprof on /debug; migration if DB (sumpter has none, but future-proof).
- **UX**: Config enable_ui for /ui (e.g., inspect results dashboard).
- **Makefile**: Ensure compliance (bootstrap installs gofulmen, no sync).

This refactoring reduces boilerplate, aligns with ecosystem, adds server capabilities without breaking CLI focus.

4. **Observability & Telemetry**
   - Pre-wire structured logging using Crucible logging schemas (SIMPLE/STRUCTURED profiles).
   - Integrate metrics export (counters/gauges/histograms) via Telemetry/Metrics module.
   - Default middleware: Request ID correlation, severity mapping, throttling.
   - Expose health/version endpoints per API standards.
   - Refer to [Observability Logging](docs/standards/observability/logging.md) and [Telemetry/Metrics](docs/standards/library/modules/telemetry-metrics.md).

5. **Error Handling & Propagation**
   - Use standardized error types from Error Handling module (extend Pathfinder with severity/correlation).
   - Wrap errors uniformly for logging/export (JSON responses for APIs).
   - Refer to [Error Handling Standard](docs/standards/library/modules/error-handling-propagation.md).

6. **Config Path & Management**
   - Use Config Path API for discovering Fulmen/app directories.
   - Pre-load configs via Three-Layer pattern; validate against schemas.
   - Support env var overrides (e.g., `FULMEN_CONFIG_HOME`).
   - Refer to [Config Path API](docs/standards/library/modules/config-path-api.md).

7. **Docscribe Module Integration**
   - Embed examples using docscribe module for frontmatter parsing and clean doc reads.
   - Include runtime doc serving (e.g., /docs endpoint) for self-documenting apps.
   - Refer to [Docscribe Standard](docs/standards/library/modules/docscribe.md).

8. **CI/CD & Release Readiness**
   - Include basic GitHub Actions or equivalent for lint/test/build.
   - Pre-commit hooks via goneat (format, validate schemas).
   - Versioning: CalVer support with `make version-bump`.
   - Refer to [Repository Lifecycle](docs/standards/repository-lifecycle.md).

## Directory Structure

Workhorse forges MUST follow this skeleton for consistency (Python example; adapt for Go/TS):

```
forge-workhorse-pecan/
├── .goneat/                          # Optional DX tooling only
│   ├── tools.yaml                    # Linting/validation (no SSOT)
│   └── tools.local.yaml.example      # Local template
├── .github/
│   └── workflows/                    # CI/CD pipelines (test, release)
│       ├── ci.yaml
│       └── release.yaml
├── src/
│   └── workhorse_pecan/              # Main entrypoint (CLI + server)
│       ├── __init__.py
│       ├── main.py                   # CLI surface: serve, version, health, envinfo, doctor
│       └── internal/                 # App logic (server handlers, config loaders)
├── config/
│   └── workhorse-pecan.yaml          # App-specific defaults (Layer 2/3; Layer 1 via helper)
├── docs/
│   ├── README.md                     # Forge overview
│   ├── DEVELOPMENT.md                # Local setup, contribution
│   └── development/
│       └── fulmen_cdrl_guide.md      # CDRL instructions for users (naming/root conventions)
├── pyproject.toml                    # Deps: pyfulmen (mandatory), etc.
├── .env.example                      # Standard env vars (committed)
├── Makefile                          # Standard targets (bootstrap, run, build, test)
└── .gitignore                        # Ignore .env, builds, local configs
```

- **Primary Entry**: `src/<app>/main.py` launches the workhorse.
- **No Direct Synced Assets**: All Crucible/SSOT via helper library shims; no `docs/crucible-*`, `config/crucible-*`, or SSOT folders.
- **No Domain Code**: Placeholders (e.g., echo server in internal/server.py) to demonstrate; users refit via CDRL.

## Bootstrap Strategy

Forges bootstrap like libraries but add app-specific setup:

1. **Initial Clone**: Users `git clone` or degit; run `make bootstrap` to install goneat + deps.
2. **Sync Assets**: `make sync` pulls/updates Crucible.
3. **Launch**: `make run` starts with defaults; `make build` for production binary.
4. **Local Overrides**: `.goneat/tools.local.yaml` for dev iteration.

**Important**: Use symlinks for local tools; ensure `type: link` creates proper links (see helper library pitfalls).

## Testing Expectations

- Unit: Cover config loading (three-layer), logging setup, error wrapping (70%+).
- Integration: End-to-end via helper (asset access, validation); mock telemetry.
- E2E: Basic health checks, config validation, CLI invocation.
- Coverage: Enforce via CI; align with module manifest thresholds.

## Documentation Requirements

- **README.md**: CDRL guide (link to fulmen_cdrl_guide.md), quick start (`make bootstrap && make run`), ecosystem links.
- **DEVELOPMENT.md**: Local workflow, testing, release process.
- **Architecture Overview**: `docs/workhorse-overview.md` with module catalog template (adapt from helper standard).
- **Examples**: Inline code for config/telemetry setup.
- **Frontmatter**: All docs use YAML headers for metadata.

Adapt helper library template:

```markdown
# Workhorse Forge Overview

## Purpose & Scope

- Template for durable backends; pre-wires observability/config.

## Integrated Modules

| Module        | Tier | Summary      | Spec Link |
| ------------- | ---- | ------------ | --------- |
| crucible-shim | Core | Asset access | [link]    |
| ...           | ...  | ...          | ...       |

## Launch Guide

- Follow fulmen_cdrl_guide.md → `make bootstrap` → `make run`
```

## Version Alignment & Lifecycle

- Pin helper libraries/Crucible versions.
- CalVer releases; changelog via release-notes.
- Lifecycle: `prototype` → `stable` per taxonomy.

## Common Pitfalls & Solutions

- **Missing Sync**: Always run `make sync` post-clone.
- **Dep Conflicts**: Lock helper lib versions.
- **Over-Refit**: Keep domain placeholders minimal.

## References

- [Fulmen Helper Library Standard](fulmen-helper-library-standard.md)
- [Repository Category Taxonomy](schemas/taxonomy/repository-category/v1.0.0/README.md)
- [Technical Manifesto](fulmen-technical-manifesto.md)
- Prototype: forge-cli-pecan

## Changelog

- **2025-10-20**: Initial draft for workhorse category.
