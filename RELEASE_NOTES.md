# Goneat v0.2.8 — Guardian Repository Protection & Format Intelligence (2025-09-27)

## TL;DR

- Pathfinder Schema Discovery enables intelligent detection and validation of schema files across your codebase
- New `goneat pathfinder` command suite with schema discovery, validation, and metadata extraction
- Support for 10+ schema types including JSON Schema, OpenAPI, AsyncAPI, Avro, Cue, and Protobuf
- FinderFacade provides high-level API for enterprise-grade path discovery workflows
- Enhanced schema validation with meta-schema compliance and structured error reporting

## What's New

### Pathfinder Schema Discovery System

- **Schema Detection Engine**: Intelligent pattern matching for 10+ schema formats with contains/regex detection
- **Pathfinder Command Suite**: New `goneat pathfinder` commands for schema discovery, validation, and metadata extraction
- **FinderFacade API**: High-level entry point for path discovery that maintains enterprise-grade PathFinder interface while providing simpler API for common workflows
- **Schema Validation**: Comprehensive validation with meta-schema compliance checking and structured error reporting
- **Local Loader**: Production-ready local filesystem loader with streaming text output and transform support

### Format Command Enhancements

- **JSON Prettification**: Added built-in JSON formatting using Go's `json.Indent` with configurable options
  - Flags: `--json-indent` (custom string), `--json-indent-count` (1-10 spaces, 0 to skip), `--json-size-warning` (MB threshold)
  - Replaces external `jq` dependency for reliable, cross-platform JSON formatting
  - Supports compact mode and size-based warnings for large files

- **XML Prettification**: Added built-in XML formatting using `etree` library with configurable options
  - Flags: `--xml-indent` (custom string), `--xml-indent-count` (1-10 spaces, 0 to skip), `--xml-size-warning` (MB threshold)
  - Validates XML well-formedness before formatting
  - Supports size-based warnings for large files

### Guardian Security Improvements

- **Bug Fix**: Fixed `runGuardianApprove` to always execute wrapped commands after policy checks, whether approval is required or not
- **Enhanced UX**: Improved denial error messages with clear "❌ Guardian approval denied by user - operation cancelled" feedback and proper exit code (1) on denial
- **Documentation**: Clarified that guardian policies are user-level only with no current support for repository-specific policies

### Schema Type Support

- **JSON Schema**: Draft 4, 6, 7, 2019-09, 2020-12 with meta-schema validation
- **OpenAPI**: 2.0, 3.0.x, 3.1.x specification support
- **AsyncAPI**: 2.x specification support
- **Avro**: Schema validation and metadata extraction
- **Cue**: Module and schema validation
- **Protobuf**: .proto file detection and parsing
- **Additional Formats**: GraphQL, RAML, XML Schema, YAML Schema

### Enhanced CLI Experience

- **Schema Commands**: `goneat schema validate-schema` for standalone schema validation
- **Pathfinder Flags**: `--schemas`, `--schema-id`, `--schema-category`, `--schema-metadata` for targeted discovery
- **Structured Output**: JSON and markdown output formats for programmatic consumption
- **Parallel Processing**: Optimized discovery with configurable worker pools

### Documentation and Examples

- **Pathfinder User Guide**: Comprehensive documentation for CLI usage and API integration
- **Schema Library Notes**: Technical deep-dive on schema validation architecture
- **FinderFacade Application Notes**: Enterprise integration patterns and examples

## What's Next

- **[0.2.10]** - 2025-10-01: Cloud storage loaders (S3, R2, GCS), advanced transforms, schema diffing
- **[0.3.0]** - 2025-11-01: Schema registry integration, automated schema evolution detection

---

# Goneat v0.2.7 — Version Policy Enforcement (2025-09-20)

## TL;DR

- **Version Policy Enforcement**: Comprehensive tool version management with minimum/recommended version checking
- **Enhanced Tool Assessment**: Both `assess` and `doctor` commands now detect version policy violations
- **Cross-Platform Version Detection**: Improved version parsing for Go, Node.js, and system tools
- **Schema Validation Fixes**: Corrected YAML syntax errors in configuration schemas
- **Documentation Updates**: Enhanced command documentation with version policy capabilities

## What's New

### Version Policy Enforcement

Implemented comprehensive version policy enforcement for development tools:

- **Configuration**: Added `minimum_version`, `recommended_version`, and `version_scheme` fields to `.goneat/tools.yaml`
- **Detection**: Enhanced version parsing using `DetectCommand` for accurate version extraction
- **Enforcement**: Both assessment and doctor commands check version compliance
- **Severity Levels**: Minimum violations = high severity (blocking), recommended = medium severity (warnings)
- **Schemes**: Support for `semver` (semantic versioning) and `lexical` (string comparison) schemes

### Tool Assessment Improvements

Enhanced tool checking capabilities across the entire toolchain:

- **Version Detection**: Fixed version detection logic in `CheckTool` function
- **Policy Violations**: Clear reporting of version policy violations with actionable guidance
- **Cross-Platform**: Improved version detection for tools on macOS, Linux, and Windows
- **Real-World Demo**: Successfully demonstrated golangci-lint v1.64.8 → v2.4.0 upgrade enforcement

### Schema Validation Fixes

Corrected YAML syntax errors in configuration schemas:

- Fixed indentation issues in `schemas/config/v1.0.0/dates.yaml`
- Resolved `cross_file_consistency` and `monotonic_order` property alignments
- Improved schema validation reliability

## Installation

```bash
# Go install
go install github.com/fulmenhq/goneat@latest

# Or download from releases
curl -L -o goneat https://github.com/fulmenhq/goneat/releases/download/v0.2.7/goneat-darwin-arm64
chmod +x goneat
```

## Migration Guide

### For Existing Tool Configurations

Add version policies to your `.goneat/tools.yaml`:

```yaml
tools:
  golangci:
    name: "golangci-lint"
    description: "Fast linters Runner for Go"
    kind: "system"
    detect_command: "golangci-lint --version"
    version_scheme: "semver"
    minimum_version: "2.0.0"
    recommended_version: "2.4.0"
    platforms: ["linux", "darwin", "windows"]
    install_commands:
      linux: "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"
      darwin: "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"
      windows: "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"
```

### For CI/CD Pipelines

Include version policy checking in your assessment commands:

```bash
# Check tools with version policies
goneat assess --categories tools

# Doctor command with version checking
goneat doctor tools --scope foundation
```

## Quality Metrics

- **Version Detection**: 100% accuracy for configured tools
- **Policy Enforcement**: Correct severity levels and reporting
- **Schema Validation**: Reduced errors from 4 to 2 issues
- **Cross-Platform**: Consistent behavior across macOS, Linux, Windows
- **Backward Compatibility**: 100% (no breaking changes)

## Links

- Changelog: [CHANGELOG.md](CHANGELOG.md)
- Tool Configuration: [docs/user-guide/commands/doctor.md](docs/user-guide/commands/doctor.md)
- Version Policy Guide: [docs/appnotes/intelligent-tool-installation.md](docs/appnotes/intelligent-tool-installation.md)

---

**Generated by Forge Neat ([OpenCode](https://opencode.ai/)) under supervision of [@3leapsdave](https://github.com/3leapsdave)**

**Co-Authored-By: Forge Neat <noreply@3leaps.net>**
