# Goneat v0.3.15 — Hook Command Execution (Draft)

**Status**: Draft

## Highlights

- Hook manifests now execute all declared commands (including external ones) in priority order.
- Git hook templates read the manifest directly; CLI args in generated scripts no longer override manifest categories.
- Guidance added to avoid running stateful `make` targets from hooks to prevent self-triggered loops.

## Upgrade Notes

- Update `.goneat/hooks.yaml` to use assess-based, check-only steps for hooks. Example:
  - `pre-commit`: `assess --categories format,lint,security --fail-on critical --package-mode`
  - `pre-push`: `assess --categories format,lint,security,dependencies,dates,tools,maturity,repo-status --fail-on high --package-mode`
- Regenerate hooks after updating the manifest: `goneat hooks generate && goneat hooks install`.
- Avoid invoking `make format-all`, `make verify-embeds`, or other workspace-mutating targets from hooks; they can dirty the tree mid-hook and create perceived loops.

---

# Goneat v0.3.15 — Lint Expansion & Hook Execution

**Release Date**: 2025-12-11
**Status**: Release

## TL;DR

- **Expanded Lint Coverage**: Added shell script (shfmt/shellcheck), Makefile (checkmake), and GitHub Actions (actionlint) linting
- **Hook Manifest Execution**: `goneat assess --hook` now executes ALL commands in hooks.yaml (make, assess, etc.) in priority order
- **Yamllint Integration**: Configurable YAML linting with strict mode for workflows and configs
- **DX Improvements**: Better error messages and graceful tool handling

## What's New

### Expanded Lint Capabilities

The `goneat assess --categories lint` command now includes comprehensive linting for:

- **Shell Scripts**: `shfmt` (BSD-3, format+fix) and `shellcheck` (GPL-3, verify-only)
- **Makefiles**: `checkmake` (MIT, comprehensive Makefile validation)
- **GitHub Actions**: `actionlint` (MIT, workflow validation)
- **YAML Files**: `yamllint` with configurable paths and strict mode

Tools are bundled in the goneat-tools container for CI environments. Local development can install them via `goneat doctor tools --install --tools shfmt,actionlint,checkmake,yamllint`.

### Hook Manifest Execution

**BREAKING CHANGE FOR HOOK USERS**: Hook manifests now execute ALL commands, not just assess commands.

Previously, hooks.yaml entries like:
```yaml
hooks:
  pre-commit:
    - command: "make"
      args: ["format-all"]
      priority: 5
```

Were silently ignored. Now they execute in priority order with timeout enforcement.

**Migration**: Update hooks to use check-only commands:
- `make format-all` → `make format-check`
- `make test` → `make test-fast`

### Yamllint Integration

Added configurable YAML linting:
- Default paths: `.github/workflows/**/*.yml`
- Configurable via `.goneat/assess.yaml`
- Strict mode for CI compliance

### Developer Experience Improvements

- **Helpful Error Messages**: `--output json` shows clear guidance to use `--format json`
- **Graceful Tool Skipping**: Missing tools skip with informative messages rather than failing
- **Container-Ready**: All new tools pre-installed in goneat-tools container

## Breaking Changes

None. All changes are additive and backwards compatible.

---

# Goneat v0.3.14 — Container-Based CI & ToolExecutor Infrastructure

**Release Date**: 2025-12-08
**Status**: Release

## TL;DR

- **Two-Path CI Validation**: Validates both container path (LOW friction) and package manager path (HIGHER friction)
- **Container-Probe**: Proves goneat works inside `ghcr.io/fulmenhq/goneat-tools` container (HIGH confidence)
- **Bootstrap-Probe**: Validates package manager installation (only runs if container passes)
- **Infrastructure**: New ToolExecutor package for future Docker-based tool execution
- **golangci-lint v2**: Updated to v2 install path and minimum version

## Breaking Changes

None. This is a feature/infrastructure release, fully backwards compatible.

## What's New

### Two-Path CI Validation

goneat now validates **two approaches** for tool availability with explicit dependency ordering:

```
build-test-lint
       ↓ (uploads goneat binary as artifact)
container-probe  ← LOW friction, validates first
       ↓ (only runs if container passes)
bootstrap-probe  ← HIGHER friction, validates second
```

**Why this matters for users**:

- Container path is THE recommended approach for CI - validate it first
- Don't waste cycles on package manager issues if container fails
- Gives confidence: "If goneat CI passes, my container-based CI will work"

### Container-Probe (Recommended for CI)

The `container-probe` job downloads the goneat binary and validates it works inside the container:

```yaml
container-probe:
  needs: [build-test-lint]
  container:
    image: ghcr.io/fulmenhq/goneat-tools:latest
  steps:
    - uses: actions/download-artifact@v4
    - run: ./goneat doctor tools --scope foundation
    - run: ./goneat format --check .
```

**Benefits**:

- **Proves goneat integration** - not just that tools exist (`--version`)
- **Same image everywhere** = same behavior (container IS the contract)
- **Pre-built tools**: prettier, yamlfmt, jq, yq, rg, git, bash

### Bootstrap-Probe (Validates Package Manager Path)

The `bootstrap-probe` job validates that `goneat doctor tools --install` works on fresh runners:

- Only runs if `container-probe` passes (strategic dependency)
- Important for users who can't use containers
- Future: Could expand to matrix (ubuntu, macos, windows)

### ToolExecutor Package (Phase 1)

New infrastructure in `pkg/tools/executor*.go` for future Docker-based tool execution:

- `executor.go` - Interface and factory pattern
- `executor_local.go` - Local tool execution (existing behavior)
- `executor_docker.go` - Docker-based execution via goneat-tools
- `executor_auto.go` - Smart mode selection (CI → docker, local → local)

**Note**: Phase 1 only - executor created but NOT yet integrated into `cmd/format.go`. Full integration planned for v0.3.15.

### Local CI Runner Support

New Makefile targets for running CI locally:

```bash
make local-ci-format    # Run format-check using container
make local-ci-all       # Run all CI jobs
```

### golangci-lint v2

Updated install path for golangci-lint v2:

```bash
# Old (v1.x)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# New (v2.x)
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
```

Minimum version updated from 1.54.0 to 2.0.0.

## Upgrade Notes

### For CI Pipelines

No action required. The CI workflow changes are in the repository and will take effect automatically on push to main.

### For Local Development

If you want to test the container-based format checking locally:

```bash
# Requires Docker
make local-ci-format
```

### For golangci-lint Users

If you have golangci-lint v1.x installed globally, update to v2:

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
```

## Files Changed

| Category         | Files                                         |
| ---------------- | --------------------------------------------- |
| CI Workflow      | `.github/workflows/ci.yml`                    |
| Makefile         | `Makefile` (new local-ci targets)             |
| Version          | `VERSION`                                     |
| Config           | `config/tools/foundation-tools-defaults.yaml` |
| New Package      | `pkg/tools/executor*.go` (5 files)            |
| Documentation    | `docs/cicd/local-runner.md`                   |
| Config Templates | `config/cicd/`                                |

## Full Changelog

See [CHANGELOG.md](CHANGELOG.md) for complete details.

---

**Previous Releases**:

- [v0.3.13](docs/releases/v0.3.13.md) - Dynamic yamlfmt Detection & Crucible v0.2.22
- [v0.3.12](docs/releases/v0.3.12.md) - yamlfmt Compatibility Fix
- [v0.3.11](docs/releases/v0.3.11.md) - Windows Compatibility & Test Parallelization
