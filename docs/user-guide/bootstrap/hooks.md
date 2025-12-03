# Git Hooks Setup

goneat provides intelligent git hooks that validate commits and pushes against your project's quality standards. This guide explains how hooks work and how to ensure they're properly installed.

## How Git Hooks Work

Git hooks are scripts that run automatically at specific points in the git workflow:

- **pre-commit**: Runs before each commit (format checks, quick validation)
- **pre-push**: Runs before pushing (full validation, security checks)
- **pre-reset**: Runs before destructive operations

**Important**: Git hooks live in `.git/hooks/`, which is **local to each clone** and not tracked by Git. This means hooks must be installed on each machine after cloning.

## Hook Installation

### Automatic Installation (Recommended)

If your project's Makefile includes the `hooks-ensure` pattern, hooks install automatically on first build:

```bash
git clone <repo>
make build    # Hooks install automatically
```

### Manual Installation

If hooks aren't auto-installed, run:

```bash
goneat hooks install
```

This copies hooks from `.goneat/hooks/` to `.git/hooks/`.

## Comparison with Other Hook Tools

| Tool | Installation Trigger | After Fresh Clone |
|------|---------------------|-------------------|
| **goneat** | `make build` or `goneat hooks install` | Automatic (if Makefile configured) |
| **Husky (npm)** | `npm install` postinstall | Automatic |
| **pre-commit (Python)** | `pre-commit install` | Manual |
| **lefthook (Go)** | `lefthook install` | Manual |

goneat follows the same pattern as Husky by tying hook installation to the standard development workflow.

## Adding Auto-Install to Your Project

To ensure hooks install automatically after clone, add this to your Makefile:

```makefile
# Add to .PHONY line
.PHONY: ... hooks-ensure ...

# Add hooks-ensure target
hooks-ensure: ## Ensure git hooks are installed (auto-installs if missing)
	@if [ -d .git ] && [ ! -x .git/hooks/pre-commit ]; then \
		if command -v goneat >/dev/null 2>&1; then \
			echo "🔗 Installing git hooks..."; \
			goneat hooks install 2>/dev/null || true; \
		elif [ -f "dist/goneat" ]; then \
			echo "🔗 Installing git hooks..."; \
			./dist/goneat hooks install 2>/dev/null || true; \
		fi; \
	fi

# Add to build target
build: embed-assets
	@echo "Building..."
	# ... build commands ...
	@$(MAKE) hooks-ensure
```

This pattern:
1. Checks if we're in a git repo
2. Checks if pre-commit hook is missing or not executable
3. Installs hooks using goneat (system or local binary)
4. Silently succeeds if hooks already installed

## Verifying Hook Installation

Check if hooks are installed:

```bash
ls -la .git/hooks/pre-commit
# Should show executable file, not .sample
```

Test hooks manually:

```bash
goneat assess --hook pre-commit    # Test pre-commit checks
goneat assess --hook pre-push      # Test pre-push checks
```

## Troubleshooting

### Hooks Not Running

1. **Check installation**: `ls -la .git/hooks/pre-commit`
2. **Check permissions**: `chmod +x .git/hooks/pre-commit`
3. **Reinstall**: `goneat hooks install --force`

### Hooks Installed But Not Working

1. **Check goneat is available**: `which goneat` or `./dist/goneat version`
2. **Check hook content**: `cat .git/hooks/pre-commit`
3. **Run manually**: `./git/hooks/pre-commit`

### Bypassing Hooks (Emergency Only)

```bash
git commit --no-verify -m "emergency fix"
git push --no-verify
```

**Warning**: Only use `--no-verify` in genuine emergencies. It bypasses all quality checks.

## For Template/Scaffold Users

If you're using a goneat-based project template (like groningen):

1. Clone the template
2. Run `make build` or `make dev` (hooks install automatically)
3. Start developing with protection enabled

If hooks don't auto-install, check that the template's Makefile includes the `hooks-ensure` pattern above.
