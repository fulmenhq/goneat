# goneat Tools CI/CD Runner Support

**Status**: Implemented in v0.3.7, Enhanced in v0.3.9
**Author**: Forge Neat, Arch Eagle
**Last Updated**: 2025-12-01

---

## Overview

goneat provides **zero-friction tool installation** on CI/CD runners through automatic PATH management AND automatic package manager installation. Tools installed via package managers (mise, bun, scoop, brew) are **immediately usable** without manual PATH configuration.

This document explains how goneat achieves seamless tool availability on fresh CI/CD runners.

### What's New in v0.3.9

**Automatic Package Manager Installation**: goneat now automatically installs bun or brew when needed:

```yaml
# Just run bootstrap - goneat handles everything
- name: Bootstrap tools
  run: make bootstrap
  # ✅ Installs bun automatically (if not present)
  # ✅ Installs tools via bun
  # ✅ Updates PATH for current session
  # ✅ Updates $GITHUB_PATH for subsequent steps
```

No need to pre-install package managers - goneat bootstraps them for you.

### Scope & Limitations (v0.3.9)

**✅ Fully Supported**:
- **Package manager auto-install**: bun (all platforms), brew user-local (darwin/linux)
- **Tool installation via**: mise, bun, scoop, brew, go-install
- **CI platform**: GitHub Actions (automatic `$GITHUB_PATH` updates)
- **OS**: macOS (darwin), Linux (ubuntu-latest tested), Windows (scoop)

**⚠️ Limited Support**:
- **Other CI platforms** (GitLab CI, CircleCI, Jenkins): Manual activation via `goneat doctor tools env --activate` required

**Note**: Custom package manager shim locations still use hardcoded paths (mise, bun, scoop, go-install)

---

## The PATH Management Problem

### Traditional Workflow (Manual PATH Updates)

```yaml
# ❌ Traditional approach - requires manual PATH management
- name: Install package manager
  run: curl https://mise.run | sh

- name: Install tool via package manager
  run: mise use -g yq@latest
  # Tool installed to ~/.local/share/mise/shims/yq
  # But ~/.local/share/mise/shims is NOT in PATH!

- name: Manually update PATH
  run: echo "$HOME/.local/share/mise/shims" >> $GITHUB_PATH

- name: Verify tool works
  run: yq --version  # Finally works!
```

**Problems**:
- 3 separate steps required
- User must know shim directory location
- Platform-specific (mise vs bun vs scoop)
- Error-prone (easy to forget PATH step)

### goneat Approach (Automatic PATH Management)

```yaml
# ✅ goneat approach - single command, zero manual steps
- name: Install tools
  run: goneat doctor tools --scope foundation --install --yes
  # Tools installed AND PATH updated automatically

- name: Verify tools work
  run: yq --version  # ✅ Works immediately!
```

**Benefits**:
- Single command
- No manual PATH manipulation
- Platform-agnostic
- Impossible to forget PATH step

---

## How It Works

### Architecture: Three-Layer PATH Management

```
┌─────────────────────────────────────────────────────────┐
│ 1. Package Manager Detection                           │
│    - Reads config/tools/foundation-package-managers.yaml│
│    - Identifies installed package managers              │
│    - Checks requires_path_update flag                   │
└─────────────────────────────────────────────────────────┘
                         ↓
┌─────────────────────────────────────────────────────────┐
│ 2. Shim Directory Discovery                            │
│    - mise:  ~/.local/share/mise/shims                   │
│    - bun:   ~/.bun/bin                                  │
│    - scoop: ~/scoop/shims                               │
│    - Verifies directories exist                         │
└─────────────────────────────────────────────────────────┘
                         ↓
┌─────────────────────────────────────────────────────────┐
│ 3. Automatic PATH Extension                            │
│    A. goneat's Process: os.Setenv("PATH", ...)         │
│       → Tools detectable by goneat immediately          │
│                                                         │
│    B. GitHub Actions: Append to $GITHUB_PATH file       │
│       → Tools available in subsequent workflow steps    │
└─────────────────────────────────────────────────────────┘
```

### Implementation Details

#### Phase 1: On `goneat doctor tools` Startup

Location: `cmd/doctor.go:runDoctorTools()`

```go
// Load package managers config
pkgMgrConfig, err := LoadPackageManagersConfig()

// Identify which shim directories need to be added
additions := GetRequiredPATHAdditions(pkgMgrConfig)
// Returns: ["/Users/user/.local/share/mise/shims", "/Users/user/.bun/bin"]

// Extend goneat's own PATH
pathMgr := NewPathManager()
pathMgr.AddToSessionPATH(additions...)
// Now goneat can detect tools in shim directories
```

**Result**: goneat can immediately detect tools installed to shim directories, even if user hasn't updated their shell profile.

#### Phase 2: During Tool Installation

Location: `internal/doctor/tools.go:installSystemTool()`

```go
// After successful installation via package manager
if toolInstalledButNotInPATH {
    shimPath := GetShimPath("mise")  // e.g., ~/.local/share/mise/shims

    // Extend PATH for goneat's process
    if !IsPathInPATH(shimPath) {
        pathMgr.AddToSessionPATH(shimPath)
    }

    // Provide instructions for persistent setup
    status.Instructions = BuildPATHInstructions(toolName, shimPath, "mise")
}
```

**Result**: Tools are detectable immediately after installation within the same goneat run.

#### Phase 3: GitHub Actions Integration (Automatic)

Location: `cmd/doctor.go:runDoctorTools()`

```go
// If in GitHub Actions and --install flag used
if flagDoctorInstall && os.Getenv("GITHUB_ACTIONS") == "true" {
    githubPath := os.Getenv("GITHUB_PATH")
    if githubPath != "" {
        // Append shim directories to $GITHUB_PATH file
        updateGitHubActionsPath(githubPath, additions)
        // Subsequent workflow steps will have tools in PATH
    }
}
```

**Result**: Tools are available in all subsequent GitHub Actions workflow steps automatically.

---

## Platform Support

### Supported Package Managers

| Package Manager | Platforms | Shim Directory | PATH Update | Auto-Install Safe |
|----------------|-----------|----------------|-------------|-------------------|
| **mise** | darwin, linux | `~/.local/share/mise/shims` | ✅ Automatic | ✅ Yes (no sudo) |
| **bun** | all | `~/.bun/bin` | ✅ Automatic | ✅ Yes (no sudo) |
| **scoop** | windows | `~/scoop/shims` | ✅ Automatic | ✅ Yes (no admin) |
| **go-install** | all | `~/go/bin` or `$GOBIN` | N/A (usually in PATH) | ✅ Yes |
| brew | darwin, linux | `/opt/homebrew/bin` | N/A (installer updates PATH) | ❌ No (requires sudo) |

### CI/CD Runner Support

| Platform | Status | Notes |
|----------|--------|-------|
| **GitHub Actions** (Linux) | ✅ Full Support | Automatic `$GITHUB_PATH` updates |
| **GitHub Actions** (macOS) | ✅ Full Support | Automatic `$GITHUB_PATH` updates |
| **GitHub Actions** (Windows) | ✅ Full Support | Automatic `$GITHUB_PATH` updates via scoop |
| GitLab CI | ⚠️ Manual | Use `goneat doctor tools env >> $CI_ENV_FILE` |
| CircleCI | ⚠️ Manual | Use `eval "$(goneat doctor tools env --activate)"` in run steps |
| Jenkins | ⚠️ Manual | Use `goneat doctor tools env` and update PATH in pipeline |

---

## Usage Examples

### GitHub Actions (Recommended)

```yaml
name: CI

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      # Initialize tools configuration (one-time, commit to repo)
      - name: Initialize goneat tools config
        run: goneat doctor tools init --minimal
        # Creates .goneat/tools.yaml with language-native tools only

      # Install package managers + tools (fully automatic)
      - name: Install tools
        run: goneat doctor tools --scope foundation --install --yes
        # ✅ Installs tools via mise/bun
        # ✅ Extends goneat's PATH for detection
        # ✅ Updates $GITHUB_PATH for subsequent steps

      # Tools immediately available - no manual PATH steps!
      - name: Verify tools
        run: |
          yq --version
          jq --version
          ripgrep --version
        # ✅ All work immediately

      - name: Run tests
        run: make test
```

### Local Development

```bash
# One-time setup per repository
goneat doctor tools init                    # Create .goneat/tools.yaml
goneat doctor tools --install --yes         # Install tools

# Add to shell profile for persistence (optional)
goneat doctor tools env >> ~/.bashrc        # Bash
goneat doctor tools env >> ~/.zshrc         # Zsh
goneat doctor tools env --shell fish >> ~/.config/fish/config.fish  # Fish

# Or activate temporarily in current shell
eval "$(goneat doctor tools env --activate)"

# Verify
yq --version
```

### GitLab CI (Manual PATH Update)

```yaml
test:
  script:
    # Install tools
    - goneat doctor tools --scope foundation --install --yes

    # Update PATH for subsequent commands
    - eval "$(goneat doctor tools env --activate)"

    # Tools now available
    - yq --version
    - make test
```

---

## Troubleshooting

### Tools Installed But Not Found

**Symptom**: `goneat doctor tools` reports tools present, but direct execution fails:
```bash
$ goneat doctor tools --tools yq
INFO: yq present (v4.48.2)

$ yq --version
command not found: yq
```

**Cause**: goneat extended its own PATH but not your shell's PATH.

**Solution**: Activate PATH in your shell:
```bash
# Temporary (current shell only)
eval "$(goneat doctor tools env --activate)"

# Permanent (add to shell profile)
goneat doctor tools env >> ~/.bashrc  # or ~/.zshrc
source ~/.bashrc
```

### GitHub Actions: Tools Not Available in Later Steps

**Symptom**: Tools work in installation step but fail in later steps.

**Cause**: `--install` flag not used, so `$GITHUB_PATH` was not updated.

**Solution**: Use `--install` flag:
```yaml
# ❌ Wrong - no --install flag
- run: goneat doctor tools --scope foundation --yes

# ✅ Correct - --install triggers GitHub Actions integration
- run: goneat doctor tools --scope foundation --install --yes
```

### Permission Denied on CI Runners

**Symptom**: Installation fails with permission errors.

**Cause**: Trying to use package managers that require sudo (brew, apt-get).

**Solution**: Use sudo-free package managers via `--minimal`:
```yaml
# Use only language-native tools (go-install, npm, etc.)
- run: goneat doctor tools init --minimal
- run: goneat doctor tools --install --yes
```

Or install recommended package managers first:
```yaml
# Install mise (no sudo required)
- run: curl https://mise.run | sh

# Then install tools via mise
- run: goneat doctor tools init
- run: goneat doctor tools --install --yes
```

---

## Security Considerations

### Automatic PATH Updates

**Question**: Is it safe for goneat to automatically update `$GITHUB_PATH`?

**Answer**: Yes, with caveats:

**✅ Safe**:
- Only updates PATH when `--install` flag explicitly used
- Only affects GitHub Actions environment (detected via `$GITHUB_ACTIONS`)
- Only adds shim directories for installed package managers (verified to exist)
- Changes scoped to workflow run (not permanent)

**⚠️ Considerations**:
- Tools in shim directories take precedence over system tools
- If malicious tool installed to shim, it could shadow system tool
- Mitigated by: only using trusted package managers (mise, bun, scoop)

### Package Manager Trust Model

goneat recommends **sudo-free package managers** specifically to avoid privileged operations:

| Package Manager | Trust Level | Installation Scope | Requires Privilege |
|----------------|-------------|-------------------|-------------------|
| mise | ✅ High | User (`~/.local/share/mise`) | ❌ No |
| bun | ✅ High | User (`~/.bun`) | ❌ No |
| scoop | ✅ High | User (`~/scoop`) | ❌ No |
| brew | ⚠️ Medium | System (`/opt/homebrew`) | ✅ Yes (sudo) |
| apt-get | ⚠️ Medium | System (`/usr/bin`) | ✅ Yes (sudo) |

**Recommendation**: Use `goneat doctor tools init --minimal` for maximum security (language-native tools only, no third-party package managers).

---

## Design Principles

### 1. Zero Manual Steps

**Goal**: CI runners should pass with a single command.

**Implementation**: Automatic `$GITHUB_PATH` updates when `--install` + `$GITHUB_ACTIONS` detected.

### 2. Process-Scoped PATH Extension

**Goal**: goneat can detect tools immediately after installation.

**Implementation**: `os.Setenv("PATH", ...)` extends goneat's own process PATH, allowing `exec.LookPath()` to find tools in shim directories.

**Limitation**: Does not affect parent shell or sibling processes. For persistent PATH changes, use `goneat doctor tools env`.

### 3. Explicit Opt-In

**Goal**: No surprises - goneat only updates PATH when explicitly requested.

**Implementation**:
- Automatic GitHub Actions PATH update **only** when `--install` flag used
- Local shell PATH changes **only** when user runs `goneat doctor tools env`

### 4. Platform Agnostic

**Goal**: Same workflow works on Linux, macOS, Windows.

**Implementation**:
- Config-driven shim detection (`foundation-package-managers.yaml`)
- Platform-specific package manager recommendations
- Shell-specific activation syntax (bash/zsh/fish/powershell)

---

## Future Enhancements

### Implemented in v0.3.9

1. ✅ **Package Manager Auto-Installation**
   - bun and brew auto-install when needed
   - Tries bun first (simpler, no dependencies)
   - Falls back to brew if bun fails
   - PATH updated immediately after installation

### Planned Features (v0.4.0+)

1. **Shell RC File Auto-Update**
   - `goneat doctor tools env --persist`
   - Automatically append to `~/.bashrc`, `~/.zshrc`, etc.
   - With explicit user prompt for safety

2. **CI Environment Auto-Detection**
   - Detect GitLab CI, CircleCI, Jenkins
   - Auto-update environment files (not just GitHub Actions)

3. **PATH Validation**
   - `goneat doctor tools validate-path`
   - Verify all installed tools are in PATH
   - Suggest fixes for missing shim directories

---

## FAQ

### Q: Do I need to commit `.goneat/tools.yaml` to my repository?

**A**: Yes! This is the **Single Source of Truth** for which tools your repository uses. Commit it so all developers and CI runners use the same tool configuration.

### Q: What if my CI runner doesn't support `$GITHUB_PATH`?

**A**: Use `goneat doctor tools env --activate` in your CI script:
```yaml
script:
  - goneat doctor tools --install --yes
  - eval "$(goneat doctor tools env --activate)"
  - make test
```

### Q: Can I use brew instead of mise/bun?

**A**: Yes, but brew requires sudo on CI runners, which we don't recommend. Edit `.goneat/tools.yaml` and change `installer_priority` if needed:
```yaml
tools:
  ripgrep:
    installer_priority:
      darwin:
        - brew  # Use brew instead of mise/bun
```

### Q: How do I debug PATH issues?

**A**: Use `goneat doctor tools env` to see what goneat would add to PATH:
```bash
$ goneat doctor tools env
# Installed package managers with shim directories:
#   mise (2025.9.13): /Users/user/.local/share/mise/shims

# Add these to your shell profile:
export PATH="/Users/user/.local/share/mise/shims:$PATH"
```

### Q: Does goneat work offline?

**A**: Yes! Once tools are installed, all PATH management is local. Tool installation may require network access (to download tools), but PATH configuration works offline.

---

## Related Documentation

- [Foundation Tools & Package Managers Feature Brief](.plans/active/v0.3.7/foundation-tools-package-managers.md)
- [Package Managers Bootstrap Guide](../user-guide/bootstrap/package-managers.md)
- [Tools Configuration Schema](../../schemas/tools/tools.v1.0.0.json)
- [CI/CD Best Practices](../workflows/cicd-best-practices.md)

---

**Last Updated**: 2025-12-01
**Version**: v0.3.9
**Status**: Implemented and Tested (bun/brew auto-install added)
