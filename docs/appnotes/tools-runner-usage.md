# Tools Runner Usage Guide

**Version**: v0.3.0  
**Last Updated**: 2025-10-14  
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

Goneat implements an intelligent installation strategy that prioritizes user experience and cross-platform compatibility. The system supports two primary installation approaches:

1. **Artifact-Based Installation** (preferred): SHA256-verified binary downloads
2. **Package Manager Installation**: Platform-specific package managers

See [Intelligent Tool Installation Strategy](intelligent-tool-installation.md) for comprehensive documentation.

### Artifact-Based Installation (Supply-Chain Integrity)

For critical tools like Syft (SBOM generation), goneat uses artifact-based installation with SHA256 verification to ensure supply-chain integrity.

#### Artifact-Based Installation Workflow

Artifact-based tools use a trusted manifest with SHA256 checksums:

```yaml
tools:
  syft:
    name: "syft"
    description: "SBOM generation tool"
    kind: "system"
    detect_command: "syft version"
    platforms: ["linux", "darwin", "windows"]
    artifacts:
      default_version: "1.33.0"
      versions:
        "1.33.0":
          darwin_amd64:
            url: "https://github.com/anchore/syft/releases/download/v1.33.0/syft_1.33.0_darwin_amd64.tar.gz"
            sha256: "90c4f6b6c4bbef5c1c28de84de9920ff862dbb779bfea326feb28bacba479c34"
          linux_amd64:
            url: "https://github.com/anchore/syft/releases/download/v1.33.0/syft_1.33.0_linux_amd64.tar.gz"
            sha256: "adc1b944a827ed3432bcd9f1dbdbc8fa3c0dca7d3d449e7084c90248c2c6cb50"
```

**Installation Process**:

1. **Platform Detection**: Automatically detects OS and architecture
2. **Download**: Fetches artifact from trusted URL (HTTPS only)
3. **Verification**: Computes SHA256 and compares against manifest
4. **Extraction**: Extracts binary to `$GONEAT_HOME/tools/bin/<tool>@<version>/`
5. **Permissions**: Sets executable permissions (0755)

**Benefits**:

- ✅ Reproducible builds across environments
- ✅ Supply-chain security via cryptographic verification
- ✅ No dependency on external package managers
- ✅ Version pinning for consistency
- ✅ Air-gap support via `--from-file` option

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

## Hash Management

### Updating Artifact Manifests

When a new version of a tool is released (e.g., Syft v1.34.0), follow this process to update the manifest:

#### Step 1: Obtain Official Checksums

```bash
# Download official checksums from GitHub release
curl -sSL https://github.com/anchore/syft/releases/download/v1.34.0/syft_1.34.0_checksums.txt > checksums.txt

# Extract relevant platform checksums
grep "darwin_amd64.tar.gz" checksums.txt
grep "darwin_arm64.tar.gz" checksums.txt
grep "linux_amd64.tar.gz" checksums.txt
grep "linux_arm64.tar.gz" checksums.txt
grep "windows_amd64.zip" checksums.txt
```

#### Step 2: Verify Checksum Format

Ensure checksums are 64-character hex strings:

```bash
# Valid format (64 hex characters)
90c4f6b6c4bbef5c1c28de84de9920ff862dbb779bfea326feb28bacba479c34

# Invalid formats
90c4f6b6  # Too short
90C4F6B6... # Uppercase (use lowercase)
sha256:90c4... # No prefix
```

#### Step 3: Update .goneat/tools.yaml

Add the new version to your tool manifest:

```yaml
tools:
  syft:
    artifacts:
      default_version: "1.34.0" # Update default version
      versions:
        "1.34.0": # Add new version entry
          darwin_amd64:
            url: "https://github.com/anchore/syft/releases/download/v1.34.0/syft_1.34.0_darwin_amd64.tar.gz"
            sha256: "<new_checksum_here>"
          # ... repeat for all platforms
        "1.33.0": # Keep previous version for rollback
          # ... existing entries
```

#### Step 4: Validate Configuration

```bash
# Validate schema compliance
goneat doctor tools --validate-config

# Test installation (dry-run)
goneat doctor tools --scope sbom --dry-run
```

#### Step 5: Test Installation

```bash
# Install new version
goneat doctor tools --scope sbom --install --yes

# Verify installation
syft version

# Expected output:
# Application: syft
# Version: 1.34.0
# ...
```

### Checksum Verification Process

Goneat performs rigorous checksum verification during artifact installation:

```
1. Download artifact to $GONEAT_HOME/tools/cache/<tool>/<version>/
2. Compute SHA256 hash of downloaded file
3. Compare computed hash against manifest entry
4. If mismatch:
   - Abort installation immediately
   - Delete partial download
   - Display detailed error with remediation steps
5. If match:
   - Extract binary to $GONEAT_HOME/tools/bin/<tool>@<version>/
   - Set executable permissions
   - Update installation manifest
```

### Air-Gap Installation

For environments without internet access:

```bash
# 1. Download artifact on internet-connected machine
curl -LO https://github.com/anchore/syft/releases/download/v1.33.0/syft_1.33.0_linux_amd64.tar.gz

# 2. Verify checksum manually
echo "adc1b944a827ed3432bcd9f1dbdbc8fa3c0dca7d3d449e7084c90248c2c6cb50  syft_1.33.0_linux_amd64.tar.gz" | sha256sum -c -

# 3. Transfer artifact to air-gapped machine
scp syft_1.33.0_linux_amd64.tar.gz target-machine:/tmp/

# 4. Install from file
goneat doctor tools --scope sbom --install --from-file /tmp/syft_1.33.0_linux_amd64.tar.gz
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

#### Checksum Verification Failures

**Problem**: Installation fails with checksum mismatch error

```
Error: checksum verification failed: expected adc1b944..., got 3f2a1b56...
```

**Possible Causes**:

1. **Corrupted Download**: Network interruption during download
2. **Wrong Checksum**: Manifest contains incorrect hash
3. **Modified Binary**: Upstream changed binary without version bump
4. **Man-in-the-Middle**: Security compromise (rare but serious)

**Resolution Steps**:

```bash
# Step 1: Clear cache and retry
rm -rf $GONEAT_HOME/tools/cache/syft/
goneat doctor tools --scope sbom --install

# Step 2: Verify official checksums
curl -sSL https://github.com/anchore/syft/releases/download/v1.33.0/syft_1.33.0_checksums.txt

# Step 3: Compare with manifest
cat .goneat/tools.yaml | grep -A 5 "syft"

# Step 4: If mismatch found, update manifest
# Edit .goneat/tools.yaml with correct checksum
goneat doctor tools --validate-config

# Step 5: Retry installation
goneat doctor tools --scope sbom --install --force
```

**Security Note**: If official checksums don't match, this may indicate a supply-chain compromise. Report to tool maintainers immediately.

#### Binary Not Found After Installation

**Problem**: Tool installs successfully but not accessible in PATH

```bash
goneat doctor tools --scope sbom --install
# ✅ Installed successfully

syft version
# Error: command not found
```

**Cause**: Managed binary directory not in PATH

**Resolution**:

```bash
# Check installation location
ls $GONEAT_HOME/tools/bin/syft@*/

# Option 1: Use FindToolBinary (recommended)
# Goneat automatically uses this for --sbom command
goneat dependencies --sbom .

# Option 2: Add to PATH manually
export PATH="$GONEAT_HOME/tools/bin/syft@1.33.0:$PATH"

# Option 3: Create symlink (future: automated)
ln -s $GONEAT_HOME/tools/bin/syft@1.33.0/syft /usr/local/bin/syft
```

#### Platform Detection Issues

**Problem**: Wrong platform artifact downloaded

```
Error: no artifact available for platform linux/arm
```

**Cause**: Unsupported architecture or missing manifest entry

**Resolution**:

```bash
# Check current platform
echo "$(uname -s)_$(uname -m)"

# Verify manifest has entry for your platform
goneat doctor tools --validate-config

# If missing, add artifact entry to .goneat/tools.yaml
tools:
  syft:
    artifacts:
      versions:
        "1.33.0":
          linux_arm64:  # Add missing platform
            url: "https://..."
            sha256: "..."
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

## SBOM Tool Integration

### Syft Installation and Usage

Syft is installed as an artifact-based tool with supply-chain integrity guarantees:

```bash
# Install Syft
goneat doctor tools --scope sbom --install --yes

# Verify installation
goneat doctor tools --scope sbom

# Generate SBOM
goneat dependencies --sbom .
```

**Installation Locations**:

- **Managed Binary**: `$GONEAT_HOME/tools/bin/syft@1.33.0/syft`
- **Cache**: `$GONEAT_HOME/tools/cache/syft/1.33.0/`
- **PATH Fallback**: System-installed `syft` if available

**Detection Priority**:

1. Managed binary in `$GONEAT_HOME/tools/bin/syft@*/`
2. System binary via `exec.LookPath("syft")`
3. Installation prompt if neither found

### Integration with Dependencies Command

The SBOM scope integrates seamlessly with the dependencies command:

```bash
# Standalone SBOM generation
goneat dependencies --sbom .

# Combined with license and cooling checks
goneat dependencies --licenses --cooling --sbom .

# Custom output location
goneat dependencies --sbom --sbom-output compliance/sbom.json .

# Pipe to other tools
goneat dependencies --sbom --sbom-stdout . | jq '.components | length'
```

### Fail-On Behavior

SBOM generation has distinct failure modes from license/cooling checks:

**Dependency Analysis Failures** (controlled by `--fail-on`):

- License violations
- Cooling policy violations
- Severity-based exit codes

**SBOM Generation Failures** (independent):

- Tool not installed → Exit 1 with install instructions
- Invalid Syft output → Exit 1 with error details
- Network failure during download → Exit 1 with retry guidance

**Example**:

```bash
# This command may fail for two independent reasons:
# 1. License violations (if --fail-on threshold met)
# 2. SBOM generation errors (Syft execution failure)
goneat dependencies --licenses --sbom --fail-on high .
```

## Future Enhancements

### Planned Features (v0.3.x+)

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
