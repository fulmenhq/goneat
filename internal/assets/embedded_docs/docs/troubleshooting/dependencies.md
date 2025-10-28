# Dependencies Troubleshooting Guide

**Common issues and solutions for goneat's dependency protection features**

## Quick Diagnosis

### Run Diagnostic Command

```bash
# Check dependencies status
goneat dependencies --licenses --cooling --format json | jq .

# Test individual features
goneat dependencies --licenses   # License check only
goneat dependencies --cooling    # Cooling check only
goneat dependencies --sbom       # SBOM generation only
```

### Check Configuration

```bash
# Validate config file
cat .goneat/dependencies.yaml

# Check if policy is loaded
goneat dependencies --licenses --format json | jq '.policy_loaded'
```

---

## License Compliance Issues

### Issue: "License not detected" for known package

**Symptoms:**
```
WARN: License not detected for github.com/example/pkg
```

**Causes:**
1. Package doesn't have LICENSE file
2. License file has non-standard name
3. go-licenses cannot parse the license

**Solutions:**

```bash
# 1. Verify package actually has a license
cd vendor/github.com/example/pkg
ls -la | grep -i license

# 2. Check go-licenses directly
go-licenses csv github.com/example/pkg

# 3. Add manual exception
cat >> .goneat/dependencies.yaml << EOF
licenses:
  exceptions:
    - package: "github.com/example/pkg"
      license: "MIT"  # After manual verification
      reason: "License manually verified from GitHub"
      approved_by: "@yourname"
      approved_date: "$(date +%Y-%m-%d)"
EOF
```

### Issue: False positive - package has acceptable license

**Symptoms:**
```
ERROR: Package github.com/spf13/cobra uses forbidden license: Apache-2.0
```

**Cause:** Your policy forbids Apache-2.0, but you want to allow it.

**Solution:**

```yaml
# Edit .goneat/dependencies.yaml
licenses:
  # Remove Apache-2.0 from forbidden list, or:
  allowed:
    - MIT
    - Apache-2.0  # Add to allowed list
    - BSD-3-Clause
```

### Issue: "go-licenses not found"

**Symptoms:**
```
ERROR: go-licenses binary not found
ERROR: Install with: go install github.com/google/go-licenses@latest
```

**Solution:**

```bash
# Option 1: Auto-install via doctor
goneat doctor tools --scope dependencies --install --yes

# Option 2: Manual install
go install github.com/google/go-licenses@latest

# Option 3: System package manager
# macOS:
brew install go-licenses

# Verify:
which go-licenses
go-licenses version
```

---

## Package Cooling Issues

### Issue: All packages fail cooling policy

**Symptoms:**
```
ERROR: Package github.com/spf13/cobra v1.8.0: 0 days old (< 7 days)
ERROR: Package gopkg.in/yaml.v3 v3.0.1: 0 days old (< 7 days)
ERROR: All 50 dependencies fail cooling policy
```

**Causes:**
1. Registry API returning invalid data
2. Network blocking registry access
3. System clock is wrong
4. Registry API rate limit hit

**Solutions:**

```bash
# 1. Check registry API manually
curl -v https://proxy.golang.org/github.com/spf13/cobra/@v/v1.8.0.info
# Should return: {"Version":"v1.8.0","Time":"2024-01-15T10:30:00Z"}

# 2. Verify system clock
date
# If wrong: sudo ntpdate -s time.apple.com  # macOS
#          sudo ntpd -qg                     # Linux

# 3. Check network/proxy
curl -v https://proxy.golang.org/
# If fails: export HTTPS_PROXY=http://proxy.corp.com:8080

# 4. Clear goneat cache
rm -rf ~/.goneat/cache/registry/
goneat dependencies --cooling

# 5. Temporary: Use alert mode while investigating
# Edit .goneat/dependencies.yaml:
cooling:
  alert_only: true  # Warn only, don't fail
```

### Issue: Network timeout waiting for registry

**Symptoms:**
```
ERROR: Registry API timeout for package github.com/example/pkg after 30s
ERROR: Cooling policy check failed
EXIT CODE: 1
```

**Causes:**
1. Network latency to package registry
2. Firewall blocking registry access
3. Registry temporarily down
4. Too many concurrent requests

**Solutions:**

```bash
# 1. Increase timeout in hooks
# Edit .goneat/hooks.yaml:
hooks:
  pre-push:
    - command: dependencies
      args: ["--cooling"]
      timeout: "90s"  # Increased from 45s

# 2. Configure proxy if needed
export HTTPS_PROXY=http://proxy.corp.com:8080
export HTTP_PROXY=http://proxy.corp.com:8080

# 3. Check registry status
curl -w "%{time_total}s\n" https://registry.npmjs.org/
curl -w "%{time_total}s\n" https://proxy.golang.org/

# 4. Add temporary exception
# Edit .goneat/dependencies.yaml:
cooling:
  exceptions:
    - pattern: "github.com/problematic/*"
      until: "2025-11-15"
      reason: "Network issue, packages manually reviewed"
      approved_by: "@yourname"
```

### Issue: Package blocked but manual check shows it's old

**Symptoms:**
```
ERROR: Package github.com/well-known/lib v2.0.0: 2 days old (< 7 days)
```

But when you check:
```bash
curl https://proxy.golang.org/github.com/well-known/lib/@v/v2.0.0.info
# Returns: {"Time":"2024-06-15T10:00:00Z"}  # Months ago!
```

**Causes:**
1. Stale cache with wrong metadata
2. Registry API caching issue
3. Version re-published with new timestamp

**Solutions:**

```bash
# 1. Clear goneat cache
rm -rf ~/.goneat/cache/registry/
goneat dependencies --cooling

# 2. If still fails, verify with multiple sources
curl https://pkg.go.dev/github.com/well-known/lib@v2.0.0
# Check "Published" date on webpage

# 3. Add documented exception
# Edit .goneat/dependencies.yaml:
cooling:
  exceptions:
    - module: "github.com/well-known/lib"
      reason: "Package actually published 2024-06-15, registry API caching issue"
      approved_by: "@yourname"
      approved_date: "2025-10-28"
      verified_source: "https://pkg.go.dev/..."
```

### Issue: "Conservative fallback" warnings in logs

**Symptoms:**
```
WARN: Registry API failed for package X: rate limit exceeded
INFO: Using conservative fallback: assuming package age = 365 days
INFO: Package marked with age_unknown=true
```

**Explanation:** Not an error! This is expected behavior when registry APIs fail.

**What happens:**
- goneat assumes package is 365 days old (passes cooling)
- Build continues successfully
- Dependency is flagged for manual review in report

**Actions:**

```bash
# 1. Check which packages had fallback
goneat dependencies --cooling --format json | \
  jq '.dependencies[] | select(.age_unknown == true)'

# 2. Review flagged packages manually
# Visit package registry and verify age

# 3. Add explicit exceptions if verified safe
# Edit .goneat/dependencies.yaml as needed

# 4. If many fallbacks, check registry API health
curl -I https://proxy.golang.org/
curl -I https://registry.npmjs.org/
```

### Issue: Rate limit errors from package registry

**Symptoms:**
```
ERROR: npm registry rate limit exceeded: 429 Too Many Requests
ERROR: Retry after: 60 seconds
```

**Solutions:**

```bash
# 1. Wait and retry (rate limits usually reset quickly)
sleep 60
goneat dependencies --cooling

# 2. Use authenticated registry access (npm)
npm login
# Creates ~/.npmrc with auth token

# 3. Configure corporate registry mirror
# Edit .goneat/dependencies.yaml:
# registry_mirrors:
#   npm: "https://npm.corp.com/"
#   pypi: "https://pypi.corp.com/simple/"

# 4. Reduce concurrent requests (future feature)
# Currently: goneat uses connection pooling to minimize requests
```

---

## SBOM Generation Issues

### Issue: "syft not found"

**Symptoms:**
```
ERROR: syft binary not found
ERROR: SBOM generation requires Syft
INFO: Install with: goneat doctor tools --scope sbom --install --yes
```

**Solution:**

```bash
# Option 1: Auto-install (recommended)
goneat doctor tools --scope sbom --install --yes

# Option 2: Manual install
# macOS:
brew install syft

# Linux:
curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b /usr/local/bin

# Verify:
syft version
goneat dependencies --sbom  # Should work now
```

### Issue: SBOM generation fails with "invalid output"

**Symptoms:**
```
ERROR: Syft produced invalid CycloneDX output
ERROR: Failed to parse SBOM: unexpected JSON structure
```

**Causes:**
1. Syft version too old or too new
2. Corrupted syft installation
3. Project structure confusing syft

**Solutions:**

```bash
# 1. Check syft version
syft version
# Recommended: v0.100.0 or later

# 2. Reinstall syft
goneat doctor tools --scope sbom --install --yes --force

# 3. Run syft directly to see error
syft dir:. -o cyclonedx-json

# 4. Try different SBOM output location
goneat dependencies --sbom --sbom-output /tmp/test.json

# 5. Check for project-specific issues
# Some projects with unusual structures may confuse syft
# Try running from different directory or on specific module
```

### Issue: SBOM file not found by assessment

**Symptoms:**
```
INFO: SBOM metadata: not_generated
INFO: Run 'goneat dependencies --sbom' to generate SBOM
```

**Cause:** SBOM doesn't exist or is in wrong location.

**Solution:**

```bash
# 1. Generate SBOM
goneat dependencies --sbom

# 2. Verify it exists
ls -la sbom/
# Should see: goneat-<timestamp>.cdx.json

# 3. Check default location
cat sbom/goneat-latest.cdx.json | jq '.bomFormat'
# Should output: "CycloneDX"

# 4. Run assessment again
goneat assess --categories dependencies
# Should now show: SBOM metadata: available
```

---

## Configuration Issues

### Issue: Policy file not found or not loaded

**Symptoms:**
```
WARN: Policy file not found: .goneat/dependencies.yaml
INFO: Using default policy
```

**Solutions:**

```bash
# 1. Create policy file if missing
cp .goneat/dependencies.yaml.example .goneat/dependencies.yaml

# Or create from scratch:
cat > .goneat/dependencies.yaml << 'EOF'
version: v1
licenses:
  forbidden:
    - GPL-3.0
    - AGPL-3.0
cooling:
  enabled: true
  min_age_days: 7
EOF

# 2. Verify file location
ls -la .goneat/dependencies.yaml

# 3. Validate YAML syntax
goneat validate .goneat/dependencies.yaml

# 4. Check permissions
chmod 644 .goneat/dependencies.yaml
```

### Issue: Invalid YAML syntax in config

**Symptoms:**
```
ERROR: Failed to parse policy file: yaml: line 15: mapping values are not allowed in this context
```

**Solutions:**

```bash
# 1. Validate YAML
goneat validate .goneat/dependencies.yaml

# 2. Common YAML mistakes:
#    - Missing spaces after colons:  "key:value" should be "key: value"
#    - Tabs instead of spaces (use spaces only)
#    - Unbalanced quotes:  "reason: "foo  # Missing closing quote

# 3. Use a YAML linter
yamllint .goneat/dependencies.yaml

# 4. Start fresh if severely broken
mv .goneat/dependencies.yaml .goneat/dependencies.yaml.backup
# Create new config from documentation examples
```

---

## Hook Integration Issues

### Issue: Pre-commit hook hangs waiting for network

**Symptoms:**
- `git commit` takes 30+ seconds
- Hook shows "waiting for registry response..."

**Cause:** Cooling policy enabled in pre-commit (should be pre-push only).

**Solution:**

```yaml
# Edit .goneat/hooks.yaml

# WRONG - Don't use cooling in pre-commit:
hooks:
  pre-commit:
    - command: dependencies
      args: ["--licenses", "--cooling"]  # ❌ Remove --cooling

# CORRECT - Cooling in pre-push only:
hooks:
  pre-commit:
    - command: dependencies
      args: ["--licenses"]  # ✅ Offline only

  pre-push:
    - command: dependencies
      args: ["--licenses", "--cooling"]  # ✅ Network available
```

### Issue: Hook fails but manual command succeeds

**Symptoms:**
```
# This works:
$ goneat dependencies --licenses --cooling
✅ All checks passed

# But hook fails:
$ git push
❌ Dependencies check failed
```

**Causes:**
1. Different working directory in hook
2. Environment variables not set in hook context
3. PATH not including goneat binary

**Solutions:**

```bash
# 1. Check hook execution environment
cat .git/hooks/pre-push
# Look for: cd to wrong directory, missing PATH

# 2. Regenerate hooks
goneat hooks generate
goneat hooks install

# 3. Test hook directly
./.git/hooks/pre-push

# 4. Add debugging to hook
# Edit .git/hooks/pre-push, add near top:
echo "PWD: $(pwd)" >&2
echo "PATH: $PATH" >&2
which goneat >&2
```

---

## Performance Issues

### Issue: Dependency checks are very slow

**Symptoms:**
- Cooling checks take > 30 seconds
- Hooks timeout frequently

**Solutions:**

```bash
# 1. Check if cache is working
ls -la ~/.goneat/cache/registry/
# Should see cached files with recent timestamps

# 2. Clear and rebuild cache
rm -rf ~/.goneat/cache/registry/
time goneat dependencies --cooling  # First run (slow)
time goneat dependencies --cooling  # Second run (should be fast)

# 3. Reduce concurrent requests (if causing issues)
# Currently automatic - future flag: --max-concurrent-requests

# 4. Increase timeout in hooks
# Edit .goneat/hooks.yaml:
hooks:
  pre-push:
    - command: dependencies
      timeout: "90s"  # Increased

# 5. Consider moving to CI only
# Remove from hooks, add to GitHub Actions/GitLab CI
```

---

## Getting Help

### Enable Debug Logging

```bash
# Run with debug output
GONEAT_LOG_LEVEL=debug goneat dependencies --licenses --cooling 2>&1 | tee debug.log

# Check for specific errors
grep ERROR debug.log
grep WARN debug.log
```

### Collect Diagnostic Information

```bash
# System info
goneat envinfo > diagnostics.txt

# Dependency info
goneat dependencies --licenses --cooling --format json >> diagnostics.txt

# Tool versions
echo "=== Tool Versions ===" >> diagnostics.txt
goneat version >> diagnostics.txt
go version >> diagnostics.txt
go-licenses version >> diagnostics.txt 2>&1 || echo "go-licenses not found" >> diagnostics.txt
syft version >> diagnostics.txt 2>&1 || echo "syft not found" >> diagnostics.txt

# Config
echo "=== Configuration ===" >> diagnostics.txt
cat .goneat/dependencies.yaml >> diagnostics.txt

# Submit diagnostics.txt with bug report
```

### Report Issues

1. **Check existing issues:** https://github.com/fulmenhq/goneat/issues
2. **Create new issue** with:
   - `goneat version` output
   - `goneat envinfo` output
   - Full error message
   - Steps to reproduce
   - Diagnostics file (see above)

### Enterprise Support

For enterprise support: support@3leaps.net

---

## Quick Reference

### Common Commands

```bash
# Test license compliance only
goneat dependencies --licenses

# Test cooling policy only
goneat dependencies --cooling

# Full check
goneat dependencies --licenses --cooling --sbom

# Generate detailed report
goneat dependencies --licenses --cooling --format json --output report.json

# Assessment integration
goneat assess --categories dependencies
```

### Common Fixes

| Problem | Quick Fix |
|---------|-----------|
| All packages fail cooling | Clear cache: `rm -rf ~/.goneat/cache/registry/` |
| Network timeout | Increase timeout in hooks.yaml |
| syft not found | `goneat doctor tools --scope sbom --install --yes` |
| Hook hangs | Move cooling to pre-push only |
| Config errors | Validate: `goneat validate .goneat/dependencies.yaml` |
| Rate limits | Wait 60s and retry, or configure mirrors |

---

**Last Updated:** October 28, 2025  
**Status:** Active  
**Part of:** goneat v0.3.0 Dependency Protection Features
