---
title: Installing Package Managers on a Fresh Workstation
description: Step-by-step guidance for installing mise, Homebrew, Scoop, Winget, and Linux package managers before running goneat doctor
author: Arch Eagle (@arch-eagle)
last_updated: 2025-09-19
version: v0.2.7
---

# Installing Package Managers on a Fresh Workstation

Goneat's tooling workflow assumes a baseline set of package managers. On a brand-new machine you may need to install these helpers before `goneat doctor tools --install` can succeed. Use the sections below to bootstrap your environment quickly.

## macOS (darwin)

### 1. Install Homebrew (brew)

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
# Temporarily update PATH for this session
PATH="/opt/homebrew/bin:/usr/local/bin:$PATH" brew --version
```

### 2. Install mise (recommended)

```bash
curl https://mise.jdx.dev/install.sh | sh
# Activate immediately without restarting your shell
source "$HOME/.local/share/mise/env"
```

## Linux

### 1. Install mise (user-level, no sudo)

```bash
curl https://mise.jdx.dev/install.sh | sh
source "$HOME/.local/share/mise/env"
```

### 2. Verify distro package manager availability

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
# macOS / Linux
PATH="$HOME/.local/share/mise/shims:$PATH" goneat doctor tools --scope foundation

# Windows PowerShell
$env:PATH = "$env:USERPROFILE\.local\share\mise\shims;" + $env:PATH
```

Add the export to your shell profile (`~/.bashrc`, `~/.zshrc`, `~/.config/fish/config.fish`, or your PowerShell profile) once you're satisfied.

## See Also

- `docs/appnotes/intelligent-tool-installation.md`
- `goneat doctor tools --help`
- `goneat docs` (embedded version of this guide)
