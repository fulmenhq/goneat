---
title: "Selected module window for golang.org/x/net"
date: "2026-10-09"
expires_at: "2026-10-16T00:00:00Z"
cleanup_status: pending
closed_at: null
---

# Selected module window for golang.org/x/net

On 2026-10-09 the selected module `golang.org/x/net` is v0.60.0 and indirect.

## Policy selector

`.goneat/dependencies.yaml` matches the module name `golang.org/x/net` until 2026-10-16. The match is the module name. It does not enforce v0.60.0.

The window is active while the current time is less than or equal to 2026-10-16T00:00:00Z. It is elapsed only when the current time is after that instant.

## Selected-version control

`scripts/check-cooling-selection.py` is a separate control. It requires the selected module `golang.org/x/net` to be v0.60.0 and indirect while the window is active. After 2026-10-16T00:00:00Z the check exits 0 and does not read the version.

## Removal

A later change removes the policy entry and the selected-version check, checks that ordinary policy behavior applies, and records the closure time here. Reaching 2026-10-16T00:00:00Z does not remove the entry, does not close this memo, and does not mark it superseded.

`cleanup_status` stays `pending` and `closed_at` stays empty until that change.
