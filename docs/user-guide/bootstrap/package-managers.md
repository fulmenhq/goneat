---
title: Installing Package Managers on a Fresh Workstation
description: Step-by-step guidance for installing Homebrew, Scoop, Winget, and Linux package managers before running goneat doctor
author: Arch Eagle (@arch-eagle)
last_updated: 2025-12-01
version: v0.3.10
---

# Installing Package Managers on a Fresh Workstation

Goneat's tooling workflow assumes a baseline set of package managers. **Starting in v0.3.9**, goneat can automatically install brew for you, so on most systems you don't need to pre-install package managers at all.

## Package Manager Strategy (v0.3.10+)

| Package Manager | Use Case                                                    |
| --------------- | ----------------------------------------------------------- |
| `brew`          | System binaries on darwin/linux (ripgrep, jq, yq, prettier) |
| `scoop/winget`  | System binaries on Windows                                  |
| `go-install`    | Go tools (golangci-lint, gosec, yamlfmt, etc.)              |
| `bun/npm`       | Node.js packages ONLY (e.g., eslint for TypeScript repos)   |
| `uv/pip`        | Python packages ONLY                                        |

**Note**: bun is NOT used for system binaries - it can only install npm packages.

## Fully Automatic Bootstrap (v0.3.9+)

**goneat automatically installs package managers** when they're needed but not present:

```bash
# Just run bootstrap - goneat handles package manager installation automatically
make bootstrap
# or directly:
goneat doctor tools --scope foundation --install --yes --no-cooling
```

**What happens automatically**:

1. goneat checks which tools are needed (based on `.goneat/tools.yaml`)
2. For system binaries: installs brew if not present (user-local, no sudo)
3. For Go tools: uses `go install` directly
4. Adds the package manager's bin directory to PATH immediately
5. Installs the required tools

**The `--no-cooling` flag** (new in v0.3.10) skips package age verification, which is required for:

- CI environments without network access to check release dates
- Offline/air-gapped environments
- Faster bootstrap when you don't need freshness verification

**No manual package manager installation required** on:

- GitHub Actions runners (ubuntu-latest, macos-latest)
- Fresh macOS/Linux workstations
- Any system with curl and bash

## Semi-Automatic Bootstrap (v0.3.6 pattern)

For more control, you can use the `manual` installer pattern. If your tools configuration includes bootstrap entries with `installer_priority: ["manual"]`, goneat doctor will execute the official installation scripts automatically.

### Example: Bootstrap mise automatically

```yaml
# .goneat/tools.yaml
tools:
  mise:
    name: "mise"
    description: "Polyglot runtime manager"
    kind: "system"
    detect_command: "mise --version"
    platforms: ["linux", "darwin"]
    installer_priority:
      linux: ["manual"]
      darwin: ["manual"]
    install_commands:
      manual: |
        curl https://mise.jdx.dev/install.sh | sh && \
        echo 'mise installed. Add $HOME/.local/bin to PATH'
```

```bash
# Run doctor to automatically install mise if missing
goneat doctor tools --scope bootstrap --install --yes

# Verify mise was installed
mise --version
```

**PATH Verification**: After automatic bootstrap, verify the tool is in PATH:

```bash
# macOS/Linux: Add mise to PATH
export PATH="$HOME/.local/bin:$PATH"

# Verify
mise --version

# Add to shell profile for persistence
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc  # or ~/.zshrc
```

**When to use automatic bootstrap**:

- CI/CD environments (GitHub Actions, GitLab CI, etc.)
- Template repositories requiring standardized tooling
- Multi-platform projects with shared tool configs
- Fresh developer workstations (onboarding)

## Manual Installation (Alternative)

If you prefer to install package managers manually before running goneat, use the commands below.

## macOS (darwin)

### 1. Install Homebrew (brew)

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
# Temporarily update PATH for this session
PATH="/opt/homebrew/bin:/usr/local/bin:$PATH" brew --version
```

### 2. Install mise (optional - for version management)

```bash
curl https://mise.jdx.dev/install.sh | sh
# Activate immediately without restarting your shell
source "$HOME/.local/share/mise/env"
```

## Linux

### 1. Install Homebrew (brew) - user-level, no sudo

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
eval "$($HOME/.linuxbrew/bin/brew shellenv)"
```

### 2. Install mise (optional - for version management)

```bash
curl https://mise.jdx.dev/install.sh | sh
source "$HOME/.local/share/mise/env"
```

### 3. Verify distro package manager availability

- **Arch / Manjaro:** `pacman -V`
- **Debian / Ubuntu:** `apt-get --version`
- **Fedora / RHEL / CentOS:** `dnf --version` (or `yum --version` on older releases)

If a package manager is missing, install the distribution base packages or contact your ops team.

## Windows

### 1. Install Scoop (preferred)

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
irm get.scoop.sh | iex
```

### 2. Ensure Winget is available (fallback)

Winget ships with Windows 10 (22H2+) and Windows 11. If it's missing, install **App Installer** from the Microsoft Store.

### 3. Optional: Install mise

```powershell
irm https://mise.jdx.dev/install.ps1 | iex
# Activate mise for this session
$env:PATH = "$env:USERPROFILE\.local\share\mise\bin;" + $env:PATH
```

## Go Toolchain (foundation requirement)

Most goneat Go projects require a recent Go compiler.

### macOS / Linux via mise

```bash
mise use go@1.22.0
```

### Homebrew

```bash
brew install go
```

### Linux package managers

```bash
# Pacman
sudo pacman -S go
# Apt
sudo apt-get install golang
# Dnf
sudo dnf install golang
```

### Windows (Scoop)

```powershell
scoop bucket add main
scoop install go
```

## PATH Quick Fix Examples

When goneat reports "not in PATH," keep working by prepending the expected directory:

```bash
# macOS / Linux (Homebrew)
PATH="/opt/homebrew/bin:$HOME/.linuxbrew/bin:$PATH" goneat doctor tools --scope foundation

# macOS / Linux (mise shims)
PATH="$HOME/.local/share/mise/shims:$PATH" goneat doctor tools --scope foundation

# Windows PowerShell
$env:PATH = "$env:USERPROFILE\.local\share\mise\shims;" + $env:PATH
```

Add the export to your shell profile (`~/.bashrc`, `~/.zshrc`, `~/.config/fish/config.fish`, or your PowerShell profile) once you're satisfied.

## See Also

- `docs/appnotes/bootstrap-patterns.md`
- `goneat doctor tools --help`
- `goneat docs` (embedded version of this guide)
