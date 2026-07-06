# File Selection and Ignore Semantics

Goneat owns file selection before handing work to external tools whenever it can. That keeps generated directories, dependency caches, release artifacts, and prior scan outputs from becoming first-class findings.

## Ignore Sources

Goneat's unified matcher reads these sources, in order:

1. Built-in generated/tooling defaults: `.git/`, `node_modules/`, `.scratchpad/`, `.cache/`, `bin/`, `dist/`, `sbom/`, `vendor/`
2. Repository git ignore configuration, including `.gitignore` and standard git exclude sources
3. Repository `.goneatignore`
4. User ignore files at `~/.goneatignore` and `~/.goneat/.goneatignore`

Use `.gitignore` for normal VCS-generated files. Use `.goneatignore` for committed goneat scan policy: non-git archives, tool-specific scan exclusions, and explicit scope rules that should not depend on a developer's local git setup.

`--no-ignore` disables goneat ignore matching for discovery and fallback file-walk scans. `--force-include` can re-include specific paths or descendants that would otherwise be ignored.

## Tool Behavior Matrix

| Area | Tool or Input | Goneat Pre-Filters | Tool Native Ignore | Notes |
| ---- | ------------- | ------------------ | ------------------ | ----- |
| Format | gofmt/goimports, yamlfmt, JSON/Markdown finalizers | Yes, file list | No | `--no-ignore` and `--force-include` affect file discovery. |
| Lint | golangci-lint | Yes, package/file scope | Partial | Goneat filters discovered files/packages before invocation. |
| Lint | Biome | Yes, file list/config roots | Yes, for its own config | Goneat's scope still decides which candidates are handed off. |
| Lint | Ruff | Yes, file list | Yes, for its own config | Goneat ignore matching applies first. |
| Lint | yamllint | Yes, file list | Config-driven | Goneat filters YAML candidates before running yamllint. |
| Lint | shellcheck/shfmt | Yes, file list | No | Goneat file discovery is the primary scope boundary. |
| Lint | actionlint/checkmake | Yes, file list | No | Goneat selects workflow and Makefile candidates. |
| Security | gosec | Yes, Go modules/packages | No | Goneat prunes ignored nested modules and filters package dirs before running gosec. |
| Security | govulncheck | Go package/module scope | Go package rules | Go package tooling does not use `.gitignore`; goneat controls the package roots it invokes. |
| Security | gitleaks | Configured scan target | Yes, via gitleaks config | Treat gitleaks config as defense-in-depth; goneat still owns command scope. |
| Dependencies | Go module graph | Yes, graph input | Go module rules | `dependencies --vuln` uses `go list -m -json all` for Go roots by design. `--no-ignore` and `--force-include` do not turn this into a full-tree scan. |
| Dependencies | syft fallback SBOM | Yes, exclude args | Partial | Non-Go/fallback scans pass generated-dir and ignore-derived excludes unless `--no-ignore` is set. |
| Dependencies | grype | SBOM input | No | Grype scans the SBOM it receives. Source provenance reports `go-module-graph`, `sbom-file`, or `file-walk`. |

## Vulnerability Scope Hints

When vulnerability enforcement fails and the enforced high or critical findings are mostly sourced from generated or ignored-looking paths such as `.cache/`, `dist/`, `bin/`, `sbom/`, `vendor/`, or `node_modules/`, goneat adds a scope hint to the policy failure message. The hint points users back to file selection, `.goneatignore`/`.gitignore`, or graph-scoped or explicit SBOM input.

For Go repositories, the preferred vulnerability input is the module graph. For non-Go repositories or explicit SBOM workflows, keep generated outputs and dependency caches excluded from the SBOM input unless the scan is intentionally auditing those artifacts.
