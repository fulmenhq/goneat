This directory contains a tracked mirror of the top-level `schemas/` folder.

Source of truth (SSOT): `schemas/`

Do not edit files here directly. To update embedded assets:

1. Edit files under `schemas/`
2. Run `make embed-assets` (or `scripts/embed-assets.sh`) to sync mirrors
3. Commit both the SSOT and mirrored changes

CI will verify that the mirror is in sync with the SSOT. The mirror is committed to
enable `go install` builds to embed assets without invoking Makefile steps.
