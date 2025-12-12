# Bootstrap Patterns for goneat

**Status**: Reference Guide
**Version**: v0.3.14+
**Author**: Arch Eagle (@arch-eagle)
**Last Updated**: 2025-12-08

---

## Overview

This appnote documents recommended patterns for integrating goneat into downstream repositories. Starting with v0.3.9, goneat can automatically install package managers and foundation tools, simplifying CI bootstrap significantly. v0.3.10 refines the package manager strategy.

## Package Manager Strategy (v0.3.10+)

| Package Manager | Use Case                                                    |
| --------------- | ----------------------------------------------------------- |
| `brew`          | System binaries on darwin/linux (ripgrep, jq, yq, prettier) |
| `scoop/winget`  | System binaries on Windows                                  |
| `go-install`    | Go tools (golangci-lint, gosec, yamlfmt, etc.)              |
| `bun/npm`       | Node.js packages ONLY (e.g., eslint for TypeScript repos)   |
| `uv/pip`        | Python packages ONLY                                        |

**Key change in v0.3.10**: bun is no longer used for system binaries - it can only install npm packages.

## Key Concepts

### Two Types of tools.yaml

goneat repositories may have TWO different tools.yaml files with DIFFERENT formats:

| File                              | Purpose                         | Format                    |
| --------------------------------- | ------------------------------- | ------------------------- |
| `.goneat/bootstrap-manifest.yaml` | Download goneat binary (custom) | gofulmen/groningen format |
| `.goneat/tools.yaml`              | goneat doctor tools config      | goneat standard format    |

**Important**: These are NOT interchangeable. The formats are different.

### goneat Standard Format (tools.yaml)

Created by `goneat doctor tools init` (generates ALL 4 scopes in v0.3.10+):

```yaml
# .goneat/tools.yaml (goneat standard format)
scopes:
  foundation:
    description: Core tools
    tools:
      - ripgrep
      - jq
      - yq
      - prettier
      - yamlfmt
  security:
    tools:
      - gosec
      - govulncheck
  format:
    tools:
      - goimports
      - gofmt
  all:
    description: All tools
    tools: [...]
```

### Custom Bootstrap Manifest Format

Used by custom bootstrap scripts/packages:

```yaml
# .goneat/bootstrap-manifest.yaml (custom format)
version: v1.0.0
binDir: ./bin
tools:
  - id: goneat
    install:
      type: download
      url: https://github.com/fulmenhq/goneat/releases/download/v0.3.10/goneat_v0.3.10_{{os}}_{{arch}}.tar.gz
      checksum:
        darwin-arm64: "..."
```

---

## Pattern 0: Container-Based (Recommended for CI - v0.3.14+)

Best for: CI runners, consistent reproducible environments, zero package manager friction.

**Example**: goneat's own CI workflow

### Why Container-Based?

| Benefit           | Explanation                                           |
| ----------------- | ----------------------------------------------------- |
| **Zero friction** | No package manager installation, no PATH manipulation |
| **Consistent**    | Same tools, same versions, every run                  |
| **Fast**          | Pre-built image, no installation time                 |
| **Portable**      | Works on any runner that supports containers          |

### Structure

```
my-repo/
├── .github/workflows/
│   └── ci.yml              # Uses goneat-tools container
└── .goneat/
    └── tools.yaml          # Optional: for local dev fallback
```

### GitHub Actions Workflow

```yaml
name: CI
on: [push, pull_request]

jobs:
  # Container-based format checking (LOW friction)
  format-check:
    runs-on: ubuntu-latest
    container:
      image: ghcr.io/fulmenhq/goneat-tools:latest
    steps:
      - uses: actions/checkout@v4
      - name: Verify tools
        run: |
          prettier --version
          yamlfmt --version
          jq --version
          yq --version
          rg --version
      - name: Check formatting
        run: |
          goneat format --check . || echo "Format differences found"
          yamlfmt -lint .
          prettier --check "**/*.{md,json}"
```

### With Artifact Sharing (for custom goneat builds)

If you need to test a custom goneat binary in the container:

```yaml
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.25.x"
      - run: make build
      - uses: actions/upload-artifact@v4
        with:
          name: goneat-linux
          path: dist/goneat

  container-probe:
    needs: [build]
    runs-on: ubuntu-latest
    container:
      image: ghcr.io/fulmenhq/goneat-tools:latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/download-artifact@v4
        with:
          name: goneat-linux
          path: ./bin
      - run: |
          chmod +x ./bin/goneat
          ./bin/goneat doctor tools --scope foundation
          ./bin/goneat format --check .
```

### When to Use Container vs Package Manager

| Scenario                       | Recommended Path                                            |
| ------------------------------ | ----------------------------------------------------------- |
| CI runners                     | **Container** - zero friction, consistent                   |
| Local development (first time) | **Package manager** - tools available globally              |
| Local development (ongoing)    | Either - container via `make local-ci-*` or installed tools |
| Environments without Docker    | **Package manager** - only option                           |

---

## Pattern A: sfetch Bootstrap (Recommended for Simple Projects)

Best for: Projects without existing bootstrap infrastructure, and projects that want **high-confidence downloads** without maintaining bespoke checksum scripts.

**Example**: forge-workhorse-groningen

### Structure

```
my-repo/
├── .goneat/
│   └── tools.yaml              # Created by goneat doctor tools init
├── scripts/
│   └── bootstrap-dx.sh          # Installs sfetch (if missing), then installs goneat
└── Makefile
```

### bootstrap-dx.sh (concept)

```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="${ROOT_DIR}/bin"

mkdir -p "${BIN_DIR}"

# 1) Ensure sfetch is available (high-trust bootstrap)
if ! command -v sfetch >/dev/null 2>&1; then
  curl -sSfL https://github.com/3leaps/sfetch/releases/latest/download/install-sfetch.sh | bash
fi

# 2) Install goneat repo-locally (pinned)
sfetch --repo fulmenhq/goneat --tag vX.Y.Z --dest-dir "${BIN_DIR}"

# 3) Initialize goneat tools config if needed
if [[ ! -f "${ROOT_DIR}/.goneat/tools.yaml" ]] || ! grep -q "^scopes:" "${ROOT_DIR}/.goneat/tools.yaml" 2>/dev/null; then
  "${BIN_DIR}/goneat" doctor tools init --force
fi

# 4) Install foundation tools
"${BIN_DIR}/goneat" doctor tools --scope foundation --install --yes --no-cooling
```

### Makefile

```makefile
bootstrap:
	@./scripts/bootstrap-dx.sh
```

### Why this is preferred

- `sfetch` verifies signed checksum manifests (minisign/PGP) and then verifies checksums.
- Repos no longer maintain per-platform SHA pins in ad-hoc scripts.
- The same bootstrap logic applies to non-Go repos and CI runners.

> Note: The legacy `install-goneat.sh` pattern (manual curl + hardcoded SHA256 values) is deprecated and should be avoided for new repos.

---

## Pattern B: Go Bootstrap + goneat Doctor (Recommended for Go Projects)

Best for: Projects with existing Go bootstrap infrastructure (like gofulmen).

**Example**: gofulmen

### Structure

```
my-repo/
├── .goneat/
│   ├── bootstrap-manifest.yaml  # Custom: goneat binary download
│   └── tools.yaml               # Standard: goneat doctor tools config
├── bootstrap/                   # Go bootstrap package
│   └── *.go
├── cmd/bootstrap/
│   └── main.go
└── Makefile
```

### bootstrap-manifest.yaml (Custom Format)

```yaml
# Used by Go bootstrap package for goneat binary download
version: v1.0.0
binDir: ./bin
tools:
  - id: goneat
    description: Fulmen schema validation and automation CLI
    required: true
    install:
      type: download
      url: https://github.com/fulmenhq/goneat/releases/download/v0.3.10/goneat_v0.3.10_{{os}}_{{arch}}.tar.gz
      binName: goneat
      destination: ./bin
      checksum:
        darwin-arm64: "<sha256>"
        darwin-amd64: "<sha256>"
        linux-amd64: "<sha256>"
        linux-arm64: "<sha256>"
```

### Makefile

```makefile
bootstrap:
	@echo "Installing goneat..."
	@go run ./cmd/bootstrap --install --verbose --manifest .goneat/bootstrap-manifest.yaml
	@echo "Initializing goneat tools config..."
	@if [ ! -f .goneat/tools.yaml ] || ! grep -q "^scopes:" .goneat/tools.yaml; then \
		./bin/goneat doctor tools init --force; \
	fi
	@echo "Installing foundation tools..."
	@./bin/goneat doctor tools --scope foundation --install --yes --no-cooling
```

---

## Pattern C: CI-Only Bootstrap (Simplest)

Best for: Projects that only need goneat in CI, not local development.

### GitHub Actions Workflow

```yaml
name: CI
on: [push, pull_request]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install goneat
        run: |
          curl -sSL https://github.com/fulmenhq/goneat/releases/download/v0.3.10/goneat_v0.3.10_linux_amd64.tar.gz | tar -xz -C /usr/local/bin

      - name: Bootstrap foundation tools
        run: |
          goneat doctor tools init
          goneat doctor tools --scope foundation --install --yes --no-cooling

      - name: Build and test
        run: make test
```

---

## v0.3.10 Features

### --no-cooling Flag (New in v0.3.10)

For CI environments or offline/air-gapped systems, use `--no-cooling` to skip package age verification:

```bash
goneat doctor tools --scope foundation --install --yes --no-cooling
```

Without this flag, goneat verifies packages aren't too new (cooling policy) by checking release dates online.

### Multi-Scope Init (New in v0.3.10)

`goneat doctor tools init` now generates ALL 4 standard scopes (foundation, security, format, all) regardless of `--scope` flag:

```bash
goneat doctor tools init
# Creates: foundation, security, format, all scopes
# Tools: 13 (for Go repos)
```

### Package Manager Auto-Install

When `goneat doctor tools --install` runs and no package manager is available:

1. **Tries brew first** (user-local, no sudo required)
   - Installs to `~/homebrew/` or `/opt/homebrew/`
   - Updates PATH immediately

2. **Falls back to bun** for Node.js packages only
   - Installs to `~/.bun/bin/`

### Foundation Tools Scope

Default tools installed with `--scope foundation`:

- ripgrep, jq, yq (CLI utilities) - via brew
- go, go-licenses, golangci-lint (Go tooling) - via go-install
- yamlfmt (formatter) - via go-install
- prettier (formatter) - via brew

---

## Adding Custom Tools

Edit `.goneat/tools.yaml` to add project-specific tools:

```yaml
scopes:
  foundation:
    tools: [ripgrep, jq, yq, prettier, yamlfmt, ...]

  # Add custom scope
  my-project:
    description: Project-specific tools
    tools:
      - protoc
      - grpcurl
```

Install with:

```bash
goneat doctor tools --scope my-project --install --yes
```

---

## Updating goneat Version

### For Pattern A (Shell Script)

1. Update `GONEAT_VERSION` in `scripts/install-goneat.sh`
2. Update SHA256 checksums from release page

### For Pattern B (Go Bootstrap)

1. Update version in `.goneat/bootstrap-manifest.yaml`
2. Update checksums in same file
3. Get checksums from: `https://github.com/fulmenhq/goneat/releases/download/vX.Y.Z/SHA256SUMS`

---

## Troubleshooting

### "No available installer succeeded"

**Cause**: No package manager available and auto-install failed.

**Fix**: Ensure brew is installed or use --no-cooling in CI:

```bash
# Manual brew install (if needed)
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# Then retry
goneat doctor tools --scope foundation --install --yes --no-cooling
```

### "tools.yaml not found" or format errors

**Cause**: Missing or wrong format tools.yaml.

**Fix**:

```bash
# Initialize standard format (creates all 4 scopes)
goneat doctor tools init --force
```

### "Cooling policy violation"

**Cause**: Package is too new or can't verify release date.

**Fix**: Use `--no-cooling` flag for CI/offline environments:

```bash
goneat doctor tools --scope foundation --install --yes --no-cooling
```

### Conflict between custom manifest and goneat format

**Cause**: Using same filename for different purposes.

**Fix**: Rename custom manifest to `.goneat/bootstrap-manifest.yaml`

---

## Migration from Pre-v0.3.7

If your repo has an old `.goneat/tools.yaml` in custom format:

1. Rename to `.goneat/bootstrap-manifest.yaml`
2. Update Makefile to use `--manifest .goneat/bootstrap-manifest.yaml`
3. Run `goneat doctor tools init` to create standard format
4. Commit the new `.goneat/tools.yaml`

---

## See Also

- [CI/CD Runner Support Guide](../guides/goneat-tools-cicd-runner-support.md)
- [Package Managers Bootstrap](../user-guide/bootstrap/package-managers.md)
- [Tools Configuration Schema](../../schemas/tools/tools.v1.0.0.json)

---

**Document Version**: 3.0
**Applies to**: goneat v0.3.14+
