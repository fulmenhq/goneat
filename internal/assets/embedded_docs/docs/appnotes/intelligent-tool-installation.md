---
title: "Intelligent Tool Installation Strategy"
description: "Advanced cross-platform tool installation with ranked choice methods, language-native approaches, and zero-sudo strategies for development environments"
author: "goneat contributors"
date: "2025-09-15"
last_updated: "2025-09-15"
version: "v0.2.5"
component: "Infrastructure Tools Management"
tags:
  [
    "tool-installation",
    "cross-platform",
    "package-management",
    "mise",
    "language-native",
  ]
---

# Intelligent Tool Installation Strategy

## Overview

Goneat's Intelligent Tool Installation Strategy represents a significant advancement in cross-platform tool management for development environments. This strategy prioritizes user experience, system compatibility, and operational efficiency by implementing ranked choice installation methods that adapt to different environments and use cases.

## Core Philosophy

### 1. User-Centric Design

- **Zero Friction**: Prefer methods that don't require sudo/admin privileges
- **Clear Communication**: Provide actionable instructions when manual intervention is needed
- **Platform Intelligence**: Automatically detect and adapt to the target environment
- **Graceful Degradation**: Multiple fallback options ensure installation always works

### 2. Environment Awareness

- **CI/CD Ready**: No interactive prompts, suitable for automated environments
- **REPL Compatible**: Avoid commands that fail in restricted environments
- **Enterprise Friendly**: Support air-gapped environments and corporate policies
- **Developer Focused**: Optimize for local development workflows

## Installation Strategy Architecture

### Ranked Choice System

The system implements a sophisticated ranked choice approach that tries multiple installation methods in order of preference:

#### Priority 1: Language-Native Methods

```yaml
# Go tools use go install (no sudo required)
golangci:
  install_commands:
    linux: "go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"

# Node.js tools could use npm/bun/yarn
prettier:
  install_commands:
    linux: "mise use prettier@latest || npm install -g prettier"
```

#### Priority 2: Version Managers (No Sudo)

```yaml
# Mise/asdf for cross-platform tool management
ripgrep:
  install_commands:
    linux: "mise use ripgrep@latest 2>/dev/null || echo '📦 Run: sudo apt-get install ripgrep' && exit 1"
    darwin: "mise use ripgrep@latest || brew install ripgrep"
    windows: "winget install BurntSushi.ripgrep.MSVC || scoop install ripgrep"
```

#### Priority 3: System Package Managers

```yaml
# Distribution-specific package managers
jq:
  install_commands:
    linux: "mise use jq@latest 2>/dev/null || echo '📦 Run: sudo apt-get install jq' && exit 1"
    darwin: "mise use jq@latest || brew install jq"
    windows: "winget install jqlang.jq || scoop install jq"
```

### Tool Categorization

#### Foundation Tools (Always Available)

```yaml
foundation:
  description: "Core foundation tools required for goneat and basic AI agent operation"
  tools: ["mise", "ripgrep", "jq", "yamlfmt", "prettier"]
```

#### Project-Specific Tools (Language-Based)

```yaml
project-go:
  description: "Go language project development tools"
  tools: ["golangci", "go-licenses", "gosec", "govulncheck"]

project-python:
  description: "Python project development tools"
  tools: ["ruff", "mypy", "black", "isort"] # Future implementation

project-node:
  description: "Node.js project development tools"
  tools: ["eslint", "prettier", "typescript"] # Future implementation
```

#### Version Policy Configuration

Tools can be configured with version policies to ensure minimum and recommended version requirements:

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

  jq:
    name: "jq"
    description: "JSON processing tool for CI/CD scripts"
    kind: "system"
    detect_command: "jq --version"
    version_scheme: "lexical"
    minimum_version: "1.6"
    recommended_version: "1.7"
    platforms: ["linux", "darwin", "windows"]
    install_commands:
      linux: "mise use jq@latest 2>/dev/null || echo '📦 Run: sudo apt-get install jq' && exit 1"
      darwin: "mise use jq@latest || brew install jq"
      windows: "winget install jqlang.jq || scoop install jq"
```

## Implementation Details

### Mise Integration

#### Why Mise?

- **Zero Sudo**: Installs tools to user directories without root privileges
- **Cross-Platform**: Works on Linux, macOS, Windows (limited Windows support)
- **Version Management**: Handles multiple versions of the same tool
- **Broad Compatibility**: Supports hundreds of tools and runtimes
- **Fast Installation**: Uses pre-compiled binaries when available

#### Mise Installation Strategy

```yaml
mise:
  name: "mise"
  description: "Polyglot runtime manager for Linux/macOS"
  platforms: ["linux", "darwin"] # Intentionally exclude Windows
  install_commands:
    linux: "curl https://mise.jdx.dev/install.sh | sh && echo '📦 mise installed. Restart your shell or run: source ~/.bashrc'"
    darwin: "curl https://mise.jdx.dev/install.sh | sh && echo '📦 mise installed. Restart your shell or run: source ~/.zshrc'"
```

### Platform-Specific Optimizations

#### Linux Strategy

```bash
# 1. Try mise first (no sudo, works on all distros)
mise use ripgrep@latest 2>/dev/null ||
# 2. Provide clear sudo instructions for package manager
echo '📦 Run: sudo apt-get install ripgrep' && exit 1
```

#### macOS Strategy

```bash
# 1. Try mise first (consistent with Linux)
mise use ripgrep@latest ||
# 2. Fall back to Homebrew (standard on macOS)
brew install ripgrep
```

#### Windows Strategy

```bash
# 1. Try Winget (built into Windows 10/11)
winget install BurntSushi.ripgrep.MSVC ||
# 2. Fall back to Scoop (popular Windows package manager)
scoop install ripgrep
```

### Language-Native Installation

#### Go Tools

```yaml
golangci:
  install_commands:
    linux: "go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
    darwin: "go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
    windows: "go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
```

**Benefits:**

- No external dependencies required
- Version controlled by go.mod
- Works in air-gapped environments
- Consistent across all platforms

#### Node.js Tools

```yaml
prettier:
  install_commands:
    linux: "mise use prettier@latest || npm install -g prettier"
    darwin: "mise use prettier@latest || npm install -g prettier"
    windows: "mise use prettier@latest || npm install -g prettier"
```

**Benefits:**

- Respects project's Node.js version
- Works with npm, yarn, pnpm, bun
- Handles peer dependencies automatically

## Error Handling and User Experience

### Graceful Failure Strategy

#### When Mise Fails

```bash
# Try mise quietly, if it fails provide clear instructions
mise use ripgrep@latest 2>/dev/null ||
echo '📦 Run: sudo apt-get install ripgrep' && exit 1
```

#### When Package Managers Need Sudo

```bash
# Agent provides clear instructions and exits gracefully
echo '📦 ripgrep requires installation. Run: sudo apt-get update && sudo apt-get install -y ripgrep' && exit 1
```

### User Communication

#### Clear Instructions

```
📦 ripgrep requires installation. Run: sudo apt-get install ripgrep
```

#### Platform-Specific Guidance

- **Linux**: Suggests apt/pacman/dnf based on detected distribution
- **macOS**: Suggests Homebrew (standard package manager)
- **Windows**: Suggests Winget (built-in) or Scoop (popular alternative)

## Configuration Options

### Optional Mise Installation

Future enhancement: Add `enabled` boolean to tool definitions:

```yaml
tools:
  mise:
    name: "mise"
    enabled: true # User can disable if preferred
    platforms: ["linux", "darwin"]
    install_commands:
      linux: "curl https://mise.jdx.dev/install.sh | sh"
      darwin: "curl https://mise.jdx.dev/install.sh | sh"
```

### Advanced Ranked Choice Configuration

Future enhancement: Allow users to customize installation priority with platform-specific preferences:

```yaml
preferences:
  installation_methods:
    global_priority:
      - "language-native" # go install, npm install, etc.
      - "version-manager" # mise, asdf, etc.
      - "system-package" # apt, brew, winget, etc.
      - "manual" # Clear instructions for user

    # Platform-specific overrides
    macos:
      priority:
        ["version-manager", "system-package", "language-native", "manual"]
      notes: "macOS users often prefer Homebrew for system packages"

    windows:
      priority:
        ["system-package", "version-manager", "language-native", "manual"]
      notes: "Windows users prefer Winget (built-in) over version managers"

    linux:
      priority:
        ["version-manager", "language-native", "system-package", "manual"]
      notes: "Linux prioritizes version managers to avoid sudo requirements"
```

#### Platform-Specific Preferences

**macOS Considerations:**

- Homebrew is the de facto standard package manager
- Many developers prefer `brew install` over version managers
- Could implement: `brew install` → mise → manual instructions

**Windows Considerations:**

- Winget is built into Windows 10/11
- Scoop is popular for development tools
- Could implement: winget → scoop → manual instructions

**Linux Considerations:**

- Multiple distributions with different package managers
- Version managers work well across all distros
- Could implement: mise → distro-specific package manager → manual

## Version Policy Enforcement

### Assessment Integration

Version policies are enforced through goneat's assessment system:

```bash
# Check all tools for version compliance
goneat assess --categories tools

# Check specific tools
goneat doctor tools --scope foundation
```

### Policy Violation Handling

- **Minimum Version Violations**: Reported as high severity (blocking) issues
- **Recommended Version Violations**: Reported as medium severity (warning) issues
- **Version Schemes**:
  - `semver`: Semantic versioning (major.minor.patch)
  - `lexical`: String comparison for tools with non-standard versioning

### Example Policy Violations

```
❌ Tool golangci-lint version 1.64.8 does not meet minimum requirement 2.0.0 (scheme: semver)
⚠️  Tool prettier version 3.2.0 does not meet recommended version 3.3.0 (scheme: semver)
```

## Benefits and Impact

### For Developers

- **Faster Setup**: Tools install without sudo in most cases
- **Consistent Experience**: Same commands work across platforms
- **Clear Guidance**: Helpful instructions when manual intervention needed
- **Project-Specific**: Language-native tools respect project constraints

### For CI/CD Systems

- **Automated**: No interactive prompts required
- **Reliable**: Multiple fallback options ensure success
- **Fast**: Mise provides pre-compiled binaries
- **Secure**: No sudo required in container environments

### For Enterprise Environments

- **Policy Compliant**: Respects corporate package manager preferences
- **Air-Gapped Ready**: Works with language-native installation methods
- **Auditable**: Clear installation methods and fallback paths
- **Supportable**: Standardized approach across teams

## Future Enhancements

### 1. Dynamic Package Manager Detection

```bash
# Automatically detect and use the best available package manager
detect_package_manager() {
  command -v apt-get >/dev/null 2>&1 && echo "apt-get" && return
  command -v pacman >/dev/null 2>&1 && echo "pacman" && return
  command -v dnf >/dev/null 2>&1 && echo "dnf" && return
  command -v yum >/dev/null 2>&1 && echo "yum" && return
}
```

### 2. Tool Version Pinning

```yaml
tools:
  golangci:
    version: "v1.55.0" # Pin specific versions for reproducibility
    install_commands:
      linux: "go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.55.0"
```

### 3. Custom Installation Scripts

```yaml
tools:
  custom-tool:
    install_commands:
      linux: "./scripts/install-custom-tool.sh"
      darwin: "./scripts/install-custom-tool.sh"
      windows: "powershell ./scripts/install-custom-tool.ps1"
```

## Conclusion

Goneat's Intelligent Tool Installation Strategy represents a significant improvement over traditional package management approaches. By implementing ranked choice installation methods, prioritizing user experience, and providing clear fallback paths, goneat ensures that development tools are installed reliably across diverse environments while maintaining security and operational best practices.

The strategy's emphasis on language-native installation methods, version manager integration, and graceful error handling makes it particularly well-suited for modern development workflows, CI/CD pipelines, and enterprise environments.

---

**Related Documents:**

- [Tools Runner Usage Guide](tools-runner-usage.md) - Basic usage patterns
- [Configuration Schema](schema-validation.md) - Schema validation patterns
- [Assessment Integration](assessment-workflow.md) - Assessment system integration
