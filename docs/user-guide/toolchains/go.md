---
title: "Go Toolchain"
description: "How goneat handles Go codebases: golangci-lint, gosec, gofmt, goimports, govulncheck — common findings, configuration, and version-sensitive behaviors"
author: "goneat contributors"
date: "2026-02-26"
last_updated: "2026-02-26"
status: "published"
tags: ["go", "golangci-lint", "gosec", "gofmt", "govulncheck", "toolchain"]
category: "user-guide"
---

# Go Toolchain

goneat orchestrates the standard Go quality toolchain and surfaces findings
through a unified interface. This page covers what runs, what the findings mean,
and how to configure or suppress them.

## Tools

| Tool | Category | Install |
|------|----------|---------|
| `gofmt` / `goimports` | format | bundled with Go / `go install golang.org/x/tools/cmd/goimports@latest` |
| `golangci-lint` | lint | `brew install golangci-lint` |
| `gosec` | security | `brew install gosec` |
| `govulncheck` | security | `go install golang.org/x/vuln/cmd/govulncheck@latest` |
| `go-licenses` | dependencies | `go install github.com/google/go-licenses@latest` |

```bash
# Install all Go tools via goneat
goneat doctor tools --scope go --install --yes
```

## Format

goneat uses `gofmt` for Go formatting and `goimports` for import organization
when available. Format check mode (`goneat format --check` or
`goneat assess --categories format`) reports files that differ from their
formatted form without modifying them.

### What goneat runs

When formatting Go code, `goneat` discovers all `.go` files across your project. It passes them to `gofmt -l` (to list files needing formatting) or `gofmt -w` (to write changes). If `goimports` is installed, it will use `goimports -l` or `-w` instead, which handles both formatting and automatic import grouping and sorting.

Using `goneat format` utilizes a parallel worker pool, allowing large Go projects to be formatted across all available CPU cores, making it significantly faster than standard `gofmt` loops in bash scripts.

### Common Findings

| Finding | Meaning | Fix |
|---------|---------|-----|
| "File needs formatting" | `gofmt` or `goimports` output differs from on-disk content. | Run `goneat format <file>` or `gofmt -w <file>` |
| "Imports not sorted" | `goimports` detected out-of-order or un-grouped imports. | Run `goneat format <file>` with `goimports` installed. |

*Note: Sometimes a file shows as needing formatting but `gofmt` alone says it's clean. This is often due to line endings (CRLF vs LF) or Byte Order Marks (BOM). `goneat` ensures normalization.*

## Lint

goneat runs `golangci-lint` for Go linting. The linter set is controlled by
`.golangci.yml` in your repository root.

```bash
goneat assess --categories lint
goneat assess --categories lint --new-issues-only   # only new since HEAD~
```

### Configuration

golangci-lint reads `.golangci.yml` from the repository root. goneat passes
`--package-mode` in hook contexts to lint by package rather than by file.

You can tune the behavior using standard `golangci-lint` configuration:
- **`max-same-issues`**: By default, this is 3. If you have a widespread issue, only the first 3 will show. Set to `0` to see all occurrences.
- **Package mode vs file mode**: `golangci-lint` is generally designed to run on packages. `goneat` handles bridging file-level git hooks into package-level lints to prevent false positives that occur when linting single files out of context.
- **`--new-issues-only`**: goneat natively translates its diff-aware assessment directly into `golangci-lint`'s `--new-from-rev` flag for accurate delta linting.

### Common Findings

**QF1012 — Prefer `fmt.Fprintf` over `WriteString(fmt.Sprintf(…))`**

Introduced by staticcheck / golangci-lint 2.10+ across more code patterns.
The fix is mechanical:

```go
// Before
sb.WriteString(fmt.Sprintf("value: %s\n", v))

// After
fmt.Fprintf(&sb, "value: %s\n", v)
```

**G104 — Errors unhandled**

```go
// Before
os.Remove(path)

// After
if err := os.Remove(path); err != nil {
    // handle error
}
```

### Version Notes

| golangci-lint version | Notable behavior change |
|-----------------------|------------------------|
| 2.10 | QF1012 expanded to more `WriteString(fmt.Sprintf)` patterns |

## Security

goneat runs `gosec` and `govulncheck` in the security category.

```bash
goneat assess --categories security
goneat assess --categories security --track-suppressions   # report #nosec usage
```

### gosec

gosec performs static analysis on Go source for common security issues.
Findings are reported by rule ID (e.g., `G304`, `G204`).

`goneat` automatically shards `gosec` runs across large repositories using a parallel pool to minimize execution time. 

Suppress false positives using `// #nosec GXXX` comments. When using `goneat assess --categories security --track-suppressions`, `goneat` will audit and report the usage of these suppression comments, ensuring security exceptions are tracked during reviews.

#### Taint Analysis Rules (gosec 2.23.0+)

gosec 2.23.0 introduced inter-procedural taint analysis rules that trace data
flow across function boundaries. These are **distinct from the older rules** and
require separate suppression:

| Rule | Triggers on | Commonly FP in |
|------|-------------|----------------|
| G702 | `exec.Command` / `exec.CommandContext` with taint-traced args | CLI tools that run external programs by design |
| G703 | `os.Open`, `os.Stat`, etc. with taint-traced paths | Tools that read env-var-configured or user-specified files |
| G704 | `http.Client.Do`, `http.Get`, etc. with taint-traced URLs | Tools that make HTTP calls to config-derived endpoints |

**Critical**: `#nosec G304` does **not** suppress G703. The taint rules are
independent and must be named explicitly:

```go
// Wrong: G303/G304 suppressed, G703 still fires
f, err := os.Open(path) // #nosec G304

// Correct: both suppressed
f, err := os.Open(path) // #nosec G304 G703 - path from validated config, not user input
```

For code that intentionally handles file paths from configuration or environment
variables, G703 is typically a false positive. The suppression comment should
explain why.

#### Version Notes

| gosec version | Notable behavior change |
|---------------|------------------------|
| 2.23.0 | Added G702, G703, G704 taint analysis rules |

### govulncheck

`govulncheck` analyzes your `go.mod` dependency graph against the Go
vulnerability database (vuln.go.dev).

Unlike broad SBOM scanners (like `grype`), `govulncheck` is deeply aware of your Go code. It distinguishes between:
- **Imported**: A vulnerable package is in your dependency tree.
- **Called**: Your code actually invokes the specific vulnerable function.

`goneat` surfaces this context in its JSON output, allowing you to prioritize "Called" vulnerabilities over "Imported" ones.

## Dependencies

`go-licenses` enumerates Go module licenses for compliance reporting.

```bash
goneat dependencies --licenses
```

`go-licenses` scans the `go.mod` graph and inspects the `LICENSE` files of your dependencies. `goneat` parses this output to evaluate against the allowed/forbidden license lists defined in your `.goneat/dependencies.yaml`.

Example `.goneat/dependencies.yaml`:
```yaml
version: v1
licenses:
  allowed: [MIT, Apache-2.0, BSD-3-Clause]
  forbidden: [GPL-3.0, AGPL-3.0]
```

## Known Behaviors and Edge Cases

**jq version string parsing**: The macOS Homebrew build of jq reports its
version as `jq-1.7.1-apple`, which does not conform to semver. goneat logs a
warning but continues; the minimum version policy check for jq is skipped on
affected systems.

**checkmake version**: checkmake does not embed its version in CLI output.
goneat cannot evaluate version policies for checkmake and skips that check.

## See Also

- [`assess` command reference](../commands/assess.md)
- [`format` command reference](../commands/format.md)
- [`security` command reference](../commands/security.md)
- [Lint: Shell, Make, GitHub Actions](../../assess/lint-shell-make-gha.md)