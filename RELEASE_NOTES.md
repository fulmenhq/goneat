# Goneat v0.5.4 — Biome Nested-Config Fixes and YAML Parse Error Clarity

**Release Date**: 2026-02-25
**Status**: Stable

## TL;DR

- **Biome monorepo fix**: `goneat format` and `goneat format --check` now run Biome from per-file resolved config context (nearest `biome.json`/`biome.jsonc`), fixing nested root config failures
- **Assess parity fix**: `goneat assess --categories format` now applies the same nested-config grouping strategy, preventing Biome JSON parse failures in nested TS/JS package layouts
- **YAML check UX fix**: yamlfmt parse errors are now reported as syntax errors instead of misleading "needs formatting" results

## What Changed

### Biome Context Resolution in `format`

Biome 2.4+ rejects certain invocations in monorepos that contain nested Biome roots.
`goneat format` previously invoked Biome from a shared working directory with outer
paths, which caused failures like:

```
Found a nested root configuration, but there's already a root configuration.
```

`goneat` now resolves invocation context per file and runs Biome from the nearest
config root directory, using a relative file argument. This fixes `format` and
`format --check` behavior in nested package layouts.

### Biome Context Resolution in `assess`

`assess --categories format` and Biome lint assess paths now group JS/TS files by
resolved Biome context and execute Biome once per group. This avoids mixed-root
invocations that previously produced non-JSON output and errors such as:

```
failed to parse biome json: no json output from biome
```

Error reporting was also improved so non-JSON Biome output is surfaced with
actionable context.

This closes the remaining gap for `make fmt-check` workflows that call assess
before falling back to `goneat format --check`.

### YAML Parse Error vs Formatting Error

`yamlfmt` exits with status 1 for both syntax errors and formatting differences.
Check mode now inspects output and classifies parse failures correctly:

- Invalid YAML: reported as syntax error with yamlfmt details
- Valid but unformatted YAML: reported as needs formatting

Detection is case-insensitive to cover variant yamlfmt output formats.

### Regression Guard: `--fix --new-issues-only`

Biome lint fix mode now executes before incremental `--new-issues-only` reporting,
preserving expected behavior for combined workflows.

## Toolchain Updates

goneat dogfoods its own `doctor tools --upgrade` infrastructure. v0.5.4 absorbed
two Go ecosystem changes that affect any project goneat assesses.

### Go: gosec 2.23.0 — Taint Analysis

gosec 2.23.0 introduced three inter-procedural taint analysis rules that are
distinct from the older file-access and subprocess rules:

| Rule | Name | Description |
|------|------|-------------|
| G702 | Command injection via taint | Exec calls with taint-traced args |
| G703 | Path traversal via taint | File ops with taint-traced paths |
| G704 | SSRF via taint | HTTP calls with taint-traced URLs |

These rules trace data flow across function boundaries. **Existing `#nosec G304`
or `#nosec G204` comments do not suppress G703/G704/G702** — the taint rules must
be listed explicitly:

```go
// Before: suppresses file-access rule, not taint analysis
if _, err := os.Stat(path); err == nil { // #nosec G304

// After: suppresses both
if _, err := os.Stat(path); err == nil { // #nosec G304 G703 - path from validated config
```

### Go: golangci-lint 2.10 — QF1012 Expansion

golangci-lint 2.10 expands QF1012 to flag `WriteString(fmt.Sprintf(…))` across
more patterns. With the default `max-same-issues: 3`, findings rotate across files
per run. Set `max-same-issues: 0` to surface all at once.

## Upgrade Notes

Drop-in replacement for v0.5.3. No config migration required.

If you are testing local changes, ensure your installed binary matches your latest
build (`make build install`) before validating downstream repositories.

## Contributors

- gemini-3.1-pro (devlead)
- Claude Opus 4.6 (devlead)
- Claude Sonnet 4.6 (releng)
- @3leapsdave (supervision)

---

# Goneat v0.5.3 — Data Validation for All JSON Schema Drafts

**Release Date**: 2026-02-09
**Status**: Stable

## TL;DR

- **Bug fix**: Data validation (`validate data`, `pkg/schema`) now accepts all five supported JSON Schema drafts (Draft-04, Draft-06, Draft-07, 2019-09, 2020-12)
- **No breaking changes**: Strictly a bug fix release

## What Changed

### Data Validation Draft Gate Fixed

In v0.5.2, goneat added meta-schema validation support for all five JSON Schema drafts. However, the **data validation** path (`goneat validate data` and `pkg/schema.ValidateFromBytes`) still rejected schemas declaring Draft-04, Draft-06, or 2019-09 in their `$schema` field.

This meant schemas that passed `goneat schema validate-schema` would fail when used to validate data — an inconsistency that blocked downstream consumers like crucible from validating configs against 2019-09 schemas.

**Before (v0.5.2)**:
```
$ goneat validate data --schema schemas/draft-2019-09/config.json data.yaml
Error: unsupported $schema: only Draft-07 and Draft-2020-12 supported
```

**After (v0.5.3)**:
```
$ goneat validate data --schema schemas/draft-2019-09/config.json data.yaml
✅ Valid
```

### Supported Drafts (now consistent across all validation paths)

| Draft | Meta-Schema Validation | Data Validation |
|-------|----------------------|-----------------|
| Draft-04 | v0.5.2+ | **v0.5.3+** |
| Draft-06 | v0.5.2+ | **v0.5.3+** |
| Draft-07 | Always | Always |
| 2019-09 | v0.5.2+ | **v0.5.3+** |
| 2020-12 | Always | Always |

## Upgrade Notes

Drop-in replacement for v0.5.2. No configuration changes required.

## Contributors

- Claude Opus 4.6 (devlead)
- @3leapsdave (supervision)

---

# Goneat v0.5.2 — Full JSON Schema Draft Coverage

**Release Date**: 2026-01-21
**Status**: Stable

## TL;DR

- **Full JSON Schema draft coverage**: Validate Draft-04 through 2020-12 (all five major versions) with offline `$ref` resolution
- **Schema CLI improvements**: Glob patterns and recursive directory validation
- **Tools config fixes**: `min_version` alias support, schema validation now enforced
- **Dependency modernization**: 84 packages updated across 3 staged releases, including OPA and container ecosystem
- **No breaking changes**: Additive features and maintenance only

## JSON Schema: Full Draft Coverage

Goneat now validates JSON Schemas across **all five major versions**—from the 2013-era Draft-04 still used in Kubernetes CRDs to the 2020-12 standard required by OpenAPI 3.1.

### Why This Matters

Enterprise codebases accumulate schemas over years. A Kubernetes operator from 2018 uses Draft-04. Your new API gateway uses 2020-12. Before v0.5.2, you needed different tools or forced migrations. Now one command handles everything.

### Supported Drafts

| Schema ID | Version | Typical Use |
|-----------|---------|-------------|
| `json-schema-draft-04` | Draft-04 (2013) | Kubernetes CRDs, legacy enterprise configs |
| `json-schema-draft-06` | Draft-06 (2017) | Transitional schemas |
| `json-schema-draft-07` | Draft-07 (2017) | Community standard, most common |
| `json-schema-2019-09` | 2019-09 | OpenAPI 3.0.x |
| `json-schema-2020-12` | 2020-12 | OpenAPI 3.1, current standard |

All meta-schemas are embedded for **air-gapped CI environments**—no network calls required.

### Practical Workflow: Discover Then Validate

Pair `pathfinder` discovery with `schema validate-schema` for a complete workflow:

```bash
# Discover schemas in your repo
goneat pathfinder find --schemas

# Validate everything in one pass (auto-detects draft from $schema field)
goneat schema validate-schema --recursive ./schemas/

# Or pipe discovery directly to validation
goneat pathfinder find --schemas --output-mode files | xargs goneat schema validate-schema
```

### Targeted Validation by Draft

For bulk validation of known-version schemas, use `--schema-id`:

```bash
# Validate legacy Draft-04 schemas (Kubernetes, enterprise configs)
goneat schema validate-schema --schema-id json-schema-draft-04 ./k8s-schemas/

# Validate OpenAPI 3.1 schemas (2020-12)
goneat schema validate-schema --schema-id json-schema-2020-12 ./openapi/
```

### Glob and Directory Support

New in v0.5.2: validate schemas using glob patterns or entire directories:

```bash
# Glob patterns
goneat schema validate-schema "schemas/**/*.json"

# Recursive directory scan
goneat schema validate-schema --recursive ./schemas/
```

## Dependency Modernization

This release updates 84 packages through a staged approach designed for stability and traceability.

### Stage Summary

| Stage | Focus | Risk |
|-------|-------|------|
| Stage 1 | Patch versions, `golang.org/x/*` | Low |
| Stage 2 | Security updates, minor bumps | Low-Medium |
| Stage 3 | OPA, OpenTelemetry, container ecosystem | Medium |

### Notable Updates

- **OPA (Open Policy Agent)**: Policy engine used by validation workflows
- **OpenTelemetry**: Observability instrumentation libraries
- **Container ecosystem**: Dependencies used by SBOM and vulnerability tooling

Each stage passed full validation (`make prepush`, `goneat dependencies --vuln`) before proceeding.

## Additional Improvements

### Vulnerability Summary Clarity

The `--vuln` output now reports accurate counts distinguishing between total vulnerabilities found and unique CVEs after deduplication.

### Tools Config: `min_version` Alias

The `min_version` field is now accepted as a deprecated alias for `minimum_version` in `.goneat/tools.yaml`. This provides backwards compatibility for existing configs while encouraging migration to the canonical name:

```yaml
tools:
  prettier:
    name: "prettier"
    description: "Code formatter"
    kind: "node"
    detect_command: "prettier --version"
    min_version: "3.0.0"        # Accepted (deprecated alias)
    # minimum_version: "3.0.0"  # Preferred
```

Using both fields simultaneously is a schema validation error.

### Tools Config Schema Validation

Fixed a bug where `goneat doctor tools` was not validating `.goneat/tools.yaml` against the embedded schema. Invalid configs are now caught early with clear error messages.

## Upgrade Notes

**No breaking CLI changes.** This release is additive and maintenance-focused.

All features from v0.5.0 and v0.5.1 continue to work unchanged:
- Vulnerability scanning (`--vuln`)
- TypeScript typecheck (`--categories typecheck`)
- Hooks migration support (`--unset-hookspath`)

### Confidence Signals

- `make clean && make build && make test` passes
- `make prepush` (fmt, lint, test, security) passes
- `goneat dependencies --vuln` shows no new vulnerabilities
- All dependency updates validated through staged rollout

## Contributors

- Claude Opus 4.5 (devlead, prodmktg)
- @3leapsdave (supervision)

---

# Goneat v0.5.1 — Security Remediation & SDR Framework

**Release Date**: 2026-01-17
**Status**: Stable

## TL;DR

- **Security fix**: Removed 4 critical/high vulnerabilities by upgrading go-licenses (v1.6.0 → v2.0.1)
- **SDR framework**: New Security Decision Records process for transparent vulnerability management
- **UX improvements**: `--vuln` works without config, clearer output messaging
- **Dogfooding**: Found and fixed these issues using goneat's own vulnerability scanner

## Why This Release

Shortly after releasing v0.5.0 with vulnerability scanning, we ran `goneat dependencies --vuln` on goneat itself. The scanner identified transitive vulnerabilities in `gopkg.in/src-d/go-git.v4` via our `go-licenses` dependency.

This release demonstrates the value of supply chain security tooling: **we found and fixed real vulnerabilities in our own dependency graph** within 48 hours of shipping the scanning feature.

## Security Fix: go-licenses Upgrade

### What Changed

Upgraded `github.com/google/go-licenses` from v1.6.0 to v2.0.1.

### Vulnerabilities Removed

| GHSA ID | Severity | Package | Description |
|---------|----------|---------|-------------|
| GHSA-449p-3h89-pw88 | Critical | go-git.v4 | Argument injection via crafted URLs |
| GHSA-v725-9546-7q7m | High | go-git.v4 | Path traversal in git operations |

The go-licenses v2.0.0 release dropped the `go-git.v4` dependency entirely, eliminating these vulnerabilities from goneat's dependency tree.

### API Migration

Minor code changes were required for the v2 API:

```go
// Before (v1.6.0)
classifier, _ := licenses.NewClassifier(0.9)
licensePath := lib.LicensePath

// After (v2.0.1)
classifier, _ := licenses.NewClassifier()
licensePath := lib.LicenseFile
```

## Security Decision Records (SDR)

This release introduces a structured process for documenting security decisions.

### Structure

```
docs/security/
├── README.md              # Process overview
├── decisions/             # Security Decision Records
│   ├── TEMPLATE.md        # SDR template
│   └── SDR-001-*.md       # Individual decisions
└── bulletins/             # User-facing announcements
```

### When to Create an SDR

- Vulnerability assessments requiring analysis
- False positive justifications
- Accepted risk decisions
- Security architecture choices

### SDR-001: x/crypto False Positive

Our first SDR documents a grype false positive: GHSA-v778-237x-gjrc in `golang.org/x/crypto`.

**Finding**: Grype flagged the minimum version requirement (v0.17.0) from a transitive dependency, not the resolved version (v0.42.0, which is patched).

**Decision**: Suppress in allowlist with documented analysis. See [SDR-001](docs/security/decisions/SDR-001-x-crypto-false-positive.md).

### Machine-Readable Allowlist

Vulnerability suppressions in `.goneat/dependencies.yaml` now support SDR references:

```yaml
vulnerabilities:
  allow:
    - id: GHSA-v778-237x-gjrc
      status: false_positive
      reason: "Grype flags min version, not resolved version"
      sdr: SDR-001
      verified_by: "@3leapsdave"
      verified_date: "2026-01-17"
```

This links machine-readable policy to human-readable analysis, creating an audit trail for security decisions.

## UX Improvements

### Zero-Config Vulnerability Scanning

`goneat dependencies --vuln` now works without a `.goneat/dependencies.yaml` config file. Sensible defaults are applied:

```bash
# Just works - no config required
goneat dependencies --vuln
```

Previously, the command would fail or produce confusing output without an explicit vulnerabilities configuration block.

### Clearer Output Messaging

The dependencies command now reports "Packages scanned: N" instead of the misleading "Dependencies: 0" when vulnerability scanning completes. This accurately reflects what the scanner analyzed.

### Makefile Integration

Added `make install` target for local testing workflows:

```bash
make install    # Builds and installs goneat to ~/.local/bin
```

New documentation: [Makefile Integration](docs/user-guide/workflows/makefile-integration.md) covers common development workflows and CI/CD patterns.

## Upgrade Notes

No breaking changes. This is a security patch release.

**Recommended action**: Run `goneat dependencies --vuln` on your own projects to identify supply chain issues.

## Contributors

- Claude Opus 4.5 (devlead)
- @3leapsdave (supervision)

---

# Goneat v0.5.0 — Vulnerability Scanning & TypeScript Typecheck

**Release Date**: 2026-01-15
**Status**: Stable

## TL;DR

- **Vulnerability scanning**: SBOM-based CVE detection via syft + grype, with policy enforcement across Go, Rust, Python, and TypeScript
- **TypeScript typecheck**: New `typecheck` category runs `tsc --noEmit` for type error detection
- **Hooks migration support**: `core.hooksPath` detection fixes silent failures when migrating from husky/lefthook

## Vulnerability Scanning

Goneat now provides end-to-end vulnerability scanning integrated into the dependencies workflow.

### How It Works

```
Syft (CycloneDX SBOM) → Grype (vulnerability scan) → Policy evaluation → Report
```

### Quick Start

```bash
# Install scanning tools
goneat doctor tools --scope sbom --install --yes

# Generate vulnerability report
goneat dependencies --vuln

# Enforce in CI
goneat assess --categories dependencies --fail-on high
```

### Output

Reports are written to `sbom/`:

| File | Description |
|------|-------------|
| `sbom/goneat-<timestamp>.cdx.json` | CycloneDX SBOM |
| `sbom/vuln-<timestamp>.json` | Normalized vulnerability report |
| `sbom/vuln-<timestamp>.grype.json` | Raw grype output |

### Policy Configuration

Configure vulnerability policy in `.goneat/dependencies.yaml`:

```yaml
version: v1

vulnerabilities:
  enabled: true
  tool: grype
  fail_on: high              # critical|high|medium|low|any|none
  ignore_unfixed: false      # skip vulns without a fix version
  allow:
    - id: CVE-2024-12345
      until: 2026-06-30
      reason: "Vendor patch pending"
      approved_by: "@security"
  remediation_age:           # grace windows by severity
    enabled: true
    max_days:
      critical: 7
      high: 30
      medium: 90
```

### Language Support

Vulnerability scanning works across all languages that syft supports:

| Language | Detection | Tested |
|----------|-----------|--------|
| Go | `go.mod` | Yes |
| Rust | `Cargo.toml` | Yes |
| Python | `pyproject.toml`, `requirements.txt` | Yes |
| TypeScript/JS | `package.json`, `package-lock.json` | Yes |

### New Flags

| Flag | Description |
|------|-------------|
| `--vuln` | Generate vulnerability report |
| `--sbom-input <path>` | Scan an existing SBOM (skip regeneration) |
| `--quiet` | Suppress verbose output |
| `--fail-on <severity>` | Fail at severity threshold |

### Example: CI Pipeline

```yaml
# GitHub Actions
- name: Vulnerability Scan
  run: |
    goneat doctor tools --scope sbom --install --yes
    goneat dependencies --vuln --fail-on high
```

## TypeScript Typecheck

New `typecheck` assessment category runs `tsc --noEmit` to catch type errors that biome and other linters miss.

### Usage

```bash
# Run typecheck
goneat assess --categories typecheck

# Combined with format and lint
goneat assess --categories format,lint,typecheck

# With file filtering
goneat assess --categories typecheck --include "src/**/*.ts"
```

### Configuration

Configure in `.goneat/assess.yaml`:

```yaml
version: 1

typecheck:
  enabled: true
  typescript:
    enabled: true
    config: tsconfig.json    # custom tsconfig path
    strict: false            # override strict mode
    skip_lib_check: true     # faster checks
    file_mode: false         # single-file mode for --include
```

### File Mode

When `file_mode: true` and `--include` targets a single file, goneat creates a temporary tsconfig scoped to that file. This enables file-level type checking without surfacing unrelated errors.

### Toolchain

`tsc` is now included in the TypeScript doctor tools scope:

```bash
goneat doctor tools --scope typescript --install --yes
```

## Hooks Migration Support

When migrating from husky, lefthook, or similar hook managers, the `core.hooksPath` git config often remains set after uninstallation. This causes git to ignore hooks in `.git/hooks/`, making goneat hooks appear to not work.

### Detection

Goneat now detects this condition and provides clear guidance:

```bash
$ goneat hooks install

⚠️  Warning: core.hooksPath is set to '.husky/_'
   Git will ignore hooks in .git/hooks/

   Options:
   1. Run: goneat hooks install --unset-hookspath
   2. Run: goneat hooks install --respect-hookspath

❌ Hooks installation aborted due to core.hooksPath override
```

### New Flags

| Flag | Description |
|------|-------------|
| `--unset-hookspath` | Clear `core.hooksPath` and install to `.git/hooks/` |
| `--respect-hookspath` | Install hooks to the custom path instead |
| `--force` | Alias for `--unset-hookspath` |

### Migration from Husky

```bash
npm uninstall husky
rm -rf .husky
goneat hooks init
goneat hooks generate
goneat hooks install --unset-hookspath
```

### Enhanced Diagnostics

`hooks inspect` and `hooks validate` now report `core.hooksPath` status in both text and JSON output.

## Additional Changes

### Biome Config Diagnostics

Lint assessment now surfaces Biome schema mismatch warnings for `biome.json`, helping teams catch configuration issues early.

### Assess Config Validation

`.goneat/assess.yaml` is now schema-validated:

- On every read (before applying overrides)
- During `goneat doctor assess init` (before writing)

Invalid configs produce warnings and are ignored to prevent unexpected behavior.

### Bug Fixes

- **Mutually exclusive flags**: `--respect-hookspath` and `--unset-hookspath` now error if both set
- **Relative path resolution**: `core.hooksPath` detection works correctly from subdirectories

## Upgrade Notes

No breaking changes. Existing workflows continue to work unchanged.

**To enable vulnerability scanning:**

1. Install tools: `goneat doctor tools --scope sbom --install --yes`
2. Add policy to `.goneat/dependencies.yaml`
3. Run: `goneat dependencies --vuln`

**To enable typecheck:**

1. Ensure `tsconfig.json` exists
2. Run: `goneat assess --categories typecheck`

## Contributors

- Claude Opus 4.5 (devlead)
- @3leapsdave (supervision)

---

# Goneat v0.4.5 — Rust License Scanning & Biome 2.x Compatibility

**Release Date**: 2026-01-13
**Status**: Stable

## TL;DR

- **Biome 2.x compatibility**: Format assessment updated for biome 2.x breaking changes (removed `--check` flag)
- **Rich cargo-deny output**: Error messages now include specific license names, crate versions, and deny.toml file:line references
- **License enumeration for Rust**: `goneat dependencies --licenses` now lists all Rust dependencies with their licenses (like Go)
- **Format assess fix mode**: Normalizes files when running `assess --categories format --fix`

## What Changed

### Biome 2.x Compatibility

Biome 2.x introduced breaking changes that affected goneat's format assessment:

- **Removed `--check` flag**: Biome 2.x uses exit codes instead of the `--check` flag
- **JSON diagnostics**: Now parses biome JSON output for reliable format issue detection
- **Respects ignore rules**: Properly honors `.biome.json` ignore configuration
- **Version requirement**: goneat now requires biome 2.x or higher

### Rich cargo-deny Output

Previously, cargo-deny output was generic:

```
cargo-deny: license: rejected, failing due to license requirements
```

Now it includes full context:

```
cargo-deny: license: rejected, failing due to license requirements [0BSD; unmatched license allowance; at deny.toml:53:6]
```

### License Enumeration for Rust

`goneat dependencies --licenses` now works identically for Go and Rust projects, parsing `cargo deny list` output and handling SPDX-like license expressions (`MIT OR Apache-2.0`).

### Bug Fixes

- **Biome 2.x false positives**: Fixed exit code misinterpretation
- **Assess fix normalization**: Files now normalized when using `assess --categories format --fix`
- **cargo-deny STDERR**: Fixed reading from stderr (cargo-deny outputs JSON to stderr by design)
- **Severity mapping**: "note" and "help" severities now correctly map to low

## Contributors

- Claude Opus 4.5 (devlead)
- @3leapsdave (supervision)

---

# Goneat v0.4.4 — Rust Dependency Analysis via cargo-deny

**Release Date**: 2026-01-09
**Status**: Stable

## TL;DR

- **Rust license checking**: `goneat dependencies --licenses` now works for Rust projects
- **cargo-deny integration**: License compliance and banned crate detection via cargo-deny
- **Cargo tool installer**: `kind: cargo` support in tools.yaml for installing Rust tools
- **Toolchain scopes**: Language-specific tool scopes (`go`, `rust`, `python`, `typescript`)
- **Smart guidance**: Helpful messages when cargo-deny is not installed

## What Changed

### Rust Dependency Analysis

`goneat dependencies --licenses` now supports Rust projects via cargo-deny:

```bash
cd my-rust-project
goneat dependencies --licenses
```

### Toolchain Scopes

Tools are now organized into language-specific scopes:

| Scope | Purpose | Key Tools |
|-------|---------|-----------|
| `foundation` | Language-agnostic | ripgrep, jq, yq, yamlfmt, prettier, yamllint, shfmt, shellcheck, actionlint, checkmake, minisign |
| `go` | Go development | go, go-licenses, golangci-lint, goimports, gofmt, gosec, govulncheck |
| `rust` | Rust Cargo plugins | cargo-deny, cargo-audit |
| `python` | Python tools | ruff |
| `typescript` | TS/JS tools | biome |
| `sbom` | SBOM & vuln scanning | syft, grype |

### Cargo Tool Installer

New `kind: cargo` in tools.yaml enables installing Rust tools:

```bash
goneat doctor tools --scope rust --install --yes
```

### Bug Fixes

- **SSOT provenance trailing newline**: `goneat ssot sync` now writes files with trailing newlines

## Contributors

- Claude Opus 4.5 (devlead)
- @3leapsdave (supervision)

---

**Previous Releases**: See `docs/releases/` for older release notes.
