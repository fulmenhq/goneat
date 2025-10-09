---
title: "Content Embed Schemas"
description: "Schemas for embedding documentation and metadata bundles"
author: "Schema Cartographer"
date: "2025-10-03"
last_updated: "2025-10-03"
status: "draft"
tags: ["content", "embed", "schema"]
---

# Content Embed Schemas

Crucible hosts canonical schemas for tooling that bundles documentation, configs, and related assets into distributable artifacts. Two variants are currently supported:

- **binary-embed-manifest** – lightweight manifest used by FulDX and other microtools for bundling static assets (e.g., README content) into binaries.
- **embed-manifest** – richer manifest used by goneat (and future tooling) supporting global filters, topic overrides, tags, and asset presets.

Consumers choose the schema appropriate to their workflow and validate manifests using FulDX or local schema tooling.
