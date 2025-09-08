# Security Refinements — v0.2.1 Planning

## Goals
- Reduce false positives and improve clarity of security results
- Standardize file IO safety (paths + permissions)
- Improve usability (timeouts, profiles, summaries)

## Proposed Tasks

- Path safety (G304):
  - [ ] Audit remaining `os.ReadFile`/`os.Open` calls; adopt sanitized helpers (Clean+Abs+WD-bound)
  - [ ] Add safe write helpers where appropriate (validate output locations)
- Permissions (G302/G306):
  - [ ] Standardize writes to `0600` by default; document exceptions (git hooks `0700`)
  - [ ] Optional: write `0600` then `chmod 0700` for hooks; keep justification comments
- Fixtures noise reduction:
  - [ ] Centralize ignore of `tests/fixtures/**` and `test-fixtures/**` for security category
  - [ ] Expose config knob to override excludes per-repo
- Suppressions tracking:
  - [ ] Ensure gosec suppressions summarized under `summary.suppressions`
  - [ ] Consider Git metadata enrichment (age, author) when inexpensive
  - [ ] Add policy hooks (max allowed suppressions) as informational warnings
- Tooling UX:
  - [ ] Make per-tool timeouts configurable via config (retain flags)
  - [ ] Add `--profile=ci` mapping to security command as well (match assess)
  - [ ] Optional: secrets scanning adapter (gitleaks) behind opt-in flag
- Output/Docs:
  - [ ] Improve error messaging for security tools (actionable guidance)
  - [ ] Document suppression etiquette and policy guardrails

## Acceptance Signals
- Fewer false positives on fixture-heavy repos
- No new G304 findings in core IO paths
- Clear, concise security section in `--ci-summary` runs
- Hooks permission exceptions remain justified and isolated

