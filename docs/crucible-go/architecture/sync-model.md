---
title: "Crucible Sync Model Architecture"
description: "Architecture decision record for how Crucible distributes schemas, docs, and templates"
author: "@3leapsdave"
date: "2025-10-02"
last_updated: "2025-10-02"
status: "approved"
tags: ["architecture", "adr", "sync", "distribution"]
---

# Crucible Sync Model Architecture

## Status

**Approved** - 2025-10-02

## Context

Crucible serves as the single source of truth (SSOT) for schemas, standards, templates, and documentation across the FulmenHQ ecosystem. We needed to decide how downstream repositories (gofulmen, tsfulmen, tools, Fulmens) would consume these assets.

### Requirements

1. **Flexibility**: Downstream repos need control over what they sync and when
2. **Versioning**: Clear version coordination between Crucible and consumers
3. **Performance**: Minimal overhead for consumers (don't force full repo as dependency)
4. **Transparency**: Consumers should see what they're getting
5. **Cross-language**: Support Go, TypeScript, and future languages equally
6. **Offline capability**: Built artifacts should work without network access
7. **Maintainability**: Easy to update and verify sync state

### Constraints

- Crucible uses CalVer versioning (`YYYY.0M.MICRO`)
- Assets have independent lifecycles (schemas can update without affecting standards)
- Some consumers want vendored/committed assets, others prefer gitignored
- Git submodules are too heavyweight and problematic
- Pure CDN/API approach doesn't work for build-time embedding

## Decision

We will implement a **dual distribution model**:

### 1. Published Packages (Primary for Runtime Use)

**Go Package**: `github.com/fulmenhq/crucible`

```go
import "github.com/fulmenhq/crucible"

schema := crucible.Schemas.Terminal.V1_0_0()
```

**TypeScript Package**: `@fulmenhq/crucible`

```typescript
import { schemas } from "@fulmenhq/crucible";

const schema = schemas.terminal.v1_0_0;
```

**Characteristics:**

- Monolithic package with internal organization
- Schemas/docs embedded at build time
- Versioned with CalVer matching repo VERSION
- Published to standard registries (Go modules, npm)
- Tree-shakeable in modern bundlers

### 2. Pull Scripts (Reference for Custom Integration)

**Bun/TypeScript Script**: `scripts/pull/crucible-pull.ts`

- Cross-platform reference implementation
- Selective sync of assets
- Version pinning support
- Config file driven
- Copy-and-adapt model

**Shell Script** (future): `scripts/pull/crucible-pull.sh`

- For Go developers who prefer shell
- Same interface as TypeScript version
- Minimal dependencies

**Characteristics:**

- Downstream repos copy and customize
- Not prescriptive on output paths (`.crucible/` vs `crucible/`)
- Supports both gitignored and committed patterns
- Explicit version tracking via `.crucible-version`

## Architecture

### Crucible Repository Structure

```
crucible/
├── VERSION                      # CalVer: 2025.10.0
├── schemas/                     # SSOT schemas
│   ├── terminal/v1.0.0/
│   ├── pathfinder/v1.0.0/
│   └── config/
├── docs/                        # SSOT documentation
│   ├── standards/
│   ├── guides/
│   └── architecture/
├── templates/                   # SSOT templates
├── lang/                        # Language wrappers
│   ├── go/                     # Go package
│   │   ├── schemas.go          # Embedded schemas
│   │   ├── docs.go             # Embedded docs
│   │   └── go.mod
│   └── typescript/             # TypeScript package
│       ├── src/
│       │   ├── schemas.ts      # Embedded schemas
│       │   └── docs.ts         # Embedded docs
│       └── package.json
└── scripts/
    ├── sync-to-lang.sh         # Internal: sync root -> lang/
    └── pull/
        ├── crucible-pull.ts    # Reference: for downstream
        └── README.md
```

### Build Process

**Pre-publish (internal to crucible):**

1. **Update root assets** (schemas/, docs/, templates/)
2. **Bump VERSION** to new CalVer
3. **Sync to language wrappers**:
   ```bash
   # Internal sync script
   scripts/sync-to-lang.sh
   # Copies schemas/ → lang/go/schemas/
   # Copies schemas/ → lang/typescript/schemas/
   # Copies docs/ → lang/go/docs/
   # Copies docs/ → lang/typescript/docs/
   ```
4. **Embed in packages**:
   - Go: `//go:embed` directives compile schemas into binary
   - TypeScript: Build process bundles schemas as constants/JSON
5. **Publish packages**:

   ```bash
   # Go: automatic via GitHub (git tag triggers module update)
   git tag v2025.10.0
   git push origin v2025.10.0

   # TypeScript: publish to npm
   cd lang/typescript
   npm publish
   ```

### Consumption Patterns

**Pattern A: Use Published Package (Runtime)**

```typescript
// In gofulmen or other tool
import { schemas } from "@fulmenhq/crucible";

// Schemas are embedded, no file I/O needed
const terminalSchema = schemas.terminal.v1_0_0;
```

**Pattern B: Use Pull Script (Build-Time)**

```bash
# In Fulmen template repo
bun run scripts/crucible-pull.ts --templates
# Pulls templates/ into .crucible/templates/
# Used during project scaffolding
```

**Pattern C: Hybrid**

```go
// In gofulmen: use package for runtime access
import "github.com/fulmenhq/crucible"

// But also pull for development/testing
// .crucible/schemas/ for validation against source
```

## Consequences

### Positive

✅ **Flexibility**: Consumers choose package or pull script based on needs  
✅ **Performance**: Embedded assets = zero runtime I/O  
✅ **Versioning**: Clear CalVer snapshots with `.crucible-version` tracking  
✅ **Transparency**: Pull scripts show exactly what's synced  
✅ **Maintainability**: Single source in root, synced to lang wrappers  
✅ **Offline**: Published packages work without network after install  
✅ **Language agnostic**: Same patterns for Go, TypeScript, future languages

### Negative

⚠️ **Duplication**: Root assets duplicated in lang/ directories  
⚠️ **Sync burden**: Must remember to sync root → lang/ before publish  
⚠️ **Package size**: Full schemas/docs embedded increases package size  
⚠️ **Update lag**: Pull script users must manually update

### Mitigations

**For duplication:**

- Automated sync script reduces manual work
- CI validates sync state before publish

**For sync burden:**

- Pre-publish checklist enforces sync
- Git hooks can automate sync on VERSION bump

**For package size:**

- Tree-shaking helps in modern bundlers
- Future: Separate packages for schemas-only if needed

**For update lag:**

- Automated update PRs (Dependabot-style)
- Clear communication of updates via CHANGELOG

## Internal Sync Process

### scripts/sync-to-lang.sh

```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "🔄 Syncing root assets to language wrappers..."

# Sync to Go
echo "📦 Go wrapper..."
rsync -av --delete "$ROOT/schemas/" "$ROOT/lang/go/schemas/"
rsync -av --delete "$ROOT/docs/" "$ROOT/lang/go/docs/"

# Sync to TypeScript
echo "📦 TypeScript wrapper..."
rsync -av --delete "$ROOT/schemas/" "$ROOT/lang/typescript/schemas/"
rsync -av --delete "$ROOT/docs/" "$ROOT/lang/typescript/docs/"

echo "✅ Sync complete"
```

**When to run:**

- Before bumping VERSION
- Before publishing packages
- Can be automated in CI/CD

### Embedding Strategy

**Go (compile-time):**

```go
package crucible

import _ "embed"

//go:embed schemas/terminal/v1.0.0/schema.json
var terminalSchemaV1 []byte

func GetTerminalSchemaV1() ([]byte, error) {
    return terminalSchemaV1, nil
}
```

**TypeScript (build-time):**

```typescript
// Generated during build
import terminalSchemaV1 from "./schemas/terminal/v1.0.0/schema.json";

export const schemas = {
  terminal: {
    v1_0_0: terminalSchemaV1,
  },
};
```

## Versioning Strategy

### Repository Version (VERSION file)

```
2025.10.0
```

**Meaning**: Snapshot of all assets at October 2025, revision 0

### Package Versions

```
github.com/fulmenhq/crucible v2025.10.0
@fulmenhq/crucible@2025.10.0
```

**Synchronized with repository VERSION**

### Asset Versions (in frontmatter)

```yaml
---
title: "Terminal Schema"
version: "v1.0.0"
revision: 3
last_updated: "2025-10-15"
---
```

**Independent of repository VERSION**

### Coordination

- Repository version = release snapshot date
- Asset versions = semantic versions of individual artifacts
- Pull script creates `.crucible-version` file for tracking
- Packages expose VERSION constant

## Alternative Approaches Considered

### Git Submodules

**Rejected**: Too heavyweight, difficult to version pin, problematic workflows

### Monorepo with Workspaces

**Rejected**: Doesn't solve cross-repo consumption, increases coupling

### Pure CDN/API Distribution

**Rejected**: Requires network at runtime, complicates offline builds

### Separate Package per Asset Type

**Rejected**: Too much overhead, version coordination complexity

### Go Proxy + npm Registry Only

**Rejected**: Doesn't serve pull script use case, forces package dependency

## Future Considerations

### Multi-Package Strategy (if needed)

If package size becomes problematic:

```
@fulmenhq/crucible-schemas  # Schemas only
@fulmenhq/crucible-docs     # Docs only
@fulmenhq/crucible          # Full bundle (depends on above)
```

### Shell Pull Script

Add `scripts/pull/crucible-pull.sh` for Go developers who prefer shell.

### Automated Sync Validation

CI workflow to ensure root → lang/ sync is current:

```yaml
- name: Validate sync
  run: |
    scripts/sync-to-lang.sh
    git diff --exit-code lang/
```

### Asset Registry

Future: Machine-readable registry of available assets with metadata:

```json
{
  "schemas": {
    "terminal": {
      "versions": ["v1.0.0"],
      "path": "schemas/terminal"
    }
  }
}
```

## References

- [Sync Strategy Guide](../guides/sync-strategy.md) - User-facing documentation
- [Pull Script README](../../scripts/pull/README.md) - Reference implementation
- [Repository Versioning Standard](../standards/repository-versioning.md) - CalVer strategy

## Related Standards

- [Repository Structure SOP](../sop/repository-structure.md)
- [Repository Version Adoption SOP](../sop/repository-version-adoption.md)

---

**Decision Date**: 2025-10-02  
**Status**: Approved  
**Author**: @3leapsdave  
**Reviewers**: N/A (Initial architecture)
