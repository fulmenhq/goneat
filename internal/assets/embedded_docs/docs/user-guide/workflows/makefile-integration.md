# Makefile Integration Best Practices

This guide covers best practices for integrating goneat into your project's Makefile bootstrap process.

## The Problem

Hardcoded version pins and unconditional installs can cause issues:

1. **Version downgrades**: A Makefile pinned to `v0.3.21` will overwrite your `v0.5.1` install
2. **Wasted time**: Unnecessary downloads on every `make bootstrap`
3. **Broken workflows**: Different projects fighting over the global goneat version

## Best Practices

### 1. Use Conditional Assignment (`?=`)

Allow environment overrides and prevent hardcoded pins:

```makefile
# GOOD: Allows override, documents minimum version
GONEAT_VERSION ?= v0.5.1

# BAD: Hardcoded, will always use this version
GONEAT_VERSION := v0.3.21
```

### 2. Skip Install if Already Present

Never overwrite an existing installation unless explicitly requested:

```makefile
bootstrap:
	@# Only install if goneat is not found OR FORCE=1
	@if [ "$(FORCE)" = "1" ] || ! command -v goneat >/dev/null 2>&1; then \
		echo "Installing goneat $(GONEAT_VERSION)..."; \
		sfetch --repo fulmenhq/goneat --tag $(GONEAT_VERSION) --dest-dir "$$BINDIR"; \
	else \
		echo "goneat already installed, skipping (use FORCE=1 to reinstall)"; \
	fi
```

### 3. Provide Force Reinstall Option

Always provide a way to force reinstall when needed:

```makefile
bootstrap-force:  ## Force reinstall all tools
	@$(MAKE) bootstrap FORCE=1
```

### 4. Show Installed Version

Always display the version after bootstrap completes:

```makefile
	@echo "goneat: $$(goneat version 2>&1 | head -n1)"
```

## Complete Example

Here's a complete, correct bootstrap target:

```makefile
# Minimum version - won't downgrade existing installs
GONEAT_VERSION ?= v0.5.1
BINDIR ?= $(HOME)/.local/bin

bootstrap:  ## Install external tools
	@echo "Bootstrapping development environment..."
	@# Verify sfetch (trust anchor)
	@if ! command -v sfetch >/dev/null 2>&1; then \
		echo "sfetch not found. Install from: https://github.com/3leaps/sfetch"; \
		exit 1; \
	fi
	@# Install goneat (skip if present, unless FORCE=1)
	@if [ "$(FORCE)" = "1" ]; then \
		rm -f "$(BINDIR)/goneat"; \
	fi
	@if [ "$(FORCE)" = "1" ] || ! command -v goneat >/dev/null 2>&1; then \
		echo "Installing goneat $(GONEAT_VERSION)..."; \
		sfetch --repo fulmenhq/goneat --tag $(GONEAT_VERSION) --dest-dir "$(BINDIR)"; \
	else \
		echo "goneat already installed, skipping (use FORCE=1 to reinstall)"; \
	fi
	@echo "goneat: $$(goneat version 2>&1 | head -n1)"
	@# Install foundation tools
	@goneat doctor tools --scope foundation --install --yes

bootstrap-force:  ## Force reinstall all tools
	@$(MAKE) bootstrap FORCE=1
```

## Anti-Patterns to Avoid

### Unconditional Install

```makefile
# BAD: Always installs, even if newer version exists
bootstrap:
	sfetch --repo fulmenhq/goneat --tag $(GONEAT_VERSION) --dest-dir "$(BINDIR)"
```

### Hardcoded Old Version

```makefile
# BAD: Pins to old version with no override option
GONEAT_VERSION := v0.3.21
```

### No Force Option

```makefile
# BAD: No way to force reinstall when needed
bootstrap:
	@if ! command -v goneat >/dev/null 2>&1; then \
		# install...
	fi
```

## Version Selection Strategy

| Scenario | Recommendation |
|----------|----------------|
| **New project** | Use `latest` or current stable (e.g., `v0.5.1`) |
| **Existing project** | Use `?=` with minimum required version |
| **CI/CD pipelines** | Pin to specific version for reproducibility |
| **Local development** | Let existing install be used |

## Troubleshooting

### "goneat version mismatch"

If you see unexpected versions:

```bash
# Check which goneat is being used
which goneat
goneat version

# Force reinstall from a specific project
make bootstrap-force
```

### "bootstrap keeps reinstalling"

Check that your Makefile has the skip-if-present logic:

```bash
grep -A10 "bootstrap:" Makefile | grep "command -v goneat"
```

If missing, add the conditional check shown above.

## Related

- [End-to-End Setup](../end-to-end-setup.md) - Full CI/CD integration guide
- [Doctor Tools](../commands/doctor.md) - Tool installation and management
