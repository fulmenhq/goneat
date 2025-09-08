# SOP: Embedding Curated Assets (templates/ and schemas/) in Go Binaries

## Scope
This SOP defines the organization‑wide standard for embedding curated runtime assets (e.g., hook templates, schemas) into Go binaries while maintaining a single source of truth (SSOT) and preventing drift. It applies to all Fulmen projects adopting `//go:embed` for distribution‑grade assets.

## Objectives
- Preserve a single source of truth for authored assets
- Ensure `go install` builds include required assets (no Makefile dependency)
- Eliminate "works locally, fails on go install" issues
- Detect and prevent drift between SSOT and embedded mirrors in CI

## Definitions
- SSOT: The authoritative, human‑edited asset trees.
  - `templates/`
  - `schemas/`
- Embedded mirrors: Tracked copies placed under the embedding package so `//go:embed` can include them at build time (even during `go install`).
  - `internal/assets/embedded_templates/templates/…`
  - `internal/assets/embedded_schemas/schemas/…`

## Rationale
- `//go:embed` can only embed files within the same module subtree as the package. Our embedding code lives in `internal/assets`, so it cannot embed `../templates` or `../schemas` directly.
- `go install` does not run Makefiles. Relying on a pre‑build step to prepare assets will fail for users installing via `go install`.
- Therefore, we commit embedded mirrors to source control to guarantee assets are always present for the toolchain to embed, while enforcing SSOT discipline via automation.

## Do & Don’t
- DO edit only SSOT: `templates/` and `schemas/`.
- DO run `make embed-assets` after editing SSOT to refresh mirrors.
- DO commit both SSOT changes and refreshed embedded mirrors in the same PR.
- DO rely on CI to verify mirrors are in sync (`make verify-embeds`).
- DON’T hand‑edit files under `internal/assets/embedded_*`.
- DON’T assume Makefile runs on `go install` — this is why mirrors are tracked.

## Standard Layout
- SSOT (author‑edited):
  - `templates/…`
  - `schemas/…`
- Embedded mirrors (tool‑consumed):
  - `internal/assets/embedded_templates/templates/…`
  - `internal/assets/embedded_schemas/schemas/…`
- Embed code:
  - `internal/assets/assets.go` (uses `//go:embed` + `fs.Sub`)

## Update Workflow
1) Edit assets under SSOT (`templates/`, `schemas/`).
2) Refresh mirrors:
   - `make embed-assets` (runs `scripts/embed-assets.sh`)
3) Verify locally (optional):
   - `make verify-embeds` (runs `scripts/verify-embeds.sh`)
4) Commit both SSOT and embedded mirrors.
5) Push PR; CI will run `verify-embeds` to catch any drift.

## Runtime Resolution Pattern
- Embedded‑first: Consumers read from the embedded FS (e.g., `GetTemplatesFS()` returning `fs.Sub(Templates, "embedded_templates")`).
- Dev fallback (optional): Consumers may fall back to SSOT on disk for local development (e.g., `cmd/hooks.go` falls back to `templates/` if embedded read fails).

## CI Enforcement
- GitHub Actions executes `make verify-embeds` after `make build`.
- Any drift (added/updated/deleted files) between SSOT and mirrors fails CI with a diff hint and remediation (`make embed-assets`).

## Commands & Scripts
- `make embed-assets` → sync SSOT → mirrors (one‑way, destructive for removed files)
- `make verify-embeds` → fail if mirrors deviate from SSOT
- `scripts/embed-assets.sh` → rsync logic with `--delete`
- `scripts/verify-embeds.sh` → rsync `--dry-run` check

## Code Pointers
- Embedding: `internal/assets/assets.go`
- Hook templates usage: `cmd/hooks.go` (embedded‑first, filesystem fallback)
- Report template usage: `internal/assess/formatter.go` (example of filesystem probes; can be extended to embedded‑first)

## Reviewer Checklist (PR)
- [ ] SSOT only edited (no manual edits under `internal/assets/embedded_*`)
- [ ] `make embed-assets` run and mirrors updated in same PR
- [ ] `make verify-embeds` passes locally if run
- [ ] CI verify‑embeds step green
- [ ] If consumers added/renamed paths, validate they resolve against embedded FS

## Migration Guide (Adopting in a new repo)
1) Create SSOT directories: `templates/`, `schemas/`.
2) Add embedding package (`internal/assets/assets.go`) with `//go:embed` and `fs.Sub` rooted at `embedded_*` dirs.
3) Add mirrors: `internal/assets/embedded_templates/templates/` and `internal/assets/embedded_schemas/schemas/` (initially empty).
4) Add scripts: `scripts/embed-assets.sh`, `scripts/verify-embeds.sh`.
5) Add Make targets: `embed-assets`, `verify-embeds`; make `build` depend on `embed-assets`.
6) Update CI to run `verify-embeds` after build.
7) Update consumers to read embedded FS (with optional filesystem fallback for dev).
8) Document the SOP for contributors (link this file).

## FAQ
- Why not symlink mirrors to SSOT? `//go:embed` requires files within the package; symlinks are unreliable across platforms and tooling.
- Why commit mirrors instead of generating at CI? Because `go install` builds from the module cache and cannot run our Makefiles.
- What if someone edits embedded mirrors? CI will fail `verify-embeds` unless SSOT is updated and synced. Reviewers should block merges that modify embedded mirrors without corresponding SSOT edits.

## Security & Compliance Notes
- Keep embedded file reads/writes safe: clean paths, reject traversal, write with restrictive permissions (0600), and justify exceptions (e.g., git hooks need 0700 executable bit).
- Prefer embedded assets to avoid network dependencies and ensure reproducibility.

## Troubleshooting
- Error: "failed to read embedded template … does not exist"
  - Likely mirrors are stale. Run `make embed-assets`, commit, and rebuild.
- CI failed `verify-embeds`: Run `make embed-assets`, re‑commit; confirm no manual edits under mirrors.

---

Generated by Code Scout under supervision of @3leapsdave

