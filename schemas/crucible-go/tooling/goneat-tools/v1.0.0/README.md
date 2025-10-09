---
title: "Goneat Tools Config Schema v1.0.0"
description: "Schema for goneat doctor command tool catalog and configuration"
author: "Pipeline Architect"
date: "2025-10-02"
last_updated: "2025-10-02"
status: "draft"
tags: ["tooling", "goneat", "doctor", "schema"]
---

# Goneat Tools Config Schema

**Schema**: `goneat-tools-config.schema.yaml`  
**Purpose**: Define tool catalogs for the `goneat doctor` command  
**Owner**: Goneat team  
**Implementation**: `goneat/pkg/tools`

## Overview

This schema describes the structure of tool catalogs used by goneat's `doctor` command. It supports:

- **Scopes**: Logical grouping of tools (e.g., `security`, `formatting`, `build`)
- **Tool Definitions**: Rich metadata including detection, installation, and versioning
- **Platform Support**: Per-platform installation commands and installer priorities
- **Version Management**: Minimum/recommended/disallowed version constraints

## Usage

### Goneat Internal Catalog

Goneat ships with a built-in tool catalog at `goneat/internal/catalog/tools-config.yaml` that defines ecosystem tools.

### Custom Catalogs

Projects can provide custom tool definitions that extend or replace goneat's built-in catalog:

```yaml
# .goneat/tools-config.yaml
scopes:
  custom:
    description: "Project-specific tooling"
    tools:
      - my-formatter
      - my-linter

tools:
  my-formatter:
    name: my-formatter
    description: "Custom code formatter for our DSL"
    kind: go
    detect_command: my-formatter
    install_package: github.com/example/my-formatter@latest
    version_scheme: semver
    minimum_version: "1.2.0"
```

### CLI Integration

```bash
# Check all tools in security scope
goneat doctor --check security

# Install missing tools
goneat doctor --install security

# Validate custom catalog
goneat doctor --validate .goneat/tools-config.yaml
```

## Schema Features

### Scopes

Group related tools for bulk operations:

```yaml
scopes:
  security:
    description: "Security scanning and vulnerability detection"
    tools:
      - gosec
      - trivy
      - semgrep
```

### Tool Kinds

- **`go`**: Go installable tools (`go install package@version`)
- **`bundled-go`**: Tools bundled with goneat
- **`system`**: System-level tools requiring package managers

### Installation Methods

Tools can define multiple installation methods per platform:

```yaml
tools:
  ripgrep:
    kind: system
    platforms: [darwin, linux, windows]
    install_commands:
      darwin: brew install ripgrep
      linux: apt-get install ripgrep
      windows: scoop install ripgrep
    installer_priority:
      darwin: [brew, mise, go-install]
      linux: [apt-get, pacman, dnf]
```

### Version Constraints

```yaml
tools:
  golangci-lint:
    version_scheme: semver
    minimum_version: "1.54.0"
    recommended_version: "1.55.0"
    disallowed_versions:
      - "1.53.0" # Known bug
      - "1.52.0" # Performance regression
```

## Relationship to Bootstrap Manifests

**Different Use Cases**:

| Feature        | Goneat Tools Config                         | Crucible Bootstrap Manifest           |
| -------------- | ------------------------------------------- | ------------------------------------- |
| **Purpose**    | Ecosystem-wide tool catalog                 | Repo-specific dependencies            |
| **Schema**     | `goneat-tools-config.schema.yaml`           | `external-tools-manifest.schema.yaml` |
| **Scope**      | Cross-project doctor command                | Single repo bootstrap                 |
| **Complexity** | Full-featured (scopes, versions, platforms) | Minimal (install/verify)              |
| **Location**   | Goneat internal + optional project catalogs | `.goneat/tools.yaml`                  |

Goneat's `doctor --bootstrap` command can read `.goneat/tools.yaml` manifests to install repo-specific tools.

## Library Usage

Projects importing goneat as a library (e.g., Sumpter) can use `pkg/tools`:

```go
import "github.com/fulmenhq/goneat/pkg/tools"

// Parse catalog
data, _ := os.ReadFile("tools-config.yaml")
cfg, _ := tools.ParseConfig(data)

// Validate against schema
err := tools.ValidateBytes(data)

// Access tool definitions
for name, tool := range cfg.Tools {
    fmt.Printf("Tool: %s - %s\n", name, tool.Description)
}
```

## Migration Notes

**Post-refactor** (after goneat adopts crucible):

Goneat will reference this schema from crucible:

```go
import crucible "github.com/fulmenhq/crucible"

schema, _ := crucible.GetSchema("tooling/goneat-tools/v1.0.0/goneat-tools-config.schema.yaml")
// Use schema for validation
```

See `.plans/memos/goneat/goneat-fulmen-refactor.md` for migration plan.

## References

- [Goneat Repository](https://github.com/fulmenhq/goneat)
- [External Tools Manifest Schema](../../external-tools/v1.0.0/) (Crucible bootstrap)
- [Fulmen Config Path Standard](../../../docs/standards/config/fulmen-config-paths.md)
- [Library Ecosystem Architecture](../../../docs/architecture/library-ecosystem.md)
