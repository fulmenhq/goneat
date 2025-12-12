---
title: Installing Goneat with sfetch
description: Secure, verifiable bootstrap for goneat using sfetch (recommended for direct downloads)
author: Forge Neat (@forge-neat)
last_updated: 2025-12-12
version: v0.3.16
---

# Installing Goneat with sfetch

`sfetch` is a security-conscious GitHub release downloader. It selects the right asset for your platform and verifies signatures + checksums before installing.

Use this when you:

- Want a **direct download** install (no package manager)
- Need a **pinned version** in CI or per-repo tooling
- Want to avoid custom per-repo `curl` + checksum scripts

## Trust Model (why this exists)

The recommended trust chain is:

1. Install `sfetch` using its pinned-trust-anchor installer (`install-sfetch.sh`)
2. Use `sfetch` to install `goneat` with automatic verification

Once `sfetch` is present, `goneat` installs become high-confidence and low-maintenance.

## Step 1: Install sfetch

```bash
curl -sSfL https://github.com/3leaps/sfetch/releases/latest/download/install-sfetch.sh | bash

# Ensure it is available
sfetch --self-verify
```

## Step 2: Install goneat globally (recommended)

Installs to a user-local directory (`~/.local/bin`) without sudo:

```bash
sfetch --repo fulmenhq/goneat --latest --dest-dir ~/.local/bin

goneat version
```

## Step 3: Install goneat repo-locally (pinned tooling)

Installs to `./bin` so a repo can pin a specific version:

```bash
mkdir -p ./bin
sfetch --repo fulmenhq/goneat --tag v0.3.16 --dest-dir ./bin

./bin/goneat version
```

## CI guidance

- Prefer installing `sfetch` + `goneat` inside a tools container when available.
- Otherwise, use the same two-step flow: install `sfetch`, then `sfetch` installs `goneat`.

## See also

- `docs/user-guide/install.md`
- `docs/appnotes/bootstrap-patterns.md`
