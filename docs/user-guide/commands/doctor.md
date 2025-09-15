# doctor

Diagnostics and tooling checks to verify (and optionally install) external tools required by goneat features.

Available scopes:

- **security**: Security scanning tools (gosec, govulncheck, gitleaks)
- **format**: Code formatting tools (goimports, gofmt)
- **infrastructure**: Development and infrastructure CLI tools (ripgrep, jq, go-licenses)
- **all**: All tools from all scopes

- Command: `goneat doctor`
- Subcommands: `tools`, `env`

## doctor tools

Check presence and versions of supported tools, print remediation instructions, and optionally install tools using platform-specific package managers or Go install.

### Supported Tools by Scope

| Scope              | Tools                        | Purpose                                      |
| ------------------ | ---------------------------- | -------------------------------------------- |
| **security**       | gosec, govulncheck, gitleaks | Security scanning and vulnerability analysis |
| **format**         | goimports, gofmt             | Code formatting and import management        |
| **infrastructure** | ripgrep, jq, go-licenses     | Development and infrastructure CLI tools     |

### Usage

#### Basic Usage

- Check infrastructure tools (most common use case):
  - `goneat doctor tools --scope infrastructure`
- Check all tools:
  - `goneat doctor tools --scope all`
- Check specific scope:
  - `goneat doctor tools` (defaults to `--scope security`)
  - `goneat doctor tools --scope format`
- Check specific tools:
  - `goneat doctor tools --tools ripgrep,jq,go-licenses`

#### Installation & Dry Run

- Dry run (preview installations):
  - `goneat doctor tools --scope infrastructure --dry-run`
- Install missing tools (non-interactive):
  - `goneat doctor tools --scope infrastructure --install --yes`
- Install with prompts:
  - `goneat doctor tools --scope infrastructure --install`

#### Configuration & Validation

- Use custom configuration file:
  - `goneat doctor tools --config custom-tools.yaml --scope infrastructure`
- Validate configuration:
  - `goneat doctor tools --validate-config`
- List available scopes:
  - `goneat doctor tools --list-scopes`

### Flags

#### Core Flags

- `--scope security|format|infrastructure|all`
  Select the tool scope (default: `security`). Use `infrastructure` for CLI tools like ripgrep, jq, go-licenses.

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

#### Infrastructure Tools (Most Common Use Case)

```bash
# Check infrastructure tools (ripgrep, jq, go-licenses)
goneat doctor tools --scope infrastructure

# Install missing infrastructure tools non-interactively
goneat doctor tools --scope infrastructure --install --yes

# Dry run to see what would be installed
goneat doctor tools --scope infrastructure --dry-run

# JSON output for CI/CD automation
goneat doctor tools --scope infrastructure --json
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
goneat doctor tools --config custom-tools.yaml --scope infrastructure
```

#### Legacy Usage

```bash
# Interactive install (prompts for each tool)
goneat doctor tools --scope infrastructure --install

# Print instructions only (legacy, use --dry-run instead)
goneat doctor tools --scope infrastructure --print-instructions
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

```yaml
# .github/workflows/ci.yml
- name: Setup development tools
  run: |
    go install github.com/fulmenhq/goneat@latest
    goneat doctor tools --scope infrastructure --install --yes

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
