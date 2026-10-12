---
title: "Interim gosec pin for Go 1.27"
date: "2026-10-12"
expires_at: null
cleanup_status: pending
closed_at: null
---

# Interim gosec pin for Go 1.27

On 2026-10-12 the latest gosec release is v2.29.0. It is built with `golang.org/x/tools` v0.49.0, which cannot read Go 1.27 export data. Under Go 1.27 every package fails to process.

## Documented pin

`docs/user-guide/toolchains/go.md` documents `github.com/securego/gosec/v2@v2.29.1-0.20260930122349-9b318063d35c`, which is built with `golang.org/x/tools` v0.50.0. It was published 2026-09-30, which is past the cooling window. Under Go 1.27.2 it reports the same findings on this repository as v2.29.0 under Go 1.26.9.

`.goneat/tools.yaml` is unchanged. It still installs `@latest` and recommends a release version. The pin is documentation only.

## Detection

The security category reads the `golang.org/x/tools` version from the gosec binary's build information when gosec reports an export-data version error. The message names that version and both remedies. This behavior does not depend on the pin and stays after cleanup.

## Removal

When a gosec release supports Go 1.27 and passes the cooling policy:

1. Remove the "Go 1.27" paragraph and the pinned `go install` command from `docs/user-guide/toolchains/go.md`. Keep the general "Newer Go toolchains" text.
2. Raise gosec `recommended_version` in `.goneat/tools.yaml` and `config/tools/foundation-tools-defaults.yaml`.
3. Set `cleanup_status: closed` and `closed_at` here.
