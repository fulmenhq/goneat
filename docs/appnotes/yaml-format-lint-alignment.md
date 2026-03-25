---
title: "YAML Format/Lint Alignment"
description: "How goneat keeps yamlfmt and yamllint aligned, especially for inline comment spacing"
author: "goneat contributors"
date: "2026-03-25"
last_updated: "2026-03-25"
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

That means a file like this is lint-compatible by default:

```yaml
enabled: true  # inline comment
```

but a formatter running with `yamlfmt` defaults can rewrite it to:

```yaml
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

This keeps these paths aligned:

- `goneat format`
- `goneat assess --categories format --check`
- `goneat assess --categories format --fix`
- `goneat assess --categories lint`

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

If your repo wants the common strict-lint setup, this is a good baseline:

`.yamlfmt`

```yaml
formatter:
  type: basic
  indent: 2
  line_ending: lf
```

`.yamllint`

```yaml
extends: default

rules:
  document-start: disable
```

If your team intentionally wants non-default inline comment spacing, configure
goneat and `yamllint` together so formatter and linter agree.

## Why goneat Pins This Setting

The goal is not to replace `yamlfmt` configuration wholesale. The goal is to
prevent a common dogfood failure mode where developers run a formatter, get a
green result, and then immediately fail `yamllint` during hooks or CI.

goneat pins the minimum behavior needed to make its formatter and checker paths
trustworthy in normal repo automation.

## See Also

- [Format Command Reference](../user-guide/commands/format.md)
- [Assess Command Reference](../user-guide/commands/assess.md)
