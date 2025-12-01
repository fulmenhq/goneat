# Bootstrap Patterns for goneat v0.3.9+

**Status**: Reference Guide
**Version**: v0.3.9
**Author**: Arch Eagle (@arch-eagle)
**Last Updated**: 2025-12-01

---

## Overview

This appnote documents recommended patterns for integrating goneat into downstream repositories. Starting with v0.3.9, goneat can automatically install package managers (bun, brew) and foundation tools, simplifying CI bootstrap significantly.

## Key Concepts

### Two Types of tools.yaml

goneat repositories may have TWO different tools.yaml files with DIFFERENT formats:

| File | Purpose | Format |
|------|---------|--------|
| `.goneat/bootstrap-manifest.yaml` | Download goneat binary (custom) | gofulmen/groningen format |
| `.goneat/tools.yaml` | goneat doctor tools config | goneat standard format |

**Important**: These are NOT interchangeable. The formats are different.

### goneat Standard Format (tools.yaml)

Created by `goneat doctor tools init`:

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
  custom:
    tools:
      - my-special-tool
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
      url: https://github.com/fulmenhq/goneat/releases/download/v0.3.9/goneat_v0.3.9_{{os}}_{{arch}}.tar.gz
      checksum:
        darwin-arm64: "..."
```

---

## Pattern A: Shell Script Bootstrap (Recommended for Simple Projects)

Best for: Projects without existing Go bootstrap infrastructure.

**Example**: forge-workhorse-groningen

### Structure

```
my-repo/
├── .goneat/
│   └── tools.yaml          # Created by goneat doctor tools init
├── scripts/
│   └── install-goneat.sh   # Downloads goneat, calls doctor tools
└── Makefile
```

### install-goneat.sh

```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="${ROOT_DIR}/bin"
GONEAT_VERSION="v0.3.9"

# SHA256 checksums (update when changing version)
case "$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m)" in
  darwin-arm64) EXPECTED_SHA="830850afe860ec3773f5cc9f9eb693e3bb6aa6b8fd5bd30bcd54516d843d3d5a" ;;
  darwin-x86_64) EXPECTED_SHA="3a054db2d58d5a4f7a3f7fb9f8d5fba4a92c9495e9ba03ced136fcbc91be7866" ;;
  linux-x86_64) EXPECTED_SHA="2541a8d75c565ff4cca71fd090110e7ae0acaa919e5ff8c2cbcd382110a67618" ;;
  linux-aarch64) EXPECTED_SHA="15f49a33958c114916d9c7965ef3ace0b971855f626eb110abe58a9a7eae1d1b" ;;
  *) EXPECTED_SHA="" ;;
esac

# Download and verify
mkdir -p "${BIN_DIR}"
# ... (download, checksum verify, extract)

# Initialize goneat tools config if needed
if [[ ! -f "${ROOT_DIR}/.goneat/tools.yaml" ]] || ! grep -q "^scopes:" "${ROOT_DIR}/.goneat/tools.yaml"; then
  "${BIN_DIR}/goneat" doctor tools init --force
fi

# Install foundation tools (auto-installs bun/brew if needed)
"${BIN_DIR}/goneat" doctor tools --scope foundation --install --yes
```

### Makefile

```makefile
bootstrap:
	@./scripts/install-goneat.sh
```

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
      url: https://github.com/fulmenhq/goneat/releases/download/v0.3.9/goneat_v0.3.9_{{os}}_{{arch}}.tar.gz
      binName: goneat
      destination: ./bin
      checksum:
        darwin-arm64: "830850afe860ec3773f5cc9f9eb693e3bb6aa6b8fd5bd30bcd54516d843d3d5a"
        darwin-amd64: "3a054db2d58d5a4f7a3f7fb9f8d5fba4a92c9495e9ba03ced136fcbc91be7866"
        linux-amd64: "2541a8d75c565ff4cca71fd090110e7ae0acaa919e5ff8c2cbcd382110a67618"
        linux-arm64: "15f49a33958c114916d9c7965ef3ace0b971855f626eb110abe58a9a7eae1d1b"
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
	@./bin/goneat doctor tools --scope foundation --install --yes
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
          curl -sSL https://github.com/fulmenhq/goneat/releases/download/v0.3.9/goneat_v0.3.9_linux_amd64.tar.gz | tar -xz -C /usr/local/bin

      - name: Bootstrap foundation tools
        run: |
          goneat doctor tools init
          goneat doctor tools --scope foundation --install --yes

      - name: Build and test
        run: make test
```

---

## v0.3.9 Auto-Install Features

### Package Manager Auto-Install

When `goneat doctor tools --install` runs and no package manager is available:

1. **Tries bun first** (simpler, no dependencies)
   - Installs to `~/.bun/bin/`
   - Updates PATH immediately

2. **Falls back to brew** (user-local)
   - Installs to `~/homebrew/`
   - No sudo required

### Foundation Tools Scope

Default tools installed with `--scope foundation`:

- ripgrep, jq, yq (CLI utilities)
- go, go-licenses, golangci-lint (Go tooling)
- yamlfmt, prettier (formatters)

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

**Cause**: No package manager (bun/brew) available and auto-install failed.

**Fix**: v0.3.9 should auto-install bun. If failing:
```bash
# Manual bun install
curl -fsSL https://bun.sh/install | bash
export PATH="$HOME/.bun/bin:$PATH"

# Then retry
goneat doctor tools --scope foundation --install --yes
```

### "tools.yaml not found" or format errors

**Cause**: Missing or wrong format tools.yaml.

**Fix**:
```bash
# Initialize standard format
goneat doctor tools init --force
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

**Document Version**: 1.0
**Applies to**: goneat v0.3.9+
