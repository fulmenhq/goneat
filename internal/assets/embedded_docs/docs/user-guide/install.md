---
title: "Install Goneat"
description: "Installation options for macOS, Linux, and Windows"
author: "@arch-eagle"
date: "2025-09-02"
last_updated: "2025-09-02"
status: "draft"
tags: ["install", "brew", "scoop", "linux", "windows"]
---

# Install Goneat

Goneat is distributed as signed binaries via GitHub Releases and common package managers. Choose the method for your platform.

## macOS

- Homebrew (recommended once tap is published):

```bash
brew tap 3leaps/tap
brew install goneat
```

- Direct (curl):

```bash
curl -fsSL https://raw.githubusercontent.com/3leaps/goneat/main/scripts/install.sh | bash
```

## Linux

- Homebrew on Linux:

```bash
brew tap 3leaps/tap
brew install goneat
```

- Direct (curl):

```bash
curl -fsSL https://raw.githubusercontent.com/3leaps/goneat/main/scripts/install.sh | bash
```

- Arch Linux (AUR, coming soon):

```bash
yay -S goneat-bin
```

## Windows

- Scoop (coming soon):

```powershell
scoop bucket add 3leaps https://github.com/3leaps/scoop-bucket
scoop install goneat
```

- Direct (PowerShell):

```powershell
irm https://raw.githubusercontent.com/3leaps/goneat/main/scripts/install.ps1 | iex
```

## Verify installation

```bash
goneat --version
```

If your shell cannot find `goneat`, ensure its installation directory is on your PATH.

## Security & verification

- All release artifacts include SHA-256 checksums. The install scripts verify checksums and will abort on mismatch.
- When GPG is available, signatures on `SHA256SUMS` will be verified against the Fulmen release key.
