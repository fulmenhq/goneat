---
title: "Dependency Protection Workflow"
description: "Complete workflow for using goneat's dependency protection features including license compliance, cooling policies, and SBOM generation"
author: "Forge Neat"
date: "2025-10-22"
last_updated: "2025-10-22"
status: "active"
tags: ["dependencies", "security", "supply-chain", "sbom", "opa"]
---

# Dependency Protection Workflow

This guide demonstrates goneat's comprehensive dependency protection capabilities, including license compliance checking, package cooling policies, SBOM generation, and OPA policy enforcement. These features help prevent supply chain attacks and ensure compliance across your software dependencies.

## Overview

Goneat v0.3.0 introduces enterprise-grade dependency protection with:

- **License Compliance**: Multi-language license detection and policy enforcement
- **Package Cooling**: Supply chain attack mitigation via age/download thresholds
- **SBOM Generation**: CycloneDX format artifacts for compliance reporting
- **OPA Integration**: Policy-as-code evaluation with Rego policies
- **Assessment Integration**: Seamless workflow integration via `goneat assess`

## Prerequisites

- Goneat v0.3.0 or later
- Go project with `go.mod` (other languages supported in future versions)
- Optional: `.goneat/dependencies.yaml` policy configuration

## Quick Start

### 1. License Compliance Check

Run license analysis on your Go project:

```bash
# Basic license check
goneat dependencies --licenses --format=markdown

# JSON output for automation
goneat dependencies --licenses --format=json --output=.scratchpad/dependencies/licenses.json
```

**Sample Output:**
```json
{
  "Dependencies": [
    {
      "Name": "github.com/spf13/cobra",
      "Version": "v1.8.0",
      "Language": "go",
      "License": {
        "Name": "Apache-2.0",
        "URL": "https://www.apache.org/licenses/LICENSE-2.0",
        "Type": "Apache-2.0"
      },
      "Metadata": {
        "age_days": 45,
        "license_path": "/go/pkg/mod/github.com/spf13/cobra@v1.8.0/LICENSE.txt",
        "packages": ["github.com/spf13/cobra"],
        "publish_date": "2024-09-07T15:16:10Z",
        "recent_downloads": 1000,
        "total_downloads": 50000
      }
    }
  ]
}
```

### 2. Package Cooling Policy Check

Verify packages meet minimum age and popularity requirements:

```bash
# Check cooling policy compliance
goneat dependencies --cooling --format=markdown

# With custom policy file
goneat dependencies --cooling --policy=.goneat/dependencies-strict.yaml --output=.scratchpad/dependencies/cooling.json
```

**How Cooling Metadata Works:**

The cooling checker queries package registries to determine:
- **Package Age**: Days since first publication
- **Download Metrics**: Total and recent download counts
- **Registry Data**: Real-time freshness from official sources

Example cooling violation:
```json
{
  "type": "cooling",
  "severity": "high",
  "message": "Package example/pkg violates cooling policy: 2 days old < minimum 7 days",
  "dependency": {
    "name": "example/pkg",
    "version": "v1.0.0",
    "metadata": {
      "age_days": 2,
      "total_downloads": 5,
      "publish_date": "2025-10-20T10:00:00Z"
    }
  }
}
```

### 3. SBOM Generation

Generate CycloneDX SBOMs for compliance and vulnerability management:

```bash
# Generate SBOM to file
goneat dependencies --sbom --sbom-format=cyclonedx-json --sbom-output=.scratchpad/dependencies/sbom.json

# Output to stdout for piping
goneat dependencies --sbom --sbom-stdout | jq '.components | length'
```

**SBOM Structure:**
```json
{
  "$schema": "http://cyclonedx.org/schema/bom-1.6.schema.json",
  "bomFormat": "CycloneDX",
  "specVersion": "1.6",
  "metadata": {
    "timestamp": "2025-10-22T18:12:43-04:00",
    "tools": [{
      "vendor": "anchore",
      "name": "syft",
      "version": "1.33.0"
    }],
    "component": {
      "type": "application",
      "name": "your-project"
    }
  },
  "components": [
    {
      "bom-ref": "pkg:golang/github.com/spf13/cobra@v1.8.0",
      "type": "library",
      "name": "github.com/spf13/cobra",
      "version": "v1.8.0",
      "licenses": [{"license": {"id": "Apache-2.0"}}],
      "purl": "pkg:golang/github.com/spf13/cobra@v1.8.0"
    }
  ]
}
```

## Assessment Integration

Integrate dependency checks into your development workflow:

```bash
# Run all categories including dependencies
goneat assess --categories=format,lint,security,dependencies --fail-on=high

# Dependencies only
goneat assess --categories=dependencies --json --output=.scratchpad/dependencies/assessment.json
```

**Assessment Output:**
```json
{
  "command_name": "dependencies",
  "category": "dependencies",
  "success": true,
  "issues": [],
  "execution_time": "12.947403209s",
  "metrics": {
    "dependencies_analyzed": 45,
    "licenses_detected": 8,
    "cooling_violations": 0,
    "sbom_packages": 721
  }
}
```

## Transitive Dependencies Example

Goneat analyzes the complete dependency tree, including transitive dependencies. Here's how it handles a package with multiple levels:

```bash
# Analyze a specific package's transitive dependencies
goneat dependencies --licenses --format=json | jq '.Dependencies[] | select(.Name | contains("github.com/spf13/cobra"))'
```

**Transitive Analysis Results:**
```json
{
  "Name": "github.com/spf13/cobra",
  "Version": "v1.8.0",
  "Language": "go",
  "License": {
    "Name": "Apache-2.0",
    "Type": "Apache-2.0"
  },
  "Metadata": {
    "packages": [
      "github.com/spf13/cobra",
      "github.com/spf13/cobra/doc",
      "github.com/spf13/cobra/shell",
      "github.com/spf13/pflag"
    ],
    "transitive_count": 12,
    "direct_dependencies": ["github.com/spf13/pflag"]
  }
}
```

The analysis shows:
- **Direct vs Transitive**: Clear distinction between direct and transitive dependencies
- **Package Groups**: Multiple packages from the same module
- **License Inheritance**: License applies to entire module
- **Dependency Graph**: Full transitive relationship mapping

## OPA Policy Engine

Use Rego policies for enterprise-grade dependency governance:

### Sample License Policy

```rego
package goneat.dependencies

# Default policy: Allow common permissive licenses, block copyleft
allowed_licenses := {"MIT", "Apache-2.0", "BSD-2-Clause", "BSD-3-Clause", "ISC"}

# Block problematic licenses
blocked_licenses := {"GPL-2.0", "GPL-3.0", "LGPL-2.1", "LGPL-3.0", "MPL-2.0"}

# Allow internal organizations
trusted_orgs := {"github.com/fulmenhq", "github.com/3leaps"}

# License compliance check
deny[msg] {
    dep := input.dependencies[_]

    # Check if license is explicitly blocked
    blocked_licenses[dep.license.type]

    # Allow if from trusted organization
    not is_trusted_org(dep.name)

    msg := sprintf("Blocked license '%s' in package %s", [dep.license.type, dep.name])
}

# Cooling policy enforcement
deny[msg] {
    dep := input.dependencies[_]
    dep.age_days < input.policy.cooling.min_age_days

    # Allow if from trusted organization
    not is_trusted_org(dep.name)

    msg := sprintf("Package %s violates cooling policy: %d days old < minimum %d",
                  [dep.name, dep.age_days, input.policy.cooling.min_age_days])
}

# Helper: Check if package is from trusted organization
is_trusted_org(name) {
    org := trusted_orgs[_]
    strings.has_prefix(name, org)
}
```

### Policy Configuration

```yaml
# .goneat/dependencies.yaml
version: v1
policy_engine:
  type: embedded
  rego_files:
    - ".goneat/policies/license-policy.rego"

cooling:
  enabled: true
  min_age_days: 7
  min_downloads: 100
  exceptions:
    - pattern: "github.com/fulmenhq/*"
      reason: "Internal packages are pre-vetted"
    - pattern: "github.com/3leaps/*"
      reason: "Trusted organization packages"

licenses:
  allowed:
    - "MIT"
    - "Apache-2.0"
    - "BSD-2-Clause"
    - "BSD-3-Clause"
    - "ISC"
  blocked:
    - "GPL-2.0"
    - "GPL-3.0"
    - "LGPL-2.1"
    - "LGPL-3.0"
    - "MPL-2.0"
```

## Git Hook Integration

Automate dependency checks in your development workflow:

```yaml
# .goneat/hooks.yaml
hooks:
  pre-commit:
    # Fast, offline checks
    - command: assess
      args: ["--categories", "format,lint"]

  pre-push:
    # Network-dependent security checks
    - command: assess
      args: ["--categories", "dependencies", "--fail-on", "high"]
    - command: dependencies
      args: ["--sbom", "--sbom-output", ".scratchpad/sbom/goneat-$(date +%Y%m%d).cdx.json"]
```

## CI/CD Integration

### GitHub Actions Example

```yaml
# .github/workflows/dependency-check.yml
name: Dependency Protection
on: [push, pull_request]

jobs:
  dependencies:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.21'

      - name: Install goneat
        run: go install github.com/fulmenhq/goneat@latest

      - name: License Compliance
        run: goneat dependencies --licenses --fail-on=high

      - name: Package Cooling
        run: goneat dependencies --cooling --fail-on=high

      - name: Generate SBOM
        run: |
          goneat dependencies --sbom --sbom-output=sbom.json
          # Upload SBOM as artifact

      - name: Assessment Integration
        run: goneat assess --categories=dependencies --fail-on=high
```

## Performance Characteristics

- **License Analysis**: < 5s for typical projects (100-500 dependencies)
- **Cooling Checks**: < 10s with registry API calls (cached for 24h)
- **SBOM Generation**: < 10s using managed Syft tool
- **Assessment Integration**: < 15s total overhead
- **Memory Usage**: Minimal additional memory beyond standard Go tooling

## Troubleshooting

### Common Issues

**"No Go files found"**
- Ensure you're in a Go project directory with `go.mod`
- For other languages, full support coming in v0.3.1+

**Registry API timeouts**
- Cooling checks require network access
- Use `--no-op` flag for offline mode
- Configure longer timeouts if needed

**SBOM generation fails**
- Ensure Syft tool is available (`goneat doctor tools --scope sbom`)
- Check file permissions for output directory

### Debug Mode

Enable verbose logging for troubleshooting:

```bash
goneat dependencies --licenses --log-level=debug
```

## Advanced Usage

### Custom Policy Files

```bash
# Use custom policy for strict environments
goneat dependencies --cooling --policy=.goneat/dependencies-production.yaml

# Combine multiple checks
goneat dependencies --licenses --cooling --sbom --fail-on=critical
```

### Output Formats

- **JSON**: For automation and API integration
- **Markdown**: Human-readable reports
- **HTML**: Rich web reports with styling

### Integration with Security Tools

The structured JSON output integrates with:
- **SIEM Systems**: Security information and event management
- **Vulnerability Scanners**: SBOM-based vulnerability detection
- **Compliance Dashboards**: License and security metrics
- **Audit Trails**: Complete dependency history

## Next Steps

- **v0.3.1**: Full TypeScript, Python, Rust, C# support
- **Vulnerability Scanning**: Integration with OSV database
- **Advanced Policies**: Typosquatting detection, provenance verification
- **Enterprise Features**: Remote OPA policy servers, audit logging

---

**Note**: Results can be safely written to `.scratchpad/dependencies/` as this directory is gitignored and won't pollute your repository.