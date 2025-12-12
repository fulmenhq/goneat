# CI Runner Guide

## Two Paths for Tool Availability (v0.3.14+)

goneat validates **two approaches** for tool availability in CI, with clear guidance on when to use each:

| Path                | Approach                        | Friction | Recommended For                                |
| ------------------- | ------------------------------- | -------- | ---------------------------------------------- |
| **Container**       | `ghcr.io/fulmenhq/goneat-tools` | LOW      | CI runners (always)                            |
| **Package Manager** | `goneat doctor tools --install` | HIGHER   | Local dev, platforms without container support |

### CI Workflow Structure

goneat's own CI demonstrates both paths with explicit dependency ordering:

```
build-test-lint
       ↓ (uploads goneat binary)
container-probe  ← LOW friction, validates first
       ↓ (only if container passes)
bootstrap-probe  ← HIGHER friction, validates second
```

**Why this order?**

- Container path is the recommended CI approach - validate it first
- Don't waste cycles on package manager issues if container fails
- Gives users confidence: "If goneat CI passes, my container-based CI will work"

## Container Path (Recommended for CI)

**The container IS the contract.** Same image everywhere = same behavior everywhere.

### Why Container-Based CI?

| Approach                   | Confidence | Why                                                                       |
| -------------------------- | ---------- | ------------------------------------------------------------------------- |
| Install tools in runner    | LOW        | Package manager variability, arm64/amd64 differences, dependency failures |
| Use goneat-tools container | HIGH       | Same container everywhere = same behavior everywhere                      |

### Benefits for All goneat Users

This isn't just for goneat development - **any project using goneat** can leverage the goneat-tools container:

```yaml
# Your project's .github/workflows/ci.yml
jobs:
  quality:
    runs-on: ubuntu-latest
    container:
      image: ghcr.io/fulmenhq/goneat-tools:latest
      # Required: the container runs non-root, but GitHub mounts /__w from the host.
      # UID 1001 matches the hosted runner's workspace ownership.
      options: --user 1001
    steps:
      - uses: actions/checkout@v4

      # Install/bring your goneat binary into the job (example: download from release)
      # - run: curl -fsSL "https://github.com/fulmenhq/goneat/releases/download/vX.Y.Z/goneat_vX.Y.Z_linux_amd64.tar.gz" | tar -xz
      # - run: install -m 0755 goneat ./bin/goneat

      - run: ./bin/goneat assess --categories lint,format --fail-on high .
```

**What you get:**

- Pre-installed, version-pinned tools (no `npm install`, no `brew install`)
- Multi-arch support (linux/amd64 + linux/arm64)
- Consistent formatting across all contributors
- No "works on my machine" issues

### Tools in goneat-tools

| Tool     | Version | Purpose                         |
| -------- | ------- | ------------------------------- |
| prettier | 3.7.4   | JSON, Markdown, YAML formatting |
| yamlfmt  | v0.20.0 | YAML formatting/linting         |
| jq       | 1.8.1   | JSON processing                 |
| yq       | 4.49.2  | YAML processing                 |
| ripgrep  | 15.1.0  | Fast text search                |
| git      | 2.52.0  | Version control                 |
| bash     | 5.3.3   | Shell scripting                 |

Image registry: `ghcr.io/fulmenhq/goneat-tools:latest` (or `:v1.0.0` for pinned version)

---

## Quality Gates and Linting

Goneat includes comprehensive linting for multiple file types:

### Makefile Validation (checkmake)

Checkmake enforces Makefile best practices:

### GitHub Actions Validation (actionlint)

Actionlint validates workflow files for:

- Syntax correctness
- Action reference validity
- Security best practices
- Deprecated action detection
- Job dependency validation

### Severity-Based Enforcement

- **CI Pipelines**: Use `--fail-on high` to catch critical Makefile issues (missing `.PHONY`, syntax errors) while allowing style improvements
- **Pre-commit**: Use `--fail-on critical` for fast feedback on blocking issues
- **Local development**: Use `--fail-on medium` for comprehensive checking

### Checkmake Limitations

The checkmake tool has hardcoded rules that cannot be customized:

- **Maximum target body length**: 5 lines (encourages modular makefiles)
- **Required phony targets**: `.PHONY` declarations for all non-file targets
- **Style rules**: Various best practices for Makefile maintainability

### Handling Complex Makefiles

For complex build targets exceeding the 5-line limit:

1. **Refactor into scripts**: Move complex logic to shell scripts called by make
2. **Use include files**: Break large makefiles into smaller, included files
3. **Accept violations**: Some CI/CD complexity legitimately needs more than 5 lines

### Configuration

Enable/disable linting tools in `.goneat/assess.yaml`:

```yaml
lint:
  make:
    checkmake:
      enabled: true
    paths:
      - "**/Makefile"
    ignore:
      - "**/testdata/**" # Exclude test makefiles
  github_actions:
    actionlint:
      enabled: true
    paths:
      - ".github/workflows/**/*.yml"
    ignore: [] # No exclusions for security-critical workflows
```

## Local CI Runner (Optional)

Run GitHub Actions workflows locally using [nektos/act](https://github.com/nektos/act) for quick iteration.

> **Note (v0.3.14+)**: With container-based CI, local act runs are a "nice-to-have" rather than essential. The container approach already provides high confidence that GitHub CI will pass. Local runs are useful for fast feedback loops during development.

### Quick Start

```bash
# 1. Install prerequisites (macOS/Linux)
brew install docker colima act

# 2. Start Docker runtime
colima start --mount-type sshfs

# 3. Configure act (from repo root)
cp config/cicd/actrc.template ~/.actrc

# 4. Run local CI
make local-ci-format  # Format check (uses goneat-tools container)
make local-ci         # Build/test/lint (standard runner)
make local-ci-all     # All jobs
```

> **Windows**: Use Docker Desktop or Rancher Desktop. Install act via `scoop install act` or `winget install nektos.act`.

### Make Targets

| Target                 | Job             | Description                                      |
| ---------------------- | --------------- | ------------------------------------------------ |
| `make local-ci-format` | format-check    | Runs in goneat-tools container (HIGH confidence) |
| `make local-ci`        | build-test-lint | Go build, test, lint (standard runner)           |
| `make local-ci-all`    | All jobs        | Runs all CI jobs                                 |
| `make local-ci-check`  | -               | Verify prerequisites (Docker + act)              |

### Why Local CI is "Nice-to-Have" Now

**Before v0.3.14**: Local CI was important because we needed to catch tool installation failures before pushing.

**After v0.3.14**: Tool-dependent jobs run in the goneat-tools container. The container guarantees consistency:

- Same image on your laptop (via act) = same image on GitHub runners
- No package manager installs to fail
- No arm64 vs amd64 tool build differences

**When local CI is still useful:**

- Fast iteration without push/wait cycles
- Debugging workflow syntax issues
- Testing changes to non-container jobs

**When to just push to GitHub:**

- You've made code changes (not workflow changes)
- Container jobs will behave identically anyway
- You want authoritative amd64 results (if on Apple Silicon)

---

## Prerequisites

### Docker-Compatible Runtime

act requires a Docker-compatible runtime:

#### macOS / Linux

| Runtime                  | Install                                                | Start                             | Notes                             |
| ------------------------ | ------------------------------------------------------ | --------------------------------- | --------------------------------- |
| **Colima** (recommended) | `brew install docker colima`                           | `colima start --mount-type sshfs` | Lightweight, CLI-only             |
| Docker Desktop           | [Download](https://docker.com/products/docker-desktop) | Open app                          | Commercial license for large orgs |
| Rancher Desktop          | [Download](https://rancherdesktop.io/)                 | Open app                          | Use **dockerd** runtime           |

#### Windows

| Runtime         | Install                                                | Start    | Notes                   |
| --------------- | ------------------------------------------------------ | -------- | ----------------------- |
| Docker Desktop  | [Download](https://docker.com/products/docker-desktop) | Open app | Best WSL2 integration   |
| Rancher Desktop | [Download](https://rancherdesktop.io/)                 | Open app | Use **dockerd** runtime |

**Verify Docker is running:**

```bash
docker info
```

### act (GitHub Actions Runner)

```bash
# macOS / Linux
brew install act

# Windows
scoop install act
# or
winget install nektos.act

# Via goneat
goneat doctor tools --install --scope cicd
```

**Verify installation:**

```bash
act --version
```

## Configuration

### .actrc File

act reads configuration from `~/.actrc` (user-level) or `./.actrc` (repo-level).

**Recommended**: Copy the goneat template:

```bash
cp config/cicd/actrc.template ~/.actrc
```

The template configures:

- Runner images for GitHub parity
- Performance optimizations
- Apple Silicon compatibility

### Secrets

For workflows requiring secrets:

```bash
# .secrets (DO NOT COMMIT)
GITHUB_TOKEN=ghp_xxxxxxxxxxxx
```

Uncomment in `.actrc`:

```
--secret-file .secrets
```

## Troubleshooting

### Docker Not Running

```
❌ Docker is not running
```

**Fix**: Start your Docker runtime:

- Colima: `colima start`
- Docker Desktop: Open the application

### Platform Architecture (Apple Silicon)

Local CI runs on your **native architecture**:

- Apple Silicon (M1/M2/M3/M4): arm64
- Intel Mac / Linux: amd64

**For true amd64 parity**: Push to GitHub CI or use cloud infrastructure like [Daytona.io](https://daytona.io).

**Why this matters less now**: Container-based jobs (format-check) use multi-arch images that work on both arm64 and amd64. The tools behave identically.

### First Run is Slow

First run downloads:

1. **Colima VM** (~1GB): `colima start` first run
2. **Runner images** (~2-4GB): First `act` run
3. **goneat-tools image** (~150MB): First container job run

Subsequent runs use cached images.

### Colima Socket Mount Error

```
error while creating mount source path '.../.colima/docker.sock': mkdir ... operation not supported
```

**Fix**:

```bash
colima delete -f
colima start --mount-type sshfs
```

## References

- [goneat-tools image](https://github.com/fulmenhq/fulmen-toolbox/tree/main/images/goneat-tools)
- [nektos/act Repository](https://github.com/nektos/act)
- [act User Guide](https://nektosact.com/)
- [goneat CI Workflow](../../.github/workflows/ci.yml)
