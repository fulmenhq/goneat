# Dependencies Command

The `dependencies` command analyzes project dependencies for license compliance, supply chain security (cooling policy), and SBOM generation. It supports multiple languages including Go, TypeScript/JavaScript, Python, Rust, and C#.

## Usage

```bash
goneat dependencies [flags] [target]
```

**Arguments:**
- `target`: Directory to analyze (default: current directory)

## Features

### License Compliance (Wave 1 ✅)

Detect and validate software licenses against your policy:

```bash
goneat dependencies --licenses .
```

**Capabilities:**
- Automatic license type detection
- Forbidden license enforcement (GPL, AGPL, etc.)
- Multi-language support via language-specific analyzers
- Integration with `go-licenses` for Go projects

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

### SBOM Generation (Wave 2 Phase 4)

Generate Software Bill of Materials for compliance and security:

```bash
goneat dependencies --sbom --sbom-format cyclonedx
```

**Formats:**
- CycloneDX (default)
- SPDX

**Note:** SBOM generation is planned for Wave 2 Phase 4.

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

- `--sbom-format string`: SBOM format (cyclonedx, spdx) (default: "cyclonedx")
- `--sbom-enrich`: Enrich SBOM with vulnerability data (default: false)

### Failure Control

- `--fail-on string`: Fail on severity (critical, high, medium, low, any) (default: "critical")

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
  min_age_days: 7           # Minimum package age in days
  min_downloads: 100        # Minimum total downloads
  min_downloads_recent: 10  # Minimum recent downloads
  alert_only: false         # Fail build or alert only
  grace_period_days: 3      # Grace period for new packages
  exceptions:               # Trusted packages
    - pattern: "github.com/myorg/*"
      reason: "Internal packages"

# Policy engine configuration
policy_engine:
  type: embedded             # or "server" for remote OPA
  url: ""                    # OPA server URL (if type=server)

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

| Language   | Detection               | Status          |
|------------|-------------------------|-----------------|
| Go         | `go.mod`                | ✅ Wave 1       |
| JavaScript | `package.json`          | ✅ Wave 2 Phase 1 |
| TypeScript | `package.json`          | ✅ Wave 2 Phase 1 |
| Python     | `pyproject.toml`, `requirements.txt` | ✅ Wave 2 Phase 1 |
| Rust       | `Cargo.toml`            | ✅ Wave 2 Phase 1 |
| C#         | `*.csproj`              | ✅ Wave 2 Phase 1 |

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

- [Dependencies Package Documentation](../../appnotes/lib/dependencies.md) - Internal architecture
- [Registry Package Documentation](../../appnotes/lib/registry.md) - Registry client details
- [Dependency Policy Configuration](../../configuration/dependency-policy.md) - Policy schema
- [OPA v1 Migration ADR](../../architecture/decisions/adr-0001-opa-v1-rego-v1-migration.md) - Policy engine decision

## References

- Wave 2 Specification: `.plans/active/v0.3.0/wave-2-detailed-spec.md`
- OPA Documentation: https://www.openpolicyagent.org/docs/latest/