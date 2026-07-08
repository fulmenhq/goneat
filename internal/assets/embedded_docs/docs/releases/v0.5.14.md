# Goneat v0.5.14 - Scoped Scans and Dependency Maintenance

**Release Date**: 2026-07-07
**Status**: Stable

## TL;DR

- **Security scans now honor ignore scope before gosec runs**. Ignored nested modules and generated package directories are pruned before gosec receives package input.
- **Go vulnerability scans use the module graph**. `dependencies --vuln` now feeds grype a CycloneDX SBOM generated from `go list -m -json all` for Go projects instead of recursively cataloging the workspace.
- **Generated defaults are shared**. `.cache/`, `bin/`, `dist/`, `sbom/`, and `vendor/` are part of goneat's default ignore model for categories that use the unified matcher.
- **`goneat init` ships starter `.goneatignore` templates from source**. New projects get a committed scan-policy file with the common generated/tooling directories.
- **Vulnerability findings now include provenance**. Reports and JSON issues carry `source_type` and `source_path` for graph, SBOM-file, and fallback file-walk scans.
- **`make install` replaces existing binaries more reliably**. The install target removes the target binary before copying the newly built one, avoiding platform-specific replacement failures.
- **Go modules are refreshed while keeping the public Go floor**. Dependency updates include `golang.org/x/crypto` v0.53.0, `golang.org/x/net` v0.56.0, `github.com/go-git/go-git/v5` v5.19.1, `github.com/open-policy-agent/opa` v1.18.2, and the OpenTelemetry OTLP trace HTTP exporter v1.44.0. `go.mod` remains at `go 1.25.0`.

## What Changed

### Security Scope

Gosec module discovery now uses goneat's unified ignore matcher. Ignored directories are pruned before nested `go.mod` files are discovered, and package directories returned by `go list` are filtered before gosec is invoked.

`--no-ignore` remains the explicit escape hatch for full-scope security discovery. `--force-include` can re-include concrete ignored descendants without reopening every generated directory for broad glob patterns.

### Dependency Vulnerability Scope

For Go projects, vulnerability scanning now derives SBOM input from the root module graph using `go list -m -json all`. This prevents dependency caches, copied release binaries, generated SBOM output, and dependency example trees from being scanned as if they were project dependencies.

Explicit `--sbom-input` scans are reported as `source_type=sbom-file`. Go graph scans are reported as `source_type=go-module-graph`. Non-Go fallback scans remain `source_type=file-walk` and receive generated-dir plus ignore-derived syft excludes unless `--no-ignore` is set.

For Go roots, the graph path is intentional: `--no-ignore` and `--force-include` do not turn `dependencies --vuln` into a full recursive workspace scan. Use an explicit SBOM input when auditing a different artifact set.

### File Selection Documentation

The new app note at `docs/appnotes/file-selection-and-ignore-semantics.md` documents ignore sources, precedence, `--no-ignore`, `--force-include`, and a tool matrix covering gosec, govulncheck, gitleaks, syft, grype, golangci-lint, Biome, Ruff, yamllint, shellcheck/shfmt, and Go module graph scans.

### Starter `.goneatignore`

The `goneat init` templates now live under source `templates/goneatignore/` and are regenerated into embedded assets. Universal defaults include the shared generated/tooling set: `.cache/`, `bin/`, `dist/`, `sbom/`, and `vendor/`.

### Install Target Replacement

`make install` now removes `$(INSTALL_DIR)/$(BINARY_NAME)` before copying `dist/goneat` into place. This keeps local install workflows reliable on systems that can reject direct overwrite of an existing executable.

### Dependency Updates

The Go module graph has been refreshed to pick up current security and compatibility updates while preserving the downstream module floor:

- `golang.org/x/crypto` v0.51.0 -> v0.53.0
- `golang.org/x/net` v0.53.0 -> v0.56.0
- `github.com/go-git/go-git/v5` v5.19.0 -> v5.19.1
- `github.com/open-policy-agent/opa` v1.12.3 -> v1.18.2
- `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` v1.43.0 -> v1.44.0

The module directive remains `go 1.25.0`; no dependency in the updated graph declares a higher Go version requirement.

## Upgrade Notes

Most repositories need no config change. Repositories with generated directories or local dependency caches should see fewer false high/critical findings from ignored paths.

Teams should keep `.goneatignore` committed as a scan-policy layer, even though goneat now respects ordinary `.gitignore` for the fixed security and dependency paths. This makes scan scope explicit for archives, CI workspaces, and future tool integrations.

If you use `make install` from a local checkout, no workflow change is required. The target still expects `dist/goneat` to exist; use `make build install` when you want rebuild plus install in one command.
