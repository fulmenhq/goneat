# doctor

Diagnostics and tooling checks to verify (and optionally install) external tools required by goneat features.

Available scopes:

- **foundation**: Core development tools (ripgrep, jq, yq, prettier, yamlfmt, golangci-lint)
- **security**: Security scanning tools (gosec, govulncheck, gitleaks)
- **format**: Code formatting tools (goimports, gofmt)
- **all**: All tools from all scopes

- Command: `goneat doctor`
- Subcommands: `tools`, `versions`, `env`

## Two Paths for Foundation Tools (v0.3.14+)

goneat supports two approaches for ensuring foundation tools are available:

| Path                | Approach                                           | Friction | Best For                                    |
| ------------------- | -------------------------------------------------- | -------- | ------------------------------------------- |
| **Container**       | Run in `ghcr.io/fulmenhq/goneat-tools` container   | LOW      | CI runners, consistent environments         |
| **Package Manager** | `goneat doctor tools --scope foundation --install` | HIGHER   | Local development, users without containers |

### Container Path (Recommended for CI)

The `goneat-tools` container includes all foundation tools pre-installed. No package manager setup required:

```yaml
# .github/workflows/ci.yml
jobs:
  format-check:
    runs-on: ubuntu-latest
    container:
      image: ghcr.io/fulmenhq/goneat-tools:latest
    steps:
      - uses: actions/checkout@v4
      - run: |
          goneat format --check .
          yamlfmt -lint .
          prettier --check "**/*.{md,json}"
```

### Package Manager Path (Local Development)

For local development or environments where containers aren't available:

```bash
# Initialize tools configuration (creates .goneat/tools.yaml)
goneat doctor tools init

# Install all foundation tools
goneat doctor tools --scope foundation --install --yes
```

## doctor tools

Check presence, versions, and version policy compliance of supported tools, print remediation instructions, and optionally install tools using platform-specific package managers or Go install.

### Supported Tools by Scope

| Scope          | Tools                                             | Purpose                                      |
| -------------- | ------------------------------------------------- | -------------------------------------------- |
| **foundation** | ripgrep, jq, yq, prettier, yamlfmt, golangci-lint | Core development and formatting tools        |
| **security**   | gosec, govulncheck, gitleaks                      | Security scanning and vulnerability analysis |
| **format**     | goimports, gofmt                                  | Code formatting and import management        |

### Usage

#### Basic Usage

- Check foundation tools (most common use case):
  - `goneat doctor tools --scope foundation`
- Check all tools:
  - `goneat doctor tools --scope all`
- Check specific scope:
  - `goneat doctor tools` (defaults to `--scope foundation`)
  - `goneat doctor tools --scope format`
- Check specific tools:
  - `goneat doctor tools --tools ripgrep,jq,prettier`

#### Initialization (Required for v0.3.7+)

Before using `doctor tools`, initialize the tools configuration:

- Initialize with defaults:
  - `goneat doctor tools init`
- Initialize with minimal (CI-safe) tools:
  - `goneat doctor tools init --minimal`
- Force re-initialization:
  - `goneat doctor tools init --force`

This creates `.goneat/tools.yaml` with all standard scopes (foundation, security, format, all).

#### Installation & Dry Run

- Dry run (preview installations):
  - `goneat doctor tools --scope foundation --dry-run`
- Install missing tools (non-interactive):
  - `goneat doctor tools --scope foundation --install --yes`
- Install with prompts:
  - `goneat doctor tools --scope foundation --install`
- Install without cooling policy checks (CI/offline):
  - `goneat doctor tools --scope foundation --install --yes --no-cooling`

#### Configuration & Validation

- Use custom configuration file:
  - `goneat doctor tools --config custom-tools.yaml --scope foundation`
- Validate configuration:
  - `goneat doctor tools --validate-config`
- List available scopes:
  - `goneat doctor tools --list-scopes`

### Flags

#### Core Flags

- `--scope foundation|security|format|all`
  Select the tool scope (default: `foundation`). Use `foundation` for core development tools.

- `--tools string[,string]`
  Comma-separated list of tool names to target (e.g., `ripgrep,jq,go-licenses`). Overrides `--scope`.

- `--all`
  Target all tools in the selected scope (has no effect if `--tools` is specified).

#### Installation Flags

- `--install`
  Attempt to install missing tools using platform-specific package managers or Go install.

- `--dry-run`
  Preview what would be installed without executing installation commands.

- `--yes`
  Assume "yes" to install prompts (non-interactive mode). Ignored unless `--install` is set.

#### Configuration & Validation

- `--config string`
  Path to custom tools configuration file (default: uses embedded configuration).

- `--validate-config`
  Validate configuration file and exit (no tool checking).

- `--list-scopes`
  List available scopes and exit.

#### Output Flags

- `--print-instructions`
  Print explicit install guidance for missing tools (legacy, use `--dry-run` for better output).

#### Global Flags (inherited)

- `--json`
  Output results in JSON format for programmatic consumption.
- `--verbose`
  Show detailed output including version information.

### Exit codes

- `0` All requested tools are present (or were installed successfully).
- `1` One or more requested tools are missing after the run (or installation failed). The command prints guidance.

### Safety and Policies

#### Installation Safety

- **Platform-specific package managers**: Uses `brew` (macOS), `apt`/`yum` (Linux), `winget` (Windows) for system tools
- **Go tools**: Uses `go install` for Go-based tools like `go-licenses`
- **No sudo**: Avoids system-wide installations where possible
- **Fallback instructions**: Provides manual installation commands when automated installation fails
- **Dry-run support**: Preview installations before executing

#### Configuration Safety

- **Schema validation**: All configurations are validated against JSON Schema before use
- **Embedded defaults**: Safe, tested default configurations included in binary
- **User overrides**: Optional `.goneat/tools.yaml` for customization
- **Version compatibility**: Schema versioning ensures configuration compatibility

#### Cross-Platform Support

- **macOS**: Homebrew package manager integration
- **Linux**: apt, pacman, yum/dnf package manager detection
- **Windows**: winget integration with PowerShell support
- **Universal**: Go-based tools work identically across platforms

### Examples

#### Foundation Tools (Most Common Use Case)

```bash
# Check foundation tools (ripgrep, jq, yq, prettier, yamlfmt, golangci-lint)
goneat doctor tools --scope foundation

# Install missing foundation tools non-interactively
goneat doctor tools --scope foundation --install --yes

# Install without cooling checks (for CI/offline environments)
goneat doctor tools --scope foundation --install --yes --no-cooling

# Dry run to see what would be installed
goneat doctor tools --scope foundation --dry-run

# JSON output for CI/CD automation
goneat doctor tools --scope foundation --json
```

#### Specific Tools

```bash
# Check specific tools
goneat doctor tools --tools ripgrep,jq

# Check all security tools
goneat doctor tools --scope security
```

#### Configuration Management

```bash
# Validate custom configuration
goneat doctor tools --validate-config --config .goneat/tools.yaml

# List available scopes
goneat doctor tools --list-scopes

# Use custom configuration
goneat doctor tools --config custom-tools.yaml --scope foundation
```

#### Legacy Usage

```bash
# Interactive install (prompts for each tool)
goneat doctor tools --scope foundation --install

# Print instructions only (legacy, use --dry-run instead)
goneat doctor tools --scope foundation --print-instructions
```

### Integration Points

#### Assessment Integration

Tools checking is integrated into `goneat assess` as the `tools` category:

```bash
# Check tools as part of comprehensive assessment
goneat assess --categories tools

# Tools checking in git hooks (automatically configured)
goneat assess --hook pre-commit  # Includes tools checking
goneat assess --hook pre-push   # Includes tools checking
```

#### CI/CD Integration

**Option A: Container-based (Recommended - LOW friction)**

```yaml
# .github/workflows/ci.yml
jobs:
  format-check:
    runs-on: ubuntu-latest
    container:
      image: ghcr.io/fulmenhq/goneat-tools:latest
    steps:
      - uses: actions/checkout@v4
      - run: |
          goneat format --check .
          goneat assess --categories format
```

**Option B: Package manager installation (HIGHER friction)**

```yaml
# .github/workflows/ci.yml
- name: Setup development tools
  run: |
    go install github.com/fulmenhq/goneat@latest
    goneat doctor tools init
    goneat doctor tools --scope foundation --install --yes --no-cooling

- name: Run comprehensive assessment
  run: |
    goneat assess --categories tools,format,security
```

#### Git Hooks Integration

Tools checking is automatically included in pre-commit and pre-push hooks:

```bash
# Regenerate hooks with tools checking
goneat hooks policy set --hook pre-commit --categories format,lint,dates,tools
goneat hooks policy set --hook pre-push --categories format,lint,security,dates,tools
goneat hooks install
```

### Known Behaviors

#### Tool Detection

- **Version extraction**: Attempts to show version information when available
- **Platform-specific detection**: Different detection methods per operating system
- **PATH validation**: Checks if tools are in PATH and provides helpful relocation instructions
- **Go bin detection**: Special handling for Go-installed tools in GOPATH/bin

#### Version Policy Checking

- **Policy enforcement**: Checks installed tool versions against configured minimum and recommended versions
- **Severity levels**: Minimum version violations reported as high severity, recommended as medium severity
- **Version schemes**: Supports semver (semantic versioning) and lexical (string comparison) schemes
- **Configuration-driven**: Version policies defined in `.goneat/tools.yaml` with `minimum_version` and `recommended_version` fields

#### Installation Behavior

- **Platform detection**: Automatically detects macOS/Linux/Windows and uses appropriate package manager
- **Fallback instructions**: Provides manual installation commands when automated installation fails
- **Post-install validation**: Re-checks tool presence after installation
- **Non-interactive mode**: `--yes` flag for CI/CD environments

#### Configuration Behavior

- **Schema validation**: All configurations validated against JSON Schema before use
- **Embedded defaults**: Safe defaults included in binary, no external dependencies
- **User overrides**: `.goneat/tools.yaml` can extend or override defaults
- **Error reporting**: Clear error messages with line numbers for configuration issues

#### Output Behavior

- **JSON support**: Structured output for automation and AI agents
- **Color output**: Colored output by default (respects `--no-color` flag)
- **Verbose mode**: Detailed information with `--verbose` flag
- **Exit codes**: Standard exit codes (0=success, 1=missing tools)

## doctor versions

Detect and manage multiple goneat installations on your system to prevent version conflicts.

When working with multiple repositories, you may encounter situations where different goneat versions are installed:

- Global installation via `go install` (in GOPATH/bin)
- Project-local installations via bootstrap (in ./bin/goneat)
- Development builds (in ./dist/goneat)
- Other PATH locations

The `doctor versions` command scans your system, identifies all goneat installations, compares versions, and provides recommendations for resolving conflicts.

### Usage

#### Detection

```bash
# Detect all goneat installations and identify conflicts
goneat doctor versions

# JSON output for programmatic consumption
goneat doctor versions --json
```

#### Conflict Resolution

```bash
# Remove stale global installation from GOPATH/bin
goneat doctor versions --purge --yes

# Update global installation to latest version
goneat doctor versions --update --yes
```

### Flags

#### Core Flags

- `--purge`
  Remove stale global installation from GOPATH/bin. Only removes the global installation if version conflicts are detected.

- `--update`
  Update global installation to latest version using `go install github.com/fulmenhq/goneat@latest`.

- `--yes`
  Assume "yes" to prompts (non-interactive mode). Required for automated operations in CI/CD.

#### Global Flags (inherited)

- `--json`
  Output results in JSON format including all detected installations and conflict information.

- `--no-color`
  Disable colored output (useful for CI/CD logs).

### What It Detects

The command scans for goneat binaries in:

1. **GOPATH/bin** - Global installation from `go install`
2. **./bin/goneat** - Project-local bootstrap installation
3. **./dist/goneat** - Development build in current repository
4. **All PATH directories** - Any other goneat binaries in system PATH

For each installation found, it reports:

- Version information
- Installation type (global, project-local, development, path)
- Current running binary indicator (▶️)

### Examples

#### Basic Detection

```bash
# Check for version conflicts
goneat doctor versions
```

**Example Output**:

```
Goneat Version Analysis
=======================

Current running version: v0.3.2
Current binary path: /Users/you/project/dist/goneat

Detected installations:
   v0.2.11      | global | /Users/you/go/bin/goneat
▶️ v0.3.2       | development | /Users/you/project/dist/goneat

⚠️  Warning: 1 version conflict(s) detected

Recommendations:
1. Remove stale global installation:
   goneat doctor versions --purge --yes

2. Or update global installation to latest:
   goneat doctor versions --update --yes

3. Or use project-local installations (recommended):
   - Bootstrap to ./bin/goneat per project
   - See: goneat docs show user-guide/bootstrap
```

#### Purge Stale Global Installation

```bash
# Interactive (prompts for confirmation)
goneat doctor versions --purge

# Non-interactive (CI/CD friendly)
goneat doctor versions --purge --yes
```

#### Update Global Installation

```bash
# Update to latest version
goneat doctor versions --update --yes
```

#### CI/CD Integration

```bash
# Validate no version conflicts in CI
goneat doctor versions --json | jq '.conflict_count'
# Exit code 0 if no conflicts, 1 if conflicts exist
```

### Exit Codes

- `0` - No version conflicts detected (or conflicts successfully resolved)
- `1` - Version conflicts detected (and not resolved)

### Known Behaviors

#### Detection Behavior

- **Deduplication**: Same binary found in multiple PATH locations is reported once
- **Version extraction**: Attempts to extract version from each binary by running `goneat version`
- **Current binary**: Marks the currently executing binary with ▶️ indicator
- **Unknown versions**: Reports "unknown" if version cannot be determined

#### Conflict Detection

- **Version comparison**: Compares all detected versions against currently running version
- **Global priority**: Prioritizes showing global (GOPATH/bin) conflicts in recommendations
- **Clean state**: Reports "✅ No version conflicts" when all versions match

#### Purge Behavior

- **Global only**: Only removes from GOPATH/bin (never removes project-local or development builds)
- **Conflict required**: Only operates when conflicts are detected
- **Confirmation**: Requires `--yes` flag for non-interactive execution
- **Safety**: Validates path before deletion to prevent accidental removal

#### Update Behavior

- **Go install**: Uses `go install github.com/fulmenhq/goneat@latest`
- **Replaces global**: Replaces existing GOPATH/bin installation
- **Requires Go**: Needs Go toolchain installed and configured
- **Internet required**: Downloads from GitHub (fails in offline environments)

### Use Cases

#### Multi-Repository Development

**Problem**: Developer has multiple repositories using different goneat versions:

- Repository A uses v0.3.0 (bootstrapped)
- Repository B uses v0.3.2 (bootstrapped)
- Old global installation v0.2.11 in PATH causes confusion

**Solution**:

```bash
cd repo-a
goneat doctor versions  # Detects conflict
goneat doctor versions --purge --yes  # Removes stale global
```

#### Onboarding New Team Members

**Problem**: New developer clones repository and encounters unexpected goneat behavior.

**Solution**:

```bash
# First step in onboarding script
goneat doctor versions
# Identifies any pre-existing installations that might conflict
```

#### CI/CD Validation

**Problem**: Ensure build environments have correct goneat version.

**Solution**:

```yaml
# .github/workflows/ci.yml
- name: Validate goneat version
  run: |
    goneat doctor versions --json
    if [ $(goneat doctor versions --json | jq '.conflict_count') -gt 0 ]; then
      echo "Version conflicts detected"
      exit 1
    fi
```

#### Troubleshooting Unexpected Behavior

**Problem**: Commands produce unexpected results, potentially due to version mismatch.

**Solution**:

```bash
# Debug which version is actually running
goneat doctor versions
# Shows all installations and which one is being executed
```

### Best Practices

1. **Project-local installations**: Use bootstrap pattern (./bin/goneat) for version pinning per project
2. **Periodic audits**: Run `goneat doctor versions` periodically to detect conflicts
3. **Onboarding scripts**: Include version check in repository setup documentation
4. **CI/CD validation**: Add version conflict check to CI pipelines
5. **Global installations**: Avoid global `go install` unless you maintain it regularly
