---
title: "Dependencies Command"
description: "Reference for goneat dependencies – license compliance, cooling policy, and SBOM generation tooling"
author: "@arch-eagle"
date: "2025-09-30"
last_updated: "2025-10-22"
status: "approved"
tags: ["cli", "dependencies", "security", "supply-chain", "sbom", "licenses"]
category: "user-guide"
---

# Dependencies Command

The `dependencies` command analyzes project dependencies for license compliance, supply chain security (cooling policy), and SBOM generation.

Note: License compliance is currently strongest for Go projects (via `go-licenses`). SBOM generation uses Syft and can inventory polyglot repos and container images, but SBOM-to-license-inventory ingestion is planned (v0.3.22+).

## Usage

```bash
goneat dependencies [flags] [target]
```

**Arguments:**

- `target`: Directory to analyze (default: current directory)

## Features

### License Compliance (Wave 1 ✅)

**Important:** `--licenses` evaluates licenses using language-native analyzers (for Go: `go-licenses`). It does **not** currently ingest an SBOM file and derive license policy results from SBOM license fields.

Detect and validate software licenses against your policy:

```bash
goneat dependencies --licenses .
```

**Capabilities:**

- Automatic license type detection
- Forbidden license enforcement (GPL, AGPL, etc.)
- Integration with `go-licenses` for Go projects

**Monorepos / nested Go modules:** Some repos place `go.mod` in a subdirectory (e.g. `server/`) but keep `LICENSE*` at the repo root. In these cases `go-licenses` may report the local module’s license as `Unknown`. Goneat includes that local module for context (`is_local: true`) but policy gating is intended to focus on third-party dependencies.

### Cooling Policy (Wave 2 Phase 2)

Enforce minimum package age before adoption for supply chain security:

```bash
goneat dependencies --cooling .
```

**Capabilities:**

- Minimum package age enforcement (e.g., 7 days)
- Download threshold validation
- Exception patterns for trusted packages
- Conservative fallback when registry APIs fail

### SBOM Generation (Wave 3 ✅)

Generate Software Bill of Materials for compliance and security:

```bash
# Generate SBOM with default settings (outputs to sbom/ directory)
goneat dependencies --sbom .

# Specify output location
goneat dependencies --sbom --sbom-output sbom/myapp.cdx.json .

# Output to stdout for piping
goneat dependencies --sbom --sbom-stdout . > sbom.json

# Specify target platform for container images
goneat dependencies --sbom --sbom-platform linux/amd64 .
```

**Output Directory:**

- Default output: `sbom/goneat-<timestamp>.cdx.json` in the current project root
- The `sbom/` directory is automatically created if it doesn't exist
- **Important:** Add `sbom/` to your `.gitignore` to prevent committing generated SBOMs

**Features:**

- **Format**: CycloneDX JSON (fully supported); SPDX JSON generation is supported but some goneat metadata (package counts / dependency graph summary) is currently CycloneDX-only
- **Tool**: Syft (Anchore) with SHA256-verified installation
- **Metadata**: Package counts, tool version, generation timestamp
- **Platform Support**: Cross-platform with managed binary installation
- **Supply-Chain Security**: Artifact-based installation with cryptographic verification

**Installation:**

Syft is required for SBOM generation. Goneat will prompt to install if missing:

```bash
# Install Syft via goneat (recommended)
goneat doctor tools --scope sbom --install --yes

# Verify installation
syft version
```

**Combining with Other Checks:**

SBOM generation can be combined with license and cooling checks:

```bash
# Run all dependency checks together
goneat dependencies --licenses --cooling --sbom .
```

### Vulnerability Scanning (Wave 4 ✅)

Generate a vulnerability report from an SBOM using Grype.

```bash
# Generate vulnerability report (SBOM + grype)
goneat dependencies --vuln .
```

**Output Directory:**

- Reports are written under `sbom/` in the project root:
  - `sbom/vuln-<timestamp>.json` (normalized)
  - `sbom/vuln-<timestamp>.grype.json` (raw grype output)

**Policy & Enforcement:**

Vulnerability scanning is policy-driven via `.goneat/dependencies.yaml` under the `vulnerabilities:` key.

- `fail_on: none` produces the report but never fails.
- `remediation_age` (or compatibility alias `cooling_days`) can suppress findings for a grace window.
  - If a finding has no `fix_first_seen` date available from Grype, it is suppressed with `remediation_age_unknown` when remediation age is enabled.

**Tooling:**

```bash
# Install syft + grype
goneat doctor tools --scope sbom --install --yes
```

## Assessment Integration

The standalone command shares its engine with `goneat assess --categories dependencies`. Use assess when you need unified
reporting, severity gating, or JSON output that conforms to
`schemas/dependencies/v1.0.0/dependency-analysis.schema.json`.

```bash
# Run dependencies via assess (recommended for hooks/CI)
goneat assess --categories dependencies --fail-on high

# Combine with security checks and emit structured JSON
goneat assess --categories security,dependencies --format json --output deps-report.json
```

Assessment mode automatically enriches the report with:

- License and cooling policy findings mapped to Crucible severities
- Dependency metrics (counts, policy status)
- SBOM metadata (latest file path, tool version, generation timestamp) if an SBOM exists

For workflow guidance see [Dependency Gating Workflow](../workflows/dependency-gating.md).

## Security Considerations

- SBOM archives are extracted with a 500 MB safety limit. Files exceeding the limit cause extraction to fail with
  `ErrArchiveTooLarge` to prevent decompression bombs.
- Generated policy and manifest files are written with `0600` permissions to keep credentials and policy data private.
- Syft invocations use sanitized arguments and validated output paths to avoid command injection and path traversal.

## Flags

### Core Flags

- `--licenses`: Run license compliance checks (default: false)
- `--cooling`: Check package cooling policy (default: false)
- `--sbom`: Generate SBOM artifact (default: false)

### Configuration

- `--policy string`: Policy file path (default: ".goneat/dependencies.yaml")

### Output Control

- `--format string`: Output format (json, markdown, html) (default: "json")
- `--output string`: Output file (default: stdout)

### SBOM Options

- `--sbom-format string`: SBOM format (`cyclonedx-json` or `spdx-json`) (default: "cyclonedx-json")
- `--sbom-output string`: Output file path (default: "sbom/goneat-<timestamp>.cdx.json")
- `--sbom-stdout`: Output SBOM to stdout instead of file (default: false)
- `--sbom-platform string`: Target platform for SBOM (e.g., linux/amd64)

### Failure Control

- `--fail-on string`: Fail on severity (critical, high, medium, low, any) (default: "critical")

**Applies to**: License compliance and cooling policy checks. SBOM generation failures are treated independently and will terminate the command if Syft cannot be invoked or produces invalid output.

**Severity Mapping**:

| Severity   | License Issues     | Cooling Issues  | Exit Code                 |
| ---------- | ------------------ | --------------- | ------------------------- |
| `critical` | Forbidden licenses | N/A             | 1                         |
| `high`     | Missing licenses   | Age < threshold | 1                         |
| `medium`   | Warnings           | N/A             | 1 (if `--fail-on medium`) |
| `low`      | Informational      | N/A             | 1 (if `--fail-on low`)    |

**SBOM Generation Failures**:

SBOM generation has its own failure modes independent of `--fail-on`:

- **Tool Missing**: Exit code 1 with installation instructions
- **Invalid Output**: Exit code 1 with Syft error details
- **Network Failure** (during artifact install): Exit code 1 with retry guidance

**Example**:

```bash
# Fail on any license issue, but only warn on cooling
goneat dependencies --licenses --cooling --fail-on high .

# Generate SBOM alongside checks (SBOM failure is independent)
goneat dependencies --licenses --sbom --fail-on critical .
```

## Configuration File

`.goneat/dependencies.yaml`:

```yaml
version: v1

# License compliance policy
licenses:
  forbidden:
    - GPL-3.0
    - AGPL-3.0
    - LGPL-3.0

# Supply chain security (cooling policy)
cooling:
  enabled: true
  min_age_days: 7 # Minimum package age in days
  min_downloads: 100 # Minimum total downloads
  min_downloads_recent: 10 # Minimum recent downloads
  alert_only: false # Fail build or alert only
  grace_period_days: 3 # Grace period for new packages
  exceptions: # Trusted packages
    - pattern: "github.com/myorg/*"
      reason: "Internal packages"

# Policy engine configuration
policy_engine:
  type: embedded # or "server" for remote OPA
  url: "" # OPA server URL (if type=server)

# Language-specific settings (optional)
languages:
  - language: go
    paths:
      - go.mod
  - language: typescript
    paths:
      - package.json
```

See [Dependency Policy Configuration](../../configuration/dependency-policy.md) for full schema.

## Examples

### Try It Yourself (goneat Repository)

If you've cloned goneat, you can explore dependencies features hands-on:

```bash
cd /path/to/goneat

# 1. Generate SBOM (creates sbom/goneat-<timestamp>.cdx.json)
goneat dependencies --sbom

# 2. View SBOM summary
cat sbom/goneat-latest.cdx.json | jq '{
  format: .bomFormat,
  spec: .specVersion,
  component: .metadata.component.name,
  dependencies: (.components | length)
}'

# 3. Check license compliance
goneat dependencies --licenses

# 4. Run full dependencies assessment
goneat assess --categories dependencies --verbose

# 5. Compare standalone vs assessment output
goneat dependencies --licenses --format json > standalone.json
goneat assess --categories dependencies --format json > assessment.json
diff <(jq '.Dependencies' standalone.json) <(jq '.categories.dependencies' assessment.json)
```

**Expected Results:**

- SBOM with ~150+ Go dependencies
- All dependencies pass license checks (MIT, Apache-2.0, BSD)
- Cooling policy validates package ages
- Assessment includes SBOM metadata

### Basic License Check

```bash
goneat dependencies --licenses .
```

**Output:**

```json
{
  "Dependencies": [
    {
      "Module": {
        "Name": "github.com/spf13/cobra",
        "Version": "v1.8.0",
        "Language": "go"
      },
      "License": {
        "Name": "LICENSE",
        "Type": "MIT",
        "URL": "https://opensource.org/licenses/MIT"
      },
      "Metadata": {
        "license_path": "vendor/github.com/spf13/cobra/LICENSE",
        "age_days": 120,
        "publish_date": "2024-06-15T10:30:00Z"
      }
    }
  ],
  "Issues": [],
  "Passed": true,
  "Duration": "1.234s"
}
```

### Combined License and Cooling Check

```bash
goneat dependencies --licenses --cooling .
```

### Write Results to File

```bash
goneat dependencies --licenses --output report.json .
```

### Fail on Any Issue

```bash
goneat dependencies --licenses --fail-on any .
```

**Exit codes:**

- `0`: Analysis passed
- `1`: Analysis failed based on `--fail-on` threshold

## Multi-Language Support

### Supported Languages

| Language   | Detection                            | Status            |
| ---------- | ------------------------------------ | ----------------- |
| Go         | `go.mod`                             | ✅ Wave 1         |
| JavaScript | `package.json`                       | ✅ Wave 2 Phase 1 |
| TypeScript | `package.json`                       | ✅ Wave 2 Phase 1 |
| Python     | `pyproject.toml`, `requirements.txt` | ✅ Wave 2 Phase 1 |
| Rust       | `Cargo.toml`                         | ✅ Wave 2 Phase 1 |
| C#         | `*.csproj`                           | ✅ Wave 2 Phase 1 |

### Language Auto-Detection

```bash
# Analyzes detected language automatically
goneat dependencies --licenses .
```

**Detection order:**

1. Explicit config in `.goneat/dependencies.yaml`
2. Auto-detection from manifest files

## Integration

### Git Hooks

`.goneat/hooks.yaml`:

```yaml
pre-commit:
  - name: dependency-check
    run: goneat dependencies --licenses --fail-on high .
```

### CI/CD Pipeline

**GitHub Actions:**

```yaml
- name: Dependency Analysis
  run: |
    goneat dependencies --licenses --cooling . \
      --output dependency-report.json \
      --fail-on high
```

**GitLab CI:**

```yaml
dependencies:
  script:
    - goneat dependencies --licenses --cooling .
  artifacts:
    reports:
      dependency_scanning: dependency-report.json
```

## Assessment Integration (Wave 4 ✅)

The dependencies command integrates with `goneat assess` to provide unified dependency validation alongside other code quality checks:

```bash
# Run dependencies as part of comprehensive assessment
goneat assess --categories dependencies

# Combined security and supply-chain validation
goneat assess --categories security,dependencies --fail-on high

# Full pre-push assessment including dependencies
goneat assess --categories format,lint,security,dependencies --fail-on high
```

### Assessment vs Standalone Command

| Feature         | `goneat dependencies`         | `goneat assess --categories dependencies`      |
| --------------- | ----------------------------- | ---------------------------------------------- |
| **Purpose**     | Detailed dependency analysis  | Integrated assessment workflow                 |
| **Output**      | JSON report with full details | Unified assessment report (markdown/JSON/HTML) |
| **Use Case**    | Deep dependency investigation | Pre-commit/pre-push gating                     |
| **Metrics**     | Comprehensive dependency info | Issue counts, severity summary, SBOM metadata  |
| **Integration** | Standalone tool               | Part of comprehensive assessment               |
| **Hook Usage**  | Manual hook configuration     | Automatic via assess hook profiles             |

### Assessment Output

When run via `goneat assess`, dependencies appears as a standard assessment category:

```markdown
### ✅ Dependencies Issues (Priority: 2)

**Status:** 0 issues found
**Estimated Time:** 0 seconds
**Parallelizable:** No

**Metrics:**

- Dependency count: 142
- License violations: 0
- Cooling violations: 0
- Analysis passed: true
- SBOM metadata: available (sbom/goneat-latest.cdx.json)
```

### Hook Integration with Assess

Pre-push hooks automatically include dependencies when using assess:

```yaml
# .goneat/hooks.yaml
version: v1
hooks:
  pre-push:
    - command: assess
      args:
        - --categories
        - format,lint,security,dependencies
        - --fail-on
        - high
```

This replaces manual dependency hook configuration and provides:

- Unified reporting across all validation categories
- Consistent severity handling
- Integrated SBOM metadata in assessment reports
- Better CI/CD pipeline integration

See [Dependency Gating Workflow](../workflows/dependency-gating.md) for complete integration patterns.

## Troubleshooting

### Registry API Failures

If registry APIs fail (rate limits, network issues), goneat uses conservative fallbacks:

```
[WARN] Registry API failed for package X: rate limit exceeded
[INFO] Using conservative fallback: assuming package age = 365 days
```

**Conservative behavior:**

- Assumes package is 365 days old (passes cooling policy)
- Marks dependency with `age_unknown=true`
- Records error in `registry_error` metadata field

### Missing Language Support

```
Error: no supported language detected
```

**Solutions:**

1. Add explicit language config in `.goneat/dependencies.yaml`
2. Ensure manifest file exists (`go.mod`, `package.json`, etc.)
3. Check supported languages table above

### Policy Evaluation Errors

```
[WARN] Policy evaluation failed: failed to load policy file
```

**Solutions:**

1. Verify `.goneat/dependencies.yaml` exists
2. Validate YAML syntax
3. Check file permissions

## Performance

### Caching

Registry metadata is cached for 24 hours per package:

- **First run**: ~2 seconds per 100 dependencies
- **Cached runs**: ~50ms per 100 dependencies

### Concurrent Analysis

Dependency analysis runs concurrently when possible:

- Registry API calls: parallel with connection pooling
- License detection: parallel per package
- Policy evaluation: batch processing

## See Also

- [Assess Command](assess.md) - Comprehensive assessment including dependencies category
- [Dependency Gating Workflow](../workflows/dependency-gating.md) - Hook and CI integration patterns
- [Dependencies Package Documentation](../../appnotes/lib/dependencies.md) - Internal architecture
- [Registry Package Documentation](../../appnotes/lib/registry.md) - Registry client details
- [Dependency Policy Configuration](../../configuration/dependency-policy.md) - Policy schema
- [OPA v1 Migration ADR](../../architecture/decisions/adr-0001-opa-v1-rego-v1-migration.md) - Policy engine decision

## References

- Wave 2 Specification: `.plans/active/v0.3.0/wave-2-detailed-spec.md`
- OPA Documentation: https://www.openpolicyagent.org/docs/latest/
