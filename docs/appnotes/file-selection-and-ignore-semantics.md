# File Selection and Ignore Semantics

Goneat owns file selection before handing work to external tools whenever it can. That keeps generated directories, dependency caches, release artifacts, and prior scan outputs from becoming first-class findings.

## Ignore Sources

Goneat's unified matcher reads these sources, in order:

Source-tree SBOM collection uses the narrower root-only contract described below.

1. Built-in generated/tooling defaults: `.git/`, `node_modules/`, `.scratchpad/`, `.cache/`, `bin/`, `dist/`, `sbom/`, `vendor/`
2. Repository git ignore configuration, including `.gitignore` and standard git exclude sources
3. Repository `.goneatignore`
4. User ignore files at `~/.goneatignore` and `~/.goneat/.goneatignore`

Use `.gitignore` for normal VCS-generated files. Use `.goneatignore` for committed goneat scan policy: non-git archives, tool-specific scan exclusions, and explicit scope rules that should not depend on a developer's local git setup.

`--no-ignore` disables goneat ignore matching for discovery and fallback file-walk scans. `--force-include` can re-include specific paths or descendants that would otherwise be ignored.

## Tool Behavior Matrix

| Area         | Tool or Input                                      | Goneat Pre-Filters                            | Tool Native Ignore       | Notes                                                                                                                                                 |
| ------------ | -------------------------------------------------- | --------------------------------------------- | ------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| Format       | gofmt/goimports, yamlfmt, JSON/Markdown finalizers | Yes, file list                                | No                       | `--no-ignore` and `--force-include` affect file discovery.                                                                                            |
| Lint         | golangci-lint                                      | Yes, package/file scope                       | Partial                  | Goneat filters discovered files/packages before invocation.                                                                                           |
| Lint         | Biome                                              | Yes, file list/config roots                   | Yes, for its own config  | Goneat's scope still decides which candidates are handed off.                                                                                         |
| Lint         | Ruff                                               | Yes, file list                                | Yes, for its own config  | Goneat ignore matching applies first.                                                                                                                 |
| Lint         | yamllint                                           | Yes, file list                                | Config-driven            | Goneat filters YAML candidates before running yamllint.                                                                                               |
| Lint         | shellcheck/shfmt                                   | Yes, file list                                | No                       | Goneat file discovery is the primary scope boundary.                                                                                                  |
| Lint         | actionlint/checkmake                               | Yes, file list                                | No                       | Goneat selects workflow and Makefile candidates.                                                                                                      |
| Security     | gosec                                              | Yes, Go modules/packages                      | No                       | Goneat prunes ignored nested modules and filters package dirs before running gosec.                                                                   |
| Security     | govulncheck                                        | Go package/module scope                       | Go package rules         | Go package tooling does not use `.gitignore`; goneat controls the package roots it invokes.                                                           |
| Security     | gitleaks                                           | Configured scan target                        | Yes, via gitleaks config | Treat gitleaks config as defense-in-depth; goneat still owns command scope.                                                                           |
| Dependencies | Go module graph                                    | Yes, graph input                              | Go module rules          | `dependencies --vuln` uses `go list -m -json all` for Go roots by design. `--no-ignore` and `--force-include` do not turn this into a full-tree scan. |
| Dependencies | explicit directory SBOM and syft fallback SBOM     | Yes, captured subject and exact file excludes | Root-only contract       | Both routes share named-only overrides and capture/validation safeguards.                                                                             |
| Dependencies | grype                                              | SBOM input                                    | No                       | Grype scans the SBOM it receives. Source provenance reports `go-module-graph`, `sbom-file`, or `file-walk`.                                           |

## Vulnerability Scope Hints

When vulnerability enforcement fails and the enforced high or critical findings are mostly sourced from generated or ignored-looking paths such as `.cache/`, `dist/`, `bin/`, `sbom/`, `vendor/`, or `node_modules/`, goneat adds a scope hint to the policy failure message. The hint points users back to file selection, `.goneatignore`/`.gitignore`, or graph-scoped or explicit SBOM input.

For Go repositories, the preferred vulnerability input is the module graph. For non-Go repositories or explicit SBOM workflows, keep generated outputs and dependency caches excluded from the SBOM input unless the scan is intentionally auditing those artifacts.

## Source-Tree SBOM Selection

`dependencies --sbom <directory>` and the non-Go Syft vulnerability fallback
share source selection. Root-relative defaults exclude `.git/**`,
`node_modules/**`, `.scratchpad/**`, `.cache/**`, `bin/**`, `dist/**`, `sbom/**`,
and `vendor/**`. Only the target root's `.gitignore` and `.goneatignore` are
read. Parent, nested, global, and user ignore files are not imported.

The supported ignore subset is:

- A leading `/` or `./` anchors to the scan root. An interior slash also makes
  a pattern root-relative; a slashless name matches at any depth.
- A matched directory excludes its descendants. A trailing slash matches
  directories only.
- `*`, `?`, valid character classes, and whole-segment `**` are supported.
- Blank lines and comments are ignored. Escaped leading `\#` and `\!` match
  literal names. Unsupported escapes, malformed patterns, brace expansion,
  and traversal are errors with file/line context.
- An unescaped leading `!` re-includes matching paths within root ignore policy.
  Rules run in order: `.gitignore`, then `.goneatignore`; the last direct match
  wins. A child cannot reopen an excluded parent. Reopening a directory allows
  descent, but does not restore independently excluded children. For example,
  `**/sumpter` then `!cmd/sumpter/` selects `cmd/sumpter/main.go` but still excludes
  a file named `cmd/sumpter/sumpter`. Nested ignore files are not read; this is not
  full Git-ignore compatibility.
- Defaults and configured exclusions are a separate hard layer. Ignore negations
  cannot override them; configured exclusion patterns cannot themselves be
  negated. Literal force paths remain the explicit override.

`--force-include bin/current` restores only that existing file.
`--force-include bin/release` restores only that existing directory subtree.
Ignored siblings and unrelated exclusions remain excluded. These arguments
must be literal root-relative paths: globs, absolute/drive/UNC paths,
traversal, and nonexistent paths are rejected. `--no-ignore` clears defaults,
configured exclusions and both root ignore files, but retains capture, validation,
Go evidence scope checks and cleanup requirements.

When root `go.mod` or `go.work` declares a Go subject, the four exact existing
regular root names `go.mod`, `go.sum`, `go.work`, and `go.work.sum` are protected
from root-ignore suppression. Actual exceptions appear in standalone source
provenance. Optional absence does not synthesize files or fail the scan. A
configured pattern still excluding any present protected file causes an
actionable error, unless literal force or no-ignore already restores it.

Root retention is not workspace completeness. Required workspace member or local
replacement evidence must be present and selected inside the captured subject.
Missing, excluded or outside-subject evidence causes an incomplete-scope error;
goneat neither fetches external directories nor implicitly restores member trees.
Unrelated nested projects are not covered by this root graph scope check.

### Captured Subject and Compatibility

Directory scans copy the complete tree, including excluded regular files, into
an owner-private directory outside the target. Originals are never removed.
The copy is hashed and reconciled with the source, then verified before and
after Syft collection. Syft receives the canonical capture path and exact
literal file exclusions.

All symlinks and special files are rejected, including in excluded trees.
Unreadable/incomplete traversal, detected mutations, unsupported patterns,
inexact exclusions, and exhausted limits fail the scan. The limits are
100,000 filesystem entries and 2 GiB of regular-file content, including
excluded content. Conservative command/environment limits are 128 KiB on
Unix and 30,000 UTF-16 units each for quoted arguments and environment on
Windows. Narrow the target when a limit is exceeded. Temporary storage must
be outside the selected tree and large enough for its complete contents.

This is a reproducible captured-byte subject under a trusted-local-writer
boundary, not an atomic filesystem snapshot or protection from a hostile
concurrent writer. Later changes to the live source do not change the captured
subject.

### Collector Policy Isolation

Both directory source and explicit regular-file artifact collection use an
invocation-owned explicit Syft config and a neutral private working directory
outside the input. They retain the resolved Syft version's full default
catalogers and only Goneat's generated literal exclusions. Caller `.syft.yaml`,
profiles and other discovered Syft settings do not select inventory. All
`SYFT_*` environment variables are removed, with no allowed exceptions currently;
unrelated required OS/auth settings are preserved without being logged. This
applies even when no generated exclusion arguments exist. It does not change
the Go module-graph or supplied-SBOM vulnerability routes.

Collector config and working-directory privacy, identity and contents are checked
around collection. Their cleanup must also succeed before publication. Temporary
collector paths must not appear in the emitted inventory.

### Provenance and Publication

Supported outputs are CycloneDX JSON (1.6/1.7) and SPDX JSON (2.3). Other
formats are rejected before collection. Offline schema and reference checks
validate the result. Schema-qualified path mapping restores original source
provenance without removing inventory, changing opaque IDs, or merging
packages. Unmapped private staging references are errors.

Standalone output includes `goneat:source:provenance`: a CycloneDX metadata
property or SPDX annotation carrying the original subject, captured-manifest
SHA-256, and selection scope. Named artifact overrides still produce a
**scoped source inventory**. To inventory a shipped artifact, target its actual
regular file explicitly; source ignores do not apply, and file content plus
pathname identity are verified around collection.

Private capture cleanup must succeed before stdout is returned or a file is
published. Output files are staged beside their destination and replaced
without a cross-volume copy fallback. Failure preserves an existing output;
cleanup errors identify potentially retained private data.
