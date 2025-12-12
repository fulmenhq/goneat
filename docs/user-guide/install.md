---
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

Goneat is distributed as signed binaries via GitHub Releases and common package managers.

## Recommended installs

### Homebrew (macOS/Linux)

```bash
brew install fulmenhq/tap/goneat
```

### Scoop (Windows)

```powershell
scoop bucket add fulmenhq https://github.com/fulmenhq/scoop-bucket
scoop install goneat
```

### Secure direct download (recommended when not using a package manager)

Use `sfetch` to download the correct release asset and verify signatures + checksums automatically:

```bash
# Install sfetch first
curl -sSfL https://github.com/3leaps/sfetch/releases/latest/download/install-sfetch.sh | bash

# Install goneat (chooses correct asset for your platform)
sfetch --repo fulmenhq/goneat --latest --dest-dir ~/.local/bin

goneat version
```

For repo-local, pinned installs:

```bash
mkdir -p ./bin
sfetch --repo fulmenhq/goneat --tag v0.3.16 --dest-dir ./bin
./bin/goneat version
```

See `docs/user-guide/bootstrap/sfetch.md` for the full bootstrap pattern.

## Verify installation

```bash
goneat version
```

If your shell cannot find `goneat`, ensure its installation directory is on your `PATH`.

## Security & verification

- Prefer `sfetch` for direct downloads: it verifies signed checksum manifests when available.
- Homebrew/Scoop installs rely on their own distribution and integrity mechanisms.
