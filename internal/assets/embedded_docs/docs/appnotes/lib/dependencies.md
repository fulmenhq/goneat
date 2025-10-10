---
title: "Dependencies Package"
description: "Dependency analysis, license compliance, and supply chain security"
library: "pkg/dependencies"
status: "Wave 2 Phase 1 Complete"
version: "v0.3.0"
last_updated: "2025-10-10"
tags:
  - "dependencies"
  - "licenses"
  - "security"
  - "supply-chain"
  - "cooling-policy"
---

# Dependencies Package

## Purpose

The `pkg/dependencies` package provides comprehensive dependency analysis for multi-language projects, including:

- **License detection and compliance** - Identify and validate software licenses
- **Cooling policy enforcement** - Supply chain security for newly published packages
- **Multi-language support** - Go, npm, PyPI, crates.io, NuGet
- **Policy engine integration** - OPA-based policy evaluation with Rego v1
- **SBOM generation** - Software Bill of Materials (Wave 2 Phase 2+)

## Architecture

### Core Components

```
pkg/dependencies/
├── analyzer.go          # Analyzer interface and types
├── go_analyzer.go       # Go-specific implementation
├── detector.go          # Language detection
├── license_utils.go     # License detection utilities
└── policy/              # Policy engine
    └── engine.go        # OPA integration (Rego v1)
```

### Related Packages

- **`pkg/registry`** - Registry API clients for metadata fetching (see [registry.md](registry.md))
- **`pkg/config`** - Configuration management for dependencies settings

## Key Features

### Multi-Language Support

- **Go**: `go.mod`, `go.sum` via `google.com/go-licenses`
- **TypeScript/JavaScript**: `package.json` detection
- **Python**: `pyproject.toml`, `requirements.txt` detection
- **Rust**: `Cargo.toml` detection
- **C#**: `*.csproj` detection

### License Compliance

- Automatic license type detection from file content
- Forbidden license enforcement (GPL, AGPL, etc.)
- License URL mapping for common licenses
- Integration with `go-licenses` library for Go projects

### Supply Chain Security (Cooling Policy)

- Enforce minimum package age before adoption
- Configurable age thresholds (e.g., 7 days)
- Conservative fallback when registry APIs fail
- Exception patterns for trusted packages

### Policy Engine

- **OPA v1 integration** with modern Rego v1 syntax
- YAML-to-Rego transpiler for simple policies
- Embedded policy evaluation (no external OPA server)
- Path traversal protection for policy files

## Installation

```go
import "github.com/fulmenhq/goneat/pkg/dependencies"
```

## Basic Usage

### Detecting Language

```go
detector := dependencies.NewDetector(cfg)
lang, found, err := detector.Detect("./myproject")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Detected language: %s\n", lang)
```

### Analyzing Dependencies

```go
analyzer := dependencies.NewGoAnalyzer()
ctx := context.Background()

config := dependencies.AnalysisConfig{
    PolicyPath: ".goneat/dependencies.yaml",
    EngineType: "embedded",
    Languages:  []dependencies.Language{dependencies.LanguageGo},
    Target:     ".",
}

result, err := analyzer.Analyze(ctx, ".", config)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Found %d dependencies\n", len(result.Dependencies))
fmt.Printf("Policy checks passed: %t\n", result.Passed)
fmt.Printf("Issues found: %d\n", len(result.Issues))
```

### Working with Results

```go
for _, dep := range result.Dependencies {
    fmt.Printf("%s@%s - License: %s\n",
        dep.Name,
        dep.Version,
        dep.License.Type)

    if ageDays, ok := dep.Metadata["age_days"].(int); ok {
        fmt.Printf("  Package age: %d days\n", ageDays)
    }
}

for _, issue := range result.Issues {
    fmt.Printf("[%s] %s: %s\n",
        issue.Severity,
        issue.Type,
        issue.Message)
}
```

## Configuration

### Policy File Format

`.goneat/dependencies.yaml`:

```yaml
version: v1
licenses:
  forbidden:
    - GPL-3.0
    - AGPL-3.0
cooling:
  enabled: true
  min_age_days: 7
  min_downloads: 100
  alert_only: false
policy_engine:
  type: embedded
```

### Config Structure

```go
type DependenciesConfig struct {
    PolicyPath string
    Engine     struct {
        Type string // "embedded" or "server"
        URL  string // For server mode
    }
    Languages []struct {
        Language string
        Paths    []string
    }
}
```

## API Reference

### Core Interfaces

#### Analyzer

```go
type Analyzer interface {
    Analyze(ctx context.Context, target string, config AnalysisConfig) (*AnalysisResult, error)
    DetectLanguages(target string) ([]Language, error)
}
```

#### LanguageDetector

```go
type LanguageDetector interface {
    Detect(target string) (Language, bool, error)
    GetManifestFiles(target string) ([]string, error)
}
```

### Key Types

#### Dependency

```go
type Dependency struct {
    Module   Module
    License  *License
    Metadata map[string]interface{} // age_days, publish_date, etc.
}
```

#### AnalysisResult

```go
type AnalysisResult struct {
    Dependencies []Dependency
    Issues       []Issue
    Passed       bool
    Duration     time.Duration
}
```

#### Issue

```go
type Issue struct {
    Type       string // "license", "cooling", "policy"
    Severity   string // "critical", "high", "medium", "low"
    Message    string
    Dependency *Dependency
}
```

## Integration with Registry Package

The dependencies package integrates tightly with `pkg/registry` for metadata fetching:

```go
// Registry client is used internally by analyzer
registryClient := registry.NewGoClient(24 * time.Hour)
metadata, err := registryClient.GetMetadata(name, version)
if err != nil {
    // Conservative fallback
    dep.Metadata["age_days"] = 365
    dep.Metadata["age_unknown"] = true
    dep.Metadata["registry_error"] = err.Error()
} else {
    dep.Metadata["age_days"] = int(time.Since(metadata.PublishDate).Hours() / 24)
    dep.Metadata["publish_date"] = metadata.PublishDate
}
```

See [registry.md](registry.md) for details on registry client architecture.

## Best Practices

### 1. Use Conservative Fallbacks

When registry APIs fail, always use conservative values:

```go
if err != nil {
    // Assume package is old (365 days) to pass cooling policy
    dep.Metadata["age_days"] = 365
    dep.Metadata["age_unknown"] = true
}
```

### 2. Handle Missing Versions

Local packages may not have versions:

```go
if version == "" {
    dep.Metadata["age_days"] = 0 // Local package
    return // Skip registry lookup
}
```

### 3. Cache Registry Results

Registry clients include built-in caching (24-hour TTL):

```go
// Client caches results automatically
client := registry.NewGoClient(24 * time.Hour)
```

### 4. Test with Mock Registry

Use mockable clients for testing (see [registry.md](registry.md#testing)):

```go
mock := registry.NewMockHTTPFetcher()
mock.AddResponse(url, 200, fixtureData)
client := registry.NewGoClientWithFetcher(ttl, mock)
```

## Anti-Patterns

### ❌ Don't Hardcode Policy Paths

```go
// Bad: hardcoded path
config.PolicyPath = "/home/user/.goneat/dependencies.yaml"

// Good: use config or default
config.PolicyPath = cfg.GetDependenciesConfig().PolicyPath
```

### ❌ Don't Ignore Registry Errors Without Fallback

```go
// Bad: no fallback
metadata, _ := client.GetMetadata(name, version)

// Good: conservative fallback
metadata, err := client.GetMetadata(name, version)
if err != nil {
    // Use safe defaults
}
```

### ❌ Don't Skip Language Detection

```go
// Bad: assume language
analyzer := dependencies.NewGoAnalyzer()

// Good: detect language first
detector := dependencies.NewDetector(cfg)
lang, found, err := detector.Detect(target)
```

## Error Handling

### Registry Failures

Registry API failures are non-fatal and use conservative fallbacks:

```go
if err != nil {
    log.Warn("Registry API failed, using conservative fallback: %v", err)
    dep.Metadata["age_days"] = 365 // Assume old package
    dep.Metadata["age_unknown"] = true
    dep.Metadata["registry_error"] = err.Error()
}
```

### Policy Evaluation Errors

Policy errors are logged but don't block analysis:

```go
if err := engine.LoadPolicy(config.PolicyPath); err != nil {
    log.Warn("Policy load failed: %v", err)
    // Continue with license checks only
}
```

### License Detection Fallback

If license detection fails, mark as "Unknown":

```go
licenseType := detectLicenseType(content)
if licenseType == "" {
    licenseType = "Unknown"
    log.Debug("Could not detect license for %s", dep.Name)
}
```

## Testing

### Unit Tests

```go
func TestGoAnalyzer_Analyze(t *testing.T) {
    analyzer := dependencies.NewGoAnalyzer()
    config := dependencies.AnalysisConfig{Target: "../.."}

    result, err := analyzer.Analyze(context.Background(), config.Target, config)
    assert.NoError(t, err)
    assert.NotEmpty(t, result.Dependencies)
}
```

### Integration Tests

```go
func TestDependencies_CLI(t *testing.T) {
    cmd := exec.Command("goneat", "dependencies", "--licenses", ".")
    output, err := cmd.CombinedOutput()
    assert.NoError(t, err)
    assert.Contains(t, string(output), "Passed")
}
```

## Wave 2 Roadmap

- **Phase 1 (Complete)**: Registry clients with mockable HTTP
- **Phase 2**: Cooling policy checker implementation
- **Phase 3**: Multi-language analyzer integration
- **Phase 4**: SBOM generation (CycloneDX, SPDX)

## See Also

- [Registry Package Documentation](registry.md) - Registry client architecture
- [OPA v1 Migration ADR](../architecture/decisions/adr-0001-opa-v1-rego-v1-migration.md) - Policy engine decision
- [Dependencies Command](../../user-guide/commands/dependencies.md) - CLI usage
- [Dependency Policy Configuration](../../configuration/dependency-policy.md) - Policy syntax

## References

- Wave 2 Spec: `.plans/active/v0.3.0/wave-2-detailed-spec.md`
- OPA Documentation: https://www.openpolicyagent.org/docs/latest/
- go-licenses: https://github.com/google/go-licenses
