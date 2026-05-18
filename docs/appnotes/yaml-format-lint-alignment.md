---
title: "YAML Format/Lint Alignment"
description: "How goneat keeps yamlfmt and yamllint aligned, especially for inline comment spacing"
author: "goneat contributors"
date: "2026-03-25"
last_updated: "2026-05-17"
status: "approved"
tags: ["yaml", "yamlfmt", "yamllint", "format", "lint"]
category: "appnotes"
---

# YAML Format/Lint Alignment

goneat uses `yamlfmt` for YAML formatting and `yamllint` for YAML linting. In
practice, those tools are not perfectly aligned out of the box.

The most important mismatch is inline comment spacing:

- `yamllint` default `comments` rule expects `2` spaces before an inline comment
- `yamlfmt` default `pad_line_comments` behavior uses `1`

That means a file like this is lint-compatible by default (two spaces before the inline comment):

```text
enabled: true  # inline comment
```

but a formatter running with `yamlfmt` defaults can rewrite it to (one space before the inline comment):

```text
enabled: true # inline comment
```

which then fails strict `yamllint` with:

```text
too few spaces before comment: expected 2 (comments)
```

## goneat's Default Behavior

goneat pins inline comment padding to a lint-compatible default when it runs
YAML formatting.

Built-in goneat default:

```yaml
format:
  yaml:
    pad_line_comments: 2
```

This keeps these goneat-internal paths aligned:

- `goneat format` (sequential)
- `goneat format` (parallel/work-pool)
- `goneat assess --categories format --check`
- `goneat assess --categories format --fix`
- `goneat assess --categories lint`

As of v0.5.12 all three paths — sequential format (`cmd/format.go::formatYAMLFile`),
parallel format (`pkg/work/format_processor.go::formatYAMLFile`), and the
assess/check path (`pkg/work/format_processor.go::checkYAMLFile`) — pass
yamlfmt the same arguments via a single shared builder,
`pkg/config.YAMLFormatConfig.YamlfmtFormatterArgs`. Prior to v0.5.12 each
of these had its own copy of the arg-construction logic, and the sequential
path silently omitted `pad_line_comments`, so `goneat format` was a no-op
on files the parallel and assess paths would correctly flag. See the v0.5.12
release notes.

> 🛑 **Goneat's internal alignment does NOT extend to tools you invoke
> outside goneat.** Direct `yamlfmt -lint` from a CI job, hook, or script
> reads only `.yamlfmt` (plus its own built-in defaults). For those callers
> to agree with goneat, `.yamlfmt` must mirror the goneat-canonical
> settings — see the **Repository Guidance** section below.

## Precedence Model

Use this mental model when a repository has both `.yamlfmt` and `.yamllint`:

1. `.yamllint` defines lint policy
2. goneat config defines formatter behavior for settings that goneat pins
3. `.yamlfmt` defines the remaining formatter behavior that goneat does not pin

Today, the main goneat-pinned YAML compatibility setting is inline comment
padding.

That means:

- `.yamllint` is still authoritative for whether `1` space or `2` spaces is
  acceptable
- goneat deliberately defaults the formatter side to `2` spaces because that is
  the safe, lint-compatible default for most repos
- `.yamlfmt` remains the right place for indent, line endings, and related
  formatter-native behavior

## Repository Guidance

> ⚠️ **CRITICAL — read before copy-pasting.** If any tool in your stack (CI
> jobs, pre-commit hooks, IDE integrations) invokes `yamlfmt` directly
> instead of going through `goneat format`, **your `.yamlfmt` MUST set
> `formatter.pad_line_comments: 2` explicitly**. yamlfmt's built-in default
> is `1`, which silently disagrees with goneat's canonical `2` and produces
> a "goneat says clean, direct yamlfmt -lint says broken" false-red. This
> is the #1 cause of mysterious CI failures on repos using goneat. The
> baseline below already includes the setting.

If your repo wants the common strict-lint setup, this is a good baseline. The `pad_line_comments: 2` line is the load-bearing one — see the warning above.

`.yamlfmt`

```yaml
# REQUIRED: pad_line_comments must be 2 to match goneat's canonical default.
# Without this, direct `yamlfmt -lint` invocations will disagree with goneat.
formatter:
  type: basic
  indent: 2
  line_ending: lf
  pad_line_comments: 2
```

`.yamllint`

```yaml
extends: default

rules:
  document-start: disable
```

If your team intentionally wants non-default inline comment spacing,
configure goneat, `.yamlfmt`, and `.yamllint` together so all three agree.
The mental model: **goneat owns the canonical formatter policy; `.yamlfmt`
must mirror it for any direct yamlfmt invocation; `.yamllint` defines what
the linter accepts.**

### Direct `yamlfmt` callouts (CI, hooks, scripts)

Anywhere your build invokes `yamlfmt` outside of goneat — bare
`yamlfmt -lint .` in a CI job, a pre-commit hook calling `yamlfmt` directly,
a script normalizing YAML — apply one of these three patterns:

1. **Replace with goneat** _(recommended)_: switch the call to
   `goneat format --check` (or `goneat assess --categories format`). This
   eliminates the drift surface entirely; there is no `pad_line_comments`
   on the command line to forget.
2. **Pin in `.yamlfmt`**: add `formatter.pad_line_comments: 2` as shown
   above. Now direct yamlfmt invocations see the same setting goneat uses.
3. **Pass on the command line**: invoke as
   `yamlfmt -lint -formatter pad_line_comments=2 <path>`. Use only when (1)
   and (2) are infeasible; it's easy to forget on the next CI job added.

### Symptom of misalignment (debug aid)

If you see a CI job report:

```text
The following formatting differences were found:
  - foo: bar  # comment   →   - foo: bar # comment
```

— meaning yamlfmt wants to collapse two spaces before `#` down to one space —
but `goneat format` and `goneat format --check` say the file is clean, you
have hit this drift. Apply pattern 2 above (add `pad_line_comments: 2` to
`.yamlfmt`); the failure resolves immediately, and the goneat upstream is
self-consistent.

## Why goneat Pins This Setting

The goal is not to replace `yamlfmt` configuration wholesale. The goal is to
prevent a common dogfood failure mode where developers run a formatter, get a
green result, and then immediately fail `yamllint` during hooks or CI.

goneat pins the minimum behavior needed to make its formatter and checker paths
trustworthy in normal repo automation.

## See Also

- [Format Command Reference](../user-guide/commands/format.md)
- [Assess Command Reference](../user-guide/commands/assess.md)
