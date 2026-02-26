---
title: "TypeScript / JavaScript Toolchain"
description: "How goneat handles TypeScript and JavaScript: biome lint and format, tsc typecheck — monorepo nested-root behavior, configuration, and version notes"
author: "goneat contributors"
date: "2026-02-26"
last_updated: "2026-02-26"
status: "published"
tags: ["typescript", "javascript", "biome", "tsc", "toolchain"]
category: "user-guide"
---

# TypeScript / JavaScript Toolchain

goneat handles JS/TS through biome (lint + format) and optionally tsc (typecheck).
Both tools are optional — goneat skips them gracefully if not installed.

## Tools

| Tool | Category | Install |
|------|----------|---------|
| `biome` | lint, format | `brew install biome` or `npm install -g @biomejs/biome` |
| `tsc` | typecheck | bundled with TypeScript (`npm install -g typescript`) |

```bash
goneat doctor tools --scope typescript --install --yes
```

## Format

biome handles both formatting and linting for TypeScript and JavaScript. goneat
invokes biome for format checking and fixing.

```bash
goneat format                                      # fix formatting
goneat assess --categories format                  # check only
```

`goneat` discovers files with extensions `ts`, `tsx`, `js`, `jsx`, `mjs`, `cjs`, `json`, and `jsonc`. It invokes `biome format --write` or checks using exit codes and JSON diagnostics. If `biome` is missing, `goneat` gracefully skips it (and uses `prettier` for JSON/Markdown if available).

### Monorepo / Nested-Root Behavior

biome 2.x rejects invocations that mix files from different biome config roots.
goneat resolves this by grouping files per-root and invoking biome once per group
from the root's directory. This fixes errors like:

```
Found a nested root configuration, but there's already a root configuration.
```

`goneat` automatically walks up the directory tree to find the nearest `biome.json` or `biome.jsonc` context, correctly dispatching grouped file arrays to each appropriate biome invocation.

### Common Findings

| Finding | Meaning | Fix |
|---------|---------|-----|
| "File needs formatting" | biome would rewrite the file | Run `goneat format <file>` |

### Version Notes

| biome version | Notable behavior change |
|---------------|------------------------|
| 2.0 | Removed `--check` flag; goneat now uses exit codes + JSON output |
| 2.4 | Changed diagnostic JSON format; goneat updated parser in v0.5.4 |

## Lint

biome's linter covers correctness, style, and performance rules for JS/TS.

```bash
goneat assess --categories lint
goneat assess --categories lint --new-issues-only
```

Linting configuration is defined in the `linter` section of `biome.json`. `goneat` extracts diagnostic information from `biome`'s JSON output, translating internal severities to standard `goneat` severity levels.

You can suppress rules using inline comments:
```typescript
// biome-ignore lint/suspicious/noExplicitAny: needed for interop
const data: any = JSON.parse(input);
```

### Common Findings

| Rule | Meaning |
|------|---------|
| `noExplicitAny` | Usage of the `any` type is discouraged. |
| `useConst` | Variables that are never reassigned should use `const`. |
| `noUnusedVariables` | A declared variable is never used in the file. |

## Typecheck

goneat runs `tsc --noEmit` to catch type errors that biome's linter does not
cover. This requires a `tsconfig.json` in the project root.

```bash
goneat assess --categories typecheck
goneat assess --categories format,lint,typecheck   # combined
```

Since `biome` is only a parser and linter, it does not do full TypeScript type checking. `goneat` solves this by invoking `tsc --noEmit`. This is heavily bound to your `tsconfig.json`.

Type checking is inherently slower than linting. It is often configured to run in CI contexts, but can be opted into for pre-push hooks depending on performance requirements.

### Configuration

```yaml
# .goneat/assess.yaml
typecheck:
  enabled: true
  typescript:
    enabled: true
    config: tsconfig.json    # path to tsconfig
    strict: false            # override strict mode
    skip_lib_check: true
    file_mode: false         # single-file mode for --include
```

### Common Findings

| Error | Meaning |
|-------|---------|
| `TS2322` | Type 'X' is not assignable to type 'Y'. |
| `TS2531` | Object is possibly 'null'. |

## Known Behaviors and Edge Cases

**`biome.json` schema mismatch warnings**: goneat surfaces biome's own diagnostic
when the biome.json config does not match the expected schema for the installed
biome version. Upgrade biome or update the config `$schema` URL to resolve.

**No `tsconfig.json`**: typecheck is silently skipped if no tsconfig is found.
Use `goneat doctor tools --scope typescript` to verify the expected setup.

## See Also

- [`assess` command reference](../commands/assess.md)
- [`format` command reference](../commands/format.md)