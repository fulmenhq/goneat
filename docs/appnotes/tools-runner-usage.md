# Tools Runner Usage Guide

**Version**: v0.2.5  
**Last Updated**: 2025-09-13  
**Component**: Infrastructure Tools Management

## Overview

The Tools Runner is goneat's infrastructure tools management system that provides cross-platform detection, installation, and validation of essential development tools. This system is designed to prevent CI/CD failures caused by missing or outdated tools while providing a seamless developer experience.

## Architecture

### Core Components

- **Tools Runner**: Assessment runner that validates infrastructure tools
- **Configuration System**: Schema-driven tool definitions with user overrides
- **Detection Engine**: Cross-platform tool detection with version extraction
- **Installation System**: Platform-specific installation methods with fallbacks

### Integration Points

- **Assessment Engine**: Integrated as `CategoryTools` with priority 1
- **Git Hooks**: Automatic validation in pre-commit and pre-push hooks
- **CLI Interface**: Enhanced `goneat doctor tools` command with new flags
- **Schema System**: JSON Schema validation for all configurations

## Configuration Schema

### Schema Location

- **Schema**: `schemas/tools/v1.0.0/tools-config.yaml`
- **Default Config**: `internal/doctor/tools-defaults.yaml` (embedded)
- **User Override**: `.goneat/tools.yaml` (optional)

### Schema Structure

```yaml
# .goneat/tools.yaml
scopes:
  foundation:
    description: "Core foundation tools required for goneat and basic AI agent operation"
    tools: [ripgrep, jq, go-licenses]

tools:
  ripgrep:
    name: "ripgrep"
    description: "Fast text search tool"
    kind: "system" # "go" | "bundled-go" | "system"
    detect_command: "rg --version"
    install_commands:
      darwin: "brew install ripgrep"
      linux: "apt-get install -y ripgrep"
      windows: "winget install BurntSushi.ripgrep.MSVC"
    platforms: ["darwin", "linux", "windows"]
```

## Usage Patterns

### Basic Tools Checking

```bash
# Check all infrastructure tools
goneat doctor tools --scope infrastructure

# Check specific tools
goneat doctor tools --tools ripgrep,jq

# List available scopes
goneat doctor tools --list-scopes
```

### Installation Management

```bash
# Install missing tools
goneat doctor tools --scope infrastructure --install --yes

# Dry run (preview without installing)
goneat doctor tools --scope infrastructure --dry-run

# Validate configuration
goneat doctor tools --validate-config
```

### Assessment Integration

```bash
# Tools checking in comprehensive assessment
goneat assess --categories tools

# Tools checking in git hooks (automatically configured)
goneat assess --hook pre-commit
goneat assess --hook pre-push
```

### JSON Output for Automation

```bash
# Structured output for CI/CD systems
goneat doctor tools --scope infrastructure --json

# Example JSON output
{
  "tools": [
    {
      "name": "ripgrep",
      "present": true,
      "version": "14.1.0",
      "would_install": false
    }
  ],
  "total_tools": 3,
  "would_install": 0
}
```

## Tool Detection

### Detection Methods

1. **Command Execution**: Runs `detect_command` and checks exit code
2. **Version Extraction**: Parses version information from tool output
3. **Platform-Specific**: Different detection methods per platform
4. **Fallback Handling**: Clear instructions when detection fails

### Tool Name vs Binary Name

The system handles cases where the canonical tool name differs from its executable:

```yaml
tools:
  ripgrep:
    name: "ripgrep" # Canonical name
    detect_command: "rg --version" # Binary name + args
```

### Version Extraction

Version information is extracted from tool output using standard patterns:

```bash
# Example version extraction
$ rg --version
ripgrep 14.1.0
# Extracted version: "14.1.0"
```

## Installation System

### Installation Methods

Goneat implements an intelligent installation strategy that prioritizes user experience and cross-platform compatibility. See [Intelligent Tool Installation Strategy](intelligent-tool-installation.md) for comprehensive documentation.

#### Language-Native Tools

```yaml
tools:
  golangci:
    kind: "system"
    install_commands:
      linux: "go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
      darwin: "go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
      windows: "go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
```

#### Version Manager Integration

```yaml
tools:
  ripgrep:
    kind: "system"
    install_commands:
      linux: "mise use ripgrep@latest 2>/dev/null || echo '📦 Run: sudo apt-get install ripgrep' && exit 1"
      darwin: "mise use ripgrep@latest || brew install ripgrep"
      windows: "winget install BurntSushi.ripgrep.MSVC || scoop install ripgrep"
```

### Platform Detection

The system automatically detects the current platform and uses the appropriate installation method:

- **macOS**: Uses Homebrew (`brew install`)
- **Linux**: Uses apt/yum/pacman based on distribution
- **Windows**: Uses Winget or manual installation instructions

### Error Handling

When automated installation fails, the system provides clear fallback instructions:

```bash
# Example fallback message
❌ ripgrep missing
   Instructions: brew install ripgrep
   Manual: Visit https://github.com/BurntSushi/ripgrep/releases
```

## Assessment Integration

### Tools Runner

The Tools Runner implements the `AssessmentRunner` interface:

```go
type ToolsRunner struct{}

func (r *ToolsRunner) Assess(ctx context.Context, target string, config AssessmentConfig) (*AssessmentResult, error)
func (r *ToolsRunner) CanRunInParallel() bool
func (r *ToolsRunner) GetCategory() AssessmentCategory
func (r *ToolsRunner) GetEstimatedTime(target string) time.Duration
func (r *ToolsRunner) IsAvailable() bool
```

### Assessment Results

Tools checking produces structured assessment results:

```go
type AssessmentResult struct {
    CommandName   string                 `json:"command_name"`
    Category      AssessmentCategory     `json:"category"`
    Success       bool                   `json:"success"`
    Issues        []Issue                `json:"issues"`
    Metrics       map[string]interface{} `json:"metrics"`
    ExecutionTime HumanReadableDuration  `json:"execution_time"`
}
```

### Issue Severity

Tools issues are classified by severity:

- **High**: Missing required tools (blocks CI/CD)
- **Medium**: Outdated tools (warnings)
- **Low**: Optional tools (informational)

## Git Hooks Integration

### Automatic Configuration

Tools checking is automatically included in git hooks:

```bash
# Configure hooks to include tools checking
goneat hooks policy set --hook pre-commit --categories format,lint,dates,tools
goneat hooks policy set --hook pre-push --categories format,lint,security,dates,tools
goneat hooks install
```

### Hook Execution

Tools checking runs at priority 1 (early in the pipeline):

1. **Pre-commit**: Fast validation before commits
2. **Pre-push**: Comprehensive validation before pushes
3. **Parallel execution**: Runs alongside other assessments

## Customization

### Adding Custom Tools

Create `.goneat/tools.yaml` to add custom tools:

```yaml
scopes:
  infrastructure:
    tools: [ripgrep, jq, go-licenses, yamllint, shellcheck]

tools:
  yamllint:
    name: "yamllint"
    description: "YAML linter"
    kind: "system"
    detect_command: "yamllint --version"
    install_commands:
      darwin: "brew install yamllint"
      linux: "pip install yamllint"
      windows: "pip install yamllint"
    platforms: ["darwin", "linux", "windows"]
```

### Custom Scopes

Define custom tool scopes for specific use cases:

```yaml
scopes:
  security:
    description: "Security scanning tools"
    tools: [gosec, govulncheck, gitleaks]

  infrastructure:
    description: "Infrastructure tools"
    tools: [ripgrep, jq, go-licenses]

  custom:
    description: "Project-specific tools"
    tools: [terraform, kubectl, helm]
```

## Troubleshooting

### Common Issues

#### Tool Not Found

```bash
# Check if tool is in PATH
which rg

# Verify installation
rg --version

# Check goneat detection
goneat doctor tools --scope infrastructure --verbose
```

#### Configuration Errors

```bash
# Validate configuration
goneat doctor tools --validate-config

# Check schema compliance
goneat doctor tools --validate-config --config .goneat/tools.yaml
```

#### Installation Failures

```bash
# Dry run to see what would be installed
goneat doctor tools --scope infrastructure --dry-run

# Manual installation
goneat doctor tools --scope infrastructure --print-instructions
```

### Debug Mode

Enable verbose logging for troubleshooting:

```bash
# Verbose output
goneat doctor tools --scope infrastructure --log-level debug

# JSON output with detailed information
goneat doctor tools --scope infrastructure --json --log-level debug
```

## Best Practices

### Configuration Management

1. **Use defaults**: Start with goneat's default configuration
2. **Override selectively**: Only customize what you need
3. **Validate changes**: Always validate configuration changes
4. **Version control**: Commit `.goneat/tools.yaml` to version control

### CI/CD Integration

1. **Include tools checking**: Add `--categories tools` to assessment commands
2. **Use JSON output**: Parse structured output for automation
3. **Handle failures**: Implement proper error handling for missing tools
4. **Cache tools**: Consider caching tools in CI/CD environments

### Development Workflow

1. **Pre-commit hooks**: Use tools checking in pre-commit hooks
2. **Team consistency**: Share configuration via `.goneat/tools.yaml`
3. **Documentation**: Document custom tools and their purposes
4. **Regular updates**: Keep tools up to date with latest versions

## Future Enhancements

### Planned Features (v0.2.6+)

- **Tool Versioning**: Version requirements and update policies
- **Version Introspection**: Enhanced version detection and comparison
- **Policy Enforcement**: Fail assessments on version mismatches
- **Expanded Tool Catalog**: Additional infrastructure tools
- **Library Interface**: Programmatic access to tools management

### Extension Points

The tools system is designed for extensibility:

- **Custom tool kinds**: Add new installation methods
- **Platform support**: Add support for additional platforms
- **Detection methods**: Implement custom detection logic
- **Installation providers**: Add new package managers

## API Reference

### CLI Commands

```bash
# Basic commands
goneat doctor tools --scope <scope>
goneat doctor tools --tools <tool1,tool2>
goneat doctor tools --list-scopes
goneat doctor tools --validate-config

# Installation commands
goneat doctor tools --scope <scope> --install
goneat doctor tools --scope <scope> --dry-run
goneat doctor tools --scope <scope> --print-instructions

# Configuration commands
goneat doctor tools --config <file>
goneat doctor tools --validate-config --config <file>
```

### Configuration Schema

See `schemas/tools/v1.0.0/tools-config.yaml` for complete schema definition.

### Assessment Integration

```bash
# Assessment commands
goneat assess --categories tools
goneat assess --hook pre-commit
goneat assess --hook pre-push
```

---

**Generated by Code Scout under supervision of @3leapsdave**
