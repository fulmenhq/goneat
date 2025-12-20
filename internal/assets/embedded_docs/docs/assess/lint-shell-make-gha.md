# Lint: Shell, Make, GitHub Actions

## Overview

- Adds lint coverage for shell (shfmt + optional shellcheck), Makefiles (checkmake), and GitHub Actions workflows (actionlint).
- Defaults: shfmt/actionlint/checkmake enabled; shellcheck disabled (GPL verify-only).
- Configure via `.goneat/assess.yaml` or CLI flags.
- Scaffold a starter file with `goneat doctor assess init`.

## Configuration (`.goneat/assess.yaml`)

```yaml
version: 1
lint:
  shell:
    paths: ["**/*.sh", "scripts/**/*"]
    # Note: .gitignore/.goneatignore define default scope (v0.3.17+)
    ignore: []
    shfmt:
      enabled: true
      fix: false # set true to allow write
      # Optional: ensure lint agrees with your repo's shfmt style
      # (goneat controls -d/-w; only put style flags here)
      args: ["-i", "4", "-ci"]
    shellcheck:
      enabled: false # opt-in
      path: "" # optional explicit path
  github_actions:
    actionlint:
      enabled: true
      paths: [".github/workflows/**/*.yml", ".github/workflows/**/*.yaml"]
      ignore: []
  make:
    checkmake:
      enabled: true
      config:
        max_body_length: 15
        min_phony_targets: ["all", "clean", "test"]
    paths: ["**/Makefile"]
    ignore: []
```

## CLI overrides

- `--lint-shell` / `--lint-shell-fix`
- `--lint-shellcheck` (GPL, verify-only) and `--shellcheck-path`
- `--lint-gha`
- `--lint-make`
- Path/exclude overrides: `--lint-shell-paths`, `--lint-shell-exclude`, `--lint-gha-paths`, `--lint-gha-exclude`, `--lint-make-paths`, `--lint-make-exclude`

## Behavior

- shfmt: check-only by default; enables package-mode-friendly behavior with exclusions; fix when `--lint-shell-fix` or config `fix: true`.
- shellcheck: verify-only; skipped if not enabled or binary missing; uses provided path when set.
- actionlint: runs on workflow files; honors include/exclude.
- checkmake: runs on Makefiles; honors include/exclude.

## CI / container

- goneat-tools container bundles shfmt, actionlint, checkmake.
- GitHub Actions container jobs must ensure the container user can write to runner mounts under `/__w`.
  - Recommended: `container.options: --user 1001` (matches GitHub-hosted runner workspace ownership).
- shellcheck is not bundled (GPL); install in CI job or provide sidecar path.

## Troubleshooting

- Missing tool: ensure binary on PATH or configured path; disabled tools are skipped.
- Too many matches: narrow `paths` or add `ignore` globs.
- Want read-only: keep `shfmt fix: false` and do not enable shellcheck fixes (verify-only).
