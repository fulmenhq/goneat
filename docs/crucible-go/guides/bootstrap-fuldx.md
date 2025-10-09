---
title: "FulDX Bootstrap Guide"
description: "Installing FulDX via .goneat/tools.yaml with local override support"
author: "Schema Cartographer"
date: "2025-10-07"
last_updated: "2025-10-07"
status: "draft"
tags: ["bootstrap", "fuldx", "tools"]
---

# FulDX Bootstrap Guide

Foundation repositories use FulDX for DX tooling (version bumping, asset sync, etc.). This guide explains how to install FulDX via `.goneat/tools.yaml` and how to override it locally during development.

## Production / CI Workflow

1. Commit `.goneat/tools.yaml` with the published FulDX binary URLs + checksums.
2. Run `make bootstrap` (or `bun run scripts/bootstrap-tools.ts --install`) – FulDX installs into `./bin/fuldx`.
3. Run `fuldx tools install` or `make tools` to install other declared binaries.

## Local Development Override

When iterating on FulDX itself (or testing a local build):

```bash
cp .goneat/tools.local.yaml.example .goneat/tools.local.yaml
# edit source path to point to your local fuldx build
make bootstrap
```

`tools.local.yaml` is **gitignored** and takes precedence over `tools.yaml`.

Example entry:

```yaml
version: v1.0.0
binDir: ./bin
tools:
  - id: fuldx
    install:
      type: link
      source: /Users/you/dev/fulmenhq/fuldx/dist/fuldx
      binName: fuldx
      destination: ./bin
```

**Tips:**

- Use `type: link` for local binaries (copies from `source` to `destination`).
- Keep `tools.local.yaml` uncommitted; CI will ignore it.
- Add unit tests / pre-commit checks to ensure no `.local` manifest is accidentally committed.

See `schemas/tooling/external-tools/v1.0.0/external-tools-manifest.schema.yaml` for full manifest documentation.
