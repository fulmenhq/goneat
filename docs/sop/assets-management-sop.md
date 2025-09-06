---
title: "Assets Management SOP"
description: "Operational procedures for curated/cached validation assets"
author: "@arch-eagle"
date: "2025-09-05"
last_updated: "2025-09-05"
status: "approved"
---

# Assets Management SOP

## Update Curated Assets

1. Run sync script (requires network):
   ```bash
   make sync-schemas
   ```
2. Review diffs under `internal/assets/` (ensuring expected changes)
3. Update `docs/licenses/inventory.md` and registry entries if versions changed
4. Build and run schema validation: `goneat validate --format json` on repo
5. Commit with attribution and a short rationale

## Cache Management (optional)

- Cache path: `~/.goneat/cache/schemas`
- Clear cache when debugging: `rm -rf ~/.goneat/cache/schemas`
- Enable cache usage only when flags/config permit remote refs

## Incident Handling

- If a curated asset triggers widespread false positives:
  - Pin to previous version or apply a hotfix to the asset file
  - Document in CHANGELOG and release notes

