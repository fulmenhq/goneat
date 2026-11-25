# Goneat Release Signing

**Status**: Active (Manual Signing - v0.3.3+)
**Authority**: FulmenHQ Security Team
**Last Updated**: 2026-11-23

## Overview

Goneat releases are authenticated via **signed checksum manifests**:

- `SHA256SUMS` and `SHA512SUMS` are generated over the published release archives.
- Maintainer publication requires both PGP and minisign signatures on both manifests.

Users verify the signature on the checksum manifest, then verify checksums for the downloaded archives.

This document describes the signing process, key management, and verification procedures for both maintainers and users.

## Trust Model

Goneat releases are signed with these keys:

```
PGP primary fingerprint:   0CACA49B3119B6BC12B2CA11B9B485F294B9FE07
PGP signing subkey:        DAB70DD758911B26BB45A08C3B75AC591449DEBE
Minisign key blob SHA-256: 3e5b64023230a308ee0876cb9d0693e44c3daac7afe38a064174e476b2b99cb1
```

**Where the keys are published**: the repository is authoritative.
`keys/expected-fingerprints.txt`, `keys/expected-fingerprints.ndjson`,
`docs/security/release-signing-keys.asc` and `docs/security/goneat-minisign.pub`
at each release tag are the keys for that release (see
[Release-key pin](#release-key-pin) and
[Verifying a release](README.md#verifying-a-release)). The copies attached to a
GitHub release (`fulmenhq-release-signing-key.asc`,
`fulmenhq-release-minisign.pub`) are those files byte for byte; verify them
against the repository, never on their own.

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

### Verification Steps (recommended)

Goneat publishes release archives plus signed checksum manifests.

1. **Download the checksum manifest, its signatures, and the keys from the repository**:

```bash
RELEASE_VERSION="vX.Y.Z"  # tags through v0.6.3 predate the committed keys
MINISIGN_PIN="docs/security/goneat-minisign.pub"  # docs/security/release-minisign.pub for v0.6.4-v0.6.9
BASE="https://github.com/fulmenhq/goneat/releases/download/${RELEASE_VERSION}"
KEYS="https://raw.githubusercontent.com/fulmenhq/goneat/${RELEASE_VERSION}"

curl -LO "${BASE}/SHA256SUMS"
curl -LO "${BASE}/SHA256SUMS.minisig"
curl -LO "${BASE}/SHA256SUMS.asc"
curl -L -o goneat-minisign.pub "${KEYS}/${MINISIGN_PIN}"
curl -L -o release-signing-keys.asc "${KEYS}/docs/security/release-signing-keys.asc"
curl -L -o expected-fingerprints.txt "${KEYS}/keys/expected-fingerprints.txt"
curl -L -o verify-release-pgp.sh "${KEYS}/scripts/verify-release-pgp.sh"
```

Compare `expected-fingerprints.txt` with the fingerprints in
[Trust Model](#trust-model) before trusting it. The most complete check is a
goneat checkout with decernor 0.1.8 or newer (`GONEAT_DECERNOR_BIN`) and
`make release-verify-signatures`, which reads every key from the tag, checks the
keys against their anchors with decernor, and checks all four signatures.

2. **Verify the checksum manifest signature**:

```bash
# Minisign (required verification path)
minisign -Vm SHA256SUMS -p goneat-minisign.pub

# PGP: throwaway keyring; requires a good, unexpired, unrevoked signature
# by the pinned signing subkey. Do not reduce this to grepping gpg output.
bash verify-release-pgp.sh expected-fingerprints.txt release-signing-keys.asc SHA256SUMS.asc SHA256SUMS
```

3. **Download the archive for your platform and verify checksums**:

```bash
ARCHIVE="goneat_${RELEASE_VERSION}_darwin_arm64.tar.gz"
curl -LO "${BASE}/${ARCHIVE}"

# Check the downloaded archive against its signed manifest entry
grep " ${ARCHIVE}\$" SHA256SUMS | shasum -a 256 --check -
```

**Expected output** (minisign, then the PGP check):

```
Signature and comment signature verified
Trusted comment: goneat vX.Y.Z SHA256SUMS
Good signature on SHA256SUMS by pinned signing subkey DAB70DD758911B26BB45A08C3B75AC591449DEBE
```

**Do not proceed** if either command exits nonzero or prints `error:`. The
signing key's user ID is `3leaps Infosec <infosec@3leaps.net>`; trust the
fingerprints in [Trust Model](#trust-model), not the user ID.

### Verification Automation

Add to your CI/CD or installation scripts:

```bash
#!/usr/bin/env bash
set -euo pipefail

RELEASE_VERSION="vX.Y.Z"
ARCHIVE="goneat_${RELEASE_VERSION}_linux_amd64.tar.gz"
BASE="https://github.com/fulmenhq/goneat/releases/download/${RELEASE_VERSION}"
REPO="https://raw.githubusercontent.com/fulmenhq/goneat/${RELEASE_VERSION}"

for name in SHA256SUMS SHA256SUMS.asc SHA256SUMS.minisig "${ARCHIVE}"; do
    curl -fsSLO "${BASE}/${name}"
done
# Keys and the checker come from the repository at the tag, not the release.
curl -fsSL -o goneat-minisign.pub "${REPO}/docs/security/goneat-minisign.pub"
curl -fsSL -o release-signing-keys.asc "${REPO}/docs/security/release-signing-keys.asc"
curl -fsSL -o expected-fingerprints.txt "${REPO}/keys/expected-fingerprints.txt"
curl -fsSL -o verify-release-pgp.sh "${REPO}/scripts/verify-release-pgp.sh"

minisign -Vm SHA256SUMS -p goneat-minisign.pub
bash verify-release-pgp.sh expected-fingerprints.txt release-signing-keys.asc SHA256SUMS.asc SHA256SUMS
grep " ${ARCHIVE}\$" SHA256SUMS | shasum -a 256 --check -
tar -xzf "${ARCHIVE}"
```

Pin the expected fingerprints in your own automation as well, so a change to
the repository's keys is reviewed on your side before it is trusted.

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

### Signing Workflow (checksum-manifest signing)

**⚠️ Critical Timing**: Download the original CI manifests, verify archive checksums,
sign, and verify all four signatures **before** uploading supplements. Never
regenerate CI-published manifests or replace their remote assets.

In practice, maintainers should follow `RELEASE_CHECKLIST.md` and use the Makefile targets:

- `make release-download` (download five CI-built archives and both original manifests into a new or empty directory)
- `make release-verify-checksums` (verify both original manifests without rewriting them)
- `make release-sign` (sign checksum manifests)
- `make release-verify-signatures` + `make release-verify-key` (verify signatures + key safety)

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

**Alternative**: Use `make release-build` to run both `build-all` and `package` in one command.

These local build steps are candidate checks only. The release flow signs the
_CI-built_ manifests covering what users actually download, not regenerated
manifests or newly built local archives.

**Step 2: Download and verify original CI checksums**

Prefer the Makefile target (it is aware of the CI artifact naming convention):

```bash
GONEAT_RELEASE_TAG=vX.Y.Z make release-download
GONEAT_RELEASE_TAG=vX.Y.Z make release-verify-checksums
```

**Step 3: Sign checksum manifests**

```bash
GONEAT_RELEASE_TAG=vX.Y.Z \
GONEAT_DECERNOR_BIN="<absolute path to decernor 0.1.8 or newer>" \
GONEAT_MINISIGN_KEY="<approved minisign secret key path>" \
GONEAT_PGP_KEY_ID="<approved-signing-fingerprint-or-identity>" \
GONEAT_GPG_HOMEDIR="<approved GPG homedir>" \
make release-sign
```

Before signing, verify you're using the correct subkey:

```bash
# List all subkeys with long key IDs to confirm which one to use
gpg --list-keys --keyid-format LONG security@fulmenhq.dev

# Look for the subkeys (lines starting with 'sub'):
# sub   ed25519/448A539320A397AF ... [S]  ← Manual signing (first [S] subkey)
# sub   ed25519/45342521E9536A31 ... [S]  ← CI signing (second [S] subkey)
```

Goneat signs the checksum manifests (not each archive individually). The signing helper:

- writes `SHA256SUMS.asc` / `SHA512SUMS.asc` (PGP, by the committed signing subkey)
- writes `SHA256SUMS.minisig` / `SHA512SUMS.minisig` (minisign)
- writes the pins committed at the tag, byte for byte, as
  `fulmenhq-release-signing-key.asc` and `fulmenhq-release-minisign.pub`
- verifies all four staged signatures against the committed pin before
  publishing any local outputs
- refuses to overwrite existing signature/key outputs or modify original archives/manifests

The approved GPG homedir and minisign secret key must reside outside the
release directory. No public key is taken from the environment. Signing
requires the operator's PGP primary to be the committed primary and signs with
the committed signing subkey; a minisign secret key that does not match the
committed minisign pin fails the staged verification, and nothing is kept.

**Step 4: Verify Signatures Locally**

```bash
GONEAT_RELEASE_TAG=vX.Y.Z make release-verify-signatures
GONEAT_RELEASE_TAG=vX.Y.Z make release-verify-key
```

Verification and upload use only the committed release-key pin (see
[Release-key pin](#release-key-pin)); they need `GONEAT_DECERNOR_BIN` and no
operator signing variables.
Missing tools, keys, manifests or any signature, incorrect signers and bad
signatures fail nonzero. There is no warning-only successful verification.

### Release-key pin

The public trust root for goneat releases is committed in the repository
(3leaps/crucible PDR-0008 layout):

| File | Content |
| ---- | ------- |
| `keys/expected-fingerprints.txt` | `gpg <primary fingerprint>` then `minisign <sha256 of the key blob>` |
| `keys/expected-fingerprints.ndjson` | decernor's fingerprint records for the same two keys |
| `docs/security/release-signing-keys.asc` | Public PGP key: one primary and exactly one signing subkey |
| `docs/security/goneat-minisign.pub` | Public minisign key |
| `config/release/tagger-identity.txt` | The exact tagger identity for release tags |

Every release step reads these files as git objects from `refs/tags/<tag>`,
never from the working tree or the environment. decernor, bound through
`GONEAT_DECERNOR_BIN`, checks the exports against both anchors; goneat carries
no second parser for these fingerprints. The PGP key is then imported into a
temporary keyring, and verification requires:

- each PGP signature is made by the pinned signing subkey of the pinned primary
- each minisign signature verifies with the pinned minisign key
- the public keys bundled with the release are the pins, byte for byte

The release tag must exist locally (`git fetch --tags`). Bundled release keys
are compared with the pin and never trusted on their own. There is no
environment override of the pin. `make release-verify-pin` checks the pins
committed at `HEAD` (or `GONEAT_ANCHOR_COMMIT`) against their anchors; CI runs
it on every change, and the tag workflow verifies every tag against the pin in
its own commit.

**Releases before this layout.** Tags `v0.6.4` through `v0.6.9` commit
`docs/security/release-minisign.pub` and the text anchors only. The same targets
verify them: decernor derives the records from the committed exports and checks
the committed text anchors. Only those six tags may use that layout; any other
tag on a commit that carries it is refused. Tags up to and including `v0.6.3`
carry no pin, and
verification refuses them rather than falling back. They were signed with the
same keys as `v0.6.4`; verify one manually with the files committed at `v0.6.4`
and the commands under [Verification Steps](#verification-steps-recommended).

**Anchors.** `make release-insert-anchors` derives both anchor files with
decernor from the exports committed at `HEAD` (or `GONEAT_ANCHOR_COMMIT`), twice,
compares the results, and refuses to overwrite existing anchors.

**Rotation.** Change the pins in one reviewed PR, before the first release
signed with the new keys: remove the anchors, commit the new exports, run
`make release-insert-anchors`, and commit the result. Releases tagged before
the change keep verifying against the pin committed at their own tag.

**✅ If Steps 1-4 succeed**: Request the next separately authorized release action
**❌ If any step fails**: Fix issues before pushing

#### Post-Tag Release (After git push and git tag)

**Step 5: Upload to GitHub Release**

⏳ **IMPORTANT**: Wait for GitHub Actions to create the release first! After pushing the tag, GitHub Actions will build and create the release. Monitor the Actions tab until the release workflow completes.

**Using the verified supplement uploader**

```bash
GONEAT_RELEASE_TAG=vX.Y.Z make release-notes
GONEAT_RELEASE_TAG=vX.Y.Z make release-upload
```

The uploader verifies both manifests, all four signatures, matching public keys,
and actual remote bytes before any write. It uploads only missing signatures,
public keys and versioned notes, then sets the release body from matching notes.
Existing byte-identical supplements are no-ops; different existing supplements
halt. The original five archives and two manifest assets are never uploaded
again. Their asset IDs and bytes are checked before and after supplementation.
The uploader does not change Homebrew or Scoop working trees; those updates
require separate review and authorization. Do not bypass this verification with
wildcard uploads or `--clobber`.

**Step 6: Document in Release Notes**

Add to release notes:

```markdown
## Signature Verification

Goneat releases publish signed checksum manifests. Verify the checksum manifest signature first, then verify archive checksums.

\`\`\`bash
RELEASE_VERSION=vX.Y.Z

BASE="https://github.com/fulmenhq/goneat/releases/download/${RELEASE_VERSION}"
KEYS="https://raw.githubusercontent.com/fulmenhq/goneat/${RELEASE_VERSION}/docs/security"

# Minisign verification (required) with the key committed at the tag

curl -LO "${BASE}/SHA256SUMS"
curl -LO "${BASE}/SHA256SUMS.minisig"
curl -L -o goneat-minisign.pub "${KEYS}/goneat-minisign.pub"
minisign -Vm SHA256SUMS -p goneat-minisign.pub

# PGP verification: good, unexpired signature by the pinned signing subkey

REPO="https://raw.githubusercontent.com/fulmenhq/goneat/${RELEASE_VERSION}"
curl -LO "${BASE}/SHA256SUMS.asc"
curl -L -o release-signing-keys.asc "${KEYS}/release-signing-keys.asc"
curl -L -o expected-fingerprints.txt "${REPO}/keys/expected-fingerprints.txt"
curl -L -o verify-release-pgp.sh "${REPO}/scripts/verify-release-pgp.sh"
bash verify-release-pgp.sh expected-fingerprints.txt release-signing-keys.asc SHA256SUMS.asc SHA256SUMS

# Download the archive for your platform, then check it

ARCHIVE="goneat_${RELEASE_VERSION}_<os>_<arch>.tar.gz"   # .zip on Windows
curl -LO "${BASE}/${ARCHIVE}"
grep " ${ARCHIVE}\$" SHA256SUMS | shasum -a 256 --check -
\`\`\`

Key fingerprint: see the Trust Model section at the top of this document.
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

> **Historical.** The key IDs below (`448A539320A397AF`, `45342521E9536A31`)
> describe the original v0.3.x key layout and are not the current release
> keys. The current keys are listed in [Trust Model](#trust-model) and
> committed under `keys/` and `docs/security/`. Signing with any other key
> fails at `make release-sign`.

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
