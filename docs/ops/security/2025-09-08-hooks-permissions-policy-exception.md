# Security Policy Exception: Git Hooks Executable Permissions

Summary: This memo documents a repository‑level security policy to suppress gosec G302/G306 findings for `cmd/hooks.go` where executable permissions are required for Git hooks to function.

Context

- Tools: gosec (rules G302/G306) flag file/directory permissions greater than 0600 as potential risks.
- Our `hooks` command generates scripts into `.goneat/hooks/` and installs them to `.git/hooks/`.
- Git hooks must be executable to run (typically 0700 for user‑only execute).

Decision

- We suppress gosec findings for G302/G306 when and only when:
  - File path matches `cmd/hooks.go` generated paths for `.goneat/hooks/*` and `.git/hooks/*`, and
  - The rule is G302 (Chmod) or G306 (WriteFile permissions), and
  - The intended mode is 0700 for git hooks.

Rationale (Why this is acceptable)

- Functional necessity: Git refuses to execute hooks without execute permission; 0700 is the minimal, user‑only mode.
- Narrow scope: Files are created/installed strictly under the current repository’s `.git/hooks/` directory.
- Controlled content: Hook scripts are generated from trusted repository templates, not arbitrary external input.

Compensating Controls

- Path hardening:
  - All hook file paths are sanitized via `filepath.Clean` and explicitly checked to prevent `..` traversal.
  - Copy and write operations are restricted to validated, repository‑local paths.
- Minimal permissions:
  - Hook files are written with 0700 (user‑only exec). Configuration files use 0600.
  - Documentation/content mirrors are created with ≤0750 directories and ≤0640 files.
- Visibility and tracking:
  - Suppressions are tracked in assessment metrics and summaries when `--track-suppressions` is enabled.
  - A dedicated suppression policy is implemented in the security runner to ensure predictable, code‑reviewed behavior.

Implementation Notes

- The security runner (internal/assess) applies a repository policy that filters gosec issues matching the above criteria and records synthetic suppressions with the reason: “Git hooks require exec permissions (0700)”.
- Inline `#nosec` comments are not required for these lines; policy suppression provides a single, auditable control point.

References

- Existing memo: 2025‑09‑02 Security Exceptions — Git Hooks Permissions
- Git hooks documentation: https://git-scm.com/docs/githooks

Status: Accepted (documented policy exception)
Reviewed by: Forge Neat, Code Scout
Last Updated: 2025‑09‑08
