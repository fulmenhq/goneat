---
title: Tools Library
description: Tool detection, installation, and management system for development environments.
---

# Tools Library

Goneat's `pkg/tools` provides a comprehensive system for detecting, installing, and managing development tools across different platforms and environments.

## Purpose

The tools library addresses common challenges in development environment management:

- **Cross-platform Tool Detection**: Automatically detect installed tools across different operating systems
- **Intelligent Installation**: Platform-specific installation commands and package managers
- **Version Management**: Semantic versioning support with minimum/recommended versions
- **Scope-based Organization**: Logical grouping of tools by purpose (security, formatting, etc.)
- **Extensibility**: Easy to add new tools and custom installation logic

## Key Features

- **Multi-platform Support**: Linux, macOS, Windows with platform-specific commands
- **Tool Scopes**: Organize tools by purpose (foundation, security, format, etc.)
- **Version Validation**: Enforce minimum and recommended tool versions
- **Installation Automation**: Automated tool installation with fallback options
- **Configuration Management**: YAML/JSON-based tool configuration
- **Detection Logic**: Flexible command-based tool detection
- **Error Handling**: Comprehensive error reporting and recovery

## Installation

```bash
go get github.com/fulmenhq/goneat/pkg/tools
```

## Basic Usage

### Loading Tool Configuration

```go
package main

import (
    "fmt"
    "log"

    "github.com/fulmenhq/goneat/pkg/tools"
)

func main() {
    // Load tools configuration from file
    config, err := tools.LoadConfigFromFile("tools.yaml")
    if err != nil {
        log.Fatalf("Failed to load tools config: %v", err)
    }

    // List all available scopes
    fmt.Println("Available scopes:")
    for scopeName, scope := range config.Scopes {
        fmt.Printf("  %s: %s (%d tools)\n",
            scopeName, scope.Description, len(scope.Tools))
    }
}
```

### Tool Detection and Validation

```go
// Create a tool manager
manager := tools.NewManager(config)

// Check if a specific tool is available
tool, err := manager.GetTool("golangci-lint")
if err != nil {
    log.Printf("Tool not found: %v", err)
    return
}

// Check if tool is installed and meets version requirements
status, err := manager.CheckTool("golangci-lint")
if err != nil {
    log.Printf("Tool check failed: %v", err)
    return
}

fmt.Printf("Tool: %s\n", status.Name)
fmt.Printf("Installed: %v\n", status.Installed)
fmt.Printf("Version: %s\n", status.Version)
fmt.Printf("Meets Requirements: %v\n", status.MeetsRequirements)
```

### Installing Missing Tools

```go
// Install a specific tool
if err := manager.InstallTool("golangci-lint"); err != nil {
    log.Printf("Failed to install tool: %v", err)
    return
}

// Install all tools in a scope
if err := manager.InstallScope("foundation"); err != nil {
    log.Printf("Failed to install scope: %v", err)
    return
}

// Install all tools across all scopes
if err := manager.InstallAll(); err != nil {
    log.Printf("Failed to install all tools: %v", err)
    return
}
```

## Configuration Format

### Accessing the Official Schema

Goneat provides an official JSON schema for tools configuration that you can use to validate your custom configurations:

```bash
# View the tools configuration schema
goneat docs show schemas/tools/v1.0.0/tools-config

# Save schema to file for use with editors/IDEs
goneat docs show schemas/tools/v1.0.0/tools-config > tools-config-schema.json
```

### YAML Configuration Structure

```yaml
# tools.yaml
scopes:
  foundation:
    description: "Core foundation tools required for goneat and basic AI agent operation"
    tools: ["ripgrep", "jq", "yamlfmt", "golangci-lint"]

  security:
    description: "Security scanning and vulnerability detection tools"
    tools: ["gosec", "govulncheck", "gitleaks"]

  format:
    description: "Code formatting and linting tools"
    tools: ["goimports", "gofmt"]

tools:
  ripgrep:
    name: "ripgrep"
    description: "Fast text search tool"
    kind: "system"
    detect_command: "rg --version"
    platforms: ["darwin", "linux", "windows"]
    install_commands:
      darwin: "brew install ripgrep"
      linux: "sudo apt-get install ripgrep || sudo yum install ripgrep"
      windows: "winget install BurntSushi.ripgrep.MSVC"

  golangci-lint:
    name: "golangci-lint"
    description: "Fast linters Runner for Go"
    kind: "go"
    detect_command: "golangci-lint --version"
    install_package: "github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
    version_args: ["--version"]
    minimum_version: "1.54.0"
    recommended_version: "2.4.0"
    version_scheme: "semver"
```

### Schema Validation

When developing custom tools configurations, always validate against the official schema:

```go
package main

import (
    "fmt"
    "log"

    "github.com/fulmenhq/goneat/pkg/schema"
)

func validateToolsConfig() {
    // Load your tools configuration
    configData := []byte(`...your tools.yaml content...`)

    // Load the official schema
    schemaContent, err := schema.LoadEmbeddedSchema("tools/v1.0.0/tools-config")
    if err != nil {
        log.Fatalf("Failed to load schema: %v", err)
    }

    // Validate configuration against schema
    if err := schema.ValidateYAML(configData, schemaContent); err != nil {
        log.Fatalf("Configuration validation failed: %v", err)
    }

    fmt.Println("✅ Tools configuration is valid!")
}
```

Or validate from the command line:

```bash
# Validate your tools.yaml against the schema
goneat validate data --schema tools/v1.0.0/tools-config --data tools.yaml

# Or using the schema library directly
go run -c "
    import \"github.com/fulmenhq/goneat/pkg/schema\"
    schema.ValidateFile(\"tools.yaml\", \"tools/v1.0.0/tools-config\")
"
```

### Tool Definition Fields

| Field                 | Type     | Description                            | Required |
| --------------------- | -------- | -------------------------------------- | -------- |
| `name`                | string   | Display name of the tool               | Yes      |
| `description`         | string   | Human-readable description             | Yes      |
| `kind`                | string   | Tool type (system, go, bundled-go)     | Yes      |
| `detect_command`      | string   | Command to detect if tool is installed | Yes      |
| `install_package`     | string   | Go package path for installation       | No       |
| `version_args`        | []string | Arguments to get tool version          | No       |
| `check_args`          | []string | Arguments to verify tool functionality | No       |
| `platforms`           | []string | Supported platforms                    | No       |
| `install_commands`    | map      | Platform-specific install commands     | No       |
| `version_scheme`      | string   | Version format (semver, custom)        | No       |
| `minimum_version`     | string   | Minimum required version               | No       |
| `recommended_version` | string   | Recommended version                    | No       |
| `disallowed_versions` | []string | Versions to avoid                      | No       |

## Advanced Features

### Custom Tool Definitions

```go
// Define a custom tool programmatically
customTool := tools.Tool{
    Name:          "my-custom-tool",
    Description:   "Custom tool for my project",
    Kind:          "system",
    DetectCommand: "my-tool --version",
    Platforms:     []string{"linux", "darwin"},
    InstallCommands: map[string]string{
        "linux":  "wget -O /usr/local/bin/my-tool https://example.com/my-tool && chmod +x /usr/local/bin/my-tool",
        "darwin": "brew install my-tool",
    },
    MinimumVersion:     "1.0.0",
    RecommendedVersion: "2.1.0",
    VersionScheme:      "semver",
}

// Add to configuration
config.Tools["my-custom-tool"] = customTool
```

### Version Validation

```go
// Check version requirements
version, err := manager.GetToolVersion("golangci-lint")
if err != nil {
    log.Printf("Failed to get version: %v", err)
    return
}

// Validate against requirements
if err := manager.ValidateToolVersion("golangci-lint", version); err != nil {
    log.Printf("Version validation failed: %v", err)
    return
}
```

### Platform Detection

```go
// Get current platform
platform := tools.DetectPlatform()
fmt.Printf("Current platform: %s\n", platform)

// Check if tool supports current platform
tool := config.Tools["ripgrep"]
if !tools.SupportsPlatform(tool, platform) {
    fmt.Printf("Tool %s does not support platform %s\n", tool.Name, platform)
}
```

### Schema Integration

```go
// Validate tools configuration against official schema
import "github.com/fulmenhq/goneat/pkg/schema"

// Load configuration data
configData, err := os.ReadFile("tools.yaml")
if err != nil {
    log.Fatalf("Failed to read config: %v", err)
}

// Validate against embedded schema
if err := schema.ValidateYAML(configData, "tools/v1.0.0/tools-config"); err != nil {
    log.Fatalf("Schema validation failed: %v", err)
}

fmt.Println("✅ Configuration schema validation passed!")
```

### Batch Operations

```go
// Check all tools in a scope
results, err := manager.CheckScope("foundation")
if err != nil {
    log.Printf("Scope check failed: %v", err)
    return
}

for toolName, result := range results {
    if !result.Installed {
        fmt.Printf("❌ %s: not installed\n", toolName)
    } else if !result.MeetsRequirements {
        fmt.Printf("⚠️  %s: version %s does not meet requirements\n",
            toolName, result.Version)
    } else {
        fmt.Printf("✅ %s: OK (version %s)\n", toolName, result.Version)
    }
}
```

## API Reference

### Core Types

#### `Config`

Main configuration structure containing scopes and tool definitions.

```go
type Config struct {
    Scopes map[string]Scope `yaml:"scopes" json:"scopes"`
    Tools  map[string]Tool  `yaml:"tools" json:"tools"`
}
```

#### `Tool`

Represents a single tool definition with installation and detection logic.

```go
type Tool struct {
    Name               string              `yaml:"name" json:"name"`
    Description        string              `yaml:"description" json:"description"`
    Kind               string              `yaml:"kind" json:"kind"`
    DetectCommand      string              `yaml:"detect_command" json:"detect_command"`
    InstallPackage     string              `yaml:"install_package,omitempty" json:"install_package,omitempty"`
    VersionArgs        []string            `yaml:"version_args,omitempty" json:"version_args,omitempty"`
    CheckArgs          []string            `yaml:"check_args,omitempty" json:"check_args,omitempty"`
    Platforms          []string            `yaml:"platforms,omitempty" json:"platforms,omitempty"`
    InstallCommands    map[string]string   `yaml:"install_commands,omitempty" json:"install_commands,omitempty"`
    InstallerPriority  map[string][]string `yaml:"installer_priority,omitempty" json:"installer_priority,omitempty"`
    VersionScheme      string              `yaml:"version_scheme,omitempty" json:"version_scheme,omitempty"`
    MinimumVersion     string              `yaml:"minimum_version,omitempty" json:"minimum_version,omitempty"`
    RecommendedVersion string              `yaml:"recommended_version,omitempty" json:"recommended_version,omitempty"`
    DisallowedVersions []string            `yaml:"disallowed_versions,omitempty" json:"disallowed_versions,omitempty"`
}
```

#### `Scope`

Represents a logical grouping of tools.

```go
type Scope struct {
    Description string   `yaml:"description" json:"description"`
    Tools       []string `yaml:"tools" json:"tools"`
    Replace     bool     `yaml:"replace,omitempty" json:"replace,omitempty"`
}
```

### Core Functions

#### `LoadConfigFromFile(filename string) (*Config, error)`

Loads tool configuration from a YAML or JSON file.

#### `NewManager(config *Config) *Manager`

Creates a new tool manager instance.

#### `CheckTool(name string) (*ToolStatus, error)`

Checks if a tool is installed and meets version requirements.

#### `InstallTool(name string) error`

Installs a specific tool using the configured installation method.

#### `InstallScope(scopeName string) error`

Installs all tools in a specific scope.

#### `ValidateConfig(configData []byte) error`

Validates tools configuration data against the official schema.

## Tool Types

### System Tools

Tools installed via system package managers (apt, brew, winget, etc.).

```yaml
ripgrep:
  name: "ripgrep"
  description: "Fast text search tool"
  kind: "system"
  detect_command: "rg --version"
  platforms: ["darwin", "linux", "windows"]
  install_commands:
    darwin: "brew install ripgrep"
    linux: "sudo apt-get install ripgrep"
    windows: "winget install BurntSushi.ripgrep.MSVC"
```

### Go Tools

Tools installed via `go install` from GitHub repositories.

```yaml
golangci-lint:
  name: "golangci-lint"
  description: "Fast linters Runner for Go"
  kind: "go"
  detect_command: "golangci-lint --version"
  install_package: "github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
  version_args: ["--version"]
  minimum_version: "1.54.0"
  recommended_version: "2.4.0"
```

### Bundled Go Tools

Tools that come bundled with the Go toolchain.

```yaml
gofmt:
  name: "gofmt"
  description: "Go code formatter (bundled with Go)"
  kind: "bundled-go"
  detect_command: "gofmt -h"
  version_args: []
```

## Best Practices

### Schema Validation

Always validate your tools configurations against the official schema:

```bash
# Validate during development
goneat validate data --schema tools/v1.0.0/tools-config --data tools.yaml

# Integrate into CI/CD
echo "Validating tools configuration..."
goneat validate data --schema tools/v1.0.0/tools-config --data tools.yaml || exit 1
```

### Configuration Organization

```
tools/
├── base.yaml          # Core tools required by all projects
├── development.yaml   # Additional tools for development
├── ci.yaml           # Tools needed for CI/CD pipelines
└── local.yaml        # Local tool overrides (.gitignored)
```

### Tool Definition Guidelines

```yaml
# Good: Clear, descriptive tool definition
golangci-lint:
  name: "golangci-lint"
  description: "Fast linters Runner for Go with extensive rule set"
  kind: "go"
  detect_command: "golangci-lint --version"
  install_package: "github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
  minimum_version: "1.54.0"
  recommended_version: "2.4.0"

# Avoid: Vague descriptions
tool:
  name: "tool"
  description: "A tool"
  detect_command: "tool"
```

### Error Handling

```go
// Handle different types of errors
status, err := manager.CheckTool("missing-tool")
if err != nil {
    switch {
    case errors.Is(err, tools.ErrToolNotFound):
        log.Printf("Tool not defined in configuration")
    case errors.Is(err, tools.ErrToolNotInstalled):
        log.Printf("Tool not installed, attempting installation...")
        if installErr := manager.InstallTool("missing-tool"); installErr != nil {
            log.Printf("Installation failed: %v", installErr)
        }
    case errors.Is(err, tools.ErrVersionMismatch):
        log.Printf("Tool version does not meet requirements")
    default:
        log.Printf("Unexpected error: %v", err)
    }
}
```

## Examples

### Complete Tool Management System

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/fulmenhq/goneat/pkg/tools"
)

func main() {
    // Load configuration
    config, err := tools.LoadConfigFromFile("tools.yaml")
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // Create manager
    manager := tools.NewManager(config)

    // Set up signal handling for graceful shutdown
    ctx, cancel := signal.NotifyContext(context.Background(),
        os.Interrupt, syscall.SIGTERM)
    defer cancel()

    // Check and install foundation tools
    fmt.Println("🔧 Checking foundation tools...")
    if err := ensureToolsInstalled(ctx, manager, "foundation"); err != nil {
        log.Fatalf("Failed to ensure tools: %v", err)
    }

    fmt.Println("✅ All foundation tools ready!")
}

func ensureToolsInstalled(ctx context.Context, manager *tools.Manager, scope string) error {
    // Check all tools in scope
    results, err := manager.CheckScope(scope)
    if err != nil {
        return fmt.Errorf("scope check failed: %w", err)
    }

    // Install missing tools
    for toolName, status := range results {
        if !status.Installed {
            fmt.Printf("📦 Installing %s...\n", toolName)
            if err := manager.InstallTool(toolName); err != nil {
                return fmt.Errorf("failed to install %s: %w", toolName, err)
            }
        } else if !status.MeetsRequirements {
            fmt.Printf("⚠️  %s version %s does not meet requirements\n",
                toolName, status.Version)
        } else {
            fmt.Printf("✅ %s is ready\n", toolName)
        }

        // Check for context cancellation
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }
    }

    return nil
}
```

### Custom Tool Registry

```go
// Create a custom tool registry
type CustomToolRegistry struct {
    manager *tools.Manager
    customTools map[string]tools.Tool
}

func NewCustomToolRegistry(baseConfig *tools.Config) *CustomToolRegistry {
    return &CustomToolRegistry{
        manager: tools.NewManager(baseConfig),
        customTools: make(map[string]tools.Tool),
    }
}

// Add a custom tool
func (r *CustomToolRegistry) AddCustomTool(name string, tool tools.Tool) {
    r.customTools[name] = tool
    // Add to manager's configuration
    r.manager.AddTool(name, tool)
}

// Override existing tool
func (r *CustomToolRegistry) OverrideTool(name string, tool tools.Tool) {
    r.manager.UpdateTool(name, tool)
}
```

## Integration with Goneat

The tools library is extensively used throughout Goneat:

- **Doctor Command**: Uses scopes to check and install development tools
- **Assessment Runners**: Validates tool availability before running assessments
- **CI/CD Integration**: Ensures consistent tool versions across environments
- **Configuration Management**: Manages tool configurations for different environments
- **Schema Validation**: Integrates with the schema library for configuration validation

### Schema Library Integration

The tools library works seamlessly with Goneat's schema library for configuration validation:

```go
import (
    "github.com/fulmenhq/goneat/pkg/tools"
    "github.com/fulmenhq/goneat/pkg/schema"
)

// Load and validate tools configuration
config, err := tools.LoadConfigFromFile("tools.yaml")
if err != nil {
    log.Fatalf("Failed to load config: %v", err)
}

// Additional schema validation (beyond basic YAML parsing)
if err := schema.ValidateYAML(configData, "tools/v1.0.0/tools-config"); err != nil {
    log.Fatalf("Schema validation failed: %v", err)
}
```

See the Goneat codebase for real-world examples:

- [`cmd/doctor/tools.go`](https://github.com/fulmenhq/goneat/blob/main/cmd/doctor/tools.go) - Doctor command implementation
- [`internal/doctor/tools.go`](https://github.com/fulmenhq/goneat/blob/main/internal/doctor/tools.go) - Internal tool management
- [`pkg/tools/config_test.go`](https://github.com/fulmenhq/goneat/blob/main/pkg/tools/config_test.go) - Tool configuration testing
- [`pkg/schema/validator.go`](https://github.com/fulmenhq/goneat/blob/main/pkg/schema/validator.go) - Schema validation integration

## Troubleshooting

### Common Issues

**Tool not detected**

```bash
# Check if tool is in PATH
which tool-name

# Check tool version output
tool-name --version

# Verify detection command in configuration
grep "detect_command" tools.yaml
```

**Installation failures**

```bash
# Check platform detection
go run -c "fmt.Println(runtime.GOOS)"

// Check available package managers
which apt brew yum winget
```

**Version parsing issues**

```bash
# Test version command output
tool-name --version

# Check version_args in configuration
grep "version_args" tools.yaml
```

---

**Version:** 1.0.0
**Last Updated:** September 20, 2025</content>
</xai:function_call: write>
<parameter name="filePath">docs/appnotes/lib/tools.md
