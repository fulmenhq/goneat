# Goneat Release Signing

**Status**: Active (Manual Signing - v0.3.3+)
**Authority**: FulmenHQ Security Team
**Last Updated**: 2025-10-28

## Overview

All Goneat release artifacts are cryptographically signed using PGP/GPG to ensure authenticity and integrity. This document describes the signing process, key management, and verification procedures for both maintainers and users.

## Trust Model

Goneat releases are signed with the **FulmenHQ Release Signing Key** (Manual Signing Subkey):

```
Key ID: 448A539320A397AF
Fingerprint: 0A5FBDD451D444B8A5551D8C448A539320A397AF
Email: security@fulmenhq.dev
Type: ed25519
Created: 2025-10-31
```

**Key Publication Channels**:

- This documentation (authoritative)
- GitHub repository README
- Release notes for each version
- FulmenHQ website (when available)

### Extracting Key Information for Documentation

The FulmenHQ signing key has **multiple subkeys** for different purposes:

- **Primary Key**: Certification only (offline, for signing subkeys and revocation)
- **Signing Subkey (Manual)**: YubiKey-backed, used for manual artifact signing (**document this one**)
- **Signing Subkey (CI)**: Staged for future CI automation (not active in v0.3.3)

**Which key to document**: For v0.3.3+, document the **manual signing subkey** fingerprint, as this is the key that actually signs release artifacts.

```bash
# List all keys and subkeys with full details
gpg --list-keys --keyid-format LONG --with-subkey-fingerprints security@fulmenhq.dev

# Get fingerprints for all keys/subkeys
gpg --fingerprint security@fulmenhq.dev

# Export public key (includes all subkeys)
gpg --armor --export security@fulmenhq.dev > fulmenhq-release-signing-key.asc
```

**Example output**:

```
pub   ed25519/1A2B3C4D5E6F7G8H 2025-10-28 [C]
      1A2B 3C4D 5E6F 7G8H 9I0J  1K2L 3M4N 5O6P 7Q8R 9S0T
uid                 [ultimate] FulmenHQ Release Signing <security@fulmenhq.dev>
sub   ed25519/AAAA1111BBBB2222 2025-10-28 [S]  ← Manual signing subkey (YubiKey)
      AAAA 1111 BBBB 2222 CCCC  3333 DDDD 4444 EEEE 5555
sub   ed25519/FFFF9999GGGG8888 2025-10-28 [S]  ← CI signing subkey (staged, not active)
      FFFF 9999 GGGG 8888 HHHH  7777 IIII 6666 JJJJ 5555
sub   cv25519/KKKK0000LLLL9999 2025-10-28 [E]  ← Encryption subkey
      KKKK 0000 LLLL 9999 MMMM  8888 NNNN 7777 OOOO 6666
```

**Identifying subkeys**:

- `[C]` = Certification (primary key only)
- `[S]` = Signing capability
- `[E]` = Encryption capability
- Manual signing subkey: First `[S]` subkey (YubiKey-backed)
- CI signing subkey: Second `[S]` subkey (if present, staged for future)

**Note**: The convention that "first `[S]` subkey = manual signing" is specific to FulmenHQ's key management process. Other organizations may structure their signing keys differently. This is not a GPG-enforced rule, but our documented operational practice.

**What to document in the Trust Model section**:

- **Key ID**: Manual signing subkey ID (e.g., `AAAA1111BBBB2222`)
- **Fingerprint**: Manual signing subkey fingerprint (40-char hex, spaces removed)
- **Type**: Subkey algorithm (e.g., `ed25519`)
- **Created**: Subkey creation date

**To get just the manual signing subkey fingerprint**:

```bash
# Show all fingerprints
gpg --fingerprint security@fulmenhq.dev

# Look for the first [S] subkey fingerprint (not [C], not [E])
# This is your manual signing subkey
```

**Safe to publish** (these are public identifiers, not secrets):

- ✅ Key ID (short and long form) - for primary key and all subkeys
- ✅ Subkey IDs (e.g., 448A539320A397AF, 45342521E9536A31) - completely safe
- ✅ Full fingerprint (40-character hex) - for primary key and all subkeys
- ✅ Algorithm and key size
- ✅ Creation date
- ✅ Expiration date (if set)
- ✅ User ID (name and email)
- ✅ Public key file (.asc)

**Never publish**:

- ❌ Private key material
- ❌ Passphrase or PIN
- ❌ YubiKey serial numbers
- ❌ Backup locations
- ❌ Recovery phrases

## For Users: Verifying Signatures

### Prerequisites

Install GPG on your system:

```bash
# macOS
brew install gnupg

# Ubuntu/Debian
sudo apt-get install gnupg2

# Windows (via Scoop)
scoop install gpg

# Windows (via Chocolatey)
choco install gnupg
```

### Verification Steps

1. **Import the FulmenHQ public key**:

```bash
# Download from GitHub (replace with actual URL after key generation)
curl -L https://github.com/fulmenhq/goneat/releases/download/v0.3.3/fulmenhq-release-signing-key.asc | gpg --import

# Verify fingerprint matches documentation
gpg --fingerprint security@fulmenhq.dev
```

2. **Download release artifacts**:

```bash
# Example for macOS ARM64
curl -LO https://github.com/fulmenhq/goneat/releases/download/v0.3.3/goneat-darwin-arm64.tar.gz
curl -LO https://github.com/fulmenhq/goneat/releases/download/v0.3.3/goneat-darwin-arm64.tar.gz.asc
```

3. **Verify the signature**:

```bash
gpg --verify goneat-darwin-arm64.tar.gz.asc goneat-darwin-arm64.tar.gz
```

**Expected output**:

```
gpg: Signature made [timestamp]
gpg:                using EDDSA key [fingerprint]
gpg: Good signature from "FulmenHQ Release Signing <security@fulmenhq.dev>"
```

**Warning signs** (DO NOT proceed if you see):

```
gpg: BAD signature from "FulmenHQ Release Signing <security@fulmenhq.dev>"
gpg: Can't check signature: No public key
```

### Verification Automation

Add to your CI/CD or installation scripts:

```bash
#!/bin/bash
set -e

RELEASE_VERSION="v0.3.3"
PLATFORM="darwin-arm64"
ARTIFACT="goneat-${PLATFORM}.tar.gz"

# Import key (idempotent)
curl -L "https://github.com/fulmenhq/goneat/releases/download/${RELEASE_VERSION}/fulmenhq-release-signing-key.asc" | gpg --import

# Download and verify
curl -LO "https://github.com/fulmenhq/goneat/releases/download/${RELEASE_VERSION}/${ARTIFACT}"
curl -LO "https://github.com/fulmenhq/goneat/releases/download/${RELEASE_VERSION}/${ARTIFACT}.asc"

if gpg --verify "${ARTIFACT}.asc" "${ARTIFACT}"; then
    echo "✅ Signature valid"
    tar -xzf "${ARTIFACT}"
else
    echo "❌ Signature verification failed"
    exit 1
fi
```

## For Maintainers: Manual Signing Process

### Prerequisites

**Required**:

- YubiKey with FulmenHQ signing subkey configured
- GPG 2.2+ installed
- Access to release artifacts (post-build)

**Setup** (one-time):

```bash
# Verify YubiKey is connected and recognized
gpg --card-status

# Verify signing subkey is available
gpg --list-secret-keys security@fulmenhq.dev
```

### Signing Workflow (v0.3.3 Manual Process)

**⚠️ Critical Timing**: Perform Steps 1-4 (build, package, sign, verify) **BEFORE pushing** to validate the workflow works. Steps 5-6 happen **AFTER tagging** the release.

#### Pre-Push Validation (Before git push)

**Step 1: Build and Package**

```bash
# Ensure clean build
make clean

# Build raw binaries (outputs to bin/)
make build-all

# Package binaries into release artifacts (outputs to dist/release/)
make package
```

**What happens**:

- `make clean`: Removes `dist/` and `bin/` directories
- `make build-all`: Creates raw binaries in `bin/` (goneat-{os}-{arch})
- `make package`: Packages binaries from `bin/` into `dist/release/` (.tar.gz/.zip archives + SHA256SUMS)

**Alternative**: Use `make release-build` to run both `build-all` and `package` in one command

**Step 2: Generate Checksums**

```bash
cd dist/release

# Generate SHA256 checksums for all artifacts
sha256sum *.tar.gz *.zip > SHA256SUMS

# Verify checksums file
cat SHA256SUMS
```

**Step 3: Sign All Artifacts**

Before signing, verify you're using the correct subkey:

```bash
# List all subkeys with long key IDs to confirm which one to use
gpg --list-keys --keyid-format LONG security@fulmenhq.dev

# Look for the subkeys (lines starting with 'sub'):
# sub   ed25519/448A539320A397AF ... [S]  ← Manual signing (first [S] subkey)
# sub   ed25519/45342521E9536A31 ... [S]  ← CI signing (second [S] subkey)
```

Now sign all artifacts with the **manual signing subkey**:

```bash
# Sign each artifact (detached signature)
# Creates ${file}.asc for each artifact
# IMPORTANT: Use explicit subkey ID with '!' to force use of manual signing subkey
# 448A539320A397AF = Manual signing subkey (YubiKey-backed, use for v0.3.3)
# 45342521E9536A31 = CI signing subkey (reserved for future automation)
for file in *.tar.gz *.zip SHA256SUMS; do
    echo "Signing $file..."
    gpg --detach-sign --armor --local-user 448A539320A397AF! "$file"
done

# Verify signatures were created
ls -lh *.asc

# Export public key for distribution (always fresh for each release)
gpg --armor --export security@fulmenhq.dev > fulmenhq-release-signing-key.asc

# Verify public key was exported correctly (should show PGP PUBLIC KEY BLOCK)
echo "Public key exported:"
head -5 fulmenhq-release-signing-key.asc
```

**Step 4: Verify Signatures Locally**

```bash
# Test verification before upload
for file in *.tar.gz *.zip SHA256SUMS; do
    echo "Verifying $file..."
    gpg --verify "${file}.asc" "$file" || exit 1
done

# Verify correct subkey was used (should show 448A539320A397AF for manual signing)
echo ""
echo "Checking which subkey was used for signing..."
gpg --verify SHA256SUMS.asc SHA256SUMS 2>&1 | grep "using.*key"

echo "✅ All signatures verified successfully"
echo "⚠️  Confirm the key ID above is 448A539320A397AF (manual signing subkey)"
```

**✅ If Steps 1-4 succeed**: Proceed with git push and tagging
**❌ If any step fails**: Fix issues before pushing

#### Post-Tag Release (After git push and git tag)

**Step 5: Upload to GitHub Release**

⏳ **IMPORTANT**: Wait for GitHub Actions to create the release first! After pushing the tag, GitHub Actions will build and create the release. Monitor the Actions tab until the release workflow completes.

**Option A: Using gh CLI** (Preferred - required for future CI automation)

```bash
# Upload all signed artifacts and checksums
gh release upload v0.3.3 dist/release/*.tar.gz
gh release upload v0.3.3 dist/release/*.zip
gh release upload v0.3.3 dist/release/*.asc
gh release upload v0.3.3 dist/release/SHA256SUMS

# Upload public key (first release only)
gh release upload v0.3.3 fulmenhq-release-signing-key.asc
```

**Option B: Using GitHub Web UI** (Alternative if gh CLI unavailable)

1. Navigate to: https://github.com/fulmenhq/goneat/releases
2. Find the v0.3.3 release (created by GitHub Actions)
3. Click "Edit release"
4. Drag and drop files from `dist/release/`:
   - All `.tar.gz` files
   - All `.zip` files
   - All `.asc` signature files
   - `SHA256SUMS` file
   - `fulmenhq-release-signing-key.asc` (first release only)
5. Click "Update release"

**Note**: gh CLI is recommended for repeatability and will be essential when automating this process in CI.

**Step 6: Document in Release Notes**

Add to release notes:

```markdown
## Signature Verification

All artifacts are cryptographically signed. Verify with:

\`\`\`bash

# Import public key

curl -L https://github.com/fulmenhq/goneat/releases/download/v0.3.3/fulmenhq-release-signing-key.asc | gpg --import

# Verify artifact

gpg --verify goneat-darwin-arm64.tar.gz.asc goneat-darwin-arm64.tar.gz
\`\`\`

Key fingerprint: [fingerprint here]
```

### Complete Release Sequence

**Timeline** for v0.3.3 manual signing workflow:

```
1. Commit changes locally
   ↓
2. Pre-push validation (Steps 1-4):
   - make clean (removes dist/ and bin/ directories)
   - make build-all (creates raw binaries in bin/)
   - ./scripts/package-artifacts.sh (packages bin/ to dist/release/)
   - Generate checksums and signatures (in dist/release/)
   - Verify all signatures locally
   - Signed artifacts now in dist/release/ (ready to upload later)
   ↓
3. If validation succeeds:
   - git push origin main
   - git tag -a v0.3.3 -m "..."
   - git push origin v0.3.3
   ↓
4. ⏳ WAIT for GitHub Actions:
   - Monitor Actions tab at https://github.com/fulmenhq/goneat/actions
   - Wait for release workflow to complete
   - GitHub Actions builds binaries and creates release
   ↓
5. Upload signed artifacts (Step 5):
   - Use gh CLI or GitHub UI to upload files from dist/release/
   - Upload all .tar.gz, .zip, .asc files and SHA256SUMS
   - Upload public key (first release only)
   ↓
6. Update release notes with verification instructions (Step 6)
```

**Why pre-push validation matters**:

- Catches build/packaging issues before pushing
- Validates YubiKey access and GPG configuration
- Ensures signatures are valid before tagging
- Prevents "oops, can't sign" situations after public tag
- Aligns with prepush quality gates

## Key Management

### Key Structure

**Primary Key** (offline, encrypted backup):

- **Purpose**: Certification only (signs subkeys, revocation cert)
- **Location**: Encrypted backup in dual secure locations
- **Access**: Project lead only, emergency use

**Signing Subkey (Manual)** (YubiKey):

- **Key ID**: ed25519/448A539320A397AF
- **Purpose**: Release artifact signing (manual)
- **Location**: YubiKey hardware token
- **Access**: Release manager(s)
- **Usage**: v0.3.3+ manual signing workflow (use `--local-user 448A539320A397AF!`)

**Signing Subkey (CI)** (staged, not active):

- **Key ID**: ed25519/45342521E9536A31
- **Purpose**: Automated CI signing (future)
- **Location**: Encrypted in secure storage (not in CI yet)
- **Access**: None (staged for post-v0.3.3 automation)
- **Activation**: Deferred to follow-up milestone (will use `--local-user 45342521E9536A31!`)

### Key Rotation

**Schedule**: Annual renewal

**Process**:

1. Generate new signing subkey
2. Sign with primary key (offline ceremony)
3. Distribute updated public key
4. Document in release notes
5. Maintain old subkey for 90-day overlap

**Emergency Revocation**:

1. Load revocation certificate
2. Publish to keyservers
3. Update all documentation
4. Generate new keypair
5. Re-sign recent releases

### Custodianship

**Primary Key Custodian**: @3leapsdave
**YubiKey Custodians**: Release managers (documented separately)
**Backup Locations**: Documented in internal runbook (not public)

## CI Automation Setup

### Preparing CI Signing Subkey with Separate Passphrase

**Security Goal**: Use a different passphrase for the CI subkey than your primary key passphrase. This limits exposure if the CI passphrase is ever compromised.

#### Step 1: Export CI Subkey to Temporary Keyring

```bash
# Create isolated temporary keyring
mkdir -p /tmp/gpg-ci-setup
chmod 700 /tmp/gpg-ci-setup

# Export ONLY the CI signing subkey (45342521E9536A31)
gpg --armor --export-secret-subkeys 45342521E9536A31! > /tmp/gpg-ci-setup/ci-subkey-original.asc

# Also export the public key (needed for keyring)
gpg --armor --export security@fulmenhq.dev > /tmp/gpg-ci-setup/public-key.asc
```

#### Step 2: Import to Temporary Keyring

```bash
# Import to isolated keyring
GNUPGHOME=/tmp/gpg-ci-setup gpg --import /tmp/gpg-ci-setup/public-key.asc
GNUPGHOME=/tmp/gpg-ci-setup gpg --import /tmp/gpg-ci-setup/ci-subkey-original.asc

# Verify what we have (should show '#' indicating primary secret is NOT present)
GNUPGHOME=/tmp/gpg-ci-setup gpg --list-secret-keys
```

**Expected output**:

```
sec#  ed25519 2025-10-31 [SC]
      E33FE149923314AFF3BC64723E0AD9999CF3D418
uid           [ unknown] FulmenHQ Release Signing <security@fulmenhq.dev>
ssb#  cv25519 2025-10-31 [E]     ← Encryption subkey (not present)
ssb#  ed25519 2025-10-31 [S]     ← Manual signing subkey (not present)
ssb   ed25519 2025-10-31 [S]     ← CI signing subkey (PRESENT - no #)
```

Note: The `#` after `sec` and `ssb` means those secret keys are NOT present. Only the CI signing subkey is available.

#### Step 3: Change CI Subkey Passphrase

```bash
# Edit the key to change passphrase
GNUPGHOME=/tmp/gpg-ci-setup gpg --edit-key 45342521E9536A31

# At the gpg> prompt, select the CI subkey:
gpg> key 1
# (The CI subkey should show as selected with an asterisk: ssb*)

# Change the passphrase:
gpg> passwd
# Enter CURRENT passphrase (your primary key passphrase)
# Enter NEW passphrase (create a strong, different passphrase for CI only)
# Confirm NEW passphrase

# Save changes:
gpg> save
```

#### Step 4: Export with New Passphrase

```bash
# Export the subkey with its new CI-specific passphrase
GNUPGHOME=/tmp/gpg-ci-setup gpg --armor --export-secret-subkeys 45342521E9536A31! > /tmp/gpg-ci-setup/ci-subkey-new-passphrase.asc

# Base64 encode for GitHub Secrets
cat /tmp/gpg-ci-setup/ci-subkey-new-passphrase.asc | base64 > /tmp/gpg-ci-setup/ci-subkey-base64.txt

# Display for copying to GitHub Secrets
echo "Copy this for GPG_SIGNING_KEY_YYYYMMDD:"
cat /tmp/gpg-ci-setup/ci-subkey-base64.txt
```

#### Step 5: Store in GitHub Secrets

Navigate to GitHub Organization Settings → Secrets and variables → Actions:
`https://github.com/organizations/fulmenhq/settings/secrets/actions`

**Date-Stamped Secret Names** (recommended for rotation tracking):

```
Secret 1:
Name: GPG_SIGNING_KEY_20251031
Value: <paste contents of ci-subkey-base64.txt>

Secret 2:
Name: GPG_SIGNING_PASSPHRASE_20251031
Value: <your NEW CI-only passphrase>
```

**Why date stamps?**

- Clear rotation tracking
- Can stage new keys before old expire
- Audit trail for key lifecycle

**Organization vs Repository Secrets**:

- ✅ **Organization secrets**: Recommended if same maintainers manage multiple repos
- ✅ Can scope to specific repositories (e.g., only `goneat`)
- ❌ **Repository secrets**: Use if this key is truly goneat-specific only
- ⚠️ **Security note**: Organization-wide secrets require consistent maintainer trust across all scoped repos

**Configuration for Organization Secrets**:

1. Create as organization secret
2. Set "Repository access" → "Selected repositories"
3. Choose: `goneat` (add others later if needed)

#### Step 6: Secure Cleanup

```bash
# macOS: Use rm -P for secure deletion (shred not available by default)
rm -P /tmp/gpg-ci-setup/ci-subkey-original.asc
rm -P /tmp/gpg-ci-setup/ci-subkey-new-passphrase.asc
rm -P /tmp/gpg-ci-setup/ci-subkey-base64.txt
rm -P /tmp/gpg-ci-setup/public-key.asc

# Remove temporary keyring
rm -rf /tmp/gpg-ci-setup

# Verify cleanup
ls -la /tmp/gpg-ci-setup  # Should show "No such file or directory"
```

**Note**: Linux users can use `shred -u` instead of `rm -P`.

#### Step 7: Test Locally (Recommended)

```bash
# Verify the exported key works with new passphrase
mkdir -p /tmp/gpg-test
chmod 700 /tmp/gpg-test

# Get the base64 key you just created
BASE64_KEY=$(cat /tmp/gpg-ci-setup/ci-subkey-base64.txt)

# Import and test signing
echo "$BASE64_KEY" | base64 -d | GNUPGHOME=/tmp/gpg-test gpg --import --batch --yes

# Test signing with the new passphrase
echo "test" > /tmp/test.txt
echo "YOUR_NEW_CI_PASSPHRASE" | \
  GNUPGHOME=/tmp/gpg-test gpg --batch --yes \
  --pinentry-mode loopback \
  --passphrase-fd 0 \
  --detach-sign --armor \
  --local-user 45342521E9536A31! \
  /tmp/test.txt

# Should create test.txt.asc without errors
ls -la /tmp/test.txt.asc  # Should exist

# Cleanup test
rm -rf /tmp/gpg-test /tmp/test.txt*
```

### GitHub Actions Workflow

**Example workflow using date-stamped secrets**:

```yaml
name: Release with Signing
on:
  push:
    tags:
      - "v*"

jobs:
  release:
    runs-on: ubuntu-latest
    permissions:
      contents: write

    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.23"

      - name: Import GPG signing key
        run: |
          # Import CI signing subkey
          echo "${{ secrets.GPG_SIGNING_KEY_20251031 }}" | base64 -d | gpg --import --batch --yes

          # Verify key was imported
          gpg --list-secret-keys 45342521E9536A31

      - name: Build release artifacts
        run: |
          make clean
          make build-all
          make package

      - name: Sign artifacts with CI subkey
        run: |
          cd dist/release

          # Sign each artifact with CI subkey
          for file in *.tar.gz *.zip SHA256SUMS; do
            echo "Signing $file..."
            echo "${{ secrets.GPG_SIGNING_PASSPHRASE_20251031 }}" | \
              gpg --batch --yes \
              --pinentry-mode loopback \
              --passphrase-fd 0 \
              --detach-sign --armor \
              --local-user 45342521E9536A31! \
              "$file"
          done

      - name: Verify signatures
        run: |
          cd dist/release

          # Verify all signatures
          for file in *.tar.gz *.zip SHA256SUMS; do
            echo "Verifying $file..."
            gpg --verify "${file}.asc" "$file" || exit 1
          done

          echo "✅ All signatures verified successfully"

      - name: Upload artifacts to release
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          # Upload signed artifacts
          gh release upload ${{ github.ref_name }} dist/release/*.tar.gz
          gh release upload ${{ github.ref_name }} dist/release/*.zip
          gh release upload ${{ github.ref_name }} dist/release/*.asc
          gh release upload ${{ github.ref_name }} dist/release/SHA256SUMS

      - name: Clean up GPG keyring
        if: always()
        run: |
          # Remove imported key from CI environment
          gpg --batch --yes --delete-secret-keys 45342521E9536A31 || true
```

### Security Benefits

**Defense in Depth**:

- ✅ Primary key passphrase: Only you know, protects certification
- ✅ CI subkey passphrase: Different, stored in GitHub, protects signing only
- ✅ If CI passphrase leaks → only signing capability compromised → revoke CI subkey
- ✅ Primary key and manual subkey remain secure

**Principle of Least Privilege**:

- ✅ CI only has signing capability (not certification)
- ✅ Primary key never leaves secure offline storage
- ✅ Subkey independently revocable

**Key Rotation**:

- ✅ Date-stamped secrets make rotation tracking clear
- ✅ Can stage new keys before old ones expire
- ✅ Clear audit trail for compliance

### Current Status (v0.3.3)

**Implemented**:

- ✅ GPG tooling installed in CI workflow
- ✅ Manual signing process documented and tested
- ✅ Verification procedures established
- ✅ CI subkey preparation process documented

**Ready for Implementation** (v0.3.4+):

- ⏳ GitHub Actions workflow with automated signing
- ⏳ Organization secrets configured
- ⏳ Automated signature verification gates

### Roadmap

**Phase 1** (v0.3.3): Manual signing + CI prerequisites ✅
**Phase 2** (v0.3.4+): Automated CI signing with separate passphrase
**Phase 3** (v0.4.0+): Automated verification gates in CI/CD
**Phase 4** (v0.5.0+): Sigstore integration, SLSA provenance

## Troubleshooting

### "No secret key" Error

```bash
# Check if YubiKey is connected
gpg --card-status

# Verify signing key is accessible
gpg --list-secret-keys security@fulmenhq.dev
```

### "Bad signature" After Signing

```bash
# Verify correct key was used
gpg --verify file.asc file | grep "using.*key"

# Check for file corruption
sha256sum file
```

### YubiKey Not Recognized

```bash
# Restart GPG agent
gpgconf --kill gpg-agent
gpgconf --launch gpg-agent

# Check card status
gpg --card-status
```

## Security Considerations

### For Users

- **Always verify signatures** before extracting/installing
- **Match fingerprints** with multiple sources (docs, website, release notes)
- **Report suspicious signatures** to security@fulmenhq.dev immediately

### For Maintainers

- **Never sign on untrusted machines**
- **Verify build artifacts** before signing
- **Use YubiKey PIN protection**
- **Review signature verification** before release publication
- **Document any signing issues** in release notes

## References

- **Feature Brief**: `.plans/active/v0.3.3/release-signing-feature-brief.md`
- **GnuPG Documentation**: https://gnupg.org/documentation/
- **YubiKey GPG Guide**: https://github.com/drduh/YubiKey-Guide
- **NIST SP 800-57**: Key Management Recommendations

## Changelog

- **2025-10-28**: Initial documentation for v0.3.3 manual signing process
- **[Future]**: Document CI automation implementation
- **[Future]**: Add Sigstore integration details

---

**For security concerns or key compromise reports**: security@fulmenhq.dev
**For signing questions**: See GitHub Discussions or file an issue
