---
title: "FulmenHQ Makefile Standard"
description: "Baseline make targets every Fulmen repository must implement"
author: "Codex Assistant"
date: "2025-10-02"
last_updated: "2025-10-10"
status: "approved"
tags: ["standards", "build", "cicd", "make"]
---

# Makefile Standard

## Why a Makefile Everywhere?

A shared `Makefile` gives Fulmen automation a consistent entry point across languages. Regardless of whether a project is Go, TypeScript, Python, or multi-language, the same targets exist so CI/CD pipelines, local scripts, and agent tooling can rely on them.

## Required Targets

## Compliance Requirements

**Full compliance** with the FulmenHQ Makefile Standard requires implementation of all required make targets. This ensures consistent tooling across the ecosystem, regardless of primary language or build system.

- **Makefile as Standard Interface**: The Makefile provides the canonical entry point for all standard operations (bootstrap, lint, test, build, etc.).
- **Package.json Scripts**: Optional for local development convenience, but **must not** duplicate or override Makefile logic. If present, package.json scripts should delegate to Makefile targets (e.g., `"test": "make test"`).
- **No Direct Dependencies in Scripts**: Package.json scripts cannot express dependencies directly. For full compliance, rely on Makefile for dependency management.
- **SSOT Consumers**: Repositories that receive synchronized assets (docs, schemas, configs) from Crucible or other SSOT sources MUST expose `make sync` so CI pipelines and agents can refresh upstream artifacts consistently.

### Language-Specific Notes

- **TypeScript/JavaScript Linting**: Use Biome for linting TypeScript and JavaScript files. Goneat does not currently support these languages adequately, so the `make lint` target uses Biome for TS/JS and Goneat for other languages (Go, YAML, schemas).
- **Go Linting**: Goneat handles Go linting via golangci-lint in the appropriate module directory.

### Enforcement

- CI/CD pipelines **must** use Makefile targets exclusively.
- Local development may use either, but Makefile is preferred for consistency.
- Repositories without a Makefile implementing all required targets are considered non-compliant.

Every repository **MUST** implement the following make targets:

| Target                                  | Purpose                                                                                                          |
| --------------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| `make help`                             | List available targets with short descriptions.                                                                  |
| `make bootstrap`                        | Install external prerequisites listed in `.goneat/tools.yaml` (installers must use the tooling manifest schema). |
| `make tools`                            | Verify external tools are present; may be a no-op if none are required.                                          |
| `make sync`                             | For repositories that consume SSOT artifacts, run the canonical sync pipeline (e.g., `goneat ssot sync`).        |
| `make lint`                             | Run lint/format/style checks.                                                                                    |
| `make test`                             | Execute the full test suite.                                                                                     |
| `make build`                            | Produce distributable artifacts/binaries for current platform.                                                   |
| `make build-all`                        | Build multi-platform binaries (Linux, macOS, Windows) and generate SHA256SUMS.txt checksums.                     |
| `make clean`                            | Remove build artifacts, caches, temp files.                                                                      |
| `make fmt`                              | Apply formatting (language-specific).                                                                            |
| `make version`                          | Print current repository version from `VERSION`.                                                                 |
| `make version-set VERSION=x`            | Update `VERSION` and any derived metadata. Must call repo-appropriate scripts (e.g., `bun run version:update`).  |
| `make version-bump-{major,minor,patch}` | Bump version according to strategy (SemVer or CalVer) and regenerate derived files.                              |
| `make release-check`                    | Run the release checklist validation (tests, lint, sync scripts).                                                |
| `make release-prepare`                  | Sequence of commands to ready a release (sync, tests, version bump).                                             |
| `make release-build`                    | Build release artifacts (binaries + checksums) for distribution.                                                 |
| `make prepush`                          | Run pre-push hooks (stub for now).                                                                               |
| `make precommit`                        | Run pre-commit hooks (stub for now).                                                                             |
| `make check-all`                        | Run all checks (lint, test, typecheck).                                                                          |

Repositories may add additional targets (e.g., `make docs`, `make package`). Required targets must NOT be renamed.

## Implementation Guidance

- Allow forcing bootstrap via `make bootstrap FORCE=1` or a `bootstrap-force` alias. Document this so CI/developers can reinstall tools when needed.

- Use phony targets (`.PHONY`) for commands that do not produce physical files.
- Keep commands quiet when appropriate (prefix with `@`) but print meaningful status lines.
- `make version-set` should delegate to the language-appropriate script (e.g., `bun run version:update` in Crucible) so wrappers stay synced.
- `make bootstrap`/`make tools` should call a script that reads `.goneat/tools.yaml` (validated against `schemas/tooling/external-tools/v1.0.0/external-tools-manifest.schema.yaml`).
- For `make build-all`, implement cross-compilation for common platforms (Linux amd64/arm64, macOS amd64/arm64, Windows amd64). Output binaries to `dist/` directory and generate `SHA256SUMS.txt` with checksums for all artifacts.
- `make release-build` should depend on `build-all` and may include additional packaging steps.
- Include a `help` target that parses comments, for example:
  ```makefile
  .PHONY: help
  help:
  	@grep -E '^\.PHONY: [a-zA-Z_-]+ ##' Makefile | sed -e 's/\.PHONY: //' -e 's/ ##/\t/'
  ```
- Document deviations or extensions in `CONTRIBUTING.md`.

### SSOT Sync Hooks

- Any repository designated as an SSOT consumer MUST implement `make sync` so downstream tooling can refresh Crucible assets consistently. The target should invoke the canonical sync tool (`goneat ssot sync` in current libraries such as `pyfulmen`) and may wrap additional validation.
- Repositories MAY add post-sync helper targets (for example, `make sync-foundry-assets` in `pyfulmen`) to copy artifacts or trigger language-specific regeneration. Document these hooks alongside the primary target (see `pyfulmen/docs/development/foundry-asset-sync.md` for the reference implementation).
- The required `make sync` target should remain idempotent and safe to run in CI; auxiliary hooks can depend on it or be chained via `make sync && make sync-foundry-assets`.

### Multi-Platform Build Example

For Go-based tools, implement `build-all` as follows:

```makefile
BIN_NAME := mytool
VERSION := $(shell cat VERSION)
DIST_DIR := dist

.PHONY: build-all
build-all: ## Build multi-platform binaries and generate checksums
	@echo "Building $(BIN_NAME) $(VERSION) for multiple platforms..."
	@mkdir -p $(DIST_DIR)
	@for os in linux darwin windows; do \
		for arch in amd64 arm64; do \
			if [ "$$os" = "windows" ] && [ "$$arch" = "arm64" ]; then continue; fi; \
			echo "Building $$os/$$arch..."; \
			EXT=""; \
			if [ "$$os" = "windows" ]; then EXT=".exe"; fi; \
			GOOS=$$os GOARCH=$$arch go build -ldflags="-X main.version=$(VERSION)" -o $(DIST_DIR)/$(BIN_NAME)-$$os-$$arch$$EXT ./cmd; \
		done; \
	done
	@echo "Generating SHA256SUMS.txt..."
	@cd $(DIST_DIR) && sha256sum * > SHA256SUMS.txt
	@echo "✅ Multi-platform build complete - artifacts in $(DIST_DIR)/"
```

## Verification

- CI pipelines should call `make lint`, `make test`, and `make release-check`.
- For repositories producing binaries, CI should also call `make build-all` to verify cross-compilation works.
- `make release-check` must cover all items in `RELEASE_CHECKLIST.md` (or invoke a script that does).
- Release workflows should use `make release-build` to produce distributable artifacts with checksums.

## Relationship to Other Standards

- [Release Checklist Standard](release-checklist-standard.md) – Make targets drive release validation.
- [Repository Structure SOP](../sop/repository-structure.md) – References the requirement for a Makefile and tooling manifest.
- [CI/CD Operations SOP](../sop/cicd-operations.md) – Pipelines must invoke `make bootstrap` before validation steps.
- Language-specific coding standards can recommend additional targets (e.g., `make docs`, `make fmt-go`).

## Future Enhancements

- Provide a template `Makefile` generator in Crucible with standard implementations for `build-all` and `release:build`.
- Add CI linters to ensure required targets exist and validate multi-platform build outputs.
- Extend to support remote execution wrappers (e.g., `make plan` for infrastructure).
- Standardize artifact naming conventions and checksum formats across repositories.
