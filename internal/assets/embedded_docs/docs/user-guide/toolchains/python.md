---
title: "Python Toolchain"
description: "How goneat handles Python: ruff for lint and format — pyproject.toml discovery, common findings, and configuration"
author: "goneat contributors"
date: "2026-02-26"
last_updated: "2026-02-26"
status: "published"
tags: ["python", "ruff", "toolchain"]
category: "user-guide"
---

# Python Toolchain

goneat uses ruff for both linting and formatting Python files. ruff is a single
fast tool that replaces flake8, isort, and black in one binary.

## Tools

| Tool   | Category     | Install                                   |
| ------ | ------------ | ----------------------------------------- |
| `ruff` | lint, format | `brew install ruff` or `pip install ruff` |

```bash
goneat doctor tools --scope python --install --yes
```

## Format

ruff's formatter is black-compatible and handles import sorting and code style.

```bash
goneat format                             # fix formatting
goneat assess --categories format         # check only
```

When formatting, `goneat` executes `ruff format` to resolve styling issues. If import sorting is enabled in your `ruff` configuration (usually via the `I` rule in `pyproject.toml`), `ruff check --fix --select I` logic is also addressed.

`goneat` automatically discovers `*.py` files, respecting standard `.gitignore` and `.goneatignore` conventions. If both `black` and `ruff` exist in a project, `goneat` prefers `ruff` for formatting when executing Python toolchains.

### Configuration

You can configure formatting options in your `pyproject.toml` or `ruff.toml`:

```toml
[tool.ruff.format]
quote-style = "double"
indent-style = "space"
skip-magic-trailing-comma = false
line-ending = "auto"
```

### Common Findings

| Finding                 | Meaning                                                                  | Fix                        |
| ----------------------- | ------------------------------------------------------------------------ | -------------------------- |
| "File needs formatting" | ruff formatter would rewrite the file to meet Black-compatible standards | Run `goneat format <file>` |
| "Imports not sorted"    | `ruff` (with `I` rules enabled) detected unsorted imports                | Run `goneat format <file>` |

## Lint

ruff implements hundreds of rules covering style, correctness, complexity, and
import hygiene.

```bash
goneat assess --categories lint
goneat assess --categories lint --new-issues-only
```

`ruff` replaces traditional linting suites like Flake8, pylint, and pydocstyle. It evaluates rules across different categories, mapping severities gracefully into `goneat`'s standardized severity model (Warning, Error, etc.).

### Configuration

ruff reads configuration from `pyproject.toml` (`[tool.ruff]`), `ruff.toml`, or
`.ruff.toml` in the project root.

```toml
# pyproject.toml
[tool.ruff]
line-length = 120
select = ["E", "F", "I", "N", "UP"]
ignore = ["E501"]
```

To suppress findings inline, use standard `# noqa` syntax:

```python
x = 1 / 0  # noqa: F841
```

### Common Findings

| Rule   | Meaning                                                            |
| ------ | ------------------------------------------------------------------ |
| `E501` | Line too long (often disabled in favor of trusting the formatter). |
| `F401` | Variable is declared but never used.                               |
| `F403` | Module imported but unused.                                        |
| `I001` | Import block is un-sorted or un-formatted.                         |

## Known Behaviors and Edge Cases

**Single tool for lint and format**: ruff serves both categories. goneat invokes
it separately for each (`ruff check` for lint, `ruff format` for format) but both
read the same configuration.

**Virtual environments**: goneat discovers `.py` files across the project but
respects `.gitignore` and `.goneatignore` patterns. Add `venv/`, `.venv/`, and
`__pycache__/` to your ignore file to prevent scanning installed packages.

## See Also

- [`assess` command reference](../commands/assess.md)
- [`format` command reference](../commands/format.md)
